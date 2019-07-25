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
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired resources divided
	// +optional
	Spec PoolSpec `json:"spec,omitempty"`

	// Status defines the actual enforced deserved resources and its current usage
	// +optional
	Status PoolStatus `json:"status,omitempty"`
}

// PoolSpec
type PoolSpec struct {
	// NodeSelector match node label
	// +optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`

	// DisablePreemption flag whether task can preempt resources in the same pool ,
	// if false, task in pool can preempt resources from other pools
	// if true, task cannot preempt resources from other pools and wait available
	// resource in self pool
	// +optional
	DisablePreemption bool `json:"disablePreemption,omitempty"`

	// DisableBorrowing flag whether task in self pool can borrow resources from other pool,
	// if false, task can borrow resources from other pool
	// if true, task will only can use deserved resources
	// +optional
	DisableBorrowing bool `json:"disableBorrowing,omitempty"`

	// BorrowingPools only borrow from those pools,
	// only available when DisableBorrowing is false,
	// if empty can borrow all sharing pools
	// +optional
	BorrowingPools []string `json:"borrowingPools,omitempty"`

	// DisableSharing flag if self pool share its resource to other pool,
	// if false, the pool can be preempted by task in other pool
	// if true, the pool will not be preempted.
	// +optional
	DisableSharing bool `json:"disableSharing,omitempty"`
}

// PoolStatus
type PoolStatus struct {
	// Capacity all resources of pool nodes capacity sum
	// +optional
	Capacity v1.ResourceList `json:"capacity,omitempty"`

	// Allocatable all quota of pool divided
	// +optional
	Allocatable v1.ResourceList `json:"allocatable,omitempty"`

	// Requested  is the current observed total usage of the resource by tasks in the Pool
	// Requested = (Deserved + System + Shared)
	// +optional
	Requested v1.ResourceList `json:"requested,omitempty"`

	// Deserved is the resource used by self pool task
	// +optional
	Deserved v1.ResourceList `json:"deserved,omitempty"`

	// System is the resource used by scheduled by not ai-scheduler, i.e.calico,
	// +optional
	System v1.ResourceList `json:"system,omitempty"`

	// shared is the resources that shared to tasks in others Pool
	// +optional
	Shared v1.ResourceList `json:"shared,omitempty"`

	// Borrowed is the resources that task in self Pool borrows from other Pool
	// +optional
	Borrowed v1.ResourceList `json:"borrowed,omitempty"`

	// Free = (Allocatable - allocated)
	Free v1.ResourceList `json:"free,omitempty"`

	// Phase
	PoolQueuePhase PoolQueuePhase
}

type PoolQueuePhase string
const (
	PoolPending PoolQueuePhase = "Pending"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Pool is a collection of resource pools.
type PoolList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// items is the list of Pool
	Items []Pool `json:"items"`
}
