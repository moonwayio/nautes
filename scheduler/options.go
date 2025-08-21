package scheduler

import "errors"

// options holds the configuration parameters for the scheduler.
//
// The options struct contains all configurable settings that determine how
// the scheduler behaves. It uses the functional options pattern to allow
// flexible configuration while maintaining backward compatibility.
type options struct {
	// name is the identifier for the scheduler instance
	//
	// The name is used for logging, metrics, and debugging purposes.
	// It should be descriptive and unique within the application.
	name string

	// needsLeaderElection indicates if the scheduler needs leader election
	//
	// When set to true, the scheduler will only start when the leader election
	// is active. If set to false, the scheduler will start immediately when the
	// manager starts.
	needsLeaderElection bool
}

// OptionFunc is a function that configures scheduler options.
//
// OptionFunc is a functional option pattern that allows for flexible configuration
// of scheduler instances. Each option function modifies the internal options
// struct to customize the scheduler's behavior.
type OptionFunc func(*options)

// setDefaults validates and sets default values for scheduler options.
//
// This method ensures that all required options are set and applies sensible
// defaults for optional parameters. It validates the configuration and returns
// an error if any required options are missing or invalid.
//
// Returns:
//   - error: Any error encountered during validation or default setting
func (o *options) setDefaults() error {
	if o.name == "" {
		return errors.New("name is required")
	}

	return nil
}

// WithName sets the scheduler name.
//
// The scheduler name is used for logging, metrics, and debugging purposes.
// It should be descriptive and unique within the application to help identify
// the scheduler in logs and monitoring systems.
//
// Parameters:
//   - name: The name to assign to the scheduler
//
// Returns:
//   - OptionFunc: A function that sets the scheduler name
func WithName(name string) OptionFunc {
	return func(o *options) {
		o.name = name
	}
}

// WithNeedsLeaderElection sets if the scheduler needs leader election.
//
// The leader election configuration determines whether the scheduler will
// only start when the leader election is active. If set to false, the scheduler
// will start immediately when the manager starts.
//
// Parameters:
//   - needsLeaderElection: true if the scheduler needs leader election, false otherwise
//
// Returns:
//   - OptionFunc: A function that sets the leader election configuration
func WithNeedsLeaderElection(needsLeaderElection bool) OptionFunc {
	return func(o *options) {
		o.needsLeaderElection = needsLeaderElection
	}
}
