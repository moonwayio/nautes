// Package health provides health check functionality for Kubernetes applications.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/moonwayio/nautes/manager"
)

// Check provides health check functionality.
type Check interface {
	manager.Component

	RegisterReadiness(name string, check CheckFunc) error
	RegisterLiveness(name string, check CheckFunc) error
}

// Response represents the response structure for health check endpoints.
type Response struct {
	Status  map[string]string `json:"status"`
	Healthy bool              `json:"healthy"`
}

// healthCheck implements health check functionality.
type healthCheck struct {
	addr      string
	readiness map[string]CheckFunc
	liveness  map[string]CheckFunc
	server    *http.Server
	logger    klog.Logger

	mu sync.RWMutex
}

// CheckFunc is a function that performs a health check.
type CheckFunc func(ctx context.Context) error

// NewHealthCheck creates a new health check server.
func NewHealthCheck(port int) Check {
	return &healthCheck{
		addr:      fmt.Sprintf("0.0.0.0:%d", port),
		readiness: make(map[string]CheckFunc),
		liveness:  make(map[string]CheckFunc),
		logger:    klog.Background().WithValues("component", "health"),
	}
}

// RegisterReadiness registers a readiness check.
func (h *healthCheck) RegisterReadiness(name string, check CheckFunc) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.readiness[name] = check
	return nil
}

// RegisterLiveness registers a liveness check.
func (h *healthCheck) RegisterLiveness(name string, check CheckFunc) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.liveness[name] = check
	return nil
}

// Start starts the health check server.
func (h *healthCheck) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logger.Info("starting health check server")

	mux := http.NewServeMux()
	mux.HandleFunc("/ready", h.handleReadiness)
	mux.HandleFunc("/health", h.handleLiveness)

	h.server = &http.Server{
		Addr:              h.addr,
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}

	// Listen synchronously to detect binding failures immediately
	listener, err := net.Listen("tcp", h.addr)
	if err != nil {
		return fmt.Errorf("failed to start health check server: %w", err)
	}

	// Serve in a goroutine
	go func() {
		if err := h.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			h.logger.Error(err, "health check server error")
		}
	}()

	h.logger.Info("health check server started successfully", "addr", h.addr)
	return nil
}

// Stop stops the health check server.
func (h *healthCheck) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logger.Info("stopping health check server")
	if h.server != nil {
		if err := h.server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("failed to shutdown health check server: %w", err)
		}
		return nil
	}
	return nil
}

// GetName returns the name of the health check server.
func (h *healthCheck) GetName() string {
	return "health-check"
}

// handleReadiness handles readiness probe requests.
func (h *healthCheck) handleReadiness(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	status := make(map[string]string)
	healthy := true

	for name, check := range h.readiness {
		if err := check(r.Context()); err != nil {
			status[name] = "unhealthy: " + err.Error()
			healthy = false
		} else {
			status[name] = "healthy"
		}
	}

	if healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	if err := json.NewEncoder(w).Encode(Response{
		Status:  status,
		Healthy: healthy,
	}); err != nil {
		h.logger.Error(err, "failed to encode readiness response")
	}
}

// handleLiveness handles liveness probe requests.
func (h *healthCheck) handleLiveness(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	status := make(map[string]string)
	healthy := true

	for name, check := range h.liveness {
		if err := check(r.Context()); err != nil {
			status[name] = "unhealthy: " + err.Error()
			healthy = false
		} else {
			status[name] = "healthy"
		}
	}

	if healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	if err := json.NewEncoder(w).Encode(Response{
		Status:  status,
		Healthy: healthy,
	}); err != nil {
		h.logger.Error(err, "failed to encode liveness response")
	}
}
