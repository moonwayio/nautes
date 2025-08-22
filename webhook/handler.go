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

package webhook

import (
	"context"

	admissionv1 "k8s.io/api/admission/v1"
)

// HandlerFunc defines the signature for admission webhook request handlers.
//
// A HandlerFunc processes admission webhook requests and returns admission
// responses. The function receives the admission request context and request
// data, and must return an appropriate admission response indicating whether
// the request should be allowed or denied.
//
// The function should be concurrency safe and handle errors gracefully.
// It should return quickly to avoid blocking the admission process.
//
// Parameters:
//   - ctx: Context for the request, may be cancelled
//   - req: The admission request containing the resource being processed
//
// Returns:
//   - AdmissionResponse: The response indicating whether the request is allowed
type HandlerFunc func(ctx context.Context, req AdmissionRequest) AdmissionResponse

// AdmissionRequest represents an admission webhook request.
//
// This type is an alias for admissionv1.AdmissionRequest and contains
// information about the resource being processed by the admission webhook.
// It includes the resource object, operation type, and user information.
type AdmissionRequest = admissionv1.AdmissionRequest

// AdmissionResponse represents an admission webhook response.
//
// This type is an alias for admissionv1.AdmissionResponse and contains
// the decision made by the admission webhook. It indicates whether the
// request should be allowed or denied, and may include additional
// information such as patches or warnings.
type AdmissionResponse = admissionv1.AdmissionResponse
