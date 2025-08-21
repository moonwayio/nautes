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
)

type ControllerTestSuite struct {
	suite.Suite
}

var ReconcileTimeout = 200 * time.Millisecond

func (s *ControllerTestSuite) TestNewController() {
	type testCase struct {
		name       string
		reconciler Reconciler
		opts       []OptionFunc
		err        string
	}

	testCases := []testCase{
		{
			name:       "no name should return error",
			reconciler: func(_ context.Context, _ runtime.Object) error { return nil },
			opts:       []OptionFunc{},
			err:        "name is required",
		},
		{
			name:       "with name should succeed",
			reconciler: func(_ context.Context, _ runtime.Object) error { return nil },
			opts: []OptionFunc{
				WithName("test"),
			},
			err: "",
		},
		{
			name:       "with all options should succeed",
			reconciler: func(_ context.Context, _ runtime.Object) error { return nil },
			opts: []OptionFunc{
				WithName("test"),
				WithResyncInterval(30 * time.Second),
				WithConcurrency(5),
				WithScheme(runtime.NewScheme()),
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

	reconciler := func(_ context.Context, _ runtime.Object) error { return nil }
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

	err = controller.AddRetriever(retriever)
	s.Require().NoError(err)
}

func (s *ControllerTestSuite) TestStartAndStopController() {
	type testCase struct {
		name            string
		reconciler      Reconciler
		expectReconcile bool
		expectError     bool
	}

	testCases := []testCase{
		{
			name: "normal reconciliation should succeed",
			reconciler: func(_ context.Context, _ runtime.Object) error {
				return nil
			},
			expectReconcile: true,
			expectError:     false,
		},
		{
			name: "reconciliation with error should still complete",
			reconciler: func(_ context.Context, _ runtime.Object) error {
				return errors.New("test error")
			},
			expectReconcile: true,
			expectError:     true,
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

			// Wrap the reconciler to signal completion
			wrappedReconciler := func(ctx context.Context, obj runtime.Object) error {
				defer close(ch)
				return tc.reconciler(ctx, obj)
			}

			controller, err := NewController(
				wrappedReconciler,
				WithName("test"),
				WithClient(client),
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

			err = controller.AddRetriever(retriever)
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
			name: "with custom scheme should work",
			opts: []OptionFunc{
				WithName("test"),
				WithScheme(runtime.NewScheme()),
			},
		},
		{
			name: "with resync interval should work",
			opts: []OptionFunc{
				WithName("test"),
				WithResyncInterval(1 * time.Second),
			},
		},
		{
			name: "with high concurrency should work",
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

			reconciler := func(_ context.Context, _ runtime.Object) error {
				defer close(ch)
				return nil
			}

			controller, err := NewController(reconciler, append(tc.opts, WithClient(client))...)
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

			err = controller.AddRetriever(retriever)
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
			name: "with proper GVK should reconcile",
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
			name: "without Kind should still reconcile",
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
			name: "without Version should still reconcile",
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

			reconciler := func(_ context.Context, _ runtime.Object) error {
				defer close(ch)
				return nil
			}
			controller, err := NewController(reconciler, WithName("test"), WithClient(client))
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

			err = controller.AddRetriever(retriever)
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
	reconciler := func(_ context.Context, _ runtime.Object) error { return nil }
	controller, err := NewController(reconciler, WithName("test"))
	s.Require().NoError(err)
	s.Require().Equal("controller/test", controller.GetName())
}

func (s *ControllerTestSuite) TestControllerIdempotency() {
	client := fake.NewSimpleClientset()
	reconciler := func(_ context.Context, _ runtime.Object) error { return nil }

	controller, err := NewController(reconciler, WithName("test"), WithClient(client))
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

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}
