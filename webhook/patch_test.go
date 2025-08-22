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
	"testing"

	"github.com/stretchr/testify/suite"
	jsondiff "github.com/wI2L/jsondiff"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type PatchTestSuite struct {
	suite.Suite
}

func (s *PatchTestSuite) TestGetJSONDiff() {
	type testCase struct {
		name   string
		obj    runtime.Object
		modify func(obj runtime.Object)
		patch  jsondiff.Patch
	}

	testCases := []testCase{
		{
			name: "WithNoDiffShouldReturnNil",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
		},
		{
			name: "WithDiffShouldReturnPatch",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			modify: func(obj runtime.Object) {
				pod := obj.(*corev1.Pod)
				pod.Namespace = "test"
			},
			patch: jsondiff.Patch{
				jsondiff.Operation{
					Type:  "add",
					Path:  "/metadata/namespace",
					Value: "test",
				},
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			newObj := tc.obj.DeepCopyObject()
			if tc.modify != nil {
				tc.modify(newObj)
			}

			oldJSON, err := json.Marshal(tc.obj)
			s.Require().NoError(err)

			diff, err := GetJSONDiff(oldJSON, newObj)
			s.Require().NoError(err)

			var patch jsondiff.Patch
			err = json.Unmarshal(diff.Patch, &patch)
			s.Require().NoError(err)

			s.Equal(admissionv1.PatchTypeJSONPatch, diff.PatchType)
			s.Equal(tc.patch, patch)
		})
	}
}

func TestPatchTestSuite(t *testing.T) {
	suite.Run(t, new(PatchTestSuite))
}
