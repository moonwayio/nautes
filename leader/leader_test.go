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

package leader

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/moonwayio/nautes/event"
)

type LeaderTestSuite struct {
	suite.Suite
}

var LeaderTimeout = 200 * time.Millisecond

// testSubscriber implements ElectionSubscriber for testing.
type testSubscriber struct {
	callbackCalls *[]bool
	mu            *sync.Mutex
}

func (t *testSubscriber) OnStartLeading() {
	t.mu.Lock()
	defer t.mu.Unlock()
	*t.callbackCalls = append(*t.callbackCalls, true)
}

func (t *testSubscriber) OnStopLeading() {
	t.mu.Lock()
	defer t.mu.Unlock()
	*t.callbackCalls = append(*t.callbackCalls, false)
}

type testEventRecorder struct {
	event.Recorder
}

func (s *LeaderTestSuite) TestNewLeader() {
	type testCase struct {
		name string
		opts []OptionFunc
		err  string
	}

	testCases := []testCase{
		{
			name: "NoOptionsShouldReturnError",
			opts: []OptionFunc{},
			err:  "id is required",
		},
		{
			name: "WithIDOnlyShouldReturnError",
			opts: []OptionFunc{
				WithID("test-id"),
			},
			err: "name is required",
		},
		{
			name: "WithIDAndNameShouldReturnError",
			opts: []OptionFunc{
				WithID("test-id"),
				WithName("test-name"),
			},
			err: "client is required",
		},
		{
			name: "WithIDNameAndClientShouldReturnError",
			opts: []OptionFunc{
				WithID("test-id"),
				WithName("test-name"),
				WithClient(fake.NewSimpleClientset()),
			},
			err: "leaseLockName is required",
		},
		{
			name: "WithIDNameClientAndLeaseLockNameShouldReturnError",
			opts: []OptionFunc{
				WithID("test-id"),
				WithName("test-name"),
				WithClient(fake.NewSimpleClientset()),
				WithLeaseLockName("test-lease"),
			},
			err: "leaseLockNamespace is required",
		},
		{
			name: "WithAllRequiredOptionsShouldSucceed",
			opts: []OptionFunc{
				WithID("test-id"),
				WithName("test-name"),
				WithClient(fake.NewSimpleClientset()),
				WithLeaseLockName("test-lease"),
				WithLeaseLockNamespace("test-namespace"),
			},
			err: "",
		},
		{
			name: "WithAllOptionsIncludingCustomDurationsShouldSucceed",
			opts: []OptionFunc{
				WithID("test-id"),
				WithName("test-name"),
				WithClient(fake.NewSimpleClientset()),
				WithLeaseLockName("test-lease"),
				WithLeaseLockNamespace("test-namespace"),
				WithLeaseDuration(30 * time.Second),
				WithRenewDeadline(20 * time.Second),
				WithRetryPeriod(5 * time.Second),
				WithReleaseOnCancel(true),
				WithEventRecorder(&testEventRecorder{}),
			},
			err: "",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			leader, err := NewLeader(tc.opts...)

			if tc.err != "" {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.err)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(leader)
			}
		})
	}
}

func (s *LeaderTestSuite) TestSubscribe() {
	type testCase struct {
		name       string
		subscriber ElectionSubscriber
		err        string
	}

	testCases := []testCase{
		{
			name: "RegisterSubscriberBeforeRunningShouldSucceed",
			subscriber: &testSubscriber{
				callbackCalls: &[]bool{},
				mu:            &sync.Mutex{},
			},
			err: "",
		},
		{
			name:       "RegisterNilSubscriberShouldReturnError",
			subscriber: nil,
			err:        "",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			leader, err := NewLeader(
				WithID("test-id"),
				WithName("test-name"),
				WithClient(fake.NewSimpleClientset()),
				WithLeaseLockName("test-lease"),
				WithLeaseLockNamespace("test-namespace"),
			)
			s.Require().NoError(err)

			err = leader.Subscribe(tc.subscriber)
			if tc.err != "" {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.err)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

func (s *LeaderTestSuite) TestSubscribeAfterRunning() {
	leader, err := NewLeader(
		WithID("test-id"),
		WithName("test-name"),
		WithClient(fake.NewSimpleClientset()),
		WithLeaseLockName("test-lease"),
		WithLeaseLockNamespace("test-namespace"),
	)
	s.Require().NoError(err)

	// Start the leader election in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = leader.Run(ctx)
	}()

	// Wait a bit for the leader to start
	time.Sleep(50 * time.Millisecond)

	// Try to register a subscriber after running
	err = leader.Subscribe(&testSubscriber{
		callbackCalls: &[]bool{},
		mu:            &sync.Mutex{},
	})
	s.Require().Error(err)
	s.Equal(ErrAlreadyRunning, err)
}

func (s *LeaderTestSuite) TestIsLeader() {
	leader, err := NewLeader(
		WithID("test-id"),
		WithName("test-name"),
		WithClient(fake.NewSimpleClientset()),
		WithLeaseLockName("test-lease"),
		WithLeaseLockNamespace("test-namespace"),
	)
	s.Require().NoError(err)

	// Initially should not be leader
	isLeader := leader.IsLeader()
	s.False(isLeader)
}

func (s *LeaderTestSuite) TestRunTwice() {
	leader, err := NewLeader(
		WithID("test-id"),
		WithName("test-name"),
		WithClient(fake.NewSimpleClientset()),
		WithLeaseLockName("test-lease"),
		WithLeaseLockNamespace("test-namespace"),
	)
	s.Require().NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the leader election in a goroutine
	go func() {
		_ = leader.Run(ctx)
	}()

	// Wait a bit for the leader to start
	time.Sleep(50 * time.Millisecond)

	// Try to run again
	err = leader.Run(ctx)
	s.Require().Error(err)
	s.Equal(ErrAlreadyRunning, err)
}

func (s *LeaderTestSuite) TestRunWithCallbacks() {
	var callbackCalls []bool
	var mu sync.Mutex

	leader, err := NewLeader(
		WithID("test-id"),
		WithName("test-name"),
		WithClient(fake.NewSimpleClientset()),
		WithLeaseLockName("test-lease"),
		WithLeaseLockNamespace("test-namespace"),
		WithLeaseDuration(100*time.Millisecond),
		WithRenewDeadline(50*time.Millisecond),
		WithRetryPeriod(10*time.Millisecond),
	)
	s.Require().NoError(err)

	// Register subscriber
	subscriber := &testSubscriber{
		callbackCalls: &callbackCalls,
		mu:            &mu,
	}
	err = leader.Subscribe(subscriber)
	s.Require().NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), LeaderTimeout)
	defer cancel()

	// Run the leader election
	err = leader.Run(ctx)
	s.Require().NoError(err)

	// Verify that callbacks were called
	mu.Lock()
	defer mu.Unlock()
	s.NotEmpty(callbackCalls) // Verify that callbacks were called
}

func (s *LeaderTestSuite) TestRunWithContextCancellation() {
	leader, err := NewLeader(
		WithID("test-id"),
		WithName("test-name"),
		WithClient(fake.NewSimpleClientset()),
		WithLeaseLockName("test-lease"),
		WithLeaseLockNamespace("test-namespace"),
		WithLeaseDuration(100*time.Millisecond),
		WithRenewDeadline(50*time.Millisecond),
		WithRetryPeriod(10*time.Millisecond),
	)
	s.Require().NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run the leader election
	err = leader.Run(ctx)
	s.Require().NoError(err)

	// Verify that the leader is no longer running
	isLeader := leader.IsLeader()
	s.False(isLeader)
}

func (s *LeaderTestSuite) TestRunWithInvalidRenewDeadline() {
	leader, err := NewLeader(
		WithID("test-id"),
		WithName("test-name"),
		WithClient(fake.NewSimpleClientset()),
		WithLeaseLockName("test-lease"),
		WithLeaseLockNamespace("test-namespace"),
		WithLeaseDuration(100*time.Millisecond),
		WithRenewDeadline(-1*time.Millisecond), // Invalid: negative renew deadline
		WithRetryPeriod(10*time.Millisecond),
	)
	s.Require().NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run the leader election should fail due to invalid renew deadline
	err = leader.Run(ctx)
	s.Require().Error(err)
	s.Contains(err.Error(), "renewDeadline must be greater than retryPeriod")
}

func TestLeaderTestSuite(t *testing.T) {
	suite.Run(t, new(LeaderTestSuite))
}
