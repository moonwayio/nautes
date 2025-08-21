package kubelet

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

// options holds the configuration parameters for the kubelet client.
//
// The options struct contains all configurable settings that determine how
// the kubelet client behaves, including connection configuration, address
// priority, and security settings. It uses the functional options pattern
// to allow flexible configuration while maintaining backward compatibility.
type options struct {
	// config is the REST configuration for Kubernetes API communication
	//
	// This configuration is used to establish connections to the Kubernetes
	// API server and authenticate requests to node kubelets.
	config *rest.Config

	// priority defines the order of preference for node address types
	//
	// When multiple address types are available for a node, the client
	// will use the first available address in this priority list.
	priority []corev1.NodeAddressType

	// scheme specifies the URL scheme for kubelet communication
	//
	// This determines whether to use HTTP or HTTPS when connecting to
	// node kubelets. Defaults to "https" for security.
	scheme string

	// insecure controls whether to skip TLS verification
	//
	// When true, the client will skip TLS certificate verification.
	// This should only be used in development or testing environments.
	insecure bool
}

// OptionFunc is a function that configures kubelet client options.
//
// OptionFunc is a functional option pattern that allows for flexible configuration
// of kubelet client instances. Each option function modifies the internal options
// struct to customize the client's behavior.
type OptionFunc func(*options)

// setDefaults validates and sets default values for kubelet client options.
//
// This method ensures that all required options are set and applies sensible
// defaults for optional parameters. It validates the configuration and returns
// an error if any required options are missing or invalid.
//
// Returns:
//   - error: Any error encountered during validation or default setting
func (o *options) setDefaults() error {
	// Validate that REST config is provided
	if o.config == nil {
		return errors.New("rest config is required")
	}

	// Set default address priority if not specified
	if o.priority == nil {
		o.priority = []corev1.NodeAddressType{
			corev1.NodeHostName,
			corev1.NodeInternalDNS,
			corev1.NodeInternalIP,
			corev1.NodeExternalDNS,
			corev1.NodeExternalIP,
		}
	}

	// Set default scheme if not specified
	if o.scheme == "" {
		o.scheme = "https"
	}

	return nil
}

// WithRestConfig sets the REST configuration for the kubelet client.
//
// The REST configuration is used to establish connections to the Kubernetes
// API server and authenticate requests to node kubelets. It must be properly
// configured with authentication and authorization settings.
//
// Parameters:
//   - config: The REST configuration for Kubernetes API communication
//
// Returns:
//   - OptionFunc: A function that sets the REST configuration
func WithRestConfig(config *rest.Config) OptionFunc {
	return func(o *options) {
		o.config = config
	}
}

// WithPriority sets the priority order for node address types.
//
// When multiple address types are available for a node, the client will
// use the first available address in this priority list. The default
// priority order prioritizes internal addresses over external ones.
//
// Parameters:
//   - priority: The ordered list of preferred address types
//
// Returns:
//   - OptionFunc: A function that sets the address priority
func WithPriority(priority []corev1.NodeAddressType) OptionFunc {
	return func(o *options) {
		o.priority = priority
	}
}

// WithInsecure sets whether to skip TLS verification.
//
// When enabled, the client will skip TLS certificate verification when
// connecting to node kubelets. This should only be used in development
// or testing environments where proper certificates are not available.
//
// Parameters:
//   - insecure: Whether to skip TLS verification
//
// Returns:
//   - OptionFunc: A function that sets the TLS verification mode
func WithInsecure(insecure bool) OptionFunc {
	return func(o *options) {
		o.insecure = insecure
	}
}

// WithScheme sets the URL scheme for kubelet communication.
//
// The scheme determines whether to use HTTP or HTTPS when connecting to
// node kubelets. HTTPS is recommended for production environments.
//
// Parameters:
//   - scheme: The URL scheme to use (e.g., "http" or "https")
//
// Returns:
//   - OptionFunc: A function that sets the URL scheme
func WithScheme(scheme string) OptionFunc {
	return func(o *options) {
		o.scheme = scheme
	}
}
