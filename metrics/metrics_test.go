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

package metrics

import (
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/suite"
)

type MetricsTestSuite struct {
	suite.Suite
}

func (s *MetricsTestSuite) TestRegister() {
	type testCase struct {
		name      string
		collector prometheus.Collector
		err       string
	}

	testCases := []testCase{
		{
			name: "RegisteringCounterShouldSucceed",
			collector: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "test_counter",
				Help: "A test counter",
			}),
			err: "",
		},
		{
			name: "RegisteringGaugeShouldSucceed",
			collector: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "test_gauge",
				Help: "A test gauge",
			}),
			err: "",
		},
		{
			name: "RegisteringHistogramShouldSucceed",
			collector: prometheus.NewHistogram(prometheus.HistogramOpts{
				Name: "test_histogram",
				Help: "A test histogram",
			}),
			err: "",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			server := NewMetricsServer(9090)

			err := server.Register(tc.collector)
			if tc.err != "" {
				s.Require().Error(err, tc.err)
			} else {
				s.Require().NoError(err)
			}

			err = server.Unregister(tc.collector)
			s.Require().NoError(err)
		})
	}
}

func (s *MetricsTestSuite) TestMetricsEndpoint() {
	type testCase struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
		increment      int
	}

	testCases := []testCase{
		{
			name:           "MetricsEndpointShouldReturnPrometheusFormat",
			path:           "/metrics",
			expectedStatus: http.StatusOK,
			expectedBody:   "test_counter 0",
		},
		{
			name:           "MetricsEndpointShouldReturnPrometheusFormatWithValueAt1",
			path:           "/metrics",
			expectedStatus: http.StatusOK,
			expectedBody:   "test_counter 1",
			increment:      1,
		},
	}

	for i, tc := range testCases {
		s.Run(tc.name, func() {
			server := NewMetricsServer(9090 + i)

			// Register a test metric
			counter := prometheus.NewCounter(prometheus.CounterOpts{
				Name: "test_counter",
				Help: "A test counter",
			})
			err := server.Register(counter)
			s.Require().NoError(err)

			// Start the server
			err = server.Start()
			s.Require().NoError(err)

			for i := 0; i < tc.increment; i++ {
				counter.Inc()
			}

			// Make a request to the metrics endpoint
			resp, err := http.Get("http://localhost:" + strconv.Itoa(9090+i) + tc.path)
			s.Require().NoError(err)
			s.Require().Equal(tc.expectedStatus, resp.StatusCode)

			// Read the body
			body, err := io.ReadAll(resp.Body)
			s.Require().NoError(err)
			s.Require().Contains(string(body), tc.expectedBody)
			err = resp.Body.Close()
			s.Require().NoError(err)

			// Stop the server
			err = server.Stop()
			s.Require().NoError(err)

			err = server.Unregister(counter)
			s.Require().NoError(err)
		})
	}
}

func (s *MetricsTestSuite) TestGetName() {
	server := NewMetricsServer(9090)
	s.Equal("metrics", server.GetName())
}

func (s *MetricsTestSuite) TestRegisterDuplicateMetric() {
	server := NewMetricsServer(9090)

	// Register the same metric twice
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_counter",
		Help: "A test counter",
	})

	err := server.Register(counter)
	s.Require().NoError(err)

	// Register the same metric again should fail
	err = server.Register(counter)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "failed to register collector")
}

func (s *MetricsTestSuite) TestMetricsServerIdempotency() {
	server := NewMetricsServer(9000)

	// Test Start
	err := server.Start()
	s.Require().NoError(err)

	// Start again should not error (idempotent)
	err = server.Start()
	s.Require().NoError(err)

	// Test Stop
	err = server.Stop()
	s.Require().NoError(err)

	// Stop again should not error (idempotent)
	err = server.Stop()
	s.Require().NoError(err)
}

func TestMetricsTestSuite(t *testing.T) {
	suite.Run(t, new(MetricsTestSuite))
}
