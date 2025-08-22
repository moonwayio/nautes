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

	"github.com/moonwayio/nautes/component"
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
			name: "NoOptionsShouldReturnError",
			opts: []OptionFunc{},
			err:  "name is required",
		},
		{
			name: "NoClientShouldReturnError",
			opts: []OptionFunc{
				WithName("test"),
			},
			err: "client is required",
		},
		{
			name: "WithNameAndClientShouldSucceed",
			opts: []OptionFunc{
				WithName("test"),
				WithClient(fake.NewSimpleClientset()),
			},
		},
		{
			name: "WithAllOptionsShouldSucceed",
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
			name: "BasicRecorderShouldStartAndStop",
			opts: []OptionFunc{
				WithName("test"),
				WithClient(fake.NewSimpleClientset()),
			},
		},
		{
			name: "RecorderWithCustomSchemeShouldStartAndStop",
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
		eventType string
		reason    string
		message   string
		object    runtime.Object
	}

	testCases := []testCase{
		{
			name:      "NormalEventShouldBeRecorded",
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
			name:      "WarningEventShouldBeRecorded",
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

func (s *EventTestSuite) TestEventRecorderNeedsLeaderElection() {
	client := fake.NewSimpleClientset()

	type testCase struct {
		name     string
		opts     []OptionFunc
		expected bool
	}

	testCases := []testCase{
		{
			name:     "WithNoOptionShouldReturnFalse",
			opts:     []OptionFunc{WithName("test"), WithClient(client)},
			expected: false,
		},
		{
			name: "WithNeedsLeaderElectionTrueShouldReturnTrue",
			opts: []OptionFunc{
				WithName("test"),
				WithNeedsLeaderElection(true),
				WithClient(client),
			},
			expected: true,
		},

		{
			name: "WithNeedsLeaderElectionFalseShouldReturnFalse",
			opts: []OptionFunc{
				WithName("test"),
				WithNeedsLeaderElection(false),
				WithClient(client),
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			recorder, err := NewRecorder(tc.opts...)
			s.Require().NoError(err)
			s.Require().
				Equal(tc.expected, recorder.(component.LeaderElectionAware).NeedsLeaderElection())
		})
	}
}

func TestEventTestSuite(t *testing.T) {
	suite.Run(t, new(EventTestSuite))
}
