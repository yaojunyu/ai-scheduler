// +build !ignore_autogenerated

/*
Copyright 2019 The Ai-Scheduler Authors.

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	apiv1 "k8s.io/kubernetes/pkg/scheduler/api/v1"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Pool) DeepCopyInto(out *Pool) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Pool.
func (in *Pool) DeepCopy() *Pool {
	if in == nil {
		return nil
	}
	out := new(Pool)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Pool) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PoolList) DeepCopyInto(out *PoolList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Pool, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PoolList.
func (in *PoolList) DeepCopy() *PoolList {
	if in == nil {
		return nil
	}
	out := new(PoolList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PoolList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PoolSpec) DeepCopyInto(out *PoolSpec) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.SupportResources != nil {
		in, out := &in.SupportResources, &out.SupportResources
		*out = make([]corev1.ResourceName, len(*in))
		copy(*out, *in)
	}
	if in.Weight != nil {
		in, out := &in.Weight, &out.Weight
		*out = make(map[corev1.ResourceName]int32, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Quota != nil {
		in, out := &in.Quota, &out.Quota
		*out = make(corev1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	if in.Priorities != nil {
		in, out := &in.Priorities, &out.Priorities
		*out = make([]apiv1.PriorityPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PoolSpec.
func (in *PoolSpec) DeepCopy() *PoolSpec {
	if in == nil {
		return nil
	}
	out := new(PoolSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PoolStatus) DeepCopyInto(out *PoolStatus) {
	*out = *in
	if in.Deserved != nil {
		in, out := &in.Deserved, &out.Deserved
		*out = make(corev1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	if in.Used != nil {
		in, out := &in.Used, &out.Used
		*out = make(corev1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	if in.Shared != nil {
		in, out := &in.Shared, &out.Shared
		*out = make(corev1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PoolStatus.
func (in *PoolStatus) DeepCopy() *PoolStatus {
	if in == nil {
		return nil
	}
	out := new(PoolStatus)
	in.DeepCopyInto(out)
	return out
}
