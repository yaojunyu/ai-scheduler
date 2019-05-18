/*
Copyright 2015 The Kubernetes Authors.

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

package cache

import (
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/algorithm"
	schedulerinfo "gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/info"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Cache collects pods' information and provides node-level aggregated information.
// It's intended for generic scheduler to do efficient lookup.
// Cache's operations are pod centric. It does incremental updates based on pod events.
// Pod events are sent via network. We don't have guaranteed delivery of all events:
// We use Reflector to list and watch from remote.
// Reflector might be slow and do a relist, which would lead to missing events.
//
// State Machine of a pod's events in scheduler's cache:
//
//
//   +-------------------------------------------+  +----+
//   |                            Add            |  |    |
//   |                                           |  |    | Update
//   +      Assume                Add            v  v    |
//Initial +--------> Assumed +------------+---> Added <--+
//   ^                +   +               |       +
//   |                |   |               |       |
//   |                |   |           Add |       | Remove
//   |                |   |               |       |
//   |                |   |               +       |
//   +----------------+   +-----------> Expired   +----> Deleted
//         Forget             Expire
//
//
// Note that an assumed pod can expire, because if we haven't received Add event notifying us
// for a while, there might be some problems and we shouldn't keep the pod in cache anymore.
//
// Note that "Initial", "Expired", and "Deleted" pods do not actually exist in cache.
// Based on existing use cases, we are making the following assumptions:
// - No pod would be assumed twice
// - A pod could be added without going through scheduler. In this case, we will see Add but not Assume event.
// - If a pod wasn't added, it wouldn't be removed or updated.
// - Both "Expired" and "Deleted" are valid end states. In case of some problems, e.g. network issue,
//   a pod might have changed its state (e.g. added and deleted) without delivering notification to the cache.
type Cache interface {
	// AssumePod assumes a pod scheduled and aggregates the pod's information into its node.
	// The implementation also decides the policy to expire pod before being confirmed (receiving Add event).
	// After expiration, its information would be subtracted.
	AssumePod(pod *v1.Pod) error

	// FinishBinding signals that cache for assumed pod can be expired
	FinishBinding(pod *v1.Pod) error

	// ForgetPod removes an assumed pod from cache.
	ForgetPod(pod *v1.Pod) error

	// AddPod either confirms a pod if it's assumed, or adds it back if it's expired.
	// If added back, the pod's information would be added again.
	AddPod(pod *v1.Pod) error

	// UpdatePod removes oldPod's information and adds newPod's information.
	UpdatePod(oldPod, newPod *v1.Pod) error

	// RemovePod removes a pod. The pod's information would be subtracted from assigned node.
	RemovePod(pod *v1.Pod) error

	// GetPod returns the pod from the cache with the same namespace and the
	// same name of the specified pod.
	GetPod(pod *v1.Pod) (*v1.Pod, error)

	// IsAssumedPod returns true if the pod is assumed and not expired.
	IsAssumedPod(pod *v1.Pod) (bool, error)

	// AddPool adds overall information about pool
	AddPool(pool *v1alpha1.Pool) error

	// UpdatePool updates overall information about pool
	UpdatePool(oldPool, newPool *v1alpha1.Pool) error

	// RemovePool removes overall information about pool
	RemovePool(pool *v1alpha1.Pool) error

	// Pools return the pools
	Pools() map[string]*schedulerinfo.PoolInfo

	// NumPools return the number of pools
	NumPools() int

	// NumNodes return the number of nodes
	NumNodes() int

	// GetPool return pool by name
	GetPool(poolName string) (*schedulerinfo.PoolInfo, error)

	// AddNode adds overall information about node.
	AddNode(node *v1.Node) error

	// UpdateNode updates overall information about node.
	UpdateNode(oldNode, newNode *v1.Node) error

	// RemoveNode removes overall information about node.
	RemoveNode(node *v1.Node) error

	GetPoolContainsNode(nodeName string) *schedulerinfo.PoolInfo

	// UpdateNodeInfoSnapshot updates the passed infoSnapshot to the current contents of Cache.
	// The node info contains aggregated information of pods scheduled (including assumed to be)
	// on this node.
	UpdateNodeInfoSnapshot(poolName string /*, nodeSnapshot *schedulerinfo.NodeInfoSnapshot*/) error

	// List lists all cached pods (including assumed ones).
	List(labels.Selector) ([]*v1.Pod, error)

	// FilteredList returns all cached pods that pass the filter.
	FilteredList(filter algorithm.PodFilter, selector labels.Selector) ([]*v1.Pod, error)

	// Snapshot takes a snapshot on current cache
	Snapshot() *schedulerinfo.Snapshot

	NodeInfoSnapshot(poolName string) *schedulerinfo.NodeInfoSnapshot

	// NodeTree returns a node tree structure
	NodeTree(poolName string) *schedulerinfo.NodeTree

	// DeserveAllPools compute all pools' deserved resources  quota
	DeserveAllPools() error

	// TotalAllocatableResource calculate all allocatable resources
	TotalAllocatableResource() *schedulerinfo.Resource

	// BorrowPool get pool to borrow
	BorrowPool(fromPoolName string, pod *v1.Pod) string
}
