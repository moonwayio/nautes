// Package scheduler provides functionality for scheduling and running periodic tasks.
package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/moonwayio/nautes/manager"
	"github.com/moonwayio/nautes/metrics"
)

var (
	taskExecutionCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "scheduler_task_execution_total",
			Help: "Total number of task executions",
		},
		[]string{"name"},
	)
	taskExecutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "scheduler_task_execution_duration_seconds",
			Help: "Duration of task executions",
		},
		[]string{"name"},
	)
	taskExecutionErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "scheduler_task_execution_errors_total",
			Help: "Total number of task execution errors",
		},
		[]string{"name"},
	)
	taskExecutionSuccesses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "scheduler_task_execution_successes_total",
			Help: "Total number of task execution successes",
		},
		[]string{"name"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		taskExecutionCount,
		taskExecutionDuration,
		taskExecutionErrors,
		taskExecutionSuccesses,
	)
}

// Scheduler provides a scheduler for periodic tasks.
type Scheduler interface {
	manager.Component
	AddTask(task Task, interval time.Duration) error
}

// scheduler implements the Scheduler interface.
type scheduler struct {
	opts   options
	tasks  []taskWithInterval
	logger klog.Logger

	stop chan struct{}
	mu   sync.RWMutex
}

type taskWithInterval struct {
	task     Task
	interval time.Duration
}

// NewScheduler creates a new Scheduler instance.
func NewScheduler(opts ...OptionFunc) (Scheduler, error) {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}

	if err := o.setDefaults(); err != nil {
		return nil, err
	}

	return &scheduler{
		opts:   o,
		tasks:  make([]taskWithInterval, 0),
		logger: klog.Background().WithValues("component", "scheduler/"+o.name),
		stop:   make(chan struct{}),
	}, nil
}

// AddTask adds a task to the scheduler.
func (s *scheduler) AddTask(task Task, interval time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if interval < 10*time.Millisecond {
		return errors.New("interval must be greater than 10 milliseconds")
	}

	s.tasks = append(s.tasks, taskWithInterval{task: task, interval: interval})

	return nil
}

// Start starts the scheduler.
func (s *scheduler) Start() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.logger.Info("starting scheduler")

	ctx := klog.NewContext(context.Background(), s.logger)

	for _, t := range s.tasks {
		go wait.NonSlidingUntil(func() {
			// increment the task execution count
			taskExecutionCount.WithLabelValues(s.opts.name).Inc()
			start := time.Now()
			if err := t.task.Run(ctx); err != nil {
				s.logger.Error(err, "task execution failed")

				// increment the task execution errors
				taskExecutionErrors.WithLabelValues(s.opts.name).Inc()
			} else {
				// increment the task execution successes
				taskExecutionSuccesses.WithLabelValues(s.opts.name).Inc()
			}

			// observe the task execution duration
			taskExecutionDuration.WithLabelValues(s.opts.name).Observe(time.Since(start).Seconds())
		}, t.interval, s.stop)
	}

	s.logger.Info("scheduler started successfully")
	return nil
}

// Stop stops the scheduler.
func (s *scheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Info("stopping scheduler")
	close(s.stop)
	s.logger.Info("scheduler stopped successfully")
	return nil
}

// GetName returns the name of the scheduler.
func (s *scheduler) GetName() string {
	return "scheduler/" + s.opts.name
}
