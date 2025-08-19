// Package main provides an example application demonstrating the usage of the nautes library.
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
	"github.com/moonwayio/nautes/health"
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

	cfg, err := config.GetKubernetesConfig("")
	if err != nil {
		return fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	scheme := scheme.Scheme

	reconciler := func(ctx context.Context, obj runtime.Object) error {
		log := klog.FromContext(ctx)
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return errors.New("object is not a pod")
		}
		log.Info("reconciling pod", "pod", pod.Name, "status", pod.Status.Phase)
		return nil
	}

	ctrl, err := controller.NewController(
		reconciler,
		controller.WithName("test-controller"),
		controller.WithClient(clientset),
		controller.WithScheme(scheme),
	)
	if err != nil {
		return fmt.Errorf("failed to create controller: %w", err)
	}

	err = ctrl.AddRetriever(&controller.ListerWatcher{
		ListFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return clientset.CoreV1().Pods("kube-system").List(ctx, options)
		},
		WatchFunc: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
			return clientset.CoreV1().Pods("kube-system").Watch(ctx, options)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add retriever: %w", err)
	}

	err = mgr.Register(ctrl)
	if err != nil {
		return fmt.Errorf("failed to register controller: %w", err)
	}

	healthSrv := health.NewHealthCheck(8080)

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

	err = mgr.Register(healthSrv)
	if err != nil {
		return fmt.Errorf("failed to register health server: %w", err)
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
		func(_ context.Context, _ webhook.AdmissionRequest) webhook.AdmissionResponse {
			return webhook.AdmissionResponse{
				Allowed: true,
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
