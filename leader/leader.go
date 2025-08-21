// Package leader provides Kubernetes leader election functionality.
//
// The leader package implements leader election using Kubernetes lease objects to ensure
// that only one instance of a component is active at a time across a cluster. It provides
// a framework for building highly available applications that need to coordinate work
// across multiple replicas while ensuring only one instance performs the work.
package leader

import (
	"context"
	"errors"
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// ErrAlreadyRunning is returned when attempting to start a leader elector that is already running.
var ErrAlreadyRunning = errors.New("leader elector is already running")

// Leader provides leader election functionality using Kubernetes lease objects.
//
// The Leader interface provides methods to participate in leader election, register
// callbacks for leadership changes, and check the current leadership status. It ensures
// that only one instance of a component is active at a time across a Kubernetes cluster.
//
// Implementations of this interface are concurrency safe and can be used concurrently
// from multiple goroutines.
type Leader interface {
	// Run starts the leader election process.
	//
	// This method begins participating in leader election using the configured lease
	// object. The method blocks until the context is cancelled or an error occurs.
	// Only one instance can be running at a time - attempting to call Run() on an
	// already running leader elector will return ErrAlreadyRunning.
	//
	// Parameters:
	//   - ctx: Context for the leader election process, cancellation will stop the election
	//
	// Returns:
	//   - error: Any error encountered during leader election
	Run(ctx context.Context) error

	// Subscribe registers a subscriber to be notified when leadership status changes.
	//
	// The subscriber will be notified whenever the leadership status changes through
	// the OnLeaderElectionStarted and OnLeaderElectionStopped methods. Subscribers
	// can only be registered before the leader election process starts.
	//
	// Parameters:
	//   - subscriber: The subscriber to register for leadership change notifications
	//
	// Returns:
	//   - error: ErrAlreadyRunning if the leader elector is already running
	Subscribe(subscriber ElectionSubscriber) error

	// IsLeader returns whether this instance is currently the leader.
	//
	// This method provides a concurrency safe way to check the current leadership status
	// without blocking. The result may change at any time as leadership can be
	// transferred between instances.
	//
	// Returns:
	//   - bool: true if this instance is the leader, false otherwise
	IsLeader() bool
}

// ElectionSubscriber receives notifications about leadership status changes.
//
// Components implementing this interface will be notified when leadership
// status changes occur, allowing them to react appropriately to becoming
// or ceasing to be the leader.
type ElectionSubscriber interface {
	// OnStartLeading is called when this instance becomes the leader.
	//
	// This method is called when the leader election process determines that
	// this instance should be the leader. Components should perform any
	// necessary initialization or start any leader-specific work.
	OnStartLeading()

	// OnStopLeading is called when this instance stops being the leader.
	//
	// This method is called when the leader election process determines that
	// this instance should no longer be the leader. Components should perform
	// any necessary cleanup or stop any leader-specific work.
	OnStopLeading()
}

// leader implements the Leader interface.
//
// The leader maintains state for the leader election process, including subscribers
// for leadership changes and internal state tracking. It uses read-write mutexes
// to ensure concurrent safety during state updates and subscriber registration.
type leader struct {
	opts        options
	subscribers []ElectionSubscriber
	logger      klog.Logger

	// State tracking
	mu        sync.RWMutex
	isRunning bool
	isLeader  bool
}

// NewLeader creates a new Leader instance with the provided options.
//
// The leader is initialized with the specified configuration and is ready to
// participate in leader election. The returned leader is not yet running and
// must be started by calling Run().
//
// Parameters:
//   - opts: Option functions to configure the leader elector
//
// Returns:
//   - Leader: A configured leader elector instance
//   - error: Any error encountered during configuration validation
func NewLeader(opts ...OptionFunc) (Leader, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if err := o.setDefaults(); err != nil {
		return nil, err
	}

	return &leader{
		opts:        o,
		logger:      klog.Background().WithValues("component", "leader/"+o.name),
		subscribers: make([]ElectionSubscriber, 0),
	}, nil
}

// Run starts the leader election process.
//
// This method creates a Kubernetes lease lock and begins participating in leader
// election. It sets up callbacks to handle leadership changes and manages the
// internal state tracking. The method blocks until the context is cancelled.
//
// The method is not idempotent - calling it multiple times will return ErrAlreadyRunning
// if the leader elector is already running.
func (l *leader) Run(ctx context.Context) error {
	l.mu.Lock()
	if l.isRunning {
		l.mu.Unlock()
		return ErrAlreadyRunning
	}
	l.isRunning = true
	l.mu.Unlock()

	lock := l.createLeaseLock()
	callbacks := l.createCallbacks()

	// Create and run the leader elector
	elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   l.opts.leaseDuration,
		RenewDeadline:   l.opts.renewDeadline,
		RetryPeriod:     l.opts.retryPeriod,
		ReleaseOnCancel: *l.opts.releaseOnCancel,
		Callbacks:       callbacks,
		Name:            l.opts.name,
	})
	if err != nil {
		l.mu.Lock()
		l.isRunning = false
		l.mu.Unlock()
		return fmt.Errorf("failed to create leader elector: %w", err)
	}

	elector.Run(ctx)

	// Reset state when Run returns
	l.mu.Lock()
	l.isRunning = false
	l.isLeader = false
	l.mu.Unlock()

	return nil
}

// Subscribe registers a subscriber to be notified when leadership status changes.
//
// This method adds a subscriber to the list of subscribers that will be notified
// whenever the leadership status changes. Subscribers can only be registered before
// the leader election process starts.
//
// The method is concurrency safe and will return ErrAlreadyRunning if the leader elector
// is already running.
func (l *leader) Subscribe(subscriber ElectionSubscriber) error {
	l.mu.RLock()
	if l.isRunning {
		l.mu.RUnlock()
		return ErrAlreadyRunning
	}
	l.mu.RUnlock()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.subscribers = append(l.subscribers, subscriber)
	return nil
}

// IsLeader returns whether this instance is currently the leader.
//
// This method provides a concurrency safe way to check the current leadership status.
// The result reflects the leadership state at the time of the call and may change
// as leadership is transferred between instances.
func (l *leader) IsLeader() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.isLeader
}

// createLeaseLock creates the lease lock for leader election.
func (l *leader) createLeaseLock() *resourcelock.LeaseLock {
	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      l.opts.leaseLockName,
			Namespace: l.opts.leaseLockNamespace,
		},
		Client: l.opts.client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity:      l.opts.id,
			EventRecorder: l.opts.eventRecorder,
		},
	}
}

// createCallbacks sets up callbacks for leadership changes.
func (l *leader) createCallbacks() leaderelection.LeaderCallbacks {
	return leaderelection.LeaderCallbacks{
		OnStartedLeading: func(_ context.Context) {
			l.mu.Lock()
			l.isLeader = true
			l.mu.Unlock()

			l.logger.Info("started leading")
			// Notify all subscribers that leadership has started
			for _, subscriber := range l.subscribers {
				subscriber.OnStartLeading()
			}
		},
		OnStoppedLeading: func() {
			l.mu.Lock()
			l.isLeader = false
			l.mu.Unlock()

			l.logger.Info("stopped leading")
			// Notify all subscribers that leadership has stopped
			for _, subscriber := range l.subscribers {
				subscriber.OnStopLeading()
			}
		},
	}
}
