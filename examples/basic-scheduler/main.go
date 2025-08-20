// Package main provides an example application demonstrating the usage of the nautes library.
//
// This example shows how to create a basic task scheduler using the nautes
// framework. It demonstrates the complete lifecycle of a scheduler, from
// initialization to periodic task execution and graceful shutdown.
//
// The example scheduler runs a task every 30 seconds that lists all nodes
// in the Kubernetes cluster and logs their names. This serves as a template
// for building more complex schedulers that perform periodic operations.
//
// To run this example:
//
//	go run main.go
//
// The scheduler will start executing the node listing task every 30 seconds
// and log information about each node in the cluster.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/moonwayio/nautes/config"
	"github.com/moonwayio/nautes/manager"
	"github.com/moonwayio/nautes/scheduler"
)

func main() {
	if err := Run(); err != nil {
		os.Exit(1)
	}
}

// Run starts the example application and manages its lifecycle.
//
// This function demonstrates the complete setup and execution of a nautes-based
// scheduler application. It includes:
//   - Logger configuration with structured logging
//   - Manager initialization and component registration
//   - Kubernetes client configuration
//   - Scheduler creation with periodic task
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

	// Create a new scheduler with custom name
	sch, err := scheduler.NewScheduler(scheduler.WithName("test-scheduler"))
	if err != nil {
		return fmt.Errorf("failed to create scheduler: %w", err)
	}

	// Define a task that lists all nodes in the cluster
	err = sch.AddTask(scheduler.NewTask(func(ctx context.Context) error {
		log := klog.FromContext(ctx)

		// List all nodes in the cluster
		nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list nodes: %w", err)
		}

		// Log information about each node
		for _, node := range nodes.Items {
			log.Info("node", "name", node.Name)
		}

		return nil
	}), 30*time.Second)
	if err != nil {
		return fmt.Errorf("failed to add task: %w", err)
	}

	// Register the scheduler with the manager
	err = mgr.Register(sch)
	if err != nil {
		return fmt.Errorf("failed to register scheduler: %w", err)
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
