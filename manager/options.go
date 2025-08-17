package manager

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

type options struct {
	kubeconfigPath string
	scheme         *runtime.Scheme
}

// OptionFunc is a function that configures manager options.
type OptionFunc func(*options)

func (o *options) setDefaults() error {
	if o.scheme == nil {
		o.scheme = scheme.Scheme
	}

	return nil
}

// WithKubeconfigPath sets the path to the kubeconfig file.
func WithKubeconfigPath(kubeconfigPath string) OptionFunc {
	return func(o *options) {
		o.kubeconfigPath = kubeconfigPath
	}
}

// WithScheme sets the runtime scheme for the manager.
func WithScheme(scheme *runtime.Scheme) OptionFunc {
	return func(o *options) {
		o.scheme = scheme
	}
}
