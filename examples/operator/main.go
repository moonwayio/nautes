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

// Package main provides an example application demonstrating the usage of the nautes library.
//
// This example shows how to create a complete Kubernetes operator using the nautes
// framework. It demonstrates the integration of multiple components including
// controllers, webhooks, health checks, metrics collection, and leader election.
//
// The example operator watches pods in the kube-system namespace, adds labels
// to pods through a mutating webhook, and exposes metrics and health endpoints.
// It includes leader election to ensure high availability and prevent split-brain
// scenarios when running multiple replicas. This serves as a comprehensive template
// for building production-ready operators.
//
// To run this example:
//
//	go run main.go
//
// The operator will start multiple components:
//   - Leader election using Kubernetes leases (only leader processes pods)
//   - Controller watching pods in kube-system namespace (leader-only)
//   - Webhook server on port 8081 adding labels to pods
//   - Health check server on port 8080
//   - Metrics server on port 8082
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/zerologr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	"github.com/moonwayio/nautes/config"
	"github.com/moonwayio/nautes/controller"
	"github.com/moonwayio/nautes/health"
	"github.com/moonwayio/nautes/leader"
	"github.com/moonwayio/nautes/manager"
	"github.com/moonwayio/nautes/metrics"
	"github.com/moonwayio/nautes/webhook"
)

// observedPodCount is a Prometheus counter metric that tracks the number of pods
// observed by the operator. It includes labels for namespace and pod name to
// provide detailed metrics for monitoring and alerting.
var observedPodCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "observed_pod_count",
		Help: "Number of observed pods",
	},
	[]string{"namespace", "name"},
)

func main() {
	if err := Run(); err != nil {
		os.Exit(1)
	}
}

// Run starts the example application and manages its lifecycle.
//
// This function demonstrates the complete setup and execution of a nautes-based
// operator application with leader election. It includes:
//   - Logger configuration with structured logging
//   - Leader election setup using Kubernetes leases
//   - Manager initialization with leader election support
//   - Controller creation with custom reconciler (leader-only)
//   - Webhook server with mutating admission webhook
//   - Health check server with readiness probe
//   - Metrics server with custom metrics
//   - Graceful shutdown handling
//
// The controller is configured with leader election awareness using the
// WithNeedsLeaderElection option, ensuring it only processes resources when
// this instance is the elected leader. This prevents conflicts when running
// multiple replicas of the operator.
//
// The function sets up signal handling to ensure graceful shutdown when
// the application receives termination signals.
//
// Returns:
//   - error: Any error encountered during application startup or execution
func Run() error {
	// Configure structured logging with zerolog
	logger := zerolog.New(
		zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC1123,
			PartsOrder: []string{
				zerolog.TimestampFieldName,
				zerolog.LevelFieldName,
				"component",
				zerolog.MessageFieldName,
			},
			FieldsExclude: []string{
				"component",
				"v",
			},
		},
	).Level(zerolog.InfoLevel).With().Timestamp().Logger()

	klog.SetLogger(zerologr.New(&logger))

	// Load Kubernetes configuration
	cfg, err := config.GetKubernetesConfig("")
	if err != nil {
		return fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	// Create Kubernetes client
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// Create a leader elector for high availability
	leaderElector, err := leader.NewLeader(
		leader.WithID("operator-example"),
		leader.WithName("operator-example"),
		leader.WithClient(clientset),
		leader.WithLeaseLockName("operator-example"),
		leader.WithLeaseLockNamespace("default"),
		leader.WithLeaseDuration(30*time.Second),
		leader.WithRenewDeadline(20*time.Second),
		leader.WithRetryPeriod(5*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to create leader elector: %w", err)
	}

	// Create a new manager instance with leader election
	mgr, err := manager.NewManager(manager.WithLeader(leaderElector))
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	// Use the default Kubernetes scheme
	scheme := scheme.Scheme

	// Define the reconciler function for the controller
	reconciler := func(ctx context.Context, delta controller.Delta[*corev1.Pod]) error {
		log := klog.FromContext(ctx)
		pod := delta.Object
		log.Info("reconciling pod", "pod", pod.Name, "status", pod.Status.Phase)
		// Increment the observed pod counter metric
		observedPodCount.WithLabelValues(pod.Namespace, pod.Name).Inc()
		return nil
	}

	// Create a new controller with the reconciler
	ctrl, err := controller.NewController(
		reconciler,
		controller.WithName("test-controller"),
		controller.WithScheme(scheme),
		controller.WithNeedsLeaderElection(true),
	)
	if err != nil {
		return fmt.Errorf("failed to create controller: %w", err)
	}

	// Add a resource retriever to watch pods in the kube-system namespace
	err = ctrl.AddRetriever(&controller.ListerWatcher{
		ListFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return clientset.CoreV1().Pods("kube-system").List(ctx, options)
		},
		WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
			return clientset.CoreV1().Pods("kube-system").Watch(ctx, options)
		},
	}, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to add retriever: %w", err)
	}

	// Register the controller with the manager
	err = mgr.Register(ctrl)
	if err != nil {
		return fmt.Errorf("failed to register controller: %w", err)
	}

	// Create a health check server
	healthSrv := health.NewHealthCheck(8080)

	// Register a readiness check that verifies API server connectivity
	err = healthSrv.RegisterReadiness("test-readiness", func(ctx context.Context) error {
		_, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list namespaces: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to register readiness check: %w", err)
	}

	// Register the health server with the manager
	err = mgr.Register(healthSrv)
	if err != nil {
		return fmt.Errorf("failed to register health server: %w", err)
	}

	// Create a webhook server with TLS disabled for development
	webhookSrv, err := webhook.NewWebhookServer(8081, webhook.WithTLS(false))
	if err != nil {
		return fmt.Errorf("failed to create webhook server: %w", err)
	}

	// Register the webhook server with the manager
	err = mgr.Register(webhookSrv)
	if err != nil {
		return fmt.Errorf("failed to register webhook server: %w", err)
	}

	// Register a mutating webhook handler that adds labels to pods
	err = webhookSrv.Register(
		"/mutate",
		func(ctx context.Context, req webhook.AdmissionRequest) webhook.AdmissionResponse {
			log := klog.FromContext(ctx)

			// Unmarshal the pod object from the request
			var pod *corev1.Pod
			if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
				return webhook.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: "failed to unmarshal pod",
					},
				}
			}

			// Increment the observed pod counter metric
			observedPodCount.WithLabelValues(pod.Namespace, pod.Name).Inc()

			log.Info("mutating pod", "pod", pod.Name)

			// Add a label to the pod
			pod.Labels["nautes.moonway.io/patched"] = "true"

			// Generate a JSON patch for the modification
			diff, err := webhook.GetJSONDiff(req.Object.Raw, pod)
			if err != nil {
				return webhook.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: "failed to get json patch",
					},
				}
			}

			// Return the admission response with the patch
			return webhook.AdmissionResponse{
				Allowed:   true,
				Patch:     diff.Patch,
				PatchType: &diff.PatchType,
			}
		},
	)
	if err != nil {
		return fmt.Errorf("failed to register webhook handler: %w", err)
	}

	// Create a metrics server
	metricsSrv := metrics.NewMetricsServer(8082)

	// Register the observed pod count metric
	err = metricsSrv.Register(observedPodCount)
	if err != nil {
		return fmt.Errorf("failed to register observed pod count: %w", err)
	}

	// Register the metrics server with the manager
	err = mgr.Register(metricsSrv)
	if err != nil {
		return fmt.Errorf("failed to register metrics server: %w", err)
	}

	// Start the manager
	err = mgr.Start()
	if err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	// Set up graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	// Stop the manager gracefully
	err = mgr.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop manager: %w", err)
	}

	return nil
}
