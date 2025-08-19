// Package metrics provides Prometheus metrics collection and exposition functionality.
package metrics

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"

	"github.com/moonwayio/nautes/manager"
)

// Server provides metrics collection and exposition functionality.
type Server interface {
	manager.Component

	Register(prometheus.Collector) error
}

// metricsServer implements metrics collection and exposition.
type metricsServer struct {
	addr     string
	registry *prometheus.Registry
	server   *http.Server

	mu     sync.RWMutex
	logger klog.Logger
}

// NewMetricsServer creates a new metrics server.
func NewMetricsServer(port int) Server {
	return &metricsServer{
		addr:     fmt.Sprintf("0.0.0.0:%d", port),
		registry: prometheus.NewRegistry(),
		logger:   klog.Background().WithValues("component", "metrics"),
	}
}

// Register registers a metric.
func (m *metricsServer) Register(collector prometheus.Collector) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.registry.Register(collector); err != nil {
		return fmt.Errorf("failed to register collector: %w", err)
	}

	return nil
}

// Start starts the metrics server.
func (m *metricsServer) Start() error {
	m.logger.Info("starting metrics server")

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))

	m.server = &http.Server{
		Addr:              m.addr,
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}

	// Listen synchronously to detect binding failures immediately
	listener, err := net.Listen("tcp", m.addr)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}

	// Serve in a goroutine
	go func() {
		if err := m.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			m.logger.Error(err, "metrics server error")
		}
	}()

	m.logger.Info("metrics server started successfully", "addr", m.addr)
	return nil
}

// Stop stops the metrics server.
func (m *metricsServer) Stop() error {
	m.logger.Info("Stopping metrics server")
	if m.server != nil {
		if err := m.server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("failed to shutdown metrics server: %w", err)
		}
		return nil
	}
	return nil
}

// GetName returns the name of the metrics server.
func (m *metricsServer) GetName() string {
	return "metrics"
}
