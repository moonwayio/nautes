// Package manager provides the main orchestration functionality for the nautes framework.
package manager

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

// Manager is the entry point and lifecycle orchestrator for all framework components.
type Manager interface {
	Register(component Component) error
	Start() error
	Stop() error
	GetClient() kubernetes.Interface
	GetScheme() *runtime.Scheme
	GetLogger() klog.Logger
	GetRestConfig() *rest.Config
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
	config     *rest.Config
	client     kubernetes.Interface
	scheme     *runtime.Scheme
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

	config, err := getKubernetesConfig(o.kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	logger.Info("manager initialized successfully")

	return &manager{
		opts:   o,
		logger: logger,
		config: config,
		client: client,
		scheme: o.scheme,
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

// GetClient returns the Kubernetes client.
func (m *manager) GetClient() kubernetes.Interface {
	return m.client
}

// GetScheme returns the Kubernetes scheme.
func (m *manager) GetScheme() *runtime.Scheme {
	return m.scheme
}

// GetConfig returns the Kubernetes config.
func (m *manager) GetRestConfig() *rest.Config {
	return m.config
}

// GetLogger returns the logger.
func (m *manager) GetLogger() klog.Logger {
	return m.logger
}

// getKubernetesConfig returns the Kubernetes config
// if kubeconfigPath is provided, it will be used to build the config
// otherwise, it will try to use the in-cluster config and fallback to the kubeconfig in the home directory.
func getKubernetesConfig(kubeconfigPath string) (*rest.Config, error) {
	var config *rest.Config
	var err error
	if kubeconfigPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("error loading kubernetes configuration: %w", err)
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), ".kube", "config"))
			if err != nil {
				return nil, fmt.Errorf("error loading kubernetes configuration: %w", err)
			}
		}
	}

	return config, nil
}
