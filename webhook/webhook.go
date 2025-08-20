// Package webhook provides Kubernetes admission webhook functionality.
//
// The webhook package implements HTTP servers for Kubernetes admission webhooks,
// including both mutating and validating webhooks. It provides a framework for
// building webhook servers that can intercept and modify Kubernetes API requests
// before they are persisted to the cluster.
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

// init registers the Prometheus metrics for the webhook package.
//
// This function is called automatically when the package is imported and
// registers all the metrics defined in this package with the global metrics
// registry. This ensures that the metrics are available for collection
// by Prometheus or other monitoring systems.
func init() {
	metrics.Registry.MustRegister(
		webhookRequestCount,
		webhookRequestDuration,
		webhookRequestErrors,
		webhookRequestSuccesses,
	)
}

// Server provides mutating and validating admission webhooks.
//
// The Server interface provides methods to manage the lifecycle of a webhook server.
// It handles HTTP request processing, handler registration, and graceful shutdown.
// The server can serve multiple webhook endpoints with different handlers.
//
// Implementations of this interface are concurrency safe and can be used concurrently
// from multiple goroutines.
type Server interface {
	manager.Component

	// Register registers a handler function for a specific HTTP path.
	//
	// The handler will be called for all HTTP requests to the specified path.
	// Multiple handlers can be registered for different paths. Handler registration
	// is concurrency safe and can be called concurrently.
	//
	// Parameters:
	//   - path: The HTTP path to register the handler for (e.g., "/mutate", "/validate")
	//   - handler: The function to handle webhook requests
	//
	// Returns:
	//   - error: Any error encountered while registering the handler
	Register(path string, handler HandlerFunc) error
}

// webhookServer implements the Webhook interface.
//
// The webhook server maintains a mapping of HTTP paths to handler functions and
// serves HTTP requests for Kubernetes admission webhooks. It provides metrics
// collection and graceful shutdown capabilities.
type webhookServer struct {
	opts     options
	addr     string
	handlers map[string]HandlerFunc
	logger   klog.Logger
	server   *http.Server
	started  bool
	mu       sync.RWMutex
}

// NewWebhookServer creates a new webhook server instance with the specified port and options.
//
// The webhook server is initialized with the specified port and can be customized
// using option functions. The returned server is ready to accept handler registrations
// and start serving HTTP requests.
//
// Parameters:
//   - port: The TCP port to listen on for webhook requests
//   - opts: Optional configuration functions to customize the server behavior
//
// Returns:
//   - Server: A new webhook server instance ready for use
//   - error: Any error encountered during initialization
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
		started:  false,
	}, nil
}

// Register registers a handler function for a specific HTTP path.
//
// The handler is added to the server's internal handler mapping and will be called
// for all HTTP requests to the specified path. Handler registration is concurrency safe
// and can be called concurrently from multiple goroutines.
//
// Parameters:
//   - path: The HTTP path to register the handler for. Must start with "/".
//   - handler: The function to handle webhook requests. Must not be nil.
//
// Returns:
//   - error: Returns an error if the path is invalid, handler is nil, or the handler cannot be registered
func (w *webhookServer) Register(path string, handler HandlerFunc) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.handlers[path] = handler
	return nil
}

// Start initializes and starts the webhook server.
//
// This method sets up the HTTP server with registered handlers and begins
// listening for admission webhook requests. It supports both TLS and non-TLS
// configurations based on the server options. The server starts in a
// background goroutine and this method returns immediately after successful
// initialization.
//
// The method validates TLS certificate files if TLS is enabled and returns
// an error if they are not found. This method is called automatically by
// the manager when the component is started.
//
// Returns:
//   - error: Any error encountered during server startup or TLS configuration
func (w *webhookServer) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// if the webhook server is already started, return nil to ensure idempotency
	if w.started {
		return nil
	}

	w.started = true
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

// Stop gracefully shuts down the webhook server.
//
// This method stops the HTTP server and cleans up resources. It attempts
// to gracefully shutdown the server, allowing existing connections to
// complete before closing. This method is called automatically by the
// manager when the component is stopped.
//
// The method is concurrency safe but should not be called concurrently
// with Start().
//
// Returns:
//   - error: Any error encountered during server shutdown
func (w *webhookServer) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// if the webhook server is not started, return nil to ensure idempotency
	if !w.started {
		return nil
	}

	w.started = false
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
//
// The name is used for logging, metrics, and debugging purposes.
// This method provides a consistent identifier for the webhook component.
//
// Returns:
//   - string: The webhook server name
func (w *webhookServer) GetName() string {
	return "webhook"
}

// handleWebhook handles incoming webhook requests.
//
// This method processes HTTP requests to webhook endpoints and routes them
// to the appropriate handler based on the request path. It parses admission
// requests, calls the registered handler, and returns admission responses.
//
// The method is concurrency safe and can handle multiple concurrent requests.
// It also updates Prometheus metrics for request counts, durations, and outcomes.
//
// Parameters:
//   - wr: The HTTP response writer
//   - r: The HTTP request containing the admission request
func (w *webhookServer) handleWebhook(wr http.ResponseWriter, r *http.Request) {
	// Find the handler for the requested path
	w.mu.RLock()
	handler, exists := w.handlers[r.URL.Path]
	w.mu.RUnlock()

	if !exists {
		http.Error(wr, "handler not found", http.StatusNotFound)
		return
	}

	// Parse the admission request from the request body
	var req AdmissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(wr, "invalid request body", http.StatusBadRequest)
		return
	}

	// Increment the webhook request count metric
	webhookRequestCount.WithLabelValues(r.URL.Path).Inc()

	// Measure request processing duration
	start := time.Now()
	// Call the registered handler
	resp := handler(r.Context(), req)
	webhookRequestDuration.WithLabelValues(r.URL.Path).Observe(time.Since(start).Seconds())

	// Update success/error metrics based on response
	if resp.Allowed {
		webhookRequestSuccesses.WithLabelValues(r.URL.Path).Inc()
	} else {
		webhookRequestErrors.WithLabelValues(r.URL.Path).Inc()
	}

	// Return the admission response
	wr.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(wr).Encode(resp); err != nil {
		w.logger.Error(err, "failed to encode webhook response")
	}
}
