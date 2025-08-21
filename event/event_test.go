package event

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

type EventTestSuite struct {
	suite.Suite
}

func (s *EventTestSuite) TestNewRecorder() {
	type testCase struct {
		name string
		opts []OptionFunc
		err  string
	}

	testCases := []testCase{
		{
			name: "no options should return error",
			opts: []OptionFunc{},
			err:  "name is required",
		},
		{
			name: "no client should return error",
			opts: []OptionFunc{
				WithName("test"),
			},
			err: "client is required",
		},
		{
			name: "with name and client should succeed",
			opts: []OptionFunc{
				WithName("test"),
				WithClient(fake.NewSimpleClientset()),
			},
		},
		{
			name: "with all options should succeed",
			opts: []OptionFunc{
				WithName("test"),
				WithClient(fake.NewSimpleClientset()),
				WithScheme(scheme.Scheme),
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			recorder, err := NewRecorder(tc.opts...)

			if tc.err != "" {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.err)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(recorder)
			}
		})
	}
}

func (s *EventTestSuite) TestStartAndStopRecorder() {
	type testCase struct {
		name string
		opts []OptionFunc
	}

	testCases := []testCase{
		{
			name: "basic recorder should start and stop",
			opts: []OptionFunc{
				WithName("test"),
				WithClient(fake.NewSimpleClientset()),
			},
		},
		{
			name: "recorder with custom scheme should start and stop",
			opts: []OptionFunc{
				WithName("test"),
				WithClient(fake.NewSimpleClientset()),
				WithScheme(scheme.Scheme),
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			recorder, err := NewRecorder(tc.opts...)
			s.Require().NoError(err)

			// Start the recorder
			err = recorder.Start()
			s.Require().NoError(err)

			// Stop the recorder
			err = recorder.Stop()
			s.Require().NoError(err)
		})
	}
}

func (s *EventTestSuite) TestEventRecording() {
	type testCase struct {
		name      string
		eventType EventType
		reason    string
		message   string
		object    runtime.Object
	}

	testCases := []testCase{
		{
			name:      "normal event should be recorded",
			eventType: EventTypeNormal,
			reason:    "TestReason",
			message:   "Test message %s",
			object: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
		},
		{
			name:      "warning event should be recorded",
			eventType: EventTypeWarning,
			reason:    "TestWarning",
			message:   "Test warning message %s",
			object: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			client := fake.NewSimpleClientset()
			recorder, err := NewRecorder(
				WithName("test"),
				WithClient(client),
			)
			s.Require().NoError(err)

			// Start the recorder
			err = recorder.Start()
			s.Require().NoError(err)

			// Record an event
			recorder.Event(tc.object, tc.eventType, tc.reason, fmt.Sprintf(tc.message, "test"))
			recorder.Eventf(tc.object, tc.eventType, tc.reason, tc.message, "test")
			recorder.AnnotatedEventf(
				tc.object,
				map[string]string{"key": "value"},
				tc.eventType,
				tc.reason,
				tc.message,
				"test",
			)

			// Stop the recorder
			err = recorder.Stop()
			s.Require().NoError(err)
		})
	}
}

func (s *EventTestSuite) TestGetName() {
	recorder, err := NewRecorder(
		WithName("test"),
		WithClient(fake.NewSimpleClientset()),
	)
	s.Require().NoError(err)
	s.Require().Equal("event/test", recorder.GetName())
}

func (s *EventTestSuite) TestEventRecorderIdempotency() {
	recorder, err := NewRecorder(
		WithName("test"),
		WithClient(fake.NewSimpleClientset()),
	)
	s.Require().NoError(err)

	// Test Start
	err = recorder.Start()
	s.Require().NoError(err)

	// Start again should not error (idempotent)
	err = recorder.Start()
	s.Require().NoError(err)

	// Test Stop
	err = recorder.Stop()
	s.Require().NoError(err)

	// Stop again should not error (idempotent)
	err = recorder.Stop()
	s.Require().NoError(err)
}

func TestEventTestSuite(t *testing.T) {
	suite.Run(t, new(EventTestSuite))
}
