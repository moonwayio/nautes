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

package config

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/rest"
)

type ConfigTestSuite struct {
	suite.Suite
	originalLoadInClusterConfig func() (*rest.Config, error)
	originalLoadConfigFromPath  func(string) (*rest.Config, error)
}

func (s *ConfigTestSuite) SetupTest() {
	s.originalLoadInClusterConfig = loadInClusterConfig
	s.originalLoadConfigFromPath = loadConfigFromPath
}

func (s *ConfigTestSuite) TearDownTest() {
	loadInClusterConfig = s.originalLoadInClusterConfig
	loadConfigFromPath = s.originalLoadConfigFromPath
}

func (s *ConfigTestSuite) TestGetKubernetesConfigWithPathSuccess() {
	expectedConfig := &rest.Config{Host: "test-host"}
	loadConfigFromPath = func(_ string) (*rest.Config, error) {
		return expectedConfig, nil
	}

	config, err := GetKubernetesConfig("/test/path")

	// Assert
	s.Require().NoError(err)
	s.Require().Equal(expectedConfig, config)
}

func (s *ConfigTestSuite) TestGetKubernetesConfigWithPathError() {
	expectedError := errors.New("config load error")
	loadConfigFromPath = func(_ string) (*rest.Config, error) {
		return nil, expectedError
	}

	config, err := GetKubernetesConfig("/test/path")

	s.Require().Error(err)
	s.Require().Nil(config)
	s.Require().Contains(err.Error(), "error loading kubernetes configuration")
}

func (s *ConfigTestSuite) TestGetKubernetesConfigInClusterSuccess() {
	expectedConfig := &rest.Config{Host: "in-cluster-host"}
	loadInClusterConfig = func() (*rest.Config, error) {
		return expectedConfig, nil
	}

	config, err := GetKubernetesConfig("")

	s.Require().NoError(err)
	s.Require().Equal(expectedConfig, config)
}

func (s *ConfigTestSuite) TestGetKubernetesConfigKubeconfigEnvSuccess() {
	expectedConfig := &rest.Config{Host: "env-host"}
	loadInClusterConfig = func() (*rest.Config, error) {
		return nil, errors.New("in-cluster error")
	}
	loadConfigFromPath = func(path string) (*rest.Config, error) {
		if path == "/test/kubeconfig" {
			return expectedConfig, nil
		}
		return nil, errors.New("wrong path")
	}

	err := os.Setenv("KUBECONFIG", "/test/kubeconfig")
	s.Require().NoError(err)
	defer func() {
		err := os.Unsetenv("KUBECONFIG")
		s.Require().NoError(err)
	}()

	config, err := GetKubernetesConfig("")

	s.Require().NoError(err)
	s.Require().Equal(expectedConfig, config)
}

func (s *ConfigTestSuite) TestGetKubernetesConfigHomeDirSuccess() {
	expectedConfig := &rest.Config{Host: "home-host"}
	loadInClusterConfig = func() (*rest.Config, error) {
		return nil, errors.New("in-cluster error")
	}
	loadConfigFromPath = func(path string) (*rest.Config, error) {
		if path == "/test/kubeconfig" {
			return nil, errors.New("env config error")
		}
		if path == "/home/user/.kube/config" {
			return expectedConfig, nil
		}
		return nil, errors.New("wrong path")
	}

	originalHomeDir := os.Getenv("HOME")
	err := os.Setenv("HOME", "/home/user")
	s.Require().NoError(err)
	defer func() {
		err := os.Setenv("HOME", originalHomeDir)
		s.Require().NoError(err)
	}()

	config, err := GetKubernetesConfig("")

	s.Require().NoError(err)
	s.Require().Equal(expectedConfig, config)
}

func (s *ConfigTestSuite) TestGetKubernetesConfigAllConfigsFail() {
	loadInClusterConfig = func() (*rest.Config, error) {
		return nil, errors.New("in-cluster error")
	}
	loadConfigFromPath = func(_ string) (*rest.Config, error) {
		return nil, errors.New("all configs failed")
	}

	err := os.Setenv("KUBECONFIG", "/test/kubeconfig")
	s.Require().NoError(err)
	defer func() {
		err := os.Unsetenv("KUBECONFIG")
		s.Require().NoError(err)
	}()

	originalHomeDir := os.Getenv("HOME")
	err = os.Setenv("HOME", "/home/user")
	s.Require().NoError(err)
	defer func() {
		err := os.Setenv("HOME", originalHomeDir)
		s.Require().NoError(err)
	}()

	config, err := GetKubernetesConfig("")

	s.Require().Error(err)
	s.Require().Nil(config)
	s.Require().Contains(err.Error(), "error loading kubernetes configuration")
}

func (s *ConfigTestSuite) TestIsInCluster() {
	s.Require().False(IsInCluster())

	loadInClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{Host: "in-cluster-host"}, nil
	}

	s.Require().True(IsInCluster())
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}
