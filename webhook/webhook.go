// Package webhook provides Kubernetes admission webhook functionality.
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/moonwayio/nautes/manager"
	"github.com/moonwayio/nautes/metrics"
)

var (
	webhookRequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_request_total",
			Help: "Total number of webhook requests",
		},
		[]string{"name"},
	)
	webhookRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "webhook_request_duration_seconds",
			Help: "Duration of webhook requests",
		},
		[]string{"name"},
	)
	webhookRequestErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_request_errors_total",
			Help: "Total number of webhook request errors",
		},
		[]string{"name"},
	)
	webhookRequestSuccesses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_request_successes_total",
			Help: "Total number of webhook request successes",
		},
		[]string{"name"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		webhookRequestCount,
		webhookRequestDuration,
		webhookRequestErrors,
		webhookRequestSuccesses,
	)
}

// Server provides mutating and validating admission webhooks.
type Server interface {
	manager.Component

	Register(path string, handler HandlerFunc) error
}

// webhookServer implements the Webhook interface.
type webhookServer struct {
	opts     options
	addr     string
	handlers map[string]HandlerFunc
	logger   klog.Logger
	server   *http.Server
	mu       sync.RWMutex
}

// NewWebhookServer creates a new webhook server.
func NewWebhookServer(port int, opts ...OptionFunc) (Server, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if err := o.setDefaults(); err != nil {
		return nil, err
	}

	return &webhookServer{
		opts:     o,
		addr:     fmt.Sprintf("0.0.0.0:%d", port),
		handlers: make(map[string]HandlerFunc),
		logger:   klog.Background().WithValues("component", "webhook"),
	}, nil
}

// Register registers a handler for a specific path.
func (w *webhookServer) Register(path string, handler HandlerFunc) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.handlers[path] = handler
	return nil
}

// Start starts the webhook server.
func (w *webhookServer) Start() error {
	w.logger.Info("starting webhook server")

	mux := http.NewServeMux()
	mux.HandleFunc("/", w.handleWebhook)

	w.server = &http.Server{
		Addr:              w.addr,
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}

	// Listen synchronously to detect binding failures immediately
	listener, err := net.Listen("tcp", w.addr)
	if err != nil {
		return fmt.Errorf("failed to start webhook server: %w", err)
	}

	if *w.opts.tls {
		// Check if the cert and key files exist
		if _, err := os.Stat(w.opts.certFile); os.IsNotExist(err) {
			return fmt.Errorf("cert file does not exist: %w", err)
		}

		if _, err := os.Stat(w.opts.keyFile); os.IsNotExist(err) {
			return fmt.Errorf("key file does not exist: %w", err)
		}

		// ServeTLS in a goroutine
		go func() {
			if err := w.server.ServeTLS(listener, w.opts.certFile, w.opts.keyFile); err != nil &&
				err != http.ErrServerClosed {
				w.logger.Error(err, "webhook server error")
			}
		}()
	} else {
		// Serve in a goroutine
		go func() {
			if err := w.server.Serve(listener); err != nil && err != http.ErrServerClosed {
				w.logger.Error(err, "webhook server error")
			}
		}()
	}

	w.logger.Info("webhook server started successfully", "addr", w.addr)
	return nil
}

// Stop stops the webhook server.
func (w *webhookServer) Stop() error {
	w.logger.Info("stopping webhook server")
	if w.server != nil {
		if err := w.server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("failed to shutdown webhook server: %w", err)
		}
	}
	w.logger.Info("webhook server stopped successfully")
	return nil
}

// GetName returns the name of the webhook server.
func (w *webhookServer) GetName() string {
	return "webhook"
}

// handleWebhook handles incoming webhook requests.
func (w *webhookServer) handleWebhook(wr http.ResponseWriter, r *http.Request) {
	w.mu.RLock()
	handler, exists := w.handlers[r.URL.Path]
	w.mu.RUnlock()

	if !exists {
		http.Error(wr, "handler not found", http.StatusNotFound)
		return
	}

	// Parse the admission request
	var req AdmissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(wr, "invalid request body", http.StatusBadRequest)
		return
	}

	// increment the webhook request count
	webhookRequestCount.WithLabelValues(r.URL.Path).Inc()

	start := time.Now()
	// Call the handler
	resp := handler(r.Context(), req)
	webhookRequestDuration.WithLabelValues(r.URL.Path).Observe(time.Since(start).Seconds())

	if resp.Allowed {
		webhookRequestSuccesses.WithLabelValues(r.URL.Path).Inc()
	} else {
		webhookRequestErrors.WithLabelValues(r.URL.Path).Inc()
	}

	// Return the response
	wr.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(wr).Encode(resp); err != nil {
		w.logger.Error(err, "failed to encode webhook response")
	}
}
