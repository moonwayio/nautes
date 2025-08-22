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

package leader

import (
	"errors"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/moonwayio/nautes/event"
)

// options contains all configurable parameters for a leader elector instance.
//
// The options struct holds the configuration values that determine how the
// leader elector behaves, including Kubernetes client configuration, lease
// lock settings, timing parameters, and event recording configuration.
type options struct {
	// id is the unique identifier for this leader elector instance.
	//
	// This identifier is used in the lease lock to distinguish between
	// different instances participating in leader election.
	id string

	// name is the name of the leader elector for logging and metrics.
	//
	// This name is used in log messages and metrics to identify the
	// specific leader elector instance.
	name string

	// client is the Kubernetes client used for lease operations.
	//
	// This client is used to create and manage the lease objects that
	// implement the leader election mechanism.
	client kubernetes.Interface

	// leaseLockName is the name of the lease object used for leader election.
	//
	// This name identifies the specific lease object in the Kubernetes
	// cluster that will be used for leader election coordination.
	leaseLockName string

	// leaseLockNamespace is the namespace where the lease object is created.
	//
	// This namespace determines where the lease object will be created
	// and managed in the Kubernetes cluster.
	leaseLockNamespace string

	// leaseDuration is the duration that non-leader candidates will wait
	// after observing a leadership renewal until attempting to acquire
	// leadership of a led but unrenewed leader slot.
	//
	// This is effectively the maximum duration that a leader can be stopped
	// before it is replaced by another candidate.
	leaseDuration time.Duration

	// renewDeadline is the interval between attempts by the acting master to
	// renew a leadership slot before it stops leading.
	//
	// This must be less than or equal to the lease duration.
	renewDeadline time.Duration

	// retryPeriod is the duration the clients should wait between attempts
	// of acquiring a leadership lease.
	//
	// This is the interval between attempts to acquire leadership when
	// not currently the leader.
	retryPeriod time.Duration

	// releaseOnCancel determines whether the lease should be released when
	// the context is cancelled.
	//
	// When true, the lease will be released immediately when the context
	// is cancelled, allowing other candidates to acquire leadership faster.
	releaseOnCancel *bool

	// eventRecorder is used to record events related to leader election.
	//
	// This recorder is used to log events about leadership changes and
	// can be used for monitoring and debugging purposes.
	eventRecorder event.Recorder
}

// OptionFunc is a function that configures leader elector options.
//
// OptionFunc is a functional option pattern that allows for flexible configuration
// of leader elector instances. Each option function modifies the internal options
// struct to customize the leader elector's behavior.
type OptionFunc func(*options)

// setDefaults validates and sets default values for leader elector options.
//
// This method ensures that all required options are set and applies sensible
// defaults for optional parameters. It validates the configuration and returns
// an error if any required options are missing or invalid.
//
// Returns:
//   - error: Any error encountered during validation or default setting
func (o *options) setDefaults() error {
	if o.id == "" {
		return errors.New("id is required")
	}

	if o.name == "" {
		return errors.New("name is required")
	}

	if o.client == nil {
		return errors.New("client is required")
	}

	if o.leaseLockName == "" {
		return errors.New("leaseLockName is required")
	}

	if o.leaseLockNamespace == "" {
		return errors.New("leaseLockNamespace is required")
	}

	if o.leaseDuration == 0 {
		o.leaseDuration = 15 * time.Second
	}

	if o.renewDeadline == 0 {
		o.renewDeadline = 10 * time.Second
	}

	if o.retryPeriod == 0 {
		o.retryPeriod = 2 * time.Second
	}

	if o.releaseOnCancel == nil {
		releaseOnCancel := true
		o.releaseOnCancel = &releaseOnCancel
	}

	return nil
}

// WithID sets the unique identifier for the leader elector.
//
// The unique identifier is used to distinguish this instance from other
// instances participating in the same leader election process. It should
// be unique across all instances of the same application.
//
// Parameters:
//   - id: The unique identifier for this leader elector instance
//
// Returns:
//   - OptionFunc: A function that sets the unique identifier
func WithID(id string) OptionFunc {
	return func(o *options) {
		o.id = id
	}
}

// WithName sets the name for the leader elector.
//
// The leader elector name is used for logging, metrics, and debugging purposes.
// It should be descriptive and unique within the application to help identify
// the leader elector in logs and monitoring systems.
//
// Parameters:
//   - name: The name to assign to the leader elector
//
// Returns:
//   - OptionFunc: A function that sets the leader elector name
func WithName(name string) OptionFunc {
	return func(o *options) {
		o.name = name
	}
}

// WithClient sets the Kubernetes client for the leader elector.
//
// The Kubernetes client is used for all API operations performed by the
// leader elector, including creating and managing lease objects for leader
// election. The client should be properly configured with authentication
// and authorization.
//
// Parameters:
//   - client: The Kubernetes client interface to use
//
// Returns:
//   - OptionFunc: A function that sets the Kubernetes client
func WithClient(client kubernetes.Interface) OptionFunc {
	return func(o *options) {
		o.client = client
	}
}

// WithLeaseLockName sets the name of the lease object.
//
// The lease lock name identifies the specific lease object in the Kubernetes
// cluster that will be used for leader election coordination. Multiple
// instances of the same application should use the same lease lock name
// to participate in the same leader election process.
//
// Parameters:
//   - name: The name of the lease object to use for leader election
//
// Returns:
//   - OptionFunc: A function that sets the lease lock name
func WithLeaseLockName(name string) OptionFunc {
	return func(o *options) {
		o.leaseLockName = name
	}
}

// WithLeaseLockNamespace sets the namespace for the lease object.
//
// The lease lock namespace determines where the lease object will be created
// and managed in the Kubernetes cluster. The namespace should exist and the
// leader elector should have the necessary permissions to create and manage
// lease objects in this namespace.
//
// Parameters:
//   - namespace: The namespace where the lease object will be created
//
// Returns:
//   - OptionFunc: A function that sets the lease lock namespace
func WithLeaseLockNamespace(namespace string) OptionFunc {
	return func(o *options) {
		o.leaseLockNamespace = namespace
	}
}

// WithLeaseDuration sets the lease duration for leader election.
//
// The lease duration is the maximum duration that a leader can be stopped
// before it is replaced by another candidate. This value should be set
// based on the expected time for a new leader to take over and the
// application's requirements for leader election responsiveness.
//
// Parameters:
//   - duration: The lease duration for leader election
//
// Returns:
//   - OptionFunc: A function that sets the lease duration
func WithLeaseDuration(duration time.Duration) OptionFunc {
	return func(o *options) {
		o.leaseDuration = duration
	}
}

// WithRenewDeadline sets the renew deadline for leader election.
//
// The renew deadline is the interval between attempts by the acting leader
// to renew a leadership slot before it stops leading. This value must be
// less than or equal to the lease duration and should be set based on
// the expected time for lease renewal operations.
//
// Parameters:
//   - deadline: The renew deadline duration for leader election
//
// Returns:
//   - OptionFunc: A function that sets the renew deadline
func WithRenewDeadline(deadline time.Duration) OptionFunc {
	return func(o *options) {
		o.renewDeadline = deadline
	}
}

// WithRetryPeriod sets the retry period for leader election.
//
// The retry period is the duration the clients should wait between attempts
// of acquiring a leadership lease when not currently the leader. This value
// should be set based on the desired responsiveness of leader election
// and the expected load on the Kubernetes API server.
//
// Parameters:
//   - period: The retry period duration for leader election
//
// Returns:
//   - OptionFunc: A function that sets the retry period
func WithRetryPeriod(period time.Duration) OptionFunc {
	return func(o *options) {
		o.retryPeriod = period
	}
}

// WithReleaseOnCancel sets whether to release the lease on context cancellation.
//
// When set to true, the lease will be released immediately when the context
// is cancelled, allowing other candidates to acquire leadership faster.
// This is useful for graceful shutdown scenarios where you want to quickly
// transfer leadership to another instance.
//
// Parameters:
//   - releaseOnCancel: Whether to release the lease on context cancellation
//
// Returns:
//   - OptionFunc: A function that sets the release on cancel behavior
func WithReleaseOnCancel(releaseOnCancel bool) OptionFunc {
	return func(o *options) {
		o.releaseOnCancel = &releaseOnCancel
	}
}

// WithEventRecorder sets the event recorder for the leader elector.
//
// The event recorder is used to record events related to leader election,
// such as leadership changes and lease operations. These events can be
// useful for monitoring, debugging, and auditing leader election behavior.
//
// Parameters:
//   - eventRecorder: The event recorder interface to use
//
// Returns:
//   - OptionFunc: A function that sets the event recorder
func WithEventRecorder(eventRecorder event.Recorder) OptionFunc {
	return func(o *options) {
		o.eventRecorder = eventRecorder
	}
}
