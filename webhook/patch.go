package webhook

import (
	"encoding/json"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"

	jsondiff "github.com/wI2L/jsondiff"
)

// Diff represents a JSON patch.
type Diff struct {
	Patch     []byte
	PatchType admissionv1.PatchType
}

// GetJSONDiff returns a JSON diff between the original and patched objects for the given admission request.
func GetJSONDiff(original []byte, patched runtime.Object) (*Diff, error) {
	patchedJSON, err := json.Marshal(patched)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patched object: %w", err)
	}

	patch, err := jsondiff.CompareJSON(original, patchedJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to compare JSON: %w", err)
	}

	b, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patch: %w", err)
	}

	return &Diff{
		Patch:     b,
		PatchType: admissionv1.PatchTypeJSONPatch,
	}, nil
}
