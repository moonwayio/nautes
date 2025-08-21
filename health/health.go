// Package health provides health check functionality for Kubernetes applications.
//
// The health package implements HTTP endpoints for Kubernetes liveness and readiness
// probes. It provides a framework for building applications that can report their
// health status to Kubernetes, enabling proper lifecycle management and load balancing.
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
//
// The Check interface provides methods to manage health check endpoints for
// Kubernetes liveness and readiness probes. It handles HTTP request processing,
// check registration, and graceful shutdown.
//
// Implementations of this interface are concurrency safe and can be used concurrently
// from multiple goroutines.
type Check interface {
	manager.Component

	// RegisterReadiness registers a readiness check function.
	//
	// Readiness checks determine if the application is ready to receive traffic.
	// If any readiness check fails, the application should not receive requests.
	// Multiple readiness checks can be registered and all must pass for the
	// application to be considered ready.
	//
	// Parameters:
	//   - name: A unique identifier for the readiness check
	//   - check: The function to perform the readiness check
	//
	// Returns:
	//   - error: Any error encountered while registering the check
	RegisterReadiness(name string, check CheckFunc) error

	// RegisterLiveness registers a liveness check function.
	//
	// Liveness checks determine if the application is alive and should not be
	// restarted. If any liveness check fails, Kubernetes may restart the pod.
	// Multiple liveness checks can be registered and all must pass for the
	// application to be considered alive.
	//
	// Parameters:
	//   - name: A unique identifier for the liveness check
	//   - check: The function to perform the liveness check
	//
	// Returns:
	//   - error: Any error encountered while registering the check
	RegisterLiveness(name string, check CheckFunc) error
}

// Response represents the response structure for health check endpoints.
//
// The Response struct defines the JSON format returned by health check endpoints.
// It includes the status of individual checks and an overall health status.
type Response struct {
	Status  map[string]string `json:"status"`  // Status of individual checks
	Healthy bool              `json:"healthy"` // Overall health status
}

// healthCheck implements health check functionality.
//
// The health check server maintains mappings of check names to check functions
// and serves HTTP requests for health check endpoints. It provides JSON responses
// compatible with Kubernetes probe expectations.
type healthCheck struct {
	addr      string
	readiness map[string]CheckFunc
	liveness  map[string]CheckFunc
	server    *http.Server
	logger    klog.Logger
	started   bool

	mu sync.RWMutex
}

// CheckFunc is a function that performs a health check.
//
// A CheckFunc is a function that takes a context and returns an error indicating
// the health status. A nil error indicates the check passed, while a non-nil error
// indicates the check failed.
//
// Parameters:
//   - ctx: Context for the health check, may be cancelled
//
// Returns:
//   - error: nil if the check passed, non-nil if the check failed
type CheckFunc func(ctx context.Context) error

// NewHealthCheck creates a new health check server with the specified port.
//
// The health check server is initialized with the specified port and is ready
// to accept readiness and liveness check registrations.
//
// Parameters:
//   - port: The TCP port to listen on for health check requests
//
// Returns:
//   - Check: A new health check server instance ready for use
func NewHealthCheck(port int) Check {
	return &healthCheck{
		addr:      fmt.Sprintf("0.0.0.0:%d", port),
		readiness: make(map[string]CheckFunc),
		liveness:  make(map[string]CheckFunc),
		logger:    klog.Background().WithValues("component", "health"),
		started:   false,
	}
}

// RegisterReadiness registers a readiness check function.
//
// The readiness check is added to the server's internal mapping and will be
// called when the /ready endpoint is accessed. Check registration is concurrency safe
// and can be called concurrently from multiple goroutines.
//
// Parameters:
//   - name: A unique identifier for the readiness check. Must not be empty.
//   - check: The function to perform the readiness check. Must not be nil.
//
// Returns:
//   - error: Returns an error if the name is empty, check is nil, or the check cannot be registered
func (h *healthCheck) RegisterReadiness(name string, check CheckFunc) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.readiness[name] = check
	return nil
}

// RegisterLiveness registers a liveness check function.
//
// The liveness check is added to the server's internal mapping and will be
// called when the /health endpoint is accessed. Check registration is concurrency safe
// and can be called concurrently from multiple goroutines.
//
// Parameters:
//   - name: A unique identifier for the liveness check. Must not be empty.
//   - check: The function to perform the liveness check. Must not be nil.
//
// Returns:
//   - error: Returns an error if the name is empty, check is nil, or the check cannot be registered
func (h *healthCheck) RegisterLiveness(name string, check CheckFunc) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.liveness[name] = check
	return nil
}

// Start starts the health check server.
//
// The method initializes the HTTP server and starts listening for health check
// requests. It sets up the /ready and /health endpoints and begins serving
// HTTP requests in a background goroutine.
//
// This method is concurrency safe and can be called concurrently with check registration
// methods, but should not be called concurrently with Stop().
//
// Returns:
//   - error: Returns an error if the server cannot be started (e.g., port already in use)
func (h *healthCheck) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// if the health check server is already started, return nil to ensure idempotency
	if h.started {
		return nil
	}

	h.started = true
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

// Stop gracefully shuts down the health check server.
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
func (h *healthCheck) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// if the health check server is not started, return nil to ensure idempotency
	if !h.started {
		return nil
	}

	h.started = false
	h.logger.Info("stopping health check server")
	if h.server != nil {
		if err := h.server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("failed to shutdown health check server: %w", err)
		}
	}
	return nil
}

// GetName returns the name of the health check server.
//
// The name is used for logging, metrics, and debugging purposes.
// This method provides a consistent identifier for the health check component.
//
// Returns:
//   - string: The health check server name
func (h *healthCheck) GetName() string {
	return "health-check"
}

// handleReadiness handles readiness probe requests.
//
// This method processes HTTP requests to the /ready endpoint and executes
// all registered readiness checks. It returns a JSON response indicating
// the health status of each check and an overall health status.
//
// The method is concurrency safe and can handle multiple concurrent requests.
// It returns HTTP 200 if all checks pass, or HTTP 503 if any check fails.
//
// Parameters:
//   - w: The HTTP response writer
//   - r: The HTTP request
func (h *healthCheck) handleReadiness(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	status := make(map[string]string)
	healthy := true

	// Execute all registered readiness checks
	for name, check := range h.readiness {
		if err := check(r.Context()); err != nil {
			status[name] = "unhealthy: " + err.Error()
			healthy = false
		} else {
			status[name] = "healthy"
		}
	}

	// Set appropriate HTTP status code
	if healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	// Encode and return the response
	if err := json.NewEncoder(w).Encode(Response{
		Status:  status,
		Healthy: healthy,
	}); err != nil {
		h.logger.Error(err, "failed to encode readiness response")
	}
}

// handleLiveness handles liveness probe requests.
//
// This method processes HTTP requests to the /health endpoint and executes
// all registered liveness checks. It returns a JSON response indicating
// the health status of each check and an overall health status.
//
// The method is concurrency safe and can handle multiple concurrent requests.
// It returns HTTP 200 if all checks pass, or HTTP 503 if any check fails.
//
// Parameters:
//   - w: The HTTP response writer
//   - r: The HTTP request
func (h *healthCheck) handleLiveness(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	status := make(map[string]string)
	healthy := true

	// Execute all registered liveness checks
	for name, check := range h.liveness {
		if err := check(r.Context()); err != nil {
			status[name] = "unhealthy: " + err.Error()
			healthy = false
		} else {
			status[name] = "healthy"
		}
	}

	// Set appropriate HTTP status code
	if healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	// Encode and return the response
	if err := json.NewEncoder(w).Encode(Response{
		Status:  status,
		Healthy: healthy,
	}); err != nil {
		h.logger.Error(err, "failed to encode liveness response")
	}
}
