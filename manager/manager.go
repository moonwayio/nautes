// Package manager provides the main orchestration functionality for the nautes framework.
package manager

import (
	"errors"
	"fmt"
	"sync"

	"k8s.io/klog/v2"
)

// Manager is the entry point and lifecycle orchestrator for all framework components.
type Manager interface {
	Register(component Component) error
	Start() error
	Stop() error
}

// Component represents a framework component that can be registered with the Manager.
type Component interface {
	Start() error
	Stop() error
	GetName() string
}

// manager implements the Manager interface.
type manager struct {
	opts       options
	components []Component
	logger     klog.Logger

	mu sync.RWMutex
}

// NewManager creates a new Manager instance.
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

// Register adds a component to the manager.
func (m *manager) Register(component Component) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.components = append(m.components, component)
	return nil
}

// Start starts all registered components.
func (m *manager) Start() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

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

// Stop stops all registered components.
func (m *manager) Stop() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.logger.Info("stopping manager")

	errs := []error{}
	for _, component := range m.components {
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
