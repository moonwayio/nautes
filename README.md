# Nautes Framework

[![codecov](https://codecov.io/github/moonwayio/nautes/graph/badge.svg?token=8GevoLW7M3)](https://codecov.io/github/moonwayio/nautes)
[![CI](https://github.com/moonwayio/nautes/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/moonwayio/nautes/actions/workflows/ci.yaml)

Nautes is a comprehensive Go framework for building Kubernetes-native applications.
It provides a unified approach to creating controllers, schedulers, webhooks, and
other Kubernetes components with built-in support for metrics, health checks, and
graceful lifecycle management.

## Features

- **Component Lifecycle Management**: Centralized management of application
  components with graceful startup and shutdown
- **Kubernetes Controller Framework**: Watch-queue-reconcile pattern
  implementation with configurable concurrency
- **Task Scheduling**: Periodic task execution with configurable intervals and
  error handling
- **Admission Webhooks**: HTTP server for mutating and validating admission
  webhooks
- **Health Checks**: Kubernetes-compatible liveness and readiness probe endpoints
- **Metrics Collection**: Prometheus metrics exposition with built-in monitoring
- **Kubelet Communication**: Direct API access to node kubelets for advanced
  monitoring
- **Configuration Management**: Flexible Kubernetes client configuration with
  automatic detection
- **Leader Election**: Kubernetes lease-based leader election for high availability
- **Concurrency Safe Operations**: All components designed for concurrent usage
- **Structured Logging**: Comprehensive logging with context and structured output

## Quick Start

### Installation

```bash
go get github.com/moonwayio/nautes
```

### Basic Controller Example

```go
package main

import (
    "context"
    "log"

    "github.com/moonwayio/nautes/config"
    "github.com/moonwayio/nautes/controller"
    "github.com/moonwayio/nautes/manager"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/client-go/kubernetes"
)

func main() {
    // Create manager
    mgr, err := manager.NewManager()
    if err != nil {
        log.Fatal(err)
    }

    // Load Kubernetes config
    cfg, err := config.GetKubernetesConfig("")
    if err != nil {
        log.Fatal(err)
    }

    // Create Kubernetes client
    clientset, err := kubernetes.NewForConfig(cfg)
    if err != nil {
        log.Fatal(err)
    }

    // Define reconciler
    reconciler := func(ctx context.Context, obj runtime.Object) error {
        pod, ok := obj.(*corev1.Pod)
        if !ok {
            return errors.New("object is not a pod")
        }
        log.Printf("Reconciling pod: %s", pod.Name)
        return nil
    }

    // Create controller
    ctrl, err := controller.NewController(
        reconciler,
        controller.WithName("pod-controller"),
        controller.WithClient(clientset),
        controller.WithConcurrency(3),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Register and start
    mgr.Register(ctrl)
    if err := mgr.Start(); err != nil {
        log.Fatal(err)
    }

    // Wait for shutdown signal
    select {}
}
```

## Architecture

### Core Components

#### Manager

The `manager` package provides the central orchestration for all framework
components. It handles component registration, lifecycle management, and graceful
shutdown coordination.

```go
mgr, err := manager.NewManager()
if err != nil {
    return err
}

// Register components
mgr.Register(controller)
mgr.Register(scheduler)
mgr.Register(webhook)

// Start all components
if err := mgr.Start(); err != nil {
    return err
}

// Graceful shutdown
defer mgr.Stop()
```

#### Controller

The `controller` package implements the watch-queue-reconcile pattern for
Kubernetes controllers. It provides resource watching, event queuing, and
reconciliation coordination.

```go
reconciler := func(ctx context.Context, obj runtime.Object) error {
    // Implement reconciliation logic
    return nil
}

ctrl, err := controller.NewController(
    reconciler,
    controller.WithName("my-controller"),
    controller.WithClient(clientset),
    controller.WithConcurrency(5),
)
```

#### Scheduler

The `scheduler` package provides periodic task execution with configurable
intervals and error handling.

```go
task := func(ctx context.Context) error {
    // Perform periodic work
    return nil
}

sched, err := scheduler.NewScheduler(
    scheduler.WithName("my-scheduler"),
)
if err != nil {
    return err
}

sched.AddTask(task, 5*time.Minute)
```

#### Webhook

The `webhook` package implements HTTP servers for Kubernetes admission webhooks.

```go
handler := func(ctx context.Context, request *admissionv1.AdmissionRequest) (
    *admissionv1.AdmissionResponse, error,
) {
    return &admissionv1.AdmissionResponse{Allowed: true}, nil
}

server, err := webhook.NewWebhookServer(8443)
if err != nil {
    return err
}

server.Register("/mutate", handler)
```

#### Health Checks

The `health` package provides Kubernetes-compatible liveness and readiness probe
endpoints.

```go
healthServer := health.NewHealthCheck(8080)

// Register readiness check
healthServer.RegisterReadiness("database", func(ctx context.Context) error {
    return db.PingContext(ctx)
})

// Register liveness check
healthServer.RegisterLiveness("memory", func(ctx context.Context) error {
    if memoryUsage > threshold {
        return errors.New("memory usage too high")
    }
    return nil
})
```

#### Metrics

The `metrics` package provides Prometheus metrics exposition.

```go
metricsServer := metrics.NewMetricsServer(9090)

// Register custom metrics
counter := prometheus.NewCounter(prometheus.CounterOpts{
    Name: "my_requests_total",
    Help: "Total number of requests",
})
metricsServer.Register(counter)
```

#### Leader Election

The `leader` package provides Kubernetes lease-based leader election functionality
for high availability and preventing split-brain scenarios in multi-replica
deployments.

```go
// Create leader elector
leaderElector, err := leader.NewLeader(
    leader.WithID("my-operator"),
    leader.WithName("my-operator"),
    leader.WithClient(clientset),
    leader.WithLeaseLockName("my-operator"),
    leader.WithLeaseLockNamespace("default"),
    leader.WithLeaseDuration(30*time.Second),
    leader.WithRenewDeadline(20*time.Second),
    leader.WithRetryPeriod(5*time.Second),
)
if err != nil {
    return err
}

// Create manager with leader election
mgr, err := manager.NewManager(manager.WithLeader(leaderElector))
if err != nil {
    return err
}

// Create controller that needs leader election
ctrl, err := controller.NewController(
    reconciler,
    controller.WithName("my-controller"),
    controller.WithNeedsLeaderElection(true),
)
if err != nil {
    return err
}

// Register controller (will only start when leader)
mgr.Register(ctrl)
```

Components can also implement the `ElectionSubscriber` interface to receive
notifications about leadership changes:

```go
type MyComponent struct {
    isLeading bool
}

func (m *MyComponent) OnStartLeading() {
    m.isLeading = true
    log.Info("Became leader, starting leader-specific work")
}

func (m *MyComponent) OnStopLeading() {
    m.isLeading = false
    log.Info("Lost leadership, stopping leader-specific work")
}
```

## Configuration

### Kubernetes Client Configuration

The `config` package provides flexible Kubernetes client configuration with
automatic detection:

```go
// Automatic configuration detection
config, err := config.GetKubernetesConfig("")
if err != nil {
    return err
}

// Load from specific kubeconfig file
config, err := config.GetKubernetesConfig("/path/to/kubeconfig")
if err != nil {
    return err
}

// Check if running in-cluster
if config.IsInCluster() {
    log.Info("Running inside Kubernetes cluster")
}
```

## Examples

The `examples` directory contains comprehensive examples demonstrating various use
cases:

- **basic-controller**: Simple controller watching pods
- **basic-scheduler**: Periodic task execution
- **basic-webhook**: Admission webhook implementation
- **basic-kubelet-client**: Direct kubelet communication
- **operator**: Complete operator pattern implementation with leader election
