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

package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/moonwayio/nautes/component"
)

type SchedulerTestSuite struct {
	suite.Suite
}

var TaskTimeout = 1 * time.Second

func (s *SchedulerTestSuite) TestNewScheduler() {
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
			name: "WithNameShouldSucceed",
			opts: []OptionFunc{
				WithName("test"),
			},
			err: "",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			scheduler, err := NewScheduler(tc.opts...)

			if tc.err != "" {
				s.Require().Error(err, tc.err)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(scheduler)
			}
		})
	}
}

func (s *SchedulerTestSuite) TestAddTask() {
	type testCase struct {
		name     string
		task     Task
		duration time.Duration
		err      string
	}

	testCases := []testCase{
		{
			name: "WithTaskAndDurationShouldSucceed",
			task: NewTask(func(_ context.Context) error {
				return nil
			}),
			duration: 1 * time.Second,
			err:      "",
		},
		{
			name: "WithTaskAndDurationLessThan1SecondShouldReturnError",
			task: NewTask(func(_ context.Context) error {
				return nil
			}),
			duration: 5 * time.Millisecond,
			err:      "interval must be greater than 10 milliseconds",
		},
	}

	scheduler, err := NewScheduler(WithName("test"))
	s.Require().NoError(err)

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			err := scheduler.AddTask(tc.task, tc.duration)
			if tc.err != "" {
				s.Require().Error(err, tc.err)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

func (s *SchedulerTestSuite) TestStartAndStopScheduler() {
	type testCase struct {
		name string
		err  error
	}

	testCases := []testCase{
		{
			name: "NormalTaskExecutionShouldSucceed",
			err:  nil,
		},
		{
			name: "TaskExecutionWithErrorShouldStillComplete",
			err:  errors.New("test error"),
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			ch := make(chan struct{})

			scheduler, err := NewScheduler(WithName("test"))
			s.Require().NoError(err)

			// Wrap the task to signal completion
			wrappedTask := NewTask(func(_ context.Context) error {
				defer close(ch)
				return tc.err
			})

			err = scheduler.AddTask(wrappedTask, 10*time.Millisecond)
			s.Require().NoError(err)

			err = scheduler.Start()
			s.Require().NoError(err)

			select {
			case <-ch:
			case <-time.After(TaskTimeout):
				s.Fail("task did not run")
			}

			err = scheduler.Stop()
			s.Require().NoError(err)
		})
	}
}

func (s *SchedulerTestSuite) TestSchedulerIdempotency() {
	scheduler, err := NewScheduler(WithName("test"))
	s.Require().NoError(err)

	// Test Start
	err = scheduler.Start()
	s.Require().NoError(err)

	// Start again should not error (idempotent)
	err = scheduler.Start()
	s.Require().NoError(err)

	// Test Stop
	err = scheduler.Stop()
	s.Require().NoError(err)

	// Stop again should not error (idempotent)
	err = scheduler.Stop()
	s.Require().NoError(err)
}

func (s *SchedulerTestSuite) TestGetName() {
	scheduler, err := NewScheduler(WithName("test"))
	s.Require().NoError(err)
	s.Require().Equal("scheduler/test", scheduler.GetName())
}

func (s *SchedulerTestSuite) TestSchedulerNeedsLeaderElection() {
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
			scheduler, err := NewScheduler(tc.opts...)
			s.Require().NoError(err)
			s.Require().
				Equal(tc.expected, scheduler.(component.LeaderElectionAware).NeedsLeaderElection())
		})
	}
}

func TestSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(SchedulerTestSuite))
}
