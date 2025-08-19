// Package main provides an example application demonstrating the usage of the nautes library.
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

// Run starts the example application.
func Run() error {
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

	mgr, err := manager.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	webhookSrv, err := webhook.NewWebhookServer(8081, webhook.WithTLS(false))
	if err != nil {
		return fmt.Errorf("failed to create webhook server: %w", err)
	}

	err = mgr.Register(webhookSrv)
	if err != nil {
		return fmt.Errorf("failed to register webhook server: %w", err)
	}

	err = webhookSrv.Register(
		"/mutate",
		func(ctx context.Context, req webhook.AdmissionRequest) webhook.AdmissionResponse {
			log := klog.FromContext(ctx)

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

			pod.Labels["nautes.moonway.io/patched"] = "true"

			diff, err := webhook.GetJSONDiff(req.Object.Raw, pod)
			if err != nil {
				return webhook.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Message: "failed to get json patch",
					},
				}
			}

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

	err = mgr.Start()
	if err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	err = mgr.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop manager: %w", err)
	}

	return nil
}
