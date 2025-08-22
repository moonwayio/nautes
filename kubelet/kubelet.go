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

// Package kubelet provides functionality for direct communication with Kubernetes node kubelets.
//
// The kubelet package implements a client for making direct API calls to Kubernetes
// node kubelets. This is useful for scenarios where you need to access kubelet-specific
// endpoints that are not available through the standard Kubernetes API server.
package kubelet

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

// NodeKubeletClient provides a client for direct API calls to a node's kubelet.
//
// The NodeKubeletClient interface provides methods to make HTTP requests directly
// to Kubernetes node kubelets. This is useful for accessing kubelet-specific
// endpoints such as metrics, logs, and debugging information that are not
// available through the standard Kubernetes API server.
//
// Implementations of this interface are concurrency safe and can be used concurrently
// from multiple goroutines.
type NodeKubeletClient interface {
	// Get makes a GET request to the kubelet API on the specified node.
	//
	// The method constructs the appropriate URL for the kubelet endpoint and
	// makes an HTTP GET request. It uses the node's address and kubelet port
	// to determine the target URL.
	//
	// Parameters:
	//   - ctx: Context for the request, may be cancelled
	//   - node: The Kubernetes node to query
	//   - path: The kubelet API path to request (e.g., "/stats/summary", "/pods")
	//
	// Returns:
	//   - []byte: The response body as bytes
	//   - error: Any error encountered during the request
	Get(ctx context.Context, node *corev1.Node, path string) ([]byte, error)
}

// kubeletClient implements the KubeletClient interface.
//
// The kubelet client maintains HTTP client configuration and provides methods
// for making direct requests to node kubelets. It uses buffer pooling for
// efficient memory usage and supports both secure and insecure connections.
type nodeKubeletClient struct {
	opts    options
	client  *http.Client
	buffers sync.Pool
}

// NewKubeletClient creates a new Kubelet client with the provided options.
//
// The client is initialized with the specified configuration and can be customized
// using option functions. The returned client is ready to make requests to node
// kubelets.
//
// Parameters:
//   - opts: Optional configuration functions to customize the client behavior
//
// Returns:
//   - NodeKubeletClient: A new kubelet client instance ready for use
//   - error: Any error encountered during initialization
func NewKubeletClient(opts ...OptionFunc) (NodeKubeletClient, error) {
	o := options{}

	for _, opt := range opts {
		opt(&o)
	}

	if err := o.setDefaults(); err != nil {
		return nil, fmt.Errorf("failed to set defaults: %w", err)
	}

	transport, err := rest.TransportFor(o.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	if httpTransport, ok := transport.(*http.Transport); ok {
		if httpTransport.TLSClientConfig == nil {
			httpTransport.TLSClientConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
		}
		httpTransport.TLSClientConfig.InsecureSkipVerify = o.insecure
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   o.config.Timeout,
	}

	return &nodeKubeletClient{
		opts:   o,
		client: client,
		buffers: sync.Pool{
			New: func() any {
				buf := make([]byte, 10e3)
				return &buf
			},
		},
	}, nil
}

// Get makes a GET request to the kubelet API on the specified node.
//
// The method constructs the appropriate URL for the kubelet endpoint using the
// node's address and kubelet port. It makes an HTTP GET request and returns
// the response body as bytes.
//
// The method handles URL construction, request creation, and response processing.
// It uses the client's HTTP configuration including TLS settings and timeouts.
//
// Parameters:
//   - ctx: Context for the request, may be cancelled
//   - node: The Kubernetes node to query. Must not be nil and must have valid status.
//   - path: The kubelet API path to request. Must start with "/".
//
// Returns:
//   - []byte: The response body as bytes
//   - error: Any error encountered during the request, including network errors,
//     authentication errors, or invalid node configuration
func (k *nodeKubeletClient) Get(
	ctx context.Context,
	node *corev1.Node,
	path string,
) ([]byte, error) {
	port := int(node.Status.DaemonEndpoints.KubeletEndpoint.Port)

	addr, err := k.getAddress(node)
	if err != nil {
		return nil, fmt.Errorf("failed to get address: %w", err)
	}

	url := url.URL{
		Scheme: k.opts.scheme,
		Host:   net.JoinHostPort(addr, strconv.Itoa(port)),
		Path:   path,
	}

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	response, err := k.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			// Ignore close errors as this is a cleanup operation
			_ = closeErr
		}
	}()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed, status: %q", response.Status)
	}

	bp, ok := k.buffers.Get().(*[]byte)
	if !ok {
		return nil, errors.New("failed to get buffer from pool")
	}
	b := *bp
	defer func() {
		*bp = b
		k.buffers.Put(bp)
	}()
	buf := bytes.NewBuffer(b)
	buf.Reset()
	_, err = io.Copy(buf, response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body - %v", err)
	}
	b = buf.Bytes()
	return b, nil
}

// getAddress retrieves the preferred network address for a node.
//
// This method iterates through the configured address priority list and
// returns the first available address of the preferred type for the given node.
// It searches through the node's status addresses to find a matching address type.
//
// The method is used internally by the Get method to determine which address
// to use when connecting to the node's kubelet.
//
// Parameters:
//   - node: The Kubernetes node to get the address for
//
// Returns:
//   - string: The preferred network address for the node
//   - error: An error if no suitable address is found for the node
func (k *nodeKubeletClient) getAddress(node *corev1.Node) (string, error) {
	// Search through address types in priority order
	for _, addrType := range k.opts.priority {
		for _, addr := range node.Status.Addresses {
			if addr.Type == addrType {
				return addr.Address, nil
			}
		}
	}

	return "", fmt.Errorf("no address found for node %s", node.Name)
}
