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
// This example shows how to create a basic Kubernetes controller using the nautes
// framework. It demonstrates the complete lifecycle of a controller, from
// initialization to resource watching and reconciliation.
//
// The example controller watches Pod resources in the kube-system namespace
// and logs information about each pod when it is reconciled. This serves as
// a template for building more complex controllers.
//
// To run this example:
//
//	go run main.go
//
// The controller will start watching pods in the kube-system namespace and
// log information about each pod when it is reconciled.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/zerologr"
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
	"github.com/moonwayio/nautes/manager"
)

// k8sScheme is the runtime scheme used for object serialization.
//
// This scheme is used by the controller to properly serialize and deserialize
// Kubernetes objects. It can be extended to include custom resource types.
var k8sScheme = scheme.Scheme

func init() {
	// Add your custom schemes here to support custom resource types.
	// Example: yourscheme.AddToScheme(k8sScheme)
}

func main() {
	if err := Run(); err != nil {
		os.Exit(1)
	}
}

// Run starts the example application and manages its lifecycle.
//
// This function demonstrates the complete setup and execution of a nautes-based
// controller application. It includes:
//   - Logger configuration with structured logging
//   - Manager initialization and component registration
//   - Kubernetes client configuration
//   - Controller creation with custom reconciler
//   - Resource watching setup
//   - Graceful shutdown handling
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

	// Create a new manager instance
	mgr, err := manager.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

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

	// Define the reconciler function
	//
	// This function is called for each resource that needs to be reconciled.
	// In this example, it simply logs information about the pod being reconciled.
	reconciler := func(ctx context.Context, obj controller.Delta[runtime.Object]) error {
		log := klog.FromContext(ctx)
		pod, ok := obj.Object.(*corev1.Pod)
		if !ok {
			return errors.New("object is not a pod")
		}
		log.Info("reconciling pod", "pod", pod.Name, "status", pod.Status.Phase)
		return nil
	}

	// Create a new controller with the reconciler
	ctrl, err := controller.NewController(
		reconciler,
		controller.WithName("test-controller"),
		controller.WithScheme(k8sScheme),
		controller.WithConcurrency(3),
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

	// Start the manager
	if err := mgr.Start(); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	// Set up graceful shutdown
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-stopCh

	// Stop the manager gracefully
	if err := mgr.Stop(); err != nil {
		return fmt.Errorf("failed to stop manager: %w", err)
	}

	return nil
}
