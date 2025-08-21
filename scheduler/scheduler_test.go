package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
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
			name: "no options should return error",
			opts: []OptionFunc{},
			err:  "name is required",
		},
		{
			name: "with name should succeed",
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
			name: "with task and duration should succeed",
			task: NewTask(func(_ context.Context) error {
				return nil
			}),
			duration: 1 * time.Second,
			err:      "",
		},
		{
			name: "with task and duration less than 1 second should return error",
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
			name: "normal task execution should succeed",
			err:  nil,
		},
		{
			name: "task execution with error should still complete",
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

func TestSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(SchedulerTestSuite))
}
