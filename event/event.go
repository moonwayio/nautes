// Package event provides a Kubernetes event recorder.
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
//nolint:revive
type EventType string

const (
	// EventTypeNormal represents a normal event.
	EventTypeNormal EventType = "Normal"
	// EventTypeWarning represents a warning event.
	EventTypeWarning EventType = "Warning"
)

// Recorder is the interface for event recording.
type Recorder interface {
	manager.Component

	Event(object runtime.Object, eventtype EventType, reason, message string)
	Eventf(object runtime.Object, eventtype EventType, reason, messageFmt string, args ...any)
	AnnotatedEventf(
		object runtime.Object,
		annotations map[string]string,
		eventtype EventType,
		reason, messageFmt string,
		args ...any,
	)
}

type recorder struct {
	name string

	client      kubernetes.Interface
	recorder    record.EventRecorder
	broadcaster record.EventBroadcaster
	logger      klog.Logger
}

// NewRecorder creates a new event recorder.
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
	}, nil
}

func (r *recorder) Start() error {
	r.logger.Info("starting event recorder")
	r.broadcaster.StartRecordingToSink(
		&typedv1core.EventSinkImpl{Interface: r.client.CoreV1().Events("")},
	)
	r.logger.Info("event recorder started successfully")
	return nil
}

func (r *recorder) Stop() error {
	r.logger.Info("stopping event recorder")
	r.broadcaster.Shutdown()
	r.logger.Info("event recorder stopped successfully")
	return nil
}

func (r *recorder) Event(object runtime.Object, eventtype EventType, reason, message string) {
	r.logger.V(1).
		Info("recording event", "object", object, "eventtype", eventtype, "reason", reason, "message", message)
	r.recorder.Event(object, string(eventtype), reason, message)
}

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

func (r *recorder) GetName() string {
	return "event/" + r.name
}
