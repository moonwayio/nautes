package controller

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func getGVKForObject(obj runtime.Object, scheme *runtime.Scheme) (schema.GroupVersionKind, error) {
	_, isPartial := obj.(*metav1.PartialObjectMetadata)
	_, isPartialList := obj.(*metav1.PartialObjectMetadataList)
	if isPartial || isPartialList {
		// we require that the GVK be populated in order to recognize the object
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
		// this should only trigger for things like metav1.XYZ --
		// normal versioned types should be fine
		return schema.GroupVersionKind{}, fmt.Errorf(
			"multiple group-version-kinds associated with type %T, refusing to guess at one", obj)
	}
	return gvks[0], nil
}
