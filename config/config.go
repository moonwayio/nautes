// Package config provides functions to get the Kubernetes config.
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

// GetKubernetesConfig returns the Kubernetes config
// if path is provided, it will be used to build the config. otherwise, it will try to use the in-cluster config
// and fallback to the path of the KUBECONFIG environment variable or the kubeconfig in the home directory.
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

// IsInCluster returns true if the config is in cluster, false otherwise.
func IsInCluster() bool {
	_, err := loadInClusterConfig()
	return err == nil
}
