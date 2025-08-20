package manager

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type ManagerTestSuite struct {
	suite.Suite
}

var ManagerTimeout = 200 * time.Millisecond

// mockComponent implements Component for testing.
type fakeComponent struct {
	name  string
	start func() error
	stop  func() error
}

func (m *fakeComponent) Start() error {
	if m.start != nil {
		return m.start()
	}
	return nil
}

func (m *fakeComponent) Stop() error {
	if m.stop != nil {
		return m.stop()
	}
	return nil
}

func (m *fakeComponent) GetName() string {
	return m.name
}

func (s *ManagerTestSuite) TestNewManager() {
	type testCase struct {
		name string
		opts []OptionFunc
		err  string
	}

	testCases := []testCase{
		{
			name: "with no options should succeed",
			opts: []OptionFunc{},
			err:  "",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			manager, err := NewManager(tc.opts...)

			if tc.err != "" {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.err)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(manager)
			}
		})
	}
}

func (s *ManagerTestSuite) TestRegister() {
	type testCase struct {
		name      string
		component Component
		err       string
	}

	testCases := []testCase{
		{
			name: "registering component should succeed",
			component: &fakeComponent{
				name: "test-component",
			},
			err: "",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			manager, err := NewManager()
			s.Require().NoError(err)

			err = manager.Register(tc.component)
			if tc.err != "" {
				s.Require().Error(err, tc.err)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

func (s *ManagerTestSuite) TestStartAndStopManager() {
	type testCase struct {
		name       string
		components []Component
		startError error
		stopError  error
	}

	testCases := []testCase{
		{
			name: "start and stop with healthy components should succeed",
			components: []Component{
				&fakeComponent{
					name: "healthy-component-1",
				},
				&fakeComponent{
					name: "healthy-component-2",
				},
			},
			startError: nil,
			stopError:  nil,
		},
		{
			name: "start with component that fails should return error",
			components: []Component{
				&fakeComponent{
					name: "failing-component",
					start: func() error {
						return errors.New("start failed")
					},
				},
			},
			startError: errors.New("start failed"),
			stopError:  nil,
		},
		{
			name: "stop with component that fails should still complete",
			components: []Component{
				&fakeComponent{
					name: "component-with-stop-error",
					stop: func() error {
						return errors.New("stop failed")
					},
				},
			},
			startError: nil,
			stopError:  errors.New("stop failed"),
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			manager, err := NewManager()
			s.Require().NoError(err)

			// Register components
			for _, component := range tc.components {
				err = manager.Register(component)
				s.Require().NoError(err)
			}

			// Start the manager
			err = manager.Start()
			if tc.startError != nil {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.startError.Error())
				return
			}
			s.Require().NoError(err)

			// Stop the manager
			err = manager.Stop()
			if tc.stopError != nil {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.stopError.Error())
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

func (s *ManagerTestSuite) TestManagerWithComponentStartOrder() {
	manager, err := NewManager()
	s.Require().NoError(err)

	startOrder := make([]string, 0)

	// Register components that track start order
	component1 := &fakeComponent{
		name: "component-1",
		start: func() error {
			startOrder = append(startOrder, "component-1")
			return nil
		},
	}

	component2 := &fakeComponent{
		name: "component-2",
		start: func() error {
			startOrder = append(startOrder, "component-2")
			return nil
		},
	}

	err = manager.Register(component1)
	s.Require().NoError(err)

	err = manager.Register(component2)
	s.Require().NoError(err)

	// Start the manager
	err = manager.Start()
	s.Require().NoError(err)

	// Verify start order (should be in registration order)
	s.Len(startOrder, 2)
	s.Equal("component-1", startOrder[0])
	s.Equal("component-2", startOrder[1])

	// Stop the manager
	err = manager.Stop()
	s.Require().NoError(err)
}

func TestManagerTestSuite(t *testing.T) {
	suite.Run(t, new(ManagerTestSuite))
}
