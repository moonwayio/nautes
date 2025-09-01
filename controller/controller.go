// Copyright 2025 The Moonway.io Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// Package controller provides Kubernetes controller functionality for watching and reconciling resources.
//
// The controller package implements the watch-queue-reconcile pattern commonly used in Kubernetes
// controllers. It provides a framework for building controllers that can watch Kubernetes resources,
// queue events for processing, and reconcile the desired state with the actual state.
package controller

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/moonwayio/nautes/component"
	"github.com/moonwayio/nautes/metrics"
)

var (
	reconcileCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controller_reconcile_total",
			Help: "Total number of reconciles",
		},
		[]string{"name"},
	)
	reconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "controller_reconcile_duration_seconds",
			Help: "Duration of reconciles",
		},
		[]string{"name"},
	)
	reconcileErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controller_reconcile_errors_total",
			Help: "Total number of reconcile errors",
		},
		[]string{"name"},
	)
	reconcileSuccesses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controller_reconcile_successes_total",
			Help: "Total number of reconcile successes",
		},
		[]string{"name"},
	)
)

// init registers the Prometheus metrics for the controller package.
//
// This function is called automatically when the package is imported and
// registers all the metrics defined in this package with the global metrics
// registry. This ensures that the metrics are available for collection
// by Prometheus or other monitoring systems.
func init() {
	metrics.Registry.MustRegister(
		reconcileCount,
		reconcileDuration,
		reconcileErrors,
		reconcileSuccesses,
	)
}

// Object is a common interface for Kubernetes objects.
//
// It extends the runtime.Object interface to include the comparable
// constraint, allowing for type-safe comparisons of objects.
//
// This interface is used to define the type of objects that can be
// reconciled by the controller.
type Object interface {
	runtime.Object
	comparable
}

// Reconciler function type for handling reconciliation logic.
//
// A Reconciler is a function that takes a Kubernetes object and performs the
// necessary actions to reconcile the desired state with the actual state.
// The function should be idempotent and handle errors gracefully.
//
// Parameters:
//   - ctx: Context for the reconciliation operation, may be cancelled
//   - obj: The Kubernetes object to reconcile
//
// Returns:
//   - error: Any error encountered during reconciliation
type Reconciler[T Object] func(ctx context.Context, obj Delta[T]) error

// Controller implements the watch–queue–reconcile loop.
//
// The Controller interface provides methods to manage the lifecycle of a Kubernetes
// controller. It handles resource watching, event queuing, and reconciliation
// coordination. The controller can watch multiple resource types and process
// events concurrently with advanced filtering and transformation capabilities.
//
// The controller supports:
//   - Multiple resource retrievers for different resource types
//   - Configurable event filtering to process only relevant events
//   - Event transformation for data enrichment or normalization
//   - Concurrent processing with configurable worker pools
//   - Built-in metrics and monitoring
//
// Implementations of this interface are concurrency safe and can be used concurrently
// from multiple goroutines.
type Controller[T Object] interface {
	component.Component

	// AddRetriever adds a resource retriever to the controller with optional filtering and transformation.
	//
	// The retriever provides the List and Watch functions needed to monitor
	// Kubernetes resources. Multiple retrievers can be added to watch different
	// resource types or namespaces.
	//
	// The filters parameter allows you to specify functions that determine which
	// events should be processed. Events that don't pass any filter are discarded.
	// If no filters are provided, all events are processed.
	//
	// The transformers parameter allows you to specify functions that modify
	// objects before they are queued for reconciliation. Transformers are applied
	// in order and can be used for data enrichment, normalization, or other
	// preprocessing tasks.
	//
	// Parameters:
	//   - retriever: The retriever to add for resource watching
	//   - filters: Optional list of filter functions to determine which events to process
	//   - transformers: Optional list of transformer functions to modify objects before processing
	//
	// Returns:
	//   - error: Any error encountered while adding the retriever
	AddRetriever(
		retriever Retriever,
		filters []FilterFunc[T],
		transformers []TransformerFunc[T],
	) error
}

// controller implements the Controller interface.
//
// The controller maintains a work queue for processing events and coordinates
// the reconciliation of resources. It uses informers to watch Kubernetes
// resources and enqueue events for processing by worker goroutines.
//
// The controller supports advanced event processing with filtering and transformation
// capabilities, allowing for fine-grained control over which events are processed
// and how objects are modified before reconciliation.
type controller[T Object] struct {
	opts       options
	reconciler Reconciler[T]
	queue      workqueue.TypedRateLimitingInterface[Delta[T]]
	informers  []cache.SharedIndexInformer
	logger     klog.Logger

	stop    chan struct{}
	mu      sync.Mutex
	started bool
}

// NewController creates a new Controller instance with the provided reconciler and options.
//
// The controller is initialized with the specified reconciler function and can be
// customized using option functions. The returned controller is ready to accept
// resource retrievers and start processing events.
//
// Parameters:
//   - reconciler: The function that handles reconciliation logic
//   - opts: Optional configuration functions to customize the controller behavior
//
// Returns:
//   - Controller: A new controller instance ready for use
//   - error: Any error encountered during initialization
func NewController[T Object](reconciler Reconciler[T], opts ...OptionFunc) (Controller[T], error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if err := o.setDefaults(); err != nil {
		return nil, err
	}

	return &controller[T]{
		opts:       o,
		reconciler: reconciler,
		queue: workqueue.NewTypedRateLimitingQueue(
			workqueue.DefaultTypedControllerRateLimiter[Delta[T]](),
		),
		informers: make([]cache.SharedIndexInformer, 0),
		logger:    klog.Background().WithValues("component", "controller/"+o.name),
		stop:      make(chan struct{}),
	}, nil
}

// AddRetriever adds a resource retriever to the controller with filtering and transformation capabilities.
//
// This method creates an informer for the provided retriever and registers
// event handlers to enqueue resource changes for processing. The informer
// will watch for resource creation, updates, and deletions, adding them
// to the work queue for reconciliation.
//
// The method applies filters to determine which events should be processed,
// and transformers to modify objects before they are queued. This allows
// for fine-grained control over event processing and data enrichment.
//
// The method is concurrency safe and can be called multiple times to add
// different retrievers for various resource types or namespaces.
//
// Parameters:
//   - retriever: The resource retriever to add. Must not be nil and must
//     provide valid List and Watch functions.
//   - filters: Optional list of filter functions to determine which events to process
//   - transformers: Optional list of transformer functions to modify objects before processing
//
// Returns:
//   - error: Any error encountered while setting up the informer or
//     registering event handlers
func (c *controller[T]) AddRetriever(
	retriever Retriever,
	filters []FilterFunc[T],
	transformers []TransformerFunc[T],
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return retriever.List(context.Background(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return retriever.Watch(context.Background(), options)
		},
		ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return retriever.List(ctx, options)
		},
		WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
			return retriever.Watch(ctx, options)
		},
	}

	informer := cache.NewSharedIndexInformer(
		lw,
		nil,
		c.opts.resyncInterval,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	transformers = append(transformers, GVKTransformer[T](c.opts.scheme))

	eventHandler := NewEventHandler(c.queue, filters, transformers, c.logger)

	_, err := informer.AddEventHandlerWithResyncPeriod(eventHandler, c.opts.resyncInterval)
	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	c.informers = append(c.informers, informer)

	return nil
}

// Start starts the controller.
func (c *controller[T]) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	defer utilruntime.HandleCrash()

	// if the controller is already started, return nil to ensure idempotency
	if c.started {
		return nil
	}

	c.started = true

	c.logger.Info("starting controller")

	// Start informers
	cacheSyncs := make([]cache.InformerSynced, 0, len(c.informers))
	for _, informer := range c.informers {
		cacheSyncs = append(cacheSyncs, informer.HasSynced)
		go informer.Run(c.stop)
	}

	if !cache.WaitForCacheSync(c.stop, cacheSyncs...) {
		return errors.New("timed out waiting for caches to sync")
	}

	// Start workers
	for i := 0; i < c.opts.concurrency; i++ {
		go wait.Until(c.runWorker, time.Second, c.stop)
	}

	c.logger.Info("controller started successfully")
	return nil
}

// Stop stops the controller.
func (c *controller[T]) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// if the controller is not started, return nil to ensure idempotency
	if !c.started {
		return nil
	}

	c.started = false
	c.logger.Info("stopping controller")
	c.queue.ShutDown()
	close(c.stop)
	c.logger.Info("controller stopped successfully")
	return nil
}

// GetName returns the name of the controller.
//
// The name is constructed by combining "controller/" with the configured
// controller name. This is used for logging, metrics, and debugging purposes.
//
// Returns:
//   - string: The controller name in the format "controller/{name}"
func (c *controller[T]) GetName() string {
	return "controller/" + c.opts.name
}

// NeedsLeaderElection indicates if the controller needs leader election.
//
// When set to true, the controller will only start when the leader election
// is active. If set to false, the controller will start immediately when the
// manager starts.
//
// Returns:
//   - bool: true if the controller needs leader election, false otherwise
func (c *controller[T]) NeedsLeaderElection() bool {
	return c.opts.needsLeaderElection
}

// runWorker runs the worker loop for processing items from the work queue.
//
// This method runs in a goroutine and continuously processes items from the
// work queue until the worker is shut down. It calls processNextItem in a
// loop and logs when the worker stops.
//
// The worker will continue running until the controller is stopped or the
// work queue is shut down.
func (c *controller[T]) runWorker() {
	for c.processNextItem() {
		// Process next item from the queue. If the worker is shutting down, break the loop.
	}

	c.logger.Info("worker stopped")
}

// processNextItem processes the next item from the work queue.
//
// This method retrieves the next item from the work queue, processes it through
// the reconciler, and handles any errors that occur during processing. It also
// updates metrics for reconciliation counts, durations, and errors.
//
// The method returns true if the worker should continue processing items,
// and false if the worker should stop (e.g., when the queue is shut down).
//
// Returns:
//   - bool: true if the worker should continue, false if it should stop
func (c *controller[T]) processNextItem() bool {
	ctx := context.Background()

	// Get the next item from the work queue
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	// Mark the item as done when we finish processing it
	defer c.queue.Done(item)

	loggerCtx := klog.NewContext(ctx, c.logger)

	// Increment the reconcile count metric
	reconcileCount.WithLabelValues(c.opts.name).Inc()

	// Measure reconciliation duration
	start := time.Now()
	if err := c.reconciler(loggerCtx, item); err == nil {
		// Forget the item
		c.queue.Forget(item)

		// Increment the reconcile successes metric
		reconcileSuccesses.WithLabelValues(c.opts.name).Inc()
	} else if c.queue.NumRequeues(item) < c.opts.maxRetries {
		c.logger.V(1).Error(err, "failed to reconcile object, requeuing", "item", item)

		// Requeue the item
		c.queue.AddRateLimited(item)

		// Increment the reconcile errors metric
		reconcileErrors.WithLabelValues(c.opts.name).Inc()
	} else {
		c.logger.Error(err, "failed to reconcile object")

		// Forget the item
		c.queue.Forget(item)

		// Increment the reconcile errors metric
		reconcileErrors.WithLabelValues(c.opts.name).Inc()
	}

	// Observe the reconcile duration metric
	reconcileDuration.WithLabelValues(c.opts.name).Observe(time.Since(start).Seconds())

	return true
}
