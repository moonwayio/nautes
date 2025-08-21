// Package manager provides the main orchestration functionality for the nautes framework.
//
// The manager package implements a component lifecycle manager that coordinates the startup,
// shutdown, and lifecycle management of various framework components such as controllers,
// schedulers, webhooks, and other services. It provides a centralized way to manage
// multiple components with proper error handling and graceful shutdown capabilities.
//
// The manager supports both regular components and leader election-aware components,
// allowing for distributed coordination in multi-instance deployments.
package manager

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"k8s.io/klog/v2"

	"github.com/moonwayio/nautes/component"
	"github.com/moonwayio/nautes/leader"
)

// Manager is the entry point and lifecycle orchestrator for all framework components.
//
// The Manager interface provides methods to register, start, and stop components
// in a coordinated manner. It ensures that all registered components are properly
// initialized and can be gracefully shut down when needed.
//
// The manager supports two types of components:
// - Regular components: Started immediately when the manager starts
// - Leader election-aware components: Started only when this instance becomes the leader
//
// Implementations of this interface must be concurrent safe.
type Manager interface {
	// Register adds a component to the manager's lifecycle management.
	//
	// The component will be started when Start() is called and stopped when Stop() is called.
	// Leader election-aware components are automatically detected and managed accordingly.
	//
	// Returns an error if the component cannot be registered.
	Register(component component.Component) error

	// Start initializes and starts all registered components in the order they were registered.
	//
	// Regular components are started immediately, while leader election-aware components
	// are started only when this instance becomes the leader. If any component fails to start,
	// the method returns an error and stops the startup process. Components that were already
	// started will be stopped during error handling.
	//
	// This method is idempotent - calling it multiple times has no effect after the first
	// successful call.
	Start() error

	// Stop gracefully shuts down all registered components in reverse order of registration.
	//
	// The method attempts to stop all components even if some fail. Any errors encountered
	// during shutdown are logged but do not prevent other components from being stopped.
	// Leader election-aware components are stopped when leadership is lost.
	//
	// This method is idempotent - calling it multiple times has no effect.
	Stop() error
}

// manager implements the Manager interface.
//
// The manager maintains separate lists for regular components and leader election-aware
// components. It uses read-write mutexes to ensure concurrent safety during component
// registration and lifecycle management. The manager also coordinates with the leader
// election system to properly handle leadership changes.
type manager struct {
	opts             options
	components       map[string]component.Component // Regular components started immediately
	leaderComponents map[string]component.Component // Components started only when leader
	logger           klog.Logger

	mu     sync.RWMutex       // Protects component lists and lifecycle operations
	cancel context.CancelFunc // Cancels the leader election context
}

// NewManager creates a new Manager instance with the provided options.
//
// The manager is initialized with default settings and can be customized using
// option functions. If leader election is enabled, the manager will subscribe
// to leadership events and manage leader-aware components accordingly.
//
// Parameters:
//   - opts: Optional configuration functions to customize the manager behavior
//
// Returns:
//   - Manager: A new manager instance ready for use
//   - error: Any error encountered during initialization, including leader election setup failures
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

	m := &manager{
		opts:             o,
		logger:           logger,
		components:       make(map[string]component.Component),
		leaderComponents: make(map[string]component.Component),
	}

	if o.leader != nil {
		logger.Info("leader election enabled")
		if err := o.leader.Subscribe(m); err != nil {
			return nil, fmt.Errorf("failed to subscribe to leader election: %w", err)
		}
	}

	logger.Info("manager initialized successfully")

	return m, nil
}

// Register adds a component to the manager's lifecycle management.
//
// The component is automatically categorized based on its leader election awareness:
//   - If the component implements LeaderElectionAware and NeedsLeaderElection() returns true,
//     it's added to the leader components list
//   - Otherwise, it's added to the regular components list
//
// If leader election is enabled, the component is also subscribed to leadership events
// if it implements ElectionSubscriber.
//
// Parameters:
//   - component: The component to register. Must not be nil.
//
// Returns:
//   - error: Returns an error if the component is nil or cannot be registered
func (m *manager) Register(c component.Component) error {
	// Validate input
	if c == nil {
		return errors.New("cannot register nil component")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if component is already registered to prevent duplicates
	componentName := c.GetName()
	if _, ok := m.components[componentName]; ok {
		return fmt.Errorf("component with name %q is already registered", componentName)
	}
	if _, ok := m.leaderComponents[componentName]; ok {
		return fmt.Errorf("component with name %q is already registered", componentName)
	}

	if m.opts.leader != nil {
		// Subscribe component to leader election events if it supports it
		if subscriber, ok := c.(leader.ElectionSubscriber); ok {
			if err := m.opts.leader.Subscribe(subscriber); err != nil {
				return fmt.Errorf(
					"failed to subscribe component %q to leader election: %w",
					componentName,
					err,
				)
			}
		}

		// Categorize component based on leader election awareness
		if leaderAware, ok := c.(component.LeaderElectionAware); ok &&
			leaderAware.NeedsLeaderElection() {
			m.leaderComponents[componentName] = c
			m.logger.Info("registered leader election-aware component", "component", componentName)
		} else {
			m.components[componentName] = c
			m.logger.Info("registered regular component", "component", componentName)
		}
	} else {
		// No leader election, treat all components as regular
		m.components[componentName] = c
		m.logger.Info("registered component", "component", componentName)
	}
	return nil
}

// Start initializes and starts all registered components in registration order.
//
// The method starts regular components immediately and sets up leader election
// if enabled. Leader election-aware components are started only when this instance
// becomes the leader (handled by OnLeaderElectionStarted).
//
// If any component fails to start, the method returns an error.
//
// Returns:
//   - error: Returns an error if any component fails to start, with details
//     about which component failed
func (m *manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already started
	if m.cancel != nil {
		return errors.New("manager is already started")
	}

	m.logger.Info("starting manager")

	var ctx context.Context
	ctx, m.cancel = context.WithCancel(context.Background())

	// Start leader election if enabled
	if m.opts.leader != nil {
		go func() {
			if err := m.opts.leader.Run(ctx); err != nil {
				m.logger.Error(err, "failed to start leader election")
			}
		}()
	}

	// Start all regular components (non-leader election aware)
	for _, component := range m.components {
		componentName := component.GetName()
		m.logger.V(1).Info("starting component", "component", componentName)

		if err := component.Start(); err != nil {
			m.logger.Error(err, "failed to start component", "component", component.GetName())
			return fmt.Errorf("failed to start component %q: %w", componentName, err)
		}
	}

	m.logger.Info("manager started successfully")
	return nil
}

// Stop gracefully shuts down all registered components in reverse registration order.
//
// The method stops all components (both regular and leader election-aware) and
// cancels the leader election context. Errors from individual component shutdowns
// are logged but do not prevent other components from being stopped.
//
// Returns:
//   - error: Returns an error if the shutdown process encounters critical issues,
//     combining all individual shutdown errors
func (m *manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already stopped
	if m.cancel == nil {
		return errors.New("manager is not started")
	}

	m.logger.Info("stopping manager")

	// Cancel leader election context first
	m.cancel()
	m.cancel = nil // Reset to indicate stopped state

	errs := []error{}

	// Stop all leader election-aware components first
	for name, component := range m.leaderComponents {
		if err := component.Stop(); err != nil {
			m.logger.Error(err, "failed to gracefully stop leader component", "component", name)
			errs = append(errs, fmt.Errorf("failed to stop leader component %q: %w", name, err))
		} else {
			m.logger.Info("leader component stopped successfully", "component", name)
		}
	}

	// Stop all regular components
	for name, component := range m.components {
		if err := component.Stop(); err != nil {
			m.logger.Error(err, "failed to gracefully stop component", "component", name)
			errs = append(errs, fmt.Errorf("failed to stop component %q: %w", name, err))
		} else {
			m.logger.Info("component stopped successfully", "component", name)
		}
	}

	m.logger.Info("manager stopped successfully")
	return errors.Join(errs...)
}

// OnStartLeading is called when this instance becomes the leader.
//
// This method starts all leader election-aware components that were registered
// with the manager. These components are started in the order they were registered.
// If any component fails to start, the error is logged but other components
// continue to be started.
func (m *manager) OnStartLeading() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.logger.Info("starting leader election-aware components")

	for name, component := range m.leaderComponents {
		if err := component.Start(); err != nil {
			m.logger.Error(err, "failed to start leader component", "component", name)
			continue
		}
	}

	m.logger.Info("leader election started")
}

// OnStopLeading is called when this instance loses leadership.
//
// This method stops all leader election-aware components that were started
// when this instance became the leader. Components are stopped in reverse
// order of registration. If any component fails to stop, the error is logged
// but other components continue to be stopped.
func (m *manager) OnStopLeading() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.logger.Info("stopping leader election-aware components")

	for name, component := range m.leaderComponents {
		if err := component.Stop(); err != nil {
			m.logger.Error(err, "failed to stop leader component", "component", name)
			continue
		}
		m.logger.Info("leader component stopped successfully", "component", name)
	}

	m.logger.Info("leader election stopped")
}
