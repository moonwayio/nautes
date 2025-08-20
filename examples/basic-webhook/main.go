// Package main provides an example application demonstrating the usage of the nautes library.
//
// This example shows how to create a basic admission webhook using the nautes
// framework. It demonstrates the complete lifecycle of a webhook server, from
// initialization to request handling and graceful shutdown.
//
// The example webhook implements a mutating admission webhook that adds a label
// to all pods created in the cluster. This serves as a template for building
// more complex webhooks that modify or validate Kubernetes resources.
//
// To run this example:
//
//	go run main.go
//
// The webhook server will start listening on port 8081 and will add a label
// to all pods that are created in the cluster.
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
	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/moonwayio/nautes/manager"
	"github.com/moonwayio/nautes/webhook"
)

func main() {
	if err := Run(); err != nil {
		os.Exit(1)
	}
}

// Run starts the example application and manages its lifecycle.
//
// This function demonstrates the complete setup and execution of a nautes-based
// webhook application. It includes:
//   - Logger configuration with structured logging
//   - Manager initialization and component registration
//   - Webhook server creation with custom handler
//   - Mutating admission webhook implementation
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

	// Create a new webhook server with TLS disabled for development
	webhookSrv, err := webhook.NewWebhookServer(8081, webhook.WithTLS(false))
	if err != nil {
		return fmt.Errorf("failed to create webhook server: %w", err)
	}

	// Register the webhook server with the manager
	err = mgr.Register(webhookSrv)
	if err != nil {
		return fmt.Errorf("failed to register webhook server: %w", err)
	}

	// Register a mutating webhook handler for the /mutate endpoint
	err = webhookSrv.Register(
		"/mutate",
		func(ctx context.Context, req webhook.AdmissionRequest) webhook.AdmissionResponse {
			log := klog.FromContext(ctx)

			// Unmarshal the pod object from the request
			var pod *corev1.Pod
			if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
				return webhook.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Message: "failed to unmarshal pod",
					},
				}
			}

			log.Info("mutating pod", "pod", pod.Name)

			// Add a label to the pod
			pod.Labels["nautes.moonway.io/patched"] = "true"

			// Generate a JSON patch for the modification
			diff, err := webhook.GetJSONDiff(req.Object.Raw, pod)
			if err != nil {
				return webhook.AdmissionResponse{
					Allowed: true,
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
