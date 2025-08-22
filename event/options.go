package event

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

// options holds the configuration parameters for the event recorder.
//
// The options struct contains all configurable settings that determine how
// the event recorder behaves, including client configuration and runtime
// scheme settings. It uses the functional options pattern to allow flexible
// configuration while maintaining backward compatibility.
type options struct {
	// name is the identifier for the event recorder instance
	//
	// The name is used to identify the source of events in the Kubernetes
	// event system. It should be descriptive and unique within the application.
	name string

	// scheme is the runtime scheme for object serialization
	//
	// The scheme is used to properly serialize and deserialize Kubernetes
	// objects when creating events. It defines how objects are converted
	// between different formats.
	scheme *runtime.Scheme

	// client is the Kubernetes client for API operations
	//
	// The client is used to create and manage events in the Kubernetes
	// API server. It must be properly configured with authentication
	// and authorization settings.
	client kubernetes.Interface

	// needsLeaderElection indicates if the event recorder needs leader election
	//
	// When set to true, the event recorder will only start when the leader election
	// is active. If set to false, the event recorder will start immediately when the
	// manager starts.
	needsLeaderElection bool
}

// OptionFunc is a function that configures event recorder options.
//
// OptionFunc is a functional option pattern that allows for flexible configuration
// of event recorder instances. Each option function modifies the internal options
// struct to customize the recorder's behavior.
type OptionFunc func(*options)

// setDefaults validates and sets default values for event recorder options.
//
// This method ensures that all required options are set and applies sensible
// defaults for optional parameters. It validates the configuration and returns
// an error if any required options are missing or invalid.
//
// Returns:
//   - error: Any error encountered during validation or default setting
func (o *options) setDefaults() error {
	// Validate that name is provided
	if o.name == "" {
		return errors.New("name is required")
	}

	// Validate that client is provided
	if o.client == nil {
		return errors.New("client is required")
	}

	// Set default scheme if not specified
	if o.scheme == nil {
		o.scheme = scheme.Scheme
	}

	return nil
}

// WithName sets the event recorder name.
//
// The event recorder name is used to identify the source of events in the
// Kubernetes event system. It should be descriptive and unique within the
// application to help with debugging and monitoring.
//
// Parameters:
//   - name: The name to assign to the event recorder
//
// Returns:
//   - OptionFunc: A function that sets the event recorder name
func WithName(name string) OptionFunc {
	return func(o *options) {
		o.name = name
	}
}

// WithClient sets the Kubernetes client for the event recorder.
//
// The Kubernetes client is used to create and manage events in the Kubernetes
// API server. It must be properly configured with authentication and
// authorization settings to allow event creation.
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

// WithScheme sets the runtime scheme for the event recorder.
//
// The runtime scheme is used for object serialization and deserialization
// when creating events. It defines how Kubernetes objects are converted
// between different formats. If not specified, the default Kubernetes
// scheme is used.
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

// WithNeedsLeaderElection sets if the event recorder needs leader election.
//
// The leader election configuration determines whether the event recorder will
// only start when the leader election is active. If set to false, the event recorder
// will start immediately when the manager starts.
//
// Parameters:
//   - needsLeaderElection: true if the event recorder needs leader election, false otherwise
//
// Returns:
//   - OptionFunc: A function that sets the leader election configuration
func WithNeedsLeaderElection(needsLeaderElection bool) OptionFunc {
	return func(o *options) {
		o.needsLeaderElection = needsLeaderElection
	}
}
