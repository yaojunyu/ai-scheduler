package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Pool
type Pool struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the desired resources divided
	// +optional
	Spec PoolSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	// Status defines the actual enforced deserved resources and its current usage
	// +optional
	Status PoolStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// PoolSpec
type PoolSpec struct {
	// NodeSelector match node label
	// +optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty" protobuf:"bytes,1,opt,name=nodeSelector"`

	// DisablePreemption flag whether task can preempt resources in the same pool ,
	// if false, task in pool can preempt resources from other pools
	// if true, task cannot preempt resources from other pools and wait available
	// resource in self pool
	// +optional
	DisablePreemption bool `json:"disablePreemption,omitempty" protobuf:"varint,2,opt,name=disablePreemption"`

	// DisableBorrowing flag whether task in self pool can borrow resources from other pool,
	// if false, task can borrow resources from other pool
	// if true, task will only can use deserved resources
	// +optional
	DisableBorrowing bool `json:"disableBorrowing,omitempty" protobuf:"varint,3,opt,name=disableBorrowing"`

	// BorrowingPools only borrow from those pools,
	// only available when DisableBorrowing is false,
	// if empty can borrow all sharing pools
	// +optional
	BorrowingPools []string `json:"borrowingPools,omitempty" protobuf:"bytes,4,opt,name=borrowingPools"`

	// DisableSharing flag if self pool share its resource to other pool,
	// if false, the pool can be preempted by task in other pool
	// if true, the pool will not be preempted.
	// +optional
	DisableSharing bool `json:"disableSharing,omitempty" protobuf:"varint,5,opt,name=disableSharing"`
}

// PoolStatus
type PoolStatus struct {
	// Capacity all resources of pool nodes capacity sum
	// +optional
	Capacity v1.ResourceList `json:"capacity,omitempty" protobuf:"bytes,1,opt,name=capacity,casttype=ResourceList,castkey=ResourceName"`

	// Allocatable all quota of pool divided
	// +optional
	Allocatable v1.ResourceList `json:"allocatable,omitempty" protobuf:"bytes,2,opt,name=allocatable,casttype=ResourceList,castkey=ResourceName"`

	// Requested  is the current observed total usage of the resource by tasks in the Pool
	// Requested = (Deserved + System + Shared)
	// +optional
	Requests v1.ResourceList `json:"requests,omitempty" protobuf:"bytes,3,opt,name=requests,casttype=ResourceList,castkey=ResourceName"`
	Limits   v1.ResourceList `json:"limits,omitempty" protobuf:"bytes,4,rep,name=limits,casttype=ResourceList,castkey=ResourceName"`

	// Deserved is the resource used by self pool task
	// +optional
	Deserved v1.ResourceList `json:"deserved,omitempty" protobuf:"bytes,5,opt,name=deserved,casttype=ResourceList,castkey=ResourceName"`

	// System is the resource used by scheduled by not ai-scheduler, i.e.calico,
	// +optional
	System v1.ResourceList `json:"system,omitempty" protobuf:"bytes,6,opt,name=system,casttype=ResourceList,castkey=ResourceName"`

	// shared is the resources that shared to tasks in others Pool
	// +optional
	Shared v1.ResourceList `json:"shared,omitempty" protobuf:"bytes,7,opt,name=shared,casttype=ResourceList,castkey=ResourceName"`

	// Borrowed is the resources that task in self Pool borrows from other Pool
	// +optional
	Borrowed v1.ResourceList `json:"borrowed,omitempty" protobuf:"bytes,8,opt,name=borrowed,casttype=ResourceList,castkey=ResourceName"`

	// Free = (Allocatable - allocated)
	Free v1.ResourceList `json:"free,omitempty" protobuf:"bytes,9,opt,name=free,casttype=ResourceList,castkey=ResourceName"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Pool is a collection of resource pools.
type PoolList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// items is the list of Pool
	Items []Pool `json:"items" protobuf:"bytes,2,rep,name=items"`
}
