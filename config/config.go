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

// Package config provides functions to get the Kubernetes config.
//
// The config package provides utilities for loading and managing Kubernetes client
// configurations. It supports both in-cluster and out-of-cluster configurations,
// with automatic fallback mechanisms for different deployment scenarios.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// loadInClusterConfig is a function that loads the in-cluster config, it can be overridden for testing purposes.
var loadInClusterConfig = rest.InClusterConfig

// loadConfigFromPath is a function that loads the config from a path, it can be overridden for testing purposes.
var loadConfigFromPath = func(path string) (*rest.Config, error) {
	return clientcmd.BuildConfigFromFlags("", path)
}

// GetKubernetesConfig returns the Kubernetes configuration based on the provided path.
//
// The function implements a hierarchical configuration loading strategy:
// 1. If a path is provided, it attempts to load the configuration from that path
// 2. If no path is provided, it first tries to load the in-cluster configuration
// 3. If in-cluster configuration fails, it falls back to the KUBECONFIG environment variable
// 4. If KUBECONFIG is not set, it uses the default kubeconfig location (~/.kube/config)
//
// This approach ensures compatibility with various deployment scenarios including
// local development, in-cluster deployments, and custom kubeconfig locations.
//
// Parameters:
//   - path: The path to the kubeconfig file. If empty, automatic detection is used.
//
// Returns:
//   - *rest.Config: The Kubernetes client configuration
//   - error: Any error encountered during configuration loading
func GetKubernetesConfig(path string) (*rest.Config, error) {
	var config *rest.Config
	var err error
	if path != "" {
		config, err = loadConfigFromPath(path)
		if err != nil {
			return nil, fmt.Errorf("error loading kubernetes configuration: %w", err)
		}
	} else {
		config, err = loadInClusterConfig()
		if err != nil {
			env, found := os.LookupEnv("KUBECONFIG")
			if found && env != "" {
				config, err = loadConfigFromPath(env)
			} else {
				config, err = loadConfigFromPath(filepath.Join(homedir.HomeDir(), ".kube", "config"))
			}
			if err != nil {
				return nil, fmt.Errorf("error loading kubernetes configuration: %w", err)
			}
		}
	}

	return config, nil
}

// IsInCluster returns true if the current process is running inside a Kubernetes cluster.
//
// This function attempts to load the in-cluster configuration to determine if
// the process is running inside a Kubernetes cluster. It's useful for adapting
// application behavior based on the deployment environment.
//
// Returns:
//   - bool: true if running in-cluster, false otherwise
func IsInCluster() bool {
	_, err := loadInClusterConfig()
	return err == nil
}
