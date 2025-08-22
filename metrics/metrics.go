// Copyright 2025 The Moonway.io Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// Package metrics provides Prometheus metrics collection and exposition functionality.
//
// The metrics package implements HTTP endpoints for exposing Prometheus metrics.
// It provides a framework for building applications that can collect and expose
// metrics for monitoring and alerting purposes.
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

	"github.com/moonwayio/nautes/component"
)

// Registry is the global metrics registry.
//
// The Registry is a global Prometheus registry that can be used to register
// and collect metrics from across the application. It provides a centralized
// location for all metrics collection.
var Registry = prometheus.NewRegistry()

// Server provides metrics collection and exposition functionality.
//
// The Server interface provides methods to manage the lifecycle of a metrics
// server. It handles HTTP request processing, metric registration, and graceful
// shutdown.
//
// Implementations of this interface are concurrency safe and can be used concurrently
// from multiple goroutines.
type Server interface {
	component.Component

	// Register registers one or more Prometheus collectors.
	//
	// The collectors are added to the server's metrics registry and will be
	// exposed via the /metrics endpoint. Multiple collectors can be registered
	// and all will be included in the metrics response.
	//
	// Parameters:
	//   - collector: One or more Prometheus collectors to register
	//
	// Returns:
	//   - error: Any error encountered while registering the collectors
	Register(...prometheus.Collector) error

	// Unregister removes one or more Prometheus collectors.
	//
	// The collectors are removed from the server's metrics registry and will
	// no longer be exposed via the /metrics endpoint. This is useful for
	// cleaning up metrics when components are shut down.
	//
	// Parameters:
	//   - collector: One or more Prometheus collectors to unregister
	//
	// Returns:
	//   - error: Any error encountered while unregistering the collectors
	Unregister(...prometheus.Collector) error
}

// metricsServer implements metrics collection and exposition.
//
// The metrics server maintains a Prometheus registry and serves HTTP requests
// for metrics exposition. It provides the /metrics endpoint compatible with
// Prometheus scraping.
type metricsServer struct {
	addr     string
	registry *prometheus.Registry
	server   *http.Server
	started  bool

	mu     sync.Mutex
	logger klog.Logger
}

// NewMetricsServer creates a new metrics server with the specified port.
//
// The metrics server is initialized with the specified port and uses the global
// Registry for metrics collection. The returned server is ready to accept metric
// registrations and start serving HTTP requests.
//
// Parameters:
//   - port: The TCP port to listen on for metrics requests
//
// Returns:
//   - Server: A new metrics server instance ready for use
func NewMetricsServer(port int) Server {
	return &metricsServer{
		addr:     fmt.Sprintf("0.0.0.0:%d", port),
		registry: Registry,
		logger:   klog.Background().WithValues("component", "metrics"),
		started:  false,
	}
}

// Register registers one or more Prometheus collectors.
//
// The collectors are added to the server's metrics registry and will be exposed
// via the /metrics endpoint. Registration is concurrency safe and can be called
// concurrently from multiple goroutines.
//
// Parameters:
//   - collector: One or more Prometheus collectors to register. Must not be nil.
//
// Returns:
//   - error: Returns an error if any collector is nil or cannot be registered
func (m *metricsServer) Register(collector ...prometheus.Collector) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range collector {
		if err := m.registry.Register(c); err != nil {
			return fmt.Errorf("failed to register collector: %w", err)
		}
	}

	return nil
}

// Unregister removes one or more Prometheus collectors.
//
// The collectors are removed from the server's metrics registry and will no
// longer be exposed via the /metrics endpoint. Unregistration is concurrency safe
// and can be called concurrently from multiple goroutines.
//
// Parameters:
//   - collector: One or more Prometheus collectors to unregister. Must not be nil.
//
// Returns:
//   - error: Returns an error if any collector is nil or cannot be unregistered
func (m *metricsServer) Unregister(collector ...prometheus.Collector) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range collector {
		m.registry.Unregister(c)
	}

	return nil
}

// Start starts the metrics server.
//
// The method initializes the HTTP server and starts listening for metrics
// requests. It sets up the /metrics endpoint and begins serving HTTP requests
// in a background goroutine.
//
// This method is concurrency safe and can be called concurrently with metric registration
// methods, but should not be called concurrently with Stop().
//
// Returns:
//   - error: Returns an error if the server cannot be started (e.g., port already in use)
func (m *metricsServer) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// if the metrics server is already started, return nil to ensure idempotency
	if m.started {
		return nil
	}

	m.started = true
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

// Stop gracefully shuts down the metrics server.
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
func (m *metricsServer) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// if the metrics server is not started, return nil to ensure idempotency
	if !m.started {
		return nil
	}

	m.started = false
	m.logger.Info("stopping metrics server")
	if m.server != nil {
		if err := m.server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("failed to shutdown metrics server: %w", err)
		}
	}

	m.logger.Info("metrics server stopped successfully")
	return nil
}

// GetName returns the name of the metrics server.
//
// The name is used for logging, metrics, and debugging purposes.
// This method provides a consistent identifier for the metrics component.
//
// Returns:
//   - string: The metrics server name
func (m *metricsServer) GetName() string {
	return "metrics"
}
