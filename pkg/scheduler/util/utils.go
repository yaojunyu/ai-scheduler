/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"code.aibee.cn/platform/ai-scheduler/pkg/scheduler/info"
	"sort"

	"k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/util/feature"
	"k8s.io/kubernetes/pkg/apis/scheduling"
	"k8s.io/kubernetes/pkg/features"
)

// GetContainerPorts returns the used host ports of Pods: if 'port' was used, a 'port:true' pair
// will be in the result; but it does not resolve port conflict.
func GetContainerPorts(pods ...*v1.Pod) []*v1.ContainerPort {
	var ports []*v1.ContainerPort
	for _, pod := range pods {
		for j := range pod.Spec.Containers {
			container := &pod.Spec.Containers[j]
			for k := range container.Ports {
				ports = append(ports, &container.Ports[k])
			}
		}
	}
	return ports
}

// PodPriorityEnabled indicates whether pod priority feature is enabled.
func PodPriorityEnabled() bool {
	return feature.DefaultFeatureGate.Enabled(features.PodPriority)
}

// GetPodFullName returns a name that uniquely identifies a pod.
func GetPodFullName(pod *v1.Pod) string {
	// Use underscore as the delimiter because it is not allowed in pod name
	// (DNS subdomain format).
	return pod.Name + "_" + pod.Namespace
}

// GetPodPriority return priority of the given pod.
func GetPodPriority(pod *v1.Pod) int32 {
	if pod.Spec.Priority != nil {
		return *pod.Spec.Priority
	}
	// When priority of a running pod is nil, it means it was created at a time
	// that there was no global default priority class and the priority class
	// name of the pod was empty. So, we resolve to the static default priority.
	return scheduling.DefaultPriorityWhenNoDefaultClassExists
}

// SortableList is a list that implements sort.Interface.
type SortableList struct {
	Items    []interface{}
	CompFunc LessFunc
}

// LessFunc is a function that receives two items and returns true if the first
// item should be placed before the second one when the list is sorted.
type LessFunc func(item1, item2 interface{}) bool

var _ = sort.Interface(&SortableList{})

func (l *SortableList) Len() int { return len(l.Items) }

func (l *SortableList) Less(i, j int) bool {
	return l.CompFunc(l.Items[i], l.Items[j])
}

func (l *SortableList) Swap(i, j int) {
	l.Items[i], l.Items[j] = l.Items[j], l.Items[i]
}

// Sort sorts the items in the list using the given CompFunc. Item1 is placed
// before Item2 when CompFunc(Item1, Item2) returns true.
func (l *SortableList) Sort() {
	sort.Sort(l)
}

// HigherPriorityPod return true when priority of the first pod is higher than
// the second one. It takes arguments of the type "interface{}" to be used with
// SortableList, but expects those arguments to be *v1.Pod.
func HigherPriorityPod(pod1, pod2 interface{}) bool {
	return GetPodPriority(pod1.(*v1.Pod)) > GetPodPriority(pod2.(*v1.Pod))
}

// SelfPoolHasHigherPriorityFunc
func SelfPoolHasHigherPriorityFunc(poolName string) LessFunc {
	selfPoolHasHigherPriority := func(pod1, pod2 interface{}) bool {
		p1 := pod1.(*v1.Pod)
		p2 := pod2.(*v1.Pod)
		pool1 := info.GetPodAnnotationsPoolName(p1)
		pool2 := info.GetPodAnnotationsPoolName(p2)
		if pool1 == poolName && pool2 != poolName {
			return true
		} else if pool1 != poolName && pool2 == poolName {
			return false
		}
		return GetPodPriority(p1) > GetPodPriority(p2)
	}
	return selfPoolHasHigherPriority
}

// responsibleForPod returns true if the pod has asked to be scheduled by the given scheduler.
func ResponsibleForPod(pod *v1.Pod, schedulerName string) bool {
	return schedulerName == pod.Spec.SchedulerName
}
