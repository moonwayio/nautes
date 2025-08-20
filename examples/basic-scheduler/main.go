// Package main provides an example application demonstrating the usage of the nautes library.
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

	sch, err := scheduler.NewScheduler(scheduler.WithName("test-scheduler"))
	if err != nil {
		return fmt.Errorf("failed to create scheduler: %w", err)
	}

	err = sch.AddTask(scheduler.NewTask(func(ctx context.Context) error {
		log := klog.FromContext(ctx)

		nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list nodes: %w", err)
		}

		for _, node := range nodes.Items {
			log.Info("node", "name", node.Name)
		}

		return nil
	}), 30*time.Second)
	if err != nil {
		return fmt.Errorf("failed to add task: %w", err)
	}

	err = mgr.Register(sch)
	if err != nil {
		return fmt.Errorf("failed to register scheduler: %w", err)
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
