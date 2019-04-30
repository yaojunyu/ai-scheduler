package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/kubernetes/pkg/scheduler/api/v1"
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
type PoolSpec struct{
	// NodeSelector match node label
	// +optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`
	// supportResources match nodes have supported resources
	// +optional
	SupportResources []v1.ResourceName `json:"supportResources,omitempty"`

	// Weight defines the weight of resources size in pool
	// +optional
	Weight map[v1.ResourceName]int32 `json:"weight,omitempty"`

	// Quota defines the quota of pool divide by weight/labels/manual
	// +optional
	Quota v1.ResourceList `json:"quota,omitempty"`

	// Priorities is scheduler priorities policy
	// +optional
	Priorities []apiv1.PriorityPolicy `json:"priorities,omitempty"`

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

	// DisableSharing flag if self pool share its resource to other pool,
	// if false, the pool can be preempted by task in other pool
	// if true, the pool will not be preempted.
	// +optional
	DisableSharing bool `json:"disableSharing,omitempty"`
}

// PoolStatus
type PoolStatus struct {
	// Deserved all quota of pool divided
	// +optional
	Deserved v1.ResourceList `json:"deserved,omitempty"`

	// Used  is the current observed total usage of the resource by tasks in the Pool
	// +optional
	Used v1.ResourceList `json:"used,omitempty"`

	// Borrowed is the resources that task in self Pool borrows from other Pool
	Shared v1.ResourceList `json:"borrowed,omitempty"`
}

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