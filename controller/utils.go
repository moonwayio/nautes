package controller

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// getGVKForObject extracts the GroupVersionKind from a runtime object.
//
// This function handles both regular Kubernetes objects and partial metadata objects.
// For partial metadata objects, it validates that the GVK is properly populated.
// For regular objects, it uses the runtime scheme to determine the GVK.
//
// The function is used internally by the controller to ensure proper object
// identification and type safety during reconciliation operations.
//
// Parameters:
//   - obj: The runtime object to extract GVK from
//   - scheme: The runtime scheme used for object serialization
//
// Returns:
//   - schema.GroupVersionKind: The extracted group, version, and kind
//   - error: Any error encountered during GVK extraction
func getGVKForObject(obj runtime.Object, scheme *runtime.Scheme) (schema.GroupVersionKind, error) {
	// Check if the object is a partial metadata object
	_, isPartial := obj.(*metav1.PartialObjectMetadata)
	_, isPartialList := obj.(*metav1.PartialObjectMetadataList)
	if isPartial || isPartialList {
		// For partial metadata objects, we require that the GVK be populated
		gvk := obj.GetObjectKind().GroupVersionKind()
		if len(gvk.Kind) == 0 {
			return schema.GroupVersionKind{}, fmt.Errorf(
				"failed to get object kind: %w",
				runtime.NewMissingKindErr(
					"object has no kind",
				),
			)
		}
		if len(gvk.Version) == 0 {
			return schema.GroupVersionKind{}, fmt.Errorf(
				"failed to get object kind: %w",
				runtime.NewMissingVersionErr(
					"object has no version",
				),
			)
		}
		return gvk, nil
	}

	// For regular objects, use the scheme to determine the GVK
	gvks, isUnversioned, err := scheme.ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("failed to get object kind: %w", err)
	}
	if isUnversioned {
		return schema.GroupVersionKind{}, fmt.Errorf(
			"cannot create group-version-kind for unversioned type %T",
			obj,
		)
	}

	if len(gvks) < 1 {
		return schema.GroupVersionKind{}, fmt.Errorf(
			"no group-version-kinds associated with type %T",
			obj,
		)
	}
	if len(gvks) > 1 {
		// This should only trigger for things like metav1.XYZ --
		// normal versioned types should be fine
		return schema.GroupVersionKind{}, fmt.Errorf(
			"multiple group-version-kinds associated with type %T, refusing to guess at one", obj)
	}
	return gvks[0], nil
}
