// Package event provides a Kubernetes event recorder.
//
// The event package implements a Kubernetes event recorder that can create and
// manage events associated with Kubernetes resources. Events provide a way to
// track important occurrences in the cluster and are useful for debugging,
// monitoring, and auditing purposes.
package event

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/moonwayio/nautes/manager"
)

// EventType represents the type of event.
//
// EventType defines the classification of Kubernetes events. Events can be
// either normal (informational) or warning (indicating a problem).
//
//nolint:revive
type EventType string

const (
	// EventTypeNormal represents a normal event.
	//
	// Normal events indicate that something expected happened, such as a
	// resource being created, updated, or deleted successfully.
	EventTypeNormal EventType = "Normal"

	// EventTypeWarning represents a warning event.
	//
	// Warning events indicate that something unexpected happened, such as
	// a resource failing to start, a configuration error, or a resource
	// being in an unhealthy state.
	EventTypeWarning EventType = "Warning"
)

// Recorder is the interface for event recording.
//
// The Recorder interface provides methods to create and manage Kubernetes
// events. It supports different event types, formatted messages, and
// annotated events with custom metadata.
//
// Implementations of this interface are concurrency safe and can be used concurrently
// from multiple goroutines.
type Recorder interface {
	manager.Component

	// Event records a simple event with the specified type, reason, and message.
	//
	// This method creates a Kubernetes event associated with the specified object.
	// The event will be visible in the Kubernetes API and can be viewed using
	// kubectl get events.
	//
	// Parameters:
	//   - object: The Kubernetes object to associate the event with
	//   - eventtype: The type of event (Normal or Warning)
	//   - reason: A short, machine-readable reason for the event
	//   - message: A human-readable description of the event
	Event(object runtime.Object, eventtype EventType, reason, message string)

	// Eventf records a formatted event with the specified type, reason, and message.
	//
	// This method creates a Kubernetes event with a formatted message string,
	// similar to fmt.Printf. The message is formatted using the provided arguments.
	//
	// Parameters:
	//   - object: The Kubernetes object to associate the event with
	//   - eventtype: The type of event (Normal or Warning)
	//   - reason: A short, machine-readable reason for the event
	//   - messageFmt: A format string for the event message
	//   - args: Arguments to format the message string
	Eventf(object runtime.Object, eventtype EventType, reason, messageFmt string, args ...any)

	// AnnotatedEventf records an annotated event with custom metadata.
	//
	// This method creates a Kubernetes event with additional annotations that
	// can be used for filtering, grouping, or custom processing of events.
	//
	// Parameters:
	//   - object: The Kubernetes object to associate the event with
	//   - annotations: Custom annotations to add to the event
	//   - eventtype: The type of event (Normal or Warning)
	//   - reason: A short, machine-readable reason for the event
	//   - messageFmt: A format string for the event message
	//   - args: Arguments to format the message string
	AnnotatedEventf(
		object runtime.Object,
		annotations map[string]string,
		eventtype EventType,
		reason, messageFmt string,
		args ...any,
	)
}

// recorder implements the Recorder interface.
//
// The recorder maintains a connection to the Kubernetes API and provides
// methods for creating and managing events. It uses the Kubernetes event
// broadcaster to ensure events are properly distributed.
type recorder struct {
	name string

	client      kubernetes.Interface
	recorder    record.EventRecorder
	broadcaster record.EventBroadcaster
	logger      klog.Logger
	started     bool
}

// NewRecorder creates a new event recorder with the provided options.
//
// The recorder is initialized with the specified configuration and can be
// customized using option functions. The returned recorder is ready to
// create and manage Kubernetes events.
//
// Parameters:
//   - opts: Optional configuration functions to customize the recorder behavior
//
// Returns:
//   - Recorder: A new event recorder instance ready for use
//   - error: Any error encountered during initialization
func NewRecorder(opts ...OptionFunc) (Recorder, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if err := o.setDefaults(); err != nil {
		return nil, err
	}

	broadcaster := record.NewBroadcaster()

	return &recorder{
		name:        o.name,
		client:      o.client,
		recorder:    broadcaster.NewRecorder(o.scheme, corev1.EventSource{Component: o.name}),
		broadcaster: broadcaster,
		logger:      klog.Background().WithValues("component", "event/"+o.name),
		started:     false,
	}, nil
}

// Start initializes and starts the event recorder.
//
// The method sets up the event broadcaster and begins recording events
// to the Kubernetes API. This method is called automatically by the manager
// when the component is started.
//
// Returns:
//   - error: Any error encountered during startup
func (r *recorder) Start() error {
	// if the event recorder is already started, return nil to ensure idempotency
	if r.started {
		return nil
	}

	r.started = true
	r.logger.Info("starting event recorder")
	r.broadcaster.StartRecordingToSink(
		&typedv1core.EventSinkImpl{Interface: r.client.CoreV1().Events("")},
	)
	r.logger.Info("event recorder started successfully")
	return nil
}

// Stop gracefully shuts down the event recorder.
//
// The method stops the event broadcaster and cleans up resources.
// This method is called automatically by the manager when the component
// is stopped.
//
// Returns:
//   - error: Any error encountered during shutdown
func (r *recorder) Stop() error {
	// if the event recorder is not started, return nil to ensure idempotency
	if !r.started {
		return nil
	}

	r.started = false
	r.logger.Info("stopping event recorder")
	r.broadcaster.Shutdown()
	r.logger.Info("event recorder stopped successfully")
	return nil
}

// Event records an event with the specified type, reason, and message.
//
// Parameters:
//   - object: The Kubernetes object to associate the event with
//   - eventtype: The type of event
//   - reason: A short, machine-readable reason for the event
//   - message: A human-readable description of the event
func (r *recorder) Event(object runtime.Object, eventtype EventType, reason, message string) {
	r.logger.V(1).
		Info("recording event", "object", object, "eventtype", eventtype, "reason", reason, "message", message)
	r.recorder.Event(object, string(eventtype), reason, message)
}

// Eventf records a formatted event with the specified type, reason, and message.
//
// Parameters:
//   - object: The Kubernetes object to associate the event with
//   - eventtype: The type of event (Normal or Warning)
//   - reason: A short, machine-readable reason for the event
//   - messageFmt: A format string for the event message
//   - args: Arguments to format the message string
func (r *recorder) Eventf(
	object runtime.Object,
	eventtype EventType,
	reason, messageFmt string,
	args ...any,
) {
	r.logger.V(1).
		Info("recording event", "object", object, "eventtype", eventtype, "reason", reason, "message", fmt.Sprintf(messageFmt, args...))
	r.recorder.Eventf(object, string(eventtype), reason, messageFmt, args...)
}

// AnnotatedEventf records an annotated event with custom metadata.
//
// This method creates a Kubernetes event with additional annotations that
// can be used for filtering, grouping, or custom processing of events.
//
// Parameters:
//   - object: The Kubernetes object to associate the event with
//   - annotations: Custom annotations to add to the event
//   - eventtype: The type of event (Normal or Warning)
//   - reason: A short, machine-readable reason for the event
//   - messageFmt: A format string for the event message
//   - args: Arguments to format the message string
func (r *recorder) AnnotatedEventf(
	object runtime.Object,
	annotations map[string]string,
	eventtype EventType,
	reason, messageFmt string,
	args ...any,
) {
	r.logger.V(1).
		Info("recording event", "object", object, "annotations", annotations, "eventtype", eventtype, "reason", reason, "message", fmt.Sprintf(messageFmt, args...))
	r.recorder.AnnotatedEventf(object, annotations, string(eventtype), reason, messageFmt, args...)
}

// GetName returns the name of the event recorder.
//
// The name is constructed by combining "event/" with the configured
// recorder name. This is used for logging, metrics, and debugging purposes.
//
// Returns:
//   - string: The event recorder name in the format "event/{name}"
func (r *recorder) GetName() string {
	return "event/" + r.name
}
