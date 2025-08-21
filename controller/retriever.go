package controller

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

// Retriever defines how a controller retrieves events on resources from the API server.
//
// A Retriever is bound to a single resource type and provides methods to list and watch
// resources of that type. The interface abstracts the details of API server communication
// and allows controllers to work with different resource types uniformly.
//
// Implementations of this interface must be concurrency safe and can be used concurrently
// from multiple goroutines.
type Retriever interface {
	// List retrieves a list of resources from the API server.
	//
	// The method returns all resources of the bound type that match the provided
	// list options. The returned object is typically a list type containing
	// the requested resources.
	//
	// Parameters:
	//   - ctx: Context for the request, may be cancelled
	//   - options: List options to filter and paginate the results
	//
	// Returns:
	//   - runtime.Object: A list of resources matching the criteria
	//   - error: Any error encountered during the API request
	List(ctx context.Context, options metav1.ListOptions) (runtime.Object, error)

	// Watch establishes a watch connection to the API server for resource changes.
	//
	// The method returns a watch interface that will receive events when resources
	// of the bound type are created, updated, or deleted. The watch connection
	// remains active until the context is cancelled or the watch interface is closed.
	//
	// Parameters:
	//   - ctx: Context for the request, may be cancelled
	//   - options: Watch options to filter the events
	//
	// Returns:
	//   - watch.Interface: A watch interface for receiving resource events
	//   - error: Any error encountered while establishing the watch
	Watch(ctx context.Context, options metav1.ListOptions) (watch.Interface, error)
}

// ListerWatcher implements the Retriever interface using function callbacks.
//
// This struct provides a flexible way to implement resource retrieval by allowing
// the caller to provide custom List and Watch functions. It's commonly used to
// wrap Kubernetes client methods for specific resource types.
//
// A ListerWatcher is bound to a single resource type and must be properly configured
// with valid ListFunc and WatchFunc implementations.
type ListerWatcher struct {
	// ListFunc is the function called to retrieve a list of resources.
	ListFunc func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error)

	// WatchFunc is the function called to establish a watch connection.
	WatchFunc func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error)
}

// List calls the configured ListFunc to retrieve resources from the API server.
//
// This method delegates the list operation to the ListFunc callback, providing
// a consistent interface for resource retrieval.
//
// Parameters:
//   - ctx: Context for the request, may be cancelled
//   - options: List options to filter and paginate the results
//
// Returns:
//   - runtime.Object: A list of resources matching the criteria
//   - error: Any error returned by the ListFunc callback
func (l *ListerWatcher) List(
	ctx context.Context,
	options metav1.ListOptions,
) (runtime.Object, error) {
	return l.ListFunc(ctx, options)
}

// Watch calls the configured WatchFunc to establish a watch connection.
//
// This method delegates the watch operation to the WatchFunc callback, providing
// a consistent interface for resource watching.
//
// Parameters:
//   - ctx: Context for the request, may be cancelled
//   - options: Watch options to filter the events
//
// Returns:
//   - watch.Interface: A watch interface for receiving resource events
//   - error: Any error returned by the WatchFunc callback
func (l *ListerWatcher) Watch(
	ctx context.Context,
	options metav1.ListOptions,
) (watch.Interface, error) {
	return l.WatchFunc(ctx, options)
}
