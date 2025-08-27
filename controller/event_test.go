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

package controller

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

func (s *ControllerTestSuite) TestEventHandler() {
	queue := workqueue.NewTyped[Delta[*corev1.Pod]]()
	handler := NewEventHandler(queue, nil, nil, klog.Background())

	handler.OnAdd(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-1",
		},
	}, false)

	handler.OnUpdate(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-2",
		},
	}, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-2",
		},
	})

	handler.OnDelete(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-3",
		},
	})

	s.Require().Equal(3, queue.Len())

	item, _ := queue.Get()
	s.Require().Equal("pod-1", item.Object.Name)

	item, _ = queue.Get()
	s.Require().Equal("pod-2", item.Object.Name)

	item, _ = queue.Get()
	s.Require().Equal("pod-3", item.Object.Name)
}

func (s *ControllerTestSuite) TestEventHandlerWithNotMatchingType() {
	queue := workqueue.NewTyped[Delta[*corev1.Pod]]()
	handler := NewEventHandler(queue, nil, nil, klog.Background())

	handler.OnAdd(corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-1",
		},
	}, false)

	s.Require().Equal(0, queue.Len())
}
