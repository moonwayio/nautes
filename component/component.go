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

// Package component provides interfaces for framework components.
//
// Components are the building blocks of the nautes framework and typically
// represent services like controllers, schedulers, webhooks, or other long-running
// processes.
package component

// Component represents a framework component that can be registered with the Manager.
//
// Components are the building blocks of the nautes framework and typically represent
// services like controllers, schedulers, webhooks, or other long-running processes.
// Each component must implement the lifecycle methods defined by this interface.
type Component interface {
	// Start initializes and starts the component.
	//
	// This method should perform any necessary initialization, start background
	// goroutines, and return once the component is ready to serve requests.
	// If the component cannot be started, it should return an error.
	//
	// The method should be idempotent - calling it multiple times should have
	// no effect after the first successful call.
	Start() error

	// Stop gracefully shuts down the component.
	//
	// This method should stop all background goroutines, close connections,
	// and perform any necessary cleanup. The method should return once the
	// component has been fully shut down.
	//
	// The method should be idempotent - calling it multiple times should have
	// no effect.
	Stop() error

	// GetName returns a unique identifier for the component.
	//
	// The name is used for logging, metrics, and debugging purposes.
	// It should be descriptive and unique within the application.
	GetName() string
}

// LeaderElectionAware indicates that a component can determine if it needs leader election.
//
// Components implementing this interface can indicate whether they require
// leader election to function properly. This allows the framework to make
// decisions about whether to start leader election for a particular component.
type LeaderElectionAware interface {
	// NeedsLeaderElection returns whether this component requires leader election.
	//
	// Returns:
	//   - bool: true if the component needs leader election, false otherwise
	NeedsLeaderElection() bool
}
