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
	metav1.TypeMeta
	// +optional
	metav1.ObjectMeta

	// Spec defines the desired resources divided
	// +optional
	Spec PoolSpec

	// Status defines the actual enforced deserved resources and its current usage
	// +optional
	Status PoolStatus
}

// PoolSpec
type PoolSpec struct{
	// Selector defines the label selector to collect all nodes matched label
	// +optional
	NodeSelector *metav1.LabelSelector

	// SupportResourceNames defines supported resource types of node
	// +optional
	SupportResourceNames []v1.ResourceName

	// Weight defines the weight of pool size
	// +optional
	Weight int32

	// Quota defines the quota of pool divide by weight/labels/manual
	// +optional
	Quota v1.ResourceList

	// Priorities is scheduler priorities policy
	// +optional
	Priorities []apiv1.PriorityPolicy

	// DisablePreemption flag whether task can preempt resources in the same pool ,
	// if false, task in pool can preempt resources from other pools
	// if true, task cannot preempt resources from other pools and wait available
	// resource in self pool
	// +optional
	DisablePreemption bool

	// DisableBorrowing flag whether task in self pool can borrow resources from other pool,
	// if false, task can borrow resources from other pool
	// if true, task will only can use deserved resources
	// +optional
	DisableBorrowing bool

	// DisableSharing flag if self pool share its resource to other pool,
	// if false, the pool can be preempted by task in other pool
	// if true, the pool will not be preempted.
	// +optional
	DisableSharing bool
}

// PoolStatus
type PoolStatus struct {
	// Deserved all quota of pool divided
	// +optional
	Deserved v1.ResourceList

	// Used  is the current observed total usage of the resource in the Pool
	// +optional
	Used v1.ResourceList

	// Borrowed is the resources that task in self Pool borrows from other Pool
	Borrowed v1.ResourceList
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Pool is a collection of resource pools.
type PoolList struct {
	metav1.TypeMeta
	// Standard list metadata
	// +optional
	metav1.ListMeta

	// items is the list of Pool
	Items []Pool
}