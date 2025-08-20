package controller

import (
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

// OptionFunc is a function that configures controller options.
//
// OptionFunc is a functional option pattern that allows for flexible configuration
// of controller instances. Each option function modifies the internal options
// struct to customize the controller's behavior.
type OptionFunc func(*options)

// options contains all configurable parameters for a controller instance.
//
// The options struct holds the configuration values that determine how the
// controller behaves, including client configuration, concurrency settings,
// and resource watching parameters.
type options struct {
	// name is the controller identifier used for logging and metrics
	//
	// The name should be descriptive and unique within the application
	// to help identify the controller in logs and monitoring systems.
	name string

	// resyncInterval determines how often to perform full resource resyncs
	//
	// A value of 0 disables periodic resyncing. Resyncing helps ensure
	// the controller's cache stays up-to-date and handles missed events.
	resyncInterval time.Duration

	// concurrency sets the number of concurrent reconciliation workers
	//
	// Higher values increase throughput but also increase resource usage.
	// The default value is 1 if not specified.
	concurrency int

	// client is the Kubernetes client for API operations
	//
	// The client is used for all API operations including listing,
	// watching, and updating resources. It must be properly configured
	// with authentication and authorization.
	client kubernetes.Interface

	// scheme is the runtime scheme for object serialization
	//
	// The scheme defines how Kubernetes objects are converted between
	// different formats. If not specified, the default Kubernetes
	// scheme is used.
	scheme *runtime.Scheme
}

// setDefaults validates and sets default values for controller options.
//
// This method ensures that all required options are set and applies sensible
// defaults for optional parameters. It validates the configuration and returns
// an error if any required options are missing or invalid.
//
// Returns:
//   - error: Any error encountered during validation or default setting
func (o *options) setDefaults() error {
	if o.name == "" {
		return errors.New("name is required")
	}

	if o.resyncInterval == 0 {
		o.resyncInterval = 0
	}

	if o.concurrency == 0 {
		o.concurrency = 1
	}

	if o.scheme == nil {
		o.scheme = scheme.Scheme
	}

	return nil
}

// WithName sets the controller name.
//
// The controller name is used for logging, metrics, and debugging purposes.
// It should be descriptive and unique within the application to help identify
// the controller in logs and monitoring systems.
//
// Parameters:
//   - name: The name to assign to the controller
//
// Returns:
//   - OptionFunc: A function that sets the controller name
func WithName(name string) OptionFunc {
	return func(o *options) {
		o.name = name
	}
}

// WithResyncInterval sets the resync interval for the controller.
//
// The resync interval determines how often the controller will perform a full
// resync of all watched resources. A value of 0 disables periodic resyncing.
// Resyncing is useful for ensuring the controller's cache is up-to-date and
// for handling missed events.
//
// Parameters:
//   - resyncInterval: The interval between resync operations
//
// Returns:
//   - OptionFunc: A function that sets the resync interval
func WithResyncInterval(resyncInterval time.Duration) OptionFunc {
	return func(o *options) {
		o.resyncInterval = resyncInterval
	}
}

// WithConcurrency sets the number of concurrent workers for the controller.
//
// The concurrency setting determines how many reconciliation operations can
// run simultaneously. Higher values increase throughput but also increase
// resource usage. The default value is 1 if not specified.
//
// Parameters:
//   - concurrency: The number of concurrent reconciliation workers
//
// Returns:
//   - OptionFunc: A function that sets the concurrency level
func WithConcurrency(concurrency int) OptionFunc {
	return func(o *options) {
		o.concurrency = concurrency
	}
}

// WithClient sets the Kubernetes client for the controller.
//
// The Kubernetes client is used for all API operations performed by the
// controller, including listing, watching, and updating resources. The client
// should be properly configured with authentication and authorization.
//
// Parameters:
//   - client: The Kubernetes client interface to use
//
// Returns:
//   - OptionFunc: A function that sets the Kubernetes client
func WithClient(client kubernetes.Interface) OptionFunc {
	return func(o *options) {
		o.client = client
	}
}

// WithScheme sets the runtime scheme for the controller.
//
// The runtime scheme is used for object serialization and deserialization.
// It defines how Kubernetes objects are converted between different formats.
// If not specified, the default Kubernetes scheme is used.
//
// Parameters:
//   - scheme: The runtime scheme to use for object serialization
//
// Returns:
//   - OptionFunc: A function that sets the runtime scheme
func WithScheme(scheme *runtime.Scheme) OptionFunc {
	return func(o *options) {
		o.scheme = scheme
	}
}
