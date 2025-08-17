// Package controller provides Kubernetes controller functionality for watching and reconciling resources.
package controller

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/moonwayio/nautes/manager"
)

// Reconciler function type for handling reconciliation logic.
type Reconciler func(ctx context.Context, obj runtime.Object) error

// Controller implements the watch–queue–reconcile loop.
type Controller interface {
	manager.Component
	AddRetriever(retriever Retriever) error
}

// controller implements the Controller interface.
type controller struct {
	opts       options
	reconciler Reconciler
	client     kubernetes.Interface
	scheme     *runtime.Scheme
	queue      workqueue.TypedRateLimitingInterface[string]
	informers  []cache.SharedIndexInformer
	logger     klog.Logger

	stop chan struct{}
	mu   sync.RWMutex
}

// NewController creates a new Controller instance.
func NewController(reconciler Reconciler, opts ...OptionFunc) (Controller, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if err := o.setDefaults(); err != nil {
		return nil, err
	}

	return &controller{
		opts:       o,
		reconciler: reconciler,
		client:     o.client,
		scheme:     o.scheme,
		queue: workqueue.NewTypedRateLimitingQueue(
			workqueue.DefaultTypedControllerRateLimiter[string](),
		),
		informers: make([]cache.SharedIndexInformer, 0),
		logger:    klog.Background().WithValues("component", "controller/"+o.name),
		stop:      make(chan struct{}),
	}, nil
}

// AddWatcher adds a watcher for a specific resource type.
func (c *controller) AddRetriever(retriever Retriever) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	store := cache.Indexers{}

	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return retriever.List(context.Background(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return retriever.Watch(context.Background(), options)
		},
	}

	informer := cache.NewSharedIndexInformer(lw, nil, c.opts.resyncInterval, store)

	_, err := informer.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				c.logger.Info("could not add item from 'add' event to queue", "error", err)
				return
			}
			c.queue.Add(key)
		},
		UpdateFunc: func(_, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err != nil {
				c.logger.Info("could not add item from 'update' event to queue", "error", err)
				return
			}
			c.queue.Add(key)
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err != nil {
				c.logger.Info("could not add item from 'delete' event to queue", "error", err)
				return
			}
			c.queue.Add(key)
		},
	}, c.opts.resyncInterval)
	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	c.informers = append(c.informers, informer)

	return nil
}

// Start starts the controller.
func (c *controller) Start() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

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
func (c *controller) Stop() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.logger.Info("stopping controller")
	c.queue.ShutDown()
	close(c.stop)
	c.logger.Info("controller stopped successfully")
	return nil
}

func (c *controller) GetName() string {
	return "controller/" + c.opts.name
}

// runWorker runs the worker loop.
func (c *controller) runWorker() {
	for c.processNextItem() {
		// Process next item from the queue. If the worker is shutting down, break the loop.
	}

	c.logger.Info("worker stopped")
}

// processNextItem processes the next item from the work queue and returns if the worker should continue running.
func (c *controller) processNextItem() bool {
	ctx := context.Background()

	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	defer c.queue.Done(key)

	obj, exists, err := c.getObject(key)
	if err != nil {
		c.logger.Error(err, "failed to get object", "key", key)
		return true
	}

	if !exists {
		return true
	}

	// populate the typemeta, since it's not populated by the informer
	meta, err := getGVKForObject(obj, c.scheme)
	if err == nil {
		obj.GetObjectKind().SetGroupVersionKind(meta)
	}

	loggerCtx := klog.NewContext(ctx, c.logger)

	if err := c.reconciler(loggerCtx, obj); err != nil {
		c.logger.Error(err, "failed to reconcile object", "key", key)
		return true
	}

	return true
}

func (c *controller) getObject(key string) (runtime.Object, bool, error) {
	for _, informer := range c.informers {
		idx := informer.GetIndexer()
		obj, exists, err := idx.GetByKey(key)
		if err != nil {
			return nil, false, fmt.Errorf("failed to get object from indexer: %w", err)
		}
		if exists {
			runtimeObj, ok := obj.(runtime.Object)
			if !ok {
				return nil, false, errors.New("object is not a runtime.Object")
			}
			return runtimeObj, true, nil
		}
	}
	return nil, false, nil
}
