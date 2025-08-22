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
// This example shows how to use the kubelet client to directly communicate with
// Kubernetes node kubelets. It demonstrates how to retrieve resource usage statistics
// from nodes and pods using the kubelet API.
//
// The example connects to all nodes in the cluster and retrieves CPU and memory
// usage statistics for both the nodes and the pods running on them. This serves
// as a template for building monitoring and resource management applications.
//
// To run this example:
//
//	go run main.go
//
// The application will connect to all nodes in the cluster and log resource
// usage statistics for nodes and pods.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	statsv1alpha1 "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/moonwayio/nautes/config"
	"github.com/moonwayio/nautes/kubelet"
)

// safeInt64 safely converts uint64 to int64, clamping to MaxInt64 if needed.
func safeInt64(val uint64) int64 {
	if val > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(val)
}

func main() {
	if err := Run(); err != nil {
		os.Exit(1)
	}
}

// Run starts the example application and manages its lifecycle.
//
// This function demonstrates the complete setup and execution of a nautes-based
// kubelet client application. It includes:
//   - Logger configuration with structured logging
//   - Kubernetes client configuration
//   - Kubelet client initialization
//   - Resource usage statistics retrieval
//   - Data parsing and logging
//
// The function requires the application to be running inside a Kubernetes
// cluster to access node kubelets directly.
//
// Returns:
//   - error: Any error encountered during application execution
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

	// Create kubelet client for direct node communication
	kclient, err := kubelet.NewKubeletClient(
		kubelet.WithRestConfig(cfg),
	)
	if err != nil {
		return fmt.Errorf("failed to create kubelet client: %w", err)
	}

	log := klog.Background()
	ctx := context.Background()

	// Verify that we're running inside a cluster
	if !config.IsInCluster() {
		return errors.New("not running in cluster, this example requires to be run in a cluster")
	}

	// List all nodes in the cluster
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	// Iterate through each node and retrieve resource usage statistics
	for _, node := range nodes.Items {
		// Get stats from the node's kubelet
		b, err := kclient.Get(ctx, &node, "/stats/summary")
		if err != nil {
			return fmt.Errorf("failed to get node stats: %w", err)
		}

		// Parse the JSON response
		var s statsv1alpha1.Summary
		err = json.Unmarshal(b, &s)
		if err != nil {
			return fmt.Errorf("failed to unmarshal stats: %w", err)
		}

		// Log node resource usage
		cpu := resource.NewMilliQuantity(
			safeInt64(*s.Node.CPU.UsageNanoCores/1e6),
			resource.DecimalSI,
		)
		mem := resource.NewQuantity(safeInt64(*s.Node.Memory.UsageBytes), resource.BinarySI)
		log.Info("node resource usage", "name", node.Name, "cpu", cpu, "memory", mem)

		// Log pod resource usage for each pod on the node
		for _, pod := range s.Pods {
			cpu := resource.NewMilliQuantity(
				safeInt64(*pod.CPU.UsageNanoCores/1e6),
				resource.DecimalSI,
			)
			mem := resource.NewQuantity(safeInt64(*pod.Memory.UsageBytes), resource.BinarySI)
			log.Info("pod resource usage", "name", pod.PodRef.Name, "cpu", cpu, "memory", mem)
		}
	}

	return nil
}
