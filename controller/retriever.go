package controller

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

// Retriever is how a controller will retrieve the events on the resources from
// the APÎ server.
//
// A Retriever is bound to a single type.
type Retriever interface {
	List(ctx context.Context, options metav1.ListOptions) (runtime.Object, error)
	Watch(ctx context.Context, options metav1.ListOptions) (watch.Interface, error)
}

// ListerWatcher is a struct that implements the Retriever interface.
// It is used to retrieve the events on the resources from the APÎ server.
//
// A ListerWatcher is bound to a single type.
type ListerWatcher struct {
	ListFunc  func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error)
	WatchFunc func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error)
}

// List is a function that retrieves the events on the resources from the APÎ server.
func (l *ListerWatcher) List(
	ctx context.Context,
	options metav1.ListOptions,
) (runtime.Object, error) {
	return l.ListFunc(ctx, options)
}

// Watch is a function that watches the events on the resources from the APÎ server.
func (l *ListerWatcher) Watch(
	ctx context.Context,
	options metav1.ListOptions,
) (watch.Interface, error) {
	return l.WatchFunc(ctx, options)
}
