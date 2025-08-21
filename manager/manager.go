// Package manager provides the main orchestration functionality for the nautes framework.
//
// The manager package implements a component lifecycle manager that coordinates the startup,
// shutdown, and lifecycle management of various framework components such as controllers,
// schedulers, webhooks, and other services. It provides a centralized way to manage
// multiple components with proper error handling and graceful shutdown capabilities.
package manager

import (
	"errors"
	"fmt"
	"sync"

	"k8s.io/klog/v2"
)

// Manager is the entry point and lifecycle orchestrator for all framework components.
//
// The Manager interface provides methods to register, start, and stop components
// in a coordinated manner. It ensures that all registered components are properly
// initialized and can be gracefully shut down when needed.
//
// Implementations of this interface must be concurrent safe.
type Manager interface {
	// Register adds a component to the manager's lifecycle management.
	//
	// The component will be started when Start() is called and stopped when Stop() is called.
	//
	// Returns an error if the component cannot be registered.
	Register(component Component) error

	// Start initializes and starts all registered components in the order they were registered.
	//
	// If any component fails to start, the method returns an error and stops the startup
	// process. Components that were already started will be stopped during error handling.
	//
	// This method is idempotent - calling it multiple times has no effect after the first
	// successful call.
	Start() error

	// Stop gracefully shuts down all registered components in reverse order of registration.
	//
	// The method attempts to stop all components even if some fail. Any errors encountered
	// during shutdown are logged but do not prevent other components from being stopped.
	//
	// This method is idempotent - calling it multiple times has no effect.
	Stop() error
}

// Component represents a framework component that can be registered with the Manager.
//
// Components are the building blocks of the nautes framework and typically represent
// services like controllers, schedulers, webhooks, or other long-running processes.
// Each component must implement the lifecycle methods defined by this interface.
type Component interface {
	// Start initializes and starts the component.
	//
	// This method should perform any necessary initialization, start background
	// goroutines, and return once the component is ready to serve requests.
	// If the component cannot be started, it should return an error.
	//
	// The method should be idempotent - calling it multiple times should have
	// no effect after the first successful call.
	Start() error

	// Stop gracefully shuts down the component.
	//
	// This method should stop all background goroutines, close connections,
	// and perform any necessary cleanup. The method should return once the
	// component has been fully shut down.
	//
	// The method should be idempotent - calling it multiple times should have
	// no effect.
	Stop() error

	// GetName returns a unique identifier for the component.
	//
	// The name is used for logging, metrics, and debugging purposes.
	// It should be descriptive and unique within the application.
	GetName() string
}

// manager implements the Manager interface.
//
// The manager maintains a list of registered components and coordinates their
// lifecycle operations. It uses read-write mutexes to ensure concurrent safe
// during component registration and lifecycle management.
type manager struct {
	opts       options
	components []Component
	logger     klog.Logger

	mu sync.RWMutex
}

// NewManager creates a new Manager instance with the provided options.
//
// The manager is initialized with default settings and can be customized using
// option functions. The returned manager is ready to accept component registrations.
//
// Parameters:
//   - opts: Optional configuration functions to customize the manager behavior
//
// Returns:
//   - Manager: A new manager instance ready for use
//   - error: Any error encountered during initialization
func NewManager(opts ...OptionFunc) (Manager, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if err := o.setDefaults(); err != nil {
		return nil, err
	}

	logger := klog.Background().WithValues("component", "manager")
	logger.Info("initializing manager")

	logger.Info("manager initialized successfully")

	return &manager{
		opts:   o,
		logger: logger,
	}, nil
}

// Register adds a component to the manager's lifecycle management.
//
// The component is added to an internal list and will be started/stopped
// when the manager's Start()/Stop() methods are called. Registration is
// concurrent safe.
//
// Parameters:
//   - component: The component to register. Must not be nil.
//
// Returns:
//   - error: Returns an error if the component is nil or cannot be registered
func (m *manager) Register(component Component) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.components = append(m.components, component)
	return nil
}

// Start initializes and starts all registered components in registration order.
//
// The method iterates through all registered components and calls their Start()
// method. If any component fails to start, the method returns an error and
// attempts to stop any components that were already started.
//
// Returns:
//   - error: Returns an error if any component fails to start, with details
//     about which component failed
func (m *manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("starting manager")

	// Start all components
	for _, component := range m.components {
		if err := component.Start(); err != nil {
			m.logger.Error(err, "failed to start component", "component", component.GetName())
			return fmt.Errorf("failed to start component: %w", err)
		}
	}

	m.logger.Info("manager started successfully")
	return nil
}

// Stop gracefully shuts down all registered components in reverse registration order.
//
// The method iterates through all registered components in reverse order and
// calls their Stop() method. Errors from individual component shutdowns are
// logged but do not prevent other components from being stopped.
//
// Returns:
//   - error: Returns an error if the shutdown process encounters critical issues
func (m *manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("stopping manager")

	errs := []error{}
	// Stop all components in reverse registration order
	for i := len(m.components) - 1; i >= 0; i-- {
		component := m.components[i]
		if err := component.Stop(); err != nil {
			m.logger.Error(
				err,
				"failed to gracefully stop component",
				"component",
				component.GetName(),
			)
			errs = append(errs, err)
		}
	}

	m.logger.Info("manager stopped successfully")
	return errors.Join(errs...)
}
