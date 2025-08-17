// Package webhook provides Kubernetes admission webhook functionality.
package webhook

import (
	"context"

	admissionv1 "k8s.io/api/admission/v1"
)

// HandlerFunc handles admission webhook requests.
type HandlerFunc func(ctx context.Context, req AdmissionRequest) AdmissionResponse

// AdmissionRequest represents an admission webhook request.
type AdmissionRequest = admissionv1.AdmissionRequest

// AdmissionResponse represents an admission webhook response.
type AdmissionResponse = admissionv1.AdmissionResponse
