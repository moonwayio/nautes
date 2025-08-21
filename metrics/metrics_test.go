package metrics

import (
	"io"
	"net/http"
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
			name: "registering counter should succeed",
			collector: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "test_counter",
				Help: "A test counter",
			}),
			err: "",
		},
		{
			name: "registering gauge should succeed",
			collector: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "test_gauge",
				Help: "A test gauge",
			}),
			err: "",
		},
		{
			name: "registering histogram should succeed",
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
			name:           "metrics endpoint should return prometheus format",
			path:           "/metrics",
			expectedStatus: http.StatusOK,
			expectedBody:   "test_counter 0",
		},
		{
			name:           "metrics endpoint should return prometheus format with value at 1",
			path:           "/metrics",
			expectedStatus: http.StatusOK,
			expectedBody:   "test_counter 1",
			increment:      1,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			server := NewMetricsServer(9090)

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
			resp, err := http.Get("http://localhost:9090" + tc.path)
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
	server := NewMetricsServer(9090)

	// Test Start idempotency
	err := server.Start()
	s.Require().NoError(err)

	// Start again should not error (idempotent)
	err = server.Start()
	s.Require().NoError(err)

	// Test Stop idempotency
	err = server.Stop()
	s.Require().NoError(err)

	// Stop again should not error (idempotent)
	err = server.Stop()
	s.Require().NoError(err)
}

func TestMetricsTestSuite(t *testing.T) {
	suite.Run(t, new(MetricsTestSuite))
}
