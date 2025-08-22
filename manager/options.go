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

package manager

import "github.com/moonwayio/nautes/leader"

// options contains all configurable parameters for a manager instance.
//
// The options struct holds the configuration values that determine how the
// manager behaves, including leader election configuration and component
// lifecycle management settings.
type options struct {
	// leader configures leader election for the manager
	//
	// When set, the manager will participate in leader election and only
	// start leader election-aware components when this instance becomes
	// the leader. If nil, no leader election is performed and all components
	// are treated as regular components that start immediately when the
	// manager starts.
	leader leader.Leader
}

// OptionFunc is a function that configures manager options.
//
// OptionFunc is a functional option pattern that allows for flexible configuration
// of manager instances. Each option function modifies the internal options
// struct to customize the manager's behavior.
type OptionFunc func(*options)

// setDefaults validates and sets default values for manager options.
//
// This method ensures that all required options are set and applies sensible
// defaults for optional parameters. It validates the configuration and returns
// an error if any required options are missing or invalid.
//
// Returns:
//   - error: Any error encountered during validation or default setting
func (o *options) setDefaults() error {
	// All options are currently optional with sensible defaults
	// No validation or default setting required
	return nil
}

// WithLeader configures leader election for the manager.
//
// The leader election configuration determines whether the manager will
// participate in leader election and how it handles leader election-aware
// components. When a leader instance is provided, the manager will:
//   - Subscribe to leadership events
//   - Categorize components as leader election-aware or regular based on their
//     implementation of the LeaderElectionAware interface
//   - Start leader election-aware components only when this instance becomes the leader
//   - Stop leader election-aware components when leadership is lost
//
// If this option is not provided or nil is passed, no leader election is performed
// and all components are treated as regular components that start immediately
// when the manager starts.
//
// Parameters:
//   - leader: The leader election instance to use. Can be nil to disable leader election.
//
// Returns:
//   - OptionFunc: A function that sets the leader election configuration
func WithLeader(leader leader.Leader) OptionFunc {
	return func(o *options) {
		o.leader = leader
	}
}
