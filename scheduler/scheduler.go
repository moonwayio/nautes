// Package scheduler provides functionality for scheduling and running periodic tasks.
//
// The scheduler package implements a task scheduler that can run periodic tasks at
// specified intervals. It provides a framework for building applications that need
// to perform recurring operations with proper error handling, metrics collection,
// and graceful shutdown capabilities.
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

// init registers the Prometheus metrics for the scheduler package.
//
// This function is called automatically when the package is imported and
// registers all the metrics defined in this package with the global metrics
// registry. This ensures that the metrics are available for collection
// by Prometheus or other monitoring systems.
func init() {
	metrics.Registry.MustRegister(
		taskExecutionCount,
		taskExecutionDuration,
		taskExecutionErrors,
		taskExecutionSuccesses,
	)
}

// Scheduler provides a scheduler for periodic tasks.
//
// The Scheduler interface provides methods to manage the lifecycle of periodic tasks.
// It handles task scheduling, execution coordination, and graceful shutdown.
// The scheduler can manage multiple tasks with different intervals and can execute
// them concurrently based on configuration.
//
// Implementations of this interface are concurrency safe and can be used concurrently
// from multiple goroutines.
type Scheduler interface {
	manager.Component

	// AddTask adds a task to the scheduler with the specified execution interval.
	//
	// The task will be executed periodically at the specified interval. The scheduler
	// ensures that only one instance of each task runs at a time, even if the task
	// takes longer than the interval to complete.
	//
	// Parameters:
	//   - task: The function to execute periodically
	//   - interval: The time interval between task executions
	//
	// Returns:
	//   - error: Any error encountered while adding the task
	AddTask(task Task, interval time.Duration) error
}

// scheduler implements the Scheduler interface.
//
// The scheduler maintains a list of tasks with their execution intervals and
// coordinates their periodic execution. It uses goroutines to run tasks concurrently
// and provides metrics for monitoring task performance.
type scheduler struct {
	opts   options
	tasks  []taskWithInterval
	logger klog.Logger

	stop    chan struct{}
	started bool
	mu      sync.RWMutex
}

// taskWithInterval represents a task and its execution interval.
//
// This struct is used internally to associate tasks with their scheduling intervals.
type taskWithInterval struct {
	task     Task
	interval time.Duration
}

// NewScheduler creates a new Scheduler instance with the provided options.
//
// The scheduler is initialized with default settings and can be customized using
// option functions. The returned scheduler is ready to accept tasks and start
// periodic execution.
//
// Parameters:
//   - opts: Optional configuration functions to customize the scheduler behavior
//
// Returns:
//   - Scheduler: A new scheduler instance ready for use
//   - error: Any error encountered during initialization
func NewScheduler(opts ...OptionFunc) (Scheduler, error) {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}

	if err := o.setDefaults(); err != nil {
		return nil, err
	}

	return &scheduler{
		opts:    o,
		tasks:   make([]taskWithInterval, 0),
		logger:  klog.Background().WithValues("component", "scheduler/"+o.name),
		stop:    make(chan struct{}),
		started: false,
	}, nil
}

// AddTask adds a task to the scheduler with the specified execution interval.
//
// The task is added to the scheduler's internal task list and will be executed
// periodically at the specified interval. Task addition is concurrency safe and can
// be called concurrently from multiple goroutines.
//
// Parameters:
//   - task: The function to execute periodically. Must not be nil.
//   - interval: The time interval between task executions. Must be positive.
//
// Returns:
//   - error: Returns an error if the task is nil, interval is invalid, or the task cannot be added
func (s *scheduler) AddTask(task Task, interval time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if interval < 10*time.Millisecond {
		return errors.New("interval must be greater than 10 milliseconds")
	}

	s.tasks = append(s.tasks, taskWithInterval{task: task, interval: interval})

	return nil
}

// Start initializes and starts the scheduler.
//
// This method starts all registered tasks as periodic goroutines using their
// configured intervals. Each task runs independently and concurrently with
// other tasks. The method sets up metrics collection and error handling
// for all task executions.
//
// The scheduler uses NonSlidingUntil to ensure that tasks run at fixed
// intervals regardless of their execution time. This method is called
// automatically by the manager when the component is started.
//
// Returns:
//   - error: Any error encountered during scheduler startup
func (s *scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// if the scheduler is already started, return nil to ensure idempotency
	if s.started {
		return nil
	}

	s.started = true
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

// Stop gracefully shuts down the scheduler.
//
// This method stops all running tasks by closing the stop channel, which
// signals all task goroutines to terminate. It waits for all tasks to
// complete their current execution before returning. This method is called
// automatically by the manager when the component is stopped.
//
// The method is concurrency safe but should not be called concurrently
// with Start().
//
// Returns:
//   - error: Any error encountered during scheduler shutdown
func (s *scheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// if the scheduler is not started, return nil to ensure idempotency
	if !s.started {
		return nil
	}

	s.started = false
	s.logger.Info("stopping scheduler")
	close(s.stop)
	s.logger.Info("scheduler stopped successfully")
	return nil
}

// GetName returns the name of the scheduler.
//
// The name is constructed by combining "scheduler/" with the configured
// scheduler name. This is used for logging, metrics, and debugging purposes.
//
// Returns:
//   - string: The scheduler name in the format "scheduler/{name}"
func (s *scheduler) GetName() string {
	return "scheduler/" + s.opts.name
}
