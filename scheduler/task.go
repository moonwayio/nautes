package scheduler

import (
	"context"
)

// Task represents a periodic task that can be executed by the scheduler.
//
// The Task interface defines a simple contract for tasks that need to be executed
// periodically. Tasks are typically used for maintenance operations, health checks,
// or other recurring work that needs to be performed at regular intervals.
//
// Implementations of this interface must be concurrency safe and should handle
// their own error conditions appropriately.
type Task interface {
	// Run executes the task with the provided context.
	//
	// The method performs the actual work of the task. The context may be cancelled
	// to signal that the task should stop execution. The method should return
	// promptly when the context is cancelled.
	//
	// Parameters:
	//   - ctx: Context for the task execution, may be cancelled
	//
	// Returns:
	//   - error: Any error encountered during task execution
	Run(ctx context.Context) error
}

// NewTask creates a new task from a function.
//
// This function provides a convenient way to create Task implementations from
// simple functions. The returned task will delegate its Run method to the
// provided function.
//
// Parameters:
//   - fn: The function to execute when the task runs
//
// Returns:
//   - Task: A new task that executes the provided function
func NewTask(fn func(ctx context.Context) error) Task {
	return &task{fn: fn}
}

// task implements the Task interface using a function callback.
//
// This struct wraps a function to provide a Task implementation. It's used
// internally by NewTask to create task instances from simple functions.
type task struct {
	// fn is the function to execute when the task runs
	fn func(ctx context.Context) error
}

// Run executes the task function with the provided context.
//
// This method delegates execution to the configured function, providing
// a consistent interface for task execution.
//
// Parameters:
//   - ctx: Context for the task execution, may be cancelled
//
// Returns:
//   - error: Any error returned by the task function
func (t *task) Run(ctx context.Context) error {
	return t.fn(ctx)
}
