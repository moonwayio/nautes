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

package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type HealthTestSuite struct {
	suite.Suite
}

func (s *HealthTestSuite) TestNewHealthCheck() {
	type testCase struct {
		name string
		port int
	}

	testCases := []testCase{
		{
			name: "WithValidPortShouldSucceed",
			port: 8080,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			health := NewHealthCheck(tc.port)
			s.NotNil(health)
		})
	}
}

func (s *HealthTestSuite) TestRegisterReadiness() {
	type testCase struct {
		name  string
		check CheckFunc
	}

	testCases := []testCase{
		{
			name: "RegisteringHealthyReadinessCheckShouldSucceed",
			check: func(_ context.Context) error {
				return nil
			},
		},
		{
			name: "RegisteringUnhealthyReadinessCheckShouldSucceed",
			check: func(_ context.Context) error {
				return errors.New("readiness check failed")
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			health := NewHealthCheck(8080)

			err := health.RegisterReadiness("test", tc.check)
			s.Require().NoError(err)

			err = health.RegisterLiveness("test", tc.check)
			s.Require().NoError(err)
		})
	}
}

func (s *HealthTestSuite) TestStartAndStopHealthCheck() {
	type testCase struct {
		name      string
		readiness CheckFunc
		liveness  CheckFunc
		err       error
	}

	testCases := []testCase{
		{
			name: "HealthCheckWithNoErrorShouldSucceed",
			readiness: func(_ context.Context) error {
				return nil
			},
			liveness: func(_ context.Context) error {
				return nil
			},
		},
		{
			name: "HealthCheckWithErrorShouldFail",
			readiness: func(_ context.Context) error {
				return errors.New("check failed")
			},
			liveness: func(_ context.Context) error {
				return errors.New("check failed")
			},
			err: errors.New("check failed"),
		},
		{
			name: "EmptyHealthCheckShouldSucceed",
			readiness: func(_ context.Context) error {
				return nil
			},
			liveness: func(_ context.Context) error {
				return nil
			},
		},
	}

	for i, tc := range testCases {
		s.Run(tc.name, func() {
			health := NewHealthCheck(8080 + i)

			// Register test checks
			err := health.RegisterReadiness("test", tc.readiness)
			s.Require().NoError(err)

			err = health.RegisterLiveness("test", tc.liveness)
			s.Require().NoError(err)

			// Start the server
			err = health.Start()
			s.Require().NoError(err)

			healthCheck := health.(*healthCheck)

			// Create test request for readiness
			req := httptest.NewRequest("GET", "/ready", nil)
			w := httptest.NewRecorder()

			// Call the handler directly
			healthCheck.handleReadiness(w, req)

			// Check the response
			if tc.err != nil {
				s.Equal(http.StatusServiceUnavailable, w.Code)
			} else {
				s.Equal(http.StatusOK, w.Code)
			}

			// Check the response body
			var response Response
			err = json.Unmarshal(w.Body.Bytes(), &response)
			s.Require().NoError(err)

			if tc.err != nil {
				s.False(response.Healthy)
				s.Equal(map[string]string{"test": "unhealthy: " + tc.err.Error()}, response.Status)
			} else {
				s.True(response.Healthy)
				if tc.readiness != nil {
					s.Equal(map[string]string{"test": "healthy"}, response.Status)
				}
			}

			// Create a test request for liveness
			req = httptest.NewRequest("GET", "/live", nil)
			w = httptest.NewRecorder()

			// Call the handler directly
			healthCheck.handleLiveness(w, req)

			// Check the response
			if tc.err != nil {
				s.Equal(http.StatusServiceUnavailable, w.Code)
			} else {
				s.Equal(http.StatusOK, w.Code)
			}

			// Check the response body
			err = json.Unmarshal(w.Body.Bytes(), &response)
			s.Require().NoError(err)

			if tc.err != nil {
				s.False(response.Healthy)
				s.Equal(map[string]string{"test": "unhealthy: " + tc.err.Error()}, response.Status)
			} else {
				s.True(response.Healthy)
				if tc.liveness != nil {
					s.Equal(map[string]string{"test": "healthy"}, response.Status)
				}
			}

			// Stop the server
			err = health.Stop()
			s.Require().NoError(err)
		})
	}
}

func (s *HealthTestSuite) TestHealthCheckIdempotency() {
	health := NewHealthCheck(8081)

	// Test Start
	err := health.Start()
	s.Require().NoError(err)

	// Start again should not error (idempotent)
	err = health.Start()
	s.Require().NoError(err)

	// Test Stop
	err = health.Stop()
	s.Require().NoError(err)

	// Stop again should not error (idempotent)
	err = health.Stop()
	s.Require().NoError(err)
}

func (s *HealthTestSuite) TestGetName() {
	health := NewHealthCheck(8080)
	s.Equal("health-check", health.GetName())
}

func TestHealthTestSuite(t *testing.T) {
	suite.Run(t, new(HealthTestSuite))
}
