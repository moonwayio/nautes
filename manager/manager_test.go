package manager

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/moonwayio/nautes/component"
	"github.com/moonwayio/nautes/leader"
	mock_leader "github.com/moonwayio/nautes/leader/mocks"
)

type ManagerTestSuite struct {
	suite.Suite
}

var ManagerTimeout = 200 * time.Millisecond

func (s *ManagerTestSuite) TestNewManager() {
	type testCase struct {
		name string
		opts []OptionFunc
		err  string
	}

	leader := mock_leader.NewMockLeader(s.T())
	leader.EXPECT().Subscribe(mock.Anything).Return(nil).Once()

	failingLeader := mock_leader.NewMockLeader(s.T())
	failingLeader.EXPECT().Subscribe(mock.Anything).Return(errors.New("failed to subscribe")).Once()

	testCases := []testCase{
		{
			name: "WithNoOptionsShouldSucceed",
			opts: []OptionFunc{},
			err:  "",
		},
		{
			name: "WithLeaderElectionShouldSucceed",
			opts: []OptionFunc{
				WithLeader(leader),
			},
			err: "",
		},
		{
			name: "WithLeaderElectionThatFailsToSubscribeShouldReturnError",
			opts: []OptionFunc{
				WithLeader(failingLeader),
			},
			err: "failed to subscribe",
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

	leader.AssertExpectations(s.T())
	failingLeader.AssertExpectations(s.T())
}

func (s *ManagerTestSuite) TestRegister() {
	type testCase struct {
		name         string
		component    component.Component
		err          string
		subscribeErr error
	}

	testCases := []testCase{
		{
			name:      "WithNilComponentShouldReturnError",
			component: nil,
			err:       "cannot register nil component",
		},
		{
			name: "RegisteringComponentShouldSucceed",
			component: &fakeComponent{
				name: "test-component",
			},
			err: "",
		},
		{
			name: "RegisteringLeaderElectionAwareComponentShouldSucceed",
			component: &fakeLeaderElectionAwareComponent{
				name: "test-component",
			},
			err: "",
		},
		{
			name: "RegisteringLeaderElectionAwareComponentWithNeedsLeaderElectionTrueShouldSucceed",
			component: &fakeLeaderElectionAwareComponent{
				name:                "test-component",
				needsLeaderElection: true,
			},
			err: "",
		},
		{
			name: "RegisteringElectionSubscriberShouldSucceed",
			component: &fakeLeaderElectionSubscriber{
				name: "test-component",
			},
			err: "",
		},
		{
			name: "RegisteringFailingElectionSubscriberShouldReturnError",
			component: &fakeLeaderElectionSubscriber{
				name: "test-component",
			},
			err:          "failed to subscribe component",
			subscribeErr: errors.New("failed to subscribe"),
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			manager, err := NewManager()
			s.Require().NoError(err)

			err = manager.Register(tc.component)
			if tc.err != "" && tc.subscribeErr == nil {
				s.Require().Error(err, tc.err)
				s.Contains(err.Error(), tc.err)
			} else {
				s.Require().NoError(err)
			}

			leader := mock_leader.NewMockLeader(s.T())

			leader.EXPECT().Subscribe(mock.Anything).Return(nil).Once()

			leaderManager, err := NewManager(WithLeader(leader))
			s.Require().NoError(err)

			leader.EXPECT().Subscribe(mock.Anything).Return(tc.subscribeErr).Maybe()

			err = leaderManager.Register(tc.component)
			if tc.err != "" {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.err)
			} else {
				s.Require().NoError(err)
			}

			leader.AssertExpectations(s.T())
		})
	}
}

func (s *ManagerTestSuite) TestStartAndStopManager() {
	type testCase struct {
		name       string
		components []component.Component
		startError error
		stopError  error
	}

	testCases := []testCase{
		{
			name: "StartAndStopWithHealthyComponentsShouldSucceed",
			components: []component.Component{
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
			name: "StartWithComponentThatFailsStartingShouldReturnError",
			components: []component.Component{
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
			name: "StopWithComponentThatFailsStoppingShouldStillComplete",
			components: []component.Component{
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
		{
			name: "StopWithLeaderElectionAwareComponentThatFailsStoppingShouldStillComplete",
			components: []component.Component{
				&fakeLeaderElectionAwareComponent{
					name:                "leader-election-aware-component",
					needsLeaderElection: true,
					stop: func() error {
						return errors.New("stop failed")
					},
				},
			},
			startError: nil,
			stopError:  errors.New("stop failed"),
		},
		{
			name: "StartAndStopWithLeaderElectionAwareComponentShouldSucceed",
			components: []component.Component{
				&fakeLeaderElectionAwareComponent{
					name: "leader-election-aware-component",
				},
			},
			startError: nil,
			stopError:  nil,
		},
		{
			name: "StartAndStopWithLeaderElectionAwareComponentWithNeedsLeaderElectionTrueShouldSucceed",
			components: []component.Component{
				&fakeLeaderElectionAwareComponent{
					name:                "leader-election-aware-component",
					needsLeaderElection: true,
				},
			},
			startError: nil,
			stopError:  nil,
		},
		{
			name: "StartAndStopWithElectionSubscriberShouldSucceed",
			components: []component.Component{
				&fakeLeaderElectionSubscriber{
					name: "leader-election-subscriber",
				},
			},
			startError: nil,
			stopError:  nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			leader := mock_leader.NewMockLeader(s.T())

			leader.EXPECT().Subscribe(mock.Anything).Return(nil)
			leader.EXPECT().Run(mock.Anything).Run(func(ctx context.Context) {
				<-ctx.Done()
			}).Return(nil).Maybe()

			opts := [][]OptionFunc{
				{},
				{WithLeader(leader)},
			}

			for _, opt := range opts {
				manager, err := NewManager(opt...)
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
				} else {
					s.Require().NoError(err)
				}

				// Stop the manager
				err = manager.Stop()
				if tc.stopError != nil {
					s.Require().Error(err)
					s.Contains(err.Error(), tc.stopError.Error())
				} else {
					s.Require().NoError(err)
				}
			}

			leader.AssertExpectations(s.T())
		})
	}
}

func (s *ManagerTestSuite) TestRegisteringDuplicatedComponentsShouldReturnError() {
	testCases := []struct {
		name      string
		component component.Component
	}{
		{
			name: "RegisteringDuplicatedRegularComponentsShouldReturnError",
			component: &fakeComponent{
				name: "test-component",
			},
		},
		{
			name: "RegisteringDuplicatedLeaderElectionAwareComponentsShouldReturnError",
			component: &fakeLeaderElectionAwareComponent{
				name:                "test-component",
				needsLeaderElection: true,
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			leader := mock_leader.NewMockLeader(s.T())
			leader.EXPECT().Subscribe(mock.Anything).Return(nil).Once()

			manager, err := NewManager(WithLeader(leader))
			s.Require().NoError(err)

			err = manager.Register(tc.component)
			s.Require().NoError(err)

			err = manager.Register(tc.component)
			s.Require().Error(err)
			s.Require().Contains(err.Error(), "already registered")

			leader.AssertExpectations(s.T())
		})
	}
}

func (s *ManagerTestSuite) TestDoubleStartAndDoubleStop() {
	manager, err := NewManager()
	s.Require().NoError(err)

	err = manager.Start()
	s.Require().NoError(err)

	err = manager.Start()
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "already started")

	err = manager.Stop()
	s.Require().NoError(err)

	err = manager.Stop()
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "is not started")
}

func (s *ManagerTestSuite) TestLeaderElectionFailsToRunShouldContinueRunning() {
	c := make(chan struct{})
	leader := mock_leader.NewMockLeader(s.T())
	leader.EXPECT().Subscribe(mock.Anything).Return(nil).Once()
	leader.EXPECT().Run(mock.Anything).Run(func(_ context.Context) {
		close(c)
	}).Return(errors.New("failed to run")).Maybe()

	manager, err := NewManager(WithLeader(leader))
	s.Require().NoError(err)

	err = manager.Start()
	s.Require().NoError(err)

	<-c

	err = manager.Stop()
	s.Require().NoError(err)

	leader.AssertExpectations(s.T())
}

func (s *ManagerTestSuite) TestOnStartLeading() {
	wg := sync.WaitGroup{}
	wg.Add(2)

	l := mock_leader.NewMockLeader(s.T())
	l.EXPECT().Subscribe(mock.Anything).Return(nil).Once()

	manager, err := NewManager(WithLeader(l))
	s.Require().NoError(err)

	err = manager.Register(&fakeLeaderElectionAwareComponent{
		name:                "test-component",
		needsLeaderElection: true,
		start: func() error {
			defer wg.Done()
			return nil
		},
	})
	s.Require().NoError(err)

	err = manager.Register(&fakeLeaderElectionAwareComponent{
		name:                "test-component-2",
		needsLeaderElection: true,
		start: func() error {
			defer wg.Done()
			return errors.New("start failed")
		},
	})
	s.Require().NoError(err)

	manager.(leader.ElectionSubscriber).OnStartLeading()

	wg.Wait()

	manager.(leader.ElectionSubscriber).OnStopLeading()

	l.AssertExpectations(s.T())
}

func (s *ManagerTestSuite) TestOnStopLeading() {
	wg := sync.WaitGroup{}
	wg.Add(2)

	l := mock_leader.NewMockLeader(s.T())
	l.EXPECT().Subscribe(mock.Anything).Return(nil).Once()

	manager, err := NewManager(WithLeader(l))
	s.Require().NoError(err)

	err = manager.Register(&fakeLeaderElectionAwareComponent{
		name:                "test-component",
		needsLeaderElection: true,
		stop: func() error {
			defer wg.Done()
			return nil
		},
	})
	s.Require().NoError(err)

	err = manager.Register(&fakeLeaderElectionAwareComponent{
		name:                "test-component-2",
		needsLeaderElection: true,
		stop: func() error {
			defer wg.Done()
			return errors.New("stop failed")
		},
	})
	s.Require().NoError(err)

	manager.(leader.ElectionSubscriber).OnStartLeading()
	manager.(leader.ElectionSubscriber).OnStopLeading()

	wg.Wait()

	l.AssertExpectations(s.T())
}

func TestManagerTestSuite(t *testing.T) {
	suite.Run(t, new(ManagerTestSuite))
}
