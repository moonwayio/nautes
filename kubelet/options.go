package kubelet

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

type options struct {
	config   *rest.Config
	priority []corev1.NodeAddressType
	scheme   string
	insecure bool
}

// OptionFunc is a function that configures kubelet client options.
type OptionFunc func(*options)

func (o *options) setDefaults() error {
	if o.config == nil {
		return errors.New("rest config is required")
	}

	if o.priority == nil {
		o.priority = []corev1.NodeAddressType{
			corev1.NodeHostName,
			corev1.NodeInternalDNS,
			corev1.NodeInternalIP,
			corev1.NodeExternalDNS,
			corev1.NodeExternalIP,
		}
	}

	if o.scheme == "" {
		o.scheme = "https"
	}

	return nil
}

// WithRestConfig sets the REST configuration for the kubelet client.
func WithRestConfig(config *rest.Config) OptionFunc {
	return func(o *options) {
		o.config = config
	}
}

// WithPriority sets the priority order for node address types.
func WithPriority(priority []corev1.NodeAddressType) OptionFunc {
	return func(o *options) {
		o.priority = priority
	}
}

// WithInsecure sets whether to skip TLS verification.
func WithInsecure(insecure bool) OptionFunc {
	return func(o *options) {
		o.insecure = insecure
	}
}

// WithScheme sets the URL scheme for kubelet communication.
func WithScheme(scheme string) OptionFunc {
	return func(o *options) {
		o.scheme = scheme
	}
}
