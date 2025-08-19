// Package main provides an example application demonstrating the usage of the nautes library.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/moonwayio/nautes/config"
	"github.com/moonwayio/nautes/kubelet"
)

type stats struct {
	Node struct {
		CPU struct {
			UsageNanoCores int64 `json:"usageNanoCores"`
		} `json:"cpu"`
		Memory struct {
			UsageBytes int64 `json:"usageBytes"`
		} `json:"memory"`
	} `json:"node"`
	Pods []struct {
		Ref struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			UID       string `json:"uid"`
		} `json:"podRef"`
		CPU struct {
			UsageNanoCores int64 `json:"usageNanoCores"`
		} `json:"cpu"`
		Memory struct {
			UsageBytes int64 `json:"usageBytes"`
		} `json:"memory"`
	} `json:"pods"`
}

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

	cfg, err := config.GetKubernetesConfig("")
	if err != nil {
		return fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	kclient, err := kubelet.NewKubeletClient(
		kubelet.WithRestConfig(cfg),
	)
	if err != nil {
		return fmt.Errorf("failed to create kubelet client: %w", err)
	}

	log := klog.Background()
	ctx := context.Background()

	if !config.IsInCluster() {
		return errors.New("not running in cluster, this example requires to be run in a cluster")
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	for _, node := range nodes.Items {
		b, err := kclient.Get(ctx, &node, "/stats/summary")
		if err != nil {
			return fmt.Errorf("failed to get node stats: %w", err)
		}

		var s stats
		err = json.Unmarshal(b, &s)
		if err != nil {
			return fmt.Errorf("failed to unmarshal stats: %w", err)
		}

		cpu := resource.NewMilliQuantity(s.Node.CPU.UsageNanoCores/1e6, resource.DecimalSI)
		mem := resource.NewQuantity(s.Node.Memory.UsageBytes, resource.BinarySI)
		log.Info("node resource usage", "name", node.Name, "cpu", cpu, "memory", mem)
		for _, pod := range s.Pods {
			cpu := resource.NewMilliQuantity(pod.CPU.UsageNanoCores/1e6, resource.DecimalSI)
			mem := resource.NewQuantity(pod.Memory.UsageBytes, resource.BinarySI)
			log.Info("pod resource usage", "name", pod.Ref.Name, "cpu", cpu, "memory", mem)
		}
	}

	return nil
}
