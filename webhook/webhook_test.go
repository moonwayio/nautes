package webhook

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WebhookTestSuite struct {
	suite.Suite
}

func (s *WebhookTestSuite) TestWebhookServer() {
	type testCase struct {
		name      string
		opts      []OptionFunc
		createErr string
		startErr  string
	}

	certFile, keyFile, err := generateCertAndKey(s.T())
	s.Require().NoError(err)

	testCases := []testCase{
		{
			name: "with TLS disabled should succeed",
			opts: []OptionFunc{
				WithTLS(false),
			},
		},
		{
			name: "with TLS enabled but no cert files should return error",
			opts: []OptionFunc{
				WithTLS(true),
			},
			createErr: "certFile is required",
		},
		{
			name: "with TLS enabled and invalid cert file should return error",
			opts: []OptionFunc{
				WithTLS(true),
				WithCertFile("/tmp/cert.pem"),
				WithKeyFile("/tmp/key.pem"),
			},
			startErr: "cert file does not exist",
		},
		{
			name: "with TLS enabled and invalid key file should return error",
			opts: []OptionFunc{
				WithTLS(true),
				WithCertFile(certFile),
				WithKeyFile("/tmp/key.pem"),
			},
			startErr: "key file does not exist",
		},
		{
			name: "with valid TLS enabled and cert files should succeed",
			opts: []OptionFunc{
				WithTLS(true),
				WithCertFile(certFile),
				WithKeyFile(keyFile),
			},
		},
	}

	type testRequest struct {
		path    string
		body    []byte
		status  int
		allowed bool
		message string
	}

	req := AdmissionRequest{
		UID: "test-uid",
		Kind: metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		},
		Resource: metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "pods",
		},
		Operation: "CREATE",
	}

	body, err := json.Marshal(req)
	s.Require().NoError(err)

	requests := []testRequest{
		{
			path:    "/mutate",
			body:    body,
			status:  http.StatusOK,
			allowed: true,
		},
		{
			path:    "/validate",
			body:    body,
			status:  http.StatusOK,
			allowed: false,
			message: "validation failed",
		},
		{
			path:   "/non-existing",
			body:   body,
			status: http.StatusNotFound,
		},
		{
			path:   "/mutate",
			body:   []byte("invalid JSON"),
			status: http.StatusBadRequest,
		},
	}

	for i, tc := range testCases {
		s.Run(tc.name, func() {
			server, err := NewWebhookServer(8443+i, tc.opts...)
			if tc.createErr != "" {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.createErr)
				return
			}
			s.Require().NoError(err)
			s.Require().NotNil(server)

			err = server.Register(
				"/mutate",
				func(_ context.Context, req AdmissionRequest) AdmissionResponse {
					return AdmissionResponse{
						UID:     req.UID,
						Allowed: true,
					}
				},
			)
			s.Require().NoError(err)

			err = server.Register(
				"/validate",
				func(_ context.Context, req AdmissionRequest) AdmissionResponse {
					return AdmissionResponse{
						UID:     req.UID,
						Allowed: false,
						Result: &metav1.Status{
							Message: "validation failed",
						},
					}
				},
			)
			s.Require().NoError(err)

			err = server.Start()
			if tc.startErr != "" {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.startErr)
				return
			}
			s.Require().NoError(err)

			for _, r := range requests {
				httpreq := httptest.NewRequest("POST", r.path, bytes.NewBuffer(r.body))
				httpreq.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				server.(*webhookServer).handleWebhook(w, httpreq)

				s.Equal(r.status, w.Code)

				if r.status == http.StatusOK {
					var resp AdmissionResponse
					s.Require().NoError(json.NewDecoder(w.Body).Decode(&resp))

					s.Equal(r.allowed, resp.Allowed)
					if r.message != "" {
						s.Equal(r.message, resp.Result.Message)
					}
				}
			}

			err = server.Stop()
			s.Require().NoError(err)
		})
	}
}

func (s *WebhookTestSuite) TestGetName() {
	server, err := NewWebhookServer(8443, WithTLS(false))
	s.Require().NoError(err)
	s.Require().Equal("webhook", server.GetName())
}

func generateCertAndKey(t *testing.T) (string, string, error) {
	t.Helper()
	tempDir := t.TempDir()
	certFile := filepath.Join(tempDir, "cert.pem")
	keyFile := filepath.Join(tempDir, "key.pem")

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to generate key")
	}

	cert := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &cert, &cert, &priv.PublicKey, priv)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to create certificate")
	}

	//nolint:gosec // certFile is generated from t.TempDir() which is safe
	certOut, err := os.Create(certFile)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to create cert file")
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return "", "", errors.Wrap(err, "failed to write data to cert file")
	}
	if err := certOut.Close(); err != nil {
		return "", "", errors.Wrap(err, "failed to close cert file")
	}

	//nolint:gosec // keyFile is generated from t.TempDir() which is safe
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to open key file")
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to marshal private key")
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return "", "", errors.Wrap(err, "failed to write data to key file")
	}
	if err := keyOut.Close(); err != nil {
		return "", "", errors.Wrap(err, "failed to close key file")
	}

	return certFile, keyFile, nil
}

func TestWebhookTestSuite(t *testing.T) {
	suite.Run(t, new(WebhookTestSuite))
}
