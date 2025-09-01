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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/moonwayio/nautes/component"
)

type ControllerTestSuite struct {
	suite.Suite
}

var ReconcileTimeout = 200 * time.Millisecond

const defaultNS = "default"

func (s *ControllerTestSuite) TestNewController() {
	type testCase struct {
		name       string
		reconciler Reconciler[*corev1.Pod]
		opts       []OptionFunc
		err        string
	}

	testCases := []testCase{
		{
			name:       "NoNameShouldReturnError",
			reconciler: func(_ context.Context, _ Delta[*corev1.Pod]) error { return nil },
			opts:       []OptionFunc{},
			err:        "name is required",
		},
		{
			name:       "WithNameShouldSucceed",
			reconciler: func(_ context.Context, _ Delta[*corev1.Pod]) error { return nil },
			opts: []OptionFunc{
				WithName("test"),
			},
			err: "",
		},
		{
			name:       "WithAllOptionsShouldSucceed",
			reconciler: func(_ context.Context, _ Delta[*corev1.Pod]) error { return nil },
			opts: []OptionFunc{
				WithName("test"),
				WithResyncInterval(30 * time.Second),
				WithConcurrency(5),
				WithScheme(runtime.NewScheme()),
				WithMaxRetries(10),
			},
			err: "",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			controller, err := NewController(tc.reconciler, tc.opts...)

			if tc.err != "" {
				s.Require().Error(err, tc.err)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(controller)
			}
		})
	}
}

func (s *ControllerTestSuite) TestAddRetriever() {
	clientset := fake.NewSimpleClientset()

	reconciler := func(_ context.Context, _ Delta[*corev1.Pod]) error { return nil }
	controller, err := NewController(reconciler, WithName("test"))
	s.Require().NoError(err)

	// Create a mock retriever
	retriever := &ListerWatcher{
		ListFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return clientset.CoreV1().Pods("default").List(ctx, options)
		},
		WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
			return clientset.CoreV1().Pods("default").Watch(ctx, options)
		},
	}

	err = controller.AddRetriever(retriever, nil, nil)
	s.Require().NoError(err)
}

func (s *ControllerTestSuite) TestStartAndStopController() {
	type testCase struct {
		name            string
		reconciler      Reconciler[*corev1.Pod]
		expectReconcile bool
	}

	testCases := []testCase{
		{
			name: "NormalReconciliationShouldSucceed",
			reconciler: func(_ context.Context, _ Delta[*corev1.Pod]) error {
				return nil
			},
			expectReconcile: true,
		},
		{
			name: "ReconciliationWithErrorShouldStillComplete",
			reconciler: func(_ context.Context, _ Delta[*corev1.Pod]) error {
				return errors.New("test error")
			},
			expectReconcile: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			client := fake.NewSimpleClientset()

			// Create a test pod
			_, err := client.CoreV1().Pods("default").Create(context.Background(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			}, metav1.CreateOptions{})
			s.Require().NoError(err)

			for i := range 2 {
				ch := make(chan struct{})

				// Wrap the reconciler to signal completion
				wrappedReconciler := func(ctx context.Context, obj Delta[*corev1.Pod]) error {
					defer close(ch)
					return tc.reconciler(ctx, obj)
				}

				controller, err := NewController(
					wrappedReconciler,
					WithName("test"),
					WithMaxRetries(i),
				)
				s.Require().NoError(err)

				// Create a mock retriever
				retriever := &ListerWatcher{
					ListFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
						return client.CoreV1().Pods("default").List(ctx, options)
					},
					WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
						return client.CoreV1().Pods("default").Watch(ctx, options)
					},
				}

				err = controller.AddRetriever(retriever, nil, nil)
				s.Require().NoError(err)

				// Start the controller
				err = controller.Start()
				s.Require().NoError(err)

				if tc.expectReconcile {
					select {
					case <-ch:
					case <-time.After(ReconcileTimeout):
						s.Fail("reconciler did not run")
					}
				}

				// Stop the controller
				err = controller.Stop()
				s.Require().NoError(err)
			}
		})
	}
}

func (s *ControllerTestSuite) TestControllerStartAndStopWithDifferentConfigurations() {
	type testCase struct {
		name string
		opts []OptionFunc
	}

	testCases := []testCase{
		{
			name: "WithCustomSchemeShouldWork",
			opts: []OptionFunc{
				WithName("test"),
				WithScheme(runtime.NewScheme()),
			},
		},
		{
			name: "WithResyncIntervalShouldWork",
			opts: []OptionFunc{
				WithName("test"),
				WithResyncInterval(1 * time.Second),
			},
		},
		{
			name: "WithHighConcurrencyShouldWork",
			opts: []OptionFunc{
				WithName("test"),
				WithConcurrency(3),
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			client := fake.NewSimpleClientset()

			// Create a test pod
			_, err := client.CoreV1().Pods("default").Create(context.Background(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			}, metav1.CreateOptions{})
			s.Require().NoError(err)

			ch := make(chan struct{})

			reconciler := func(_ context.Context, _ Delta[*corev1.Pod]) error {
				defer close(ch)
				return nil
			}

			controller, err := NewController(reconciler, tc.opts...)
			s.Require().NoError(err)

			// Create a mock retriever
			retriever := &ListerWatcher{
				ListFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
					return client.CoreV1().Pods("default").List(ctx, options)
				},
				WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
					return client.CoreV1().Pods("default").Watch(ctx, options)
				},
			}

			err = controller.AddRetriever(retriever, nil, nil)
			s.Require().NoError(err)

			// Start the controller
			err = controller.Start()
			s.Require().NoError(err)

			select {
			case <-ch:
			case <-time.After(ReconcileTimeout):
				s.Fail("reconciler did not run")
			}

			// Stop the controller
			err = controller.Stop()
			s.Require().NoError(err)
		})
	}
}

func (s *ControllerTestSuite) TestControllerStartAndStopWithGVKMeta() {
	type testCase struct {
		name   string
		object runtime.Object
	}

	testCases := []testCase{
		{
			name: "WithProperGVKShouldReconcile",
			object: &metav1.PartialObjectMetadataList{
				Items: []metav1.PartialObjectMetadata{
					{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Pod",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test",
							Namespace: "default",
						},
					},
				},
			},
		},
		{
			name: "WithoutKindShouldStillReconcile",
			object: &metav1.PartialObjectMetadataList{
				Items: []metav1.PartialObjectMetadata{
					{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							// Missing Kind
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test",
							Namespace: "default",
						},
					},
				},
			},
		},
		{
			name: "WithoutVersionShouldStillReconcile",
			object: &metav1.PartialObjectMetadataList{
				Items: []metav1.PartialObjectMetadata{
					{
						TypeMeta: metav1.TypeMeta{
							Kind: "Pod",
							// Missing APIVersion
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test",
							Namespace: "default",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			client := fake.NewSimpleClientset()
			ch := make(chan struct{})

			reconciler := func(_ context.Context, _ Delta[*metav1.PartialObjectMetadata]) error {
				defer close(ch)
				return nil
			}
			controller, err := NewController(reconciler, WithName("test"))
			s.Require().NoError(err)

			// Create a mock retriever that returns PartialObjectMetadata
			retriever := &ListerWatcher{
				ListFunc: func(_ context.Context, _ metav1.ListOptions) (runtime.Object, error) {
					return tc.object, nil
				},
				WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
					return client.CoreV1().Pods("default").Watch(ctx, options)
				},
			}

			err = controller.AddRetriever(retriever, nil, nil)
			s.Require().NoError(err)

			// Start the controller
			err = controller.Start()
			s.Require().NoError(err)

			select {
			case <-ch:
			case <-time.After(ReconcileTimeout):
				s.Fail("reconciler did not run")
			}

			// Stop the controller
			err = controller.Stop()
			s.Require().NoError(err)
		})
	}
}

func (s *ControllerTestSuite) TestGetName() {
	reconciler := func(_ context.Context, _ Delta[*corev1.Pod]) error { return nil }
	controller, err := NewController(reconciler, WithName("test"))
	s.Require().NoError(err)
	s.Require().Equal("controller/test", controller.GetName())
}

func (s *ControllerTestSuite) TestControllerIdempotency() {
	reconciler := func(_ context.Context, _ Delta[*corev1.Pod]) error { return nil }

	controller, err := NewController(reconciler, WithName("test"))
	s.Require().NoError(err)

	// Test Start
	err = controller.Start()
	s.Require().NoError(err)

	// Start again should not error (idempotent)
	err = controller.Start()
	s.Require().NoError(err)

	// Test Stop
	err = controller.Stop()
	s.Require().NoError(err)

	// Stop again should not error (idempotent)
	err = controller.Stop()
	s.Require().NoError(err)
}

func (s *ControllerTestSuite) TestControllerNeedsLeaderElection() {
	type testCase struct {
		name     string
		opts     []OptionFunc
		expected bool
	}

	testCases := []testCase{
		{
			name:     "WithNoOptionShouldReturnFalse",
			opts:     []OptionFunc{WithName("test")},
			expected: false,
		},
		{
			name:     "WithNeedsLeaderElectionTrueShouldReturnTrue",
			opts:     []OptionFunc{WithName("test"), WithNeedsLeaderElection(true)},
			expected: true,
		},

		{
			name:     "WithNeedsLeaderElectionFalseShouldReturnFalse",
			opts:     []OptionFunc{WithName("test"), WithNeedsLeaderElection(false)},
			expected: false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			controller, err := NewController(
				func(_ context.Context, _ Delta[*corev1.Pod]) error { return nil },
				tc.opts...)
			s.Require().NoError(err)
			s.Require().
				Equal(tc.expected, controller.(component.LeaderElectionAware).NeedsLeaderElection())
		})
	}
}

func (s *ControllerTestSuite) TestControllerWithFiltering() {
	type testCase struct {
		name           string
		filters        []FilterFunc[*corev1.Pod]
		expectedEvents int
		description    string
	}

	testCases := []testCase{
		{
			name:           "NoFiltersShouldProcessAllEvents",
			filters:        nil,
			expectedEvents: 1,
			description:    "When no filters are provided, all events should be processed",
		},
		{
			name: "FilterByNamespaceShouldProcessMatchingEvents",
			filters: []FilterFunc[*corev1.Pod]{
				func(delta Delta[*corev1.Pod]) bool {
					return delta.Object.Namespace == defaultNS
				},
			},
			expectedEvents: 1,
			description:    "Only events for resources in the default namespace should be processed",
		},
		{
			name: "FilterByNamespaceShouldRejectNonMatchingEvents",
			filters: []FilterFunc[*corev1.Pod]{
				func(delta Delta[*corev1.Pod]) bool {
					return delta.Object.Namespace == "other-namespace"
				},
			},
			expectedEvents: 0,
			description:    "Events for resources not in the specified namespace should be rejected",
		},
		{
			name: "MultipleFiltersShouldAllPass",
			filters: []FilterFunc[*corev1.Pod]{
				func(delta Delta[*corev1.Pod]) bool {
					return delta.Object.Namespace == defaultNS
				},
				func(delta Delta[*corev1.Pod]) bool {
					return delta.Object.Name == "test"
				},
			},
			expectedEvents: 1,
			description:    "Events should only be processed if all filters pass",
		},
		{
			name: "MultipleFiltersShouldRejectIfAnyFails",
			filters: []FilterFunc[*corev1.Pod]{
				func(delta Delta[*corev1.Pod]) bool {
					return delta.Object.Namespace == defaultNS
				},
				func(delta Delta[*corev1.Pod]) bool {
					return delta.Object.Name == "non-existent"
				},
			},
			expectedEvents: 0,
			description:    "Events should be rejected if any filter fails",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			client := fake.NewSimpleClientset()

			// Create a test pod
			_, err := client.CoreV1().Pods("default").Create(context.Background(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			}, metav1.CreateOptions{})
			s.Require().NoError(err)

			eventsProcessed := 0
			ch := make(chan struct{})

			reconciler := func(_ context.Context, _ Delta[*corev1.Pod]) error {
				eventsProcessed++
				defer close(ch)
				return nil
			}

			controller, err := NewController(reconciler, WithName("test"))
			s.Require().NoError(err)

			// Create a mock retriever
			retriever := &ListerWatcher{
				ListFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
					return client.CoreV1().Pods("default").List(ctx, options)
				},
				WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
					return client.CoreV1().Pods("default").Watch(ctx, options)
				},
			}

			err = controller.AddRetriever(retriever, tc.filters, nil)
			s.Require().NoError(err)

			// Start the controller
			err = controller.Start()
			s.Require().NoError(err)

			// Wait for reconciliation or timeout
			select {
			case <-ch:
			case <-time.After(ReconcileTimeout):
				// If no events were expected, this is fine
				if tc.expectedEvents == 0 {
					s.Require().Equal(0, eventsProcessed, tc.description)
				} else {
					s.Fail("reconciler did not run within timeout")
				}
			}

			s.Require().Equal(tc.expectedEvents, eventsProcessed, tc.description)

			// Stop the controller
			err = controller.Stop()
			s.Require().NoError(err)
		})
	}
}

func (s *ControllerTestSuite) TestControllerWithTransformation() {
	type testCase struct {
		name           string
		transformers   []TransformerFunc[*corev1.Pod]
		expectedName   string
		expectedLabels map[string]string
		description    string
	}

	testCases := []testCase{
		{
			name:           "NoTransformersShouldKeepOriginalObject",
			transformers:   nil,
			expectedName:   "test",
			expectedLabels: nil,
			description:    "When no transformers are provided, objects should remain unchanged",
		},
		{
			name: "SingleTransformerShouldModifyObject",
			transformers: []TransformerFunc[*corev1.Pod]{
				func(delta Delta[*corev1.Pod]) Delta[*corev1.Pod] {
					delta.Object.Name = "transformed"
					return delta
				},
			},
			expectedName:   "transformed",
			expectedLabels: nil,
			description:    "Single transformer should modify the object as expected",
		},
		{
			name: "MultipleTransformersShouldApplyInOrder",
			transformers: []TransformerFunc[*corev1.Pod]{
				func(delta Delta[*corev1.Pod]) Delta[*corev1.Pod] {
					delta.Object.Name = "first-transform"
					return delta
				},
				func(delta Delta[*corev1.Pod]) Delta[*corev1.Pod] {
					delta.Object.Name = "second-transform"
					return delta
				},
			},
			expectedName:   "second-transform",
			expectedLabels: nil,
			description:    "Multiple transformers should be applied in sequence",
		},
		{
			name: "TransformerShouldAddLabels",
			transformers: []TransformerFunc[*corev1.Pod]{
				func(delta Delta[*corev1.Pod]) Delta[*corev1.Pod] {
					if delta.Object.Labels == nil {
						delta.Object.Labels = make(map[string]string)
					}
					delta.Object.Labels["transformed"] = "true"
					delta.Object.Labels["test-label"] = "test-value"
					return delta
				},
			},
			expectedName: "test",
			expectedLabels: map[string]string{
				"transformed": "true",
				"test-label":  "test-value",
			},
			description: "Transformer should be able to add labels to objects",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			client := fake.NewSimpleClientset()

			// Create a test pod
			_, err := client.CoreV1().Pods("default").Create(context.Background(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			}, metav1.CreateOptions{})
			s.Require().NoError(err)

			var processedObject *corev1.Pod
			ch := make(chan struct{})

			reconciler := func(_ context.Context, delta Delta[*corev1.Pod]) error {
				processedObject = delta.Object
				defer close(ch)
				return nil
			}

			controller, err := NewController(reconciler, WithName("test"))
			s.Require().NoError(err)

			// Create a mock retriever
			retriever := &ListerWatcher{
				ListFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
					return client.CoreV1().Pods("default").List(ctx, options)
				},
				WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
					return client.CoreV1().Pods("default").Watch(ctx, options)
				},
			}

			err = controller.AddRetriever(retriever, nil, tc.transformers)
			s.Require().NoError(err)

			// Start the controller
			err = controller.Start()
			s.Require().NoError(err)

			select {
			case <-ch:
			case <-time.After(ReconcileTimeout):
				s.Fail("reconciler did not run within timeout")
			}

			// Verify the transformation
			s.Require().NotNil(processedObject, "Object should have been processed")
			s.Require().Equal(tc.expectedName, processedObject.Name, tc.description)
			if tc.expectedLabels != nil {
				s.Require().Equal(tc.expectedLabels, processedObject.Labels, tc.description)
			}

			// Stop the controller
			err = controller.Stop()
			s.Require().NoError(err)
		})
	}
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}
