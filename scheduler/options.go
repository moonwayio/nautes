package scheduler

import "errors"

// options holds the configuration for the scheduler.
type options struct {
	name string
}

// OptionFunc is a function that configures scheduler options.
type OptionFunc func(*options)

func (o *options) setDefaults() error {
	if o.name == "" {
		return errors.New("name is required")
	}

	return nil
}

// WithName sets the scheduler name.
func WithName(name string) OptionFunc {
	return func(o *options) {
		o.name = name
	}
}
