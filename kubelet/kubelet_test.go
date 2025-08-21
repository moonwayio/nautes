package kubelet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type KubeletTestSuite struct {
	suite.Suite
}

func (s *KubeletTestSuite) TestNewKubeletClient() {
	type testCase struct {
		name string
		opts []OptionFunc
		err  string
	}

	tests := []testCase{
		{
			name: "WithNoOptions",
			opts: []OptionFunc{},
			err:  "rest config is required",
		},
		{
			name: "WithRestConfig",
			opts: []OptionFunc{
				WithRestConfig(&rest.Config{}),
			},
		},
		{
			name: "WithAllOptions",
			opts: []OptionFunc{
				WithRestConfig(&rest.Config{}),
				WithScheme("http"),
				WithInsecure(true),
				WithPriority(
					[]corev1.NodeAddressType{corev1.NodeInternalIP, corev1.NodeExternalIP},
				),
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			client, err := NewKubeletClient(tc.opts...)
			if tc.err != "" {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.err)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(client)
			}
		})
	}
}

func (s *KubeletTestSuite) TestGet() {
	type testCase struct {
		name      string
		handler   http.HandlerFunc
		addresses []corev1.NodeAddress
		err       string
		body      []byte
	}

	tests := []testCase{
		{
			name: "WithResponseOkShouldSucceed",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`ok`))
			},
			body: []byte(`ok`),
		},
		{
			name: "WithResponseNotOkShouldReturnError",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`error`))
			},
			err: "request failed",
		},
		{
			name: "WithNoAddressShouldReturnError",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`ok`))
			},
			err:       "no address found for node",
			addresses: []corev1.NodeAddress{},
		},
		{
			name: "WithRequestFailureShouldReturnError",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`ok`))
			},
			err: "failed to execute request",
			addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: "127.127.127.127",
				},
			},
		},
	}

	for _, tc := range tests {
		srv := httptest.NewServer(tc.handler)
		defer srv.Close()

		url, err := url.Parse(srv.URL)
		s.Require().NoError(err)

		client, err := NewKubeletClient(WithRestConfig(&rest.Config{
			Transport: srv.Client().Transport,
			Timeout:   time.Millisecond * 100,
		}), WithScheme(url.Scheme))
		s.Require().NoError(err)
		s.Require().NotNil(client)

		port, err := strconv.ParseInt(url.Port(), 10, 32)
		s.Require().NoError(err)

		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Status: corev1.NodeStatus{
				DaemonEndpoints: corev1.NodeDaemonEndpoints{
					KubeletEndpoint: corev1.DaemonEndpoint{
						Port: int32(port),
					},
				},
				Addresses: []corev1.NodeAddress{
					{
						Type:    corev1.NodeInternalIP,
						Address: url.Hostname(),
					},
				},
			},
		}

		if tc.addresses != nil {
			node.Status.Addresses = tc.addresses
		}

		body, err := client.Get(context.Background(), node, "/healthz")
		if tc.err != "" {
			s.Require().Error(err)
			s.Require().ErrorContains(err, tc.err)
		} else {
			s.Require().NoError(err)
			s.Require().Equal(tc.body, body)
		}
	}
}

func TestKubeletTestSuite(t *testing.T) {
	suite.Run(t, new(KubeletTestSuite))
}
