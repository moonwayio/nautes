package controller

import (
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

// OptionFunc is a function that configures controller options.
type OptionFunc func(*options)

type options struct {
	name           string
	resyncInterval time.Duration
	concurrency    int
	client         kubernetes.Interface
	scheme         *runtime.Scheme
}

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
func WithName(name string) OptionFunc {
	return func(o *options) {
		o.name = name
	}
}

// WithResyncInterval sets the resync interval for the controller.
func WithResyncInterval(resyncInterval time.Duration) OptionFunc {
	return func(o *options) {
		o.resyncInterval = resyncInterval
	}
}

// WithConcurrency sets the number of concurrent workers for the controller.
func WithConcurrency(concurrency int) OptionFunc {
	return func(o *options) {
		o.concurrency = concurrency
	}
}

// WithClient sets the Kubernetes client for the controller.
func WithClient(client kubernetes.Interface) OptionFunc {
	return func(o *options) {
		o.client = client
	}
}

// WithScheme sets the runtime scheme for the controller.
func WithScheme(scheme *runtime.Scheme) OptionFunc {
	return func(o *options) {
		o.scheme = scheme
	}
}
