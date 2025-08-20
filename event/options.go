package event

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

type options struct {
	name   string
	scheme *runtime.Scheme
	client kubernetes.Interface
}

// OptionFunc is a function that configures event recorder options.
type OptionFunc func(*options)

func (o *options) setDefaults() error {
	if o.name == "" {
		return errors.New("name is required")
	}

	if o.client == nil {
		return errors.New("client is required")
	}

	if o.scheme == nil {
		o.scheme = scheme.Scheme
	}

	return nil
}

// WithName sets the event recorder name.
func WithName(name string) OptionFunc {
	return func(o *options) {
		o.name = name
	}
}

// WithClient sets the Kubernetes client for the event recorder.
func WithClient(client kubernetes.Interface) OptionFunc {
	return func(o *options) {
		o.client = client
	}
}

// WithScheme sets the runtime scheme for the event recorder.
func WithScheme(scheme *runtime.Scheme) OptionFunc {
	return func(o *options) {
		o.scheme = scheme
	}
}
