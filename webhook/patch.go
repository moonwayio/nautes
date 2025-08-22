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
	"encoding/json"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"

	jsondiff "github.com/wI2L/jsondiff"
)

// Diff represents a JSON patch for admission webhook responses.
//
// The Diff struct contains the patch data and type information needed to
// apply changes to Kubernetes resources in admission webhook responses.
// It encapsulates the JSON patch format used by Kubernetes admission controllers.
type Diff struct {
	// Patch contains the JSON patch data in the format specified by RFC 6902
	Patch []byte

	// PatchType specifies the type of patch being applied
	//
	// This field determines how the patch should be interpreted and applied
	// by the Kubernetes API server.
	PatchType admissionv1.PatchType
}

// GetJSONDiff generates a JSON patch between the original and patched objects.
//
// This function creates a JSON patch that represents the differences between
// the original object and the patched object. The patch can be used in
// admission webhook responses to modify resources before they are persisted
// to the cluster.
//
// The function uses the jsondiff library to generate RFC 6902 compliant
// JSON patches that are compatible with Kubernetes admission webhooks.
//
// Parameters:
//   - original: The original JSON representation of the object
//   - patched: The patched runtime object to compare against
//
// Returns:
//   - *Diff: A diff containing the JSON patch and patch type
//   - error: Any error encountered during patch generation
func GetJSONDiff(original []byte, patched runtime.Object) (*Diff, error) {
	// Marshal the patched object to JSON for comparison
	patchedJSON, err := json.Marshal(patched)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patched object: %w", err)
	}

	// Generate the JSON patch using jsondiff
	patch, err := jsondiff.CompareJSON(original, patchedJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to compare JSON: %w", err)
	}

	// Marshal the patch to JSON format
	b, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patch: %w", err)
	}

	return &Diff{
		Patch:     b,
		PatchType: admissionv1.PatchTypeJSONPatch,
	}, nil
}
