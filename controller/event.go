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

package controller

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// EventType represents the type of Kubernetes event.
//
// EventType is used to categorize resource events as they flow through the
// controller's event processing pipeline.
type EventType string

const (
	// EventAdd represents a resource creation event.
	EventAdd EventType = "add"
	// EventUpdate represents a resource modification event.
	EventUpdate EventType = "update"
	// EventDelete represents a resource deletion event.
	EventDelete EventType = "delete"
)

// FilterFunc is a function type that determines whether an event should be processed.
//
// FilterFunc is used to implement event filtering logic. Events that return false
// from any filter function are discarded and not queued for reconciliation.
// This allows controllers to focus only on relevant events and reduce unnecessary processing.
//
// Parameters:
//   - obj: The object to evaluate for filtering
//
// Returns:
//   - bool: true if the event should be processed, false to discard it
type FilterFunc[T Object] func(T) bool

// TransformerFunc is a function type that modifies objects before they are queued.
//
// TransformerFunc is used to implement event transformation logic. Transformers
// are applied in sequence and can be used for data enrichment, normalization,
// or other preprocessing tasks before reconciliation.
//
// Parameters:
//   - obj: The object to transform
//
// Returns:
//   - T: The transformed object
type TransformerFunc[T Object] func(T) T

// Delta represents a change event with metadata.
//
// Delta encapsulates a Kubernetes resource event with its type, the affected
// object, and a timestamp. This structure is used throughout the controller's
// event processing pipeline to maintain consistency and provide context.
type Delta[T Object] struct {
	// Type indicates the kind of event (add, update, delete)
	Type EventType
	// Object is the Kubernetes resource that was affected
	Object T
	// Timestamp records when the event was created
	Timestamp time.Time
}

// EventHandler manages the processing of Kubernetes events with filtering and transformation.
//
// EventHandler implements the cache.ResourceEventHandler interface and provides
// advanced event processing capabilities. It applies filters to determine which
// events should be processed and transformers to modify objects before they are
// queued for reconciliation.
//
// The EventHandler is concurrency safe and can be used with multiple informers
// simultaneously.
type EventHandler[T Object] struct {
	queue        workqueue.TypedInterface[Delta[T]]
	filters      []FilterFunc[T]
	transformers []TransformerFunc[T]

	mu     sync.Mutex
	logger klog.Logger
}

// NewEventHandler creates a new EventHandler with the specified configuration.
//
// This function creates an EventHandler that can be used with Kubernetes informers
// to process events with filtering and transformation capabilities.
//
// Parameters:
//   - queue: The work queue where processed events will be enqueued
//   - filters: Optional list of filter functions to determine which events to process
//   - transformers: Optional list of transformer functions to modify objects before processing
//   - logger: Logger for event processing operations
//
// Returns:
//   - cache.ResourceEventHandler: An event handler ready for use with informers
func NewEventHandler[T Object](
	queue workqueue.TypedInterface[Delta[T]],
	filters []FilterFunc[T],
	transformers []TransformerFunc[T],
	logger klog.Logger,
) cache.ResourceEventHandler {
	return &EventHandler[T]{
		queue:        queue,
		filters:      filters,
		transformers: transformers,
		logger:       logger,
	}
}

// OnAdd handles resource creation events.
//
// This method is called by the informer when a new resource is created.
// It processes the event through the filtering and transformation pipeline
// before enqueuing it for reconciliation.
//
// Parameters:
//   - obj: The newly created resource
//   - isInInitialList: Whether this event is part of the initial list (not used in this implementation)
func (e *EventHandler[T]) OnAdd(obj any, _ bool) {
	e.handle(EventAdd, obj)
}

// OnUpdate handles resource modification events.
//
// This method is called by the informer when an existing resource is modified.
// It processes the event through the filtering and transformation pipeline
// before enqueuing it for reconciliation.
//
// Parameters:
//   - oldObj: The previous version of the resource (not used in this implementation)
//   - newObj: The updated version of the resource
func (e *EventHandler[T]) OnUpdate(_, newObj any) {
	e.handle(EventUpdate, newObj)
}

// OnDelete handles resource deletion events.
//
// This method is called by the informer when a resource is deleted.
// It processes the event through the filtering and transformation pipeline
// before enqueuing it for reconciliation.
//
// Parameters:
//   - obj: The deleted resource
func (e *EventHandler[T]) OnDelete(obj any) {
	e.handle(EventDelete, obj)
}

// handle processes an event through the filtering and transformation pipeline.
//
// This method is the core event processing logic that applies filters to determine
// if an event should be processed, applies transformers to modify the object,
// and enqueues the resulting Delta for reconciliation.
//
// The method is concurrency safe and handles type assertions and error conditions
// gracefully.
//
// Parameters:
//   - event: The type of event being processed
//   - obj: The object associated with the event
func (e *EventHandler[T]) handle(event EventType, obj any) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Type assertion to ensure the object is of the expected type
	typedObj, ok := obj.(T)
	if !ok {
		e.logger.Error(
			fmt.Errorf("object is not of type %T", *new(T)),
			"could not assert object type",
			"object",
			obj,
		)
		return
	}

	// Apply filters to determine if the event should be processed
	for _, filter := range e.filters {
		if !filter(typedObj) {
			return
		}
	}

	// Apply transformers to modify the object
	for _, transformer := range e.transformers {
		typedObj = transformer(typedObj)
	}

	// Create the Delta and enqueue it for reconciliation
	item := Delta[T]{
		Type:      event,
		Object:    typedObj,
		Timestamp: time.Now(),
	}

	e.queue.Add(item)
}
