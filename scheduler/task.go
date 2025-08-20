package scheduler

import (
	"context"
)

// Task represents a periodic task.
type Task interface {
	Run(ctx context.Context) error
}

// NewTask creates a new task from a function.
func NewTask(fn func(ctx context.Context) error) Task {
	return &task{fn: fn}
}

type task struct {
	fn func(ctx context.Context) error
}

// Run executes the task function.
func (t *task) Run(ctx context.Context) error {
	return t.fn(ctx)
}
