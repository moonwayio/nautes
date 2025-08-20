// Package kubelet provides functionality for direct communication with Kubernetes node kubelets.
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
type NodeKubeletClient interface {
	Get(ctx context.Context, node *corev1.Node, path string) ([]byte, error)
}

// kubeletClient implements the KubeletClient interface.
type nodeKubeletClient struct {
	opts    options
	client  *http.Client
	buffers sync.Pool
}

// NewKubeletClient creates a new Kubelet client.
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

// Get makes a GET request to the kubelet API.
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

func (k *nodeKubeletClient) getAddress(node *corev1.Node) (string, error) {
	for _, addrType := range k.opts.priority {
		for _, addr := range node.Status.Addresses {
			if addr.Type == addrType {
				return addr.Address, nil
			}
		}
	}

	return "", fmt.Errorf("no address found for node %s", node.Name)
}
