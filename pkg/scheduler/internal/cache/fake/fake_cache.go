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

package fake

import (
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/algorithm"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/info"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Cache is used for testing
type Cache struct {
	AssumeFunc       func(*v1.Pod)
	ForgetFunc       func(*v1.Pod)
	IsAssumedPodFunc func(*v1.Pod) bool
	GetPodFunc       func(*v1.Pod) *v1.Pod
}

// AssumePod is a fake method for testing.
func (c *Cache) AssumePod(pod *v1.Pod) error {
	c.AssumeFunc(pod)
	return nil
}

// FinishBinding is a fake method for testing.
func (c *Cache) FinishBinding(pod *v1.Pod) error { return nil }

// ForgetPod is a fake method for testing.
func (c *Cache) ForgetPod(pod *v1.Pod) error {
	c.ForgetFunc(pod)
	return nil
}

// AddPod is a fake method for testing.
func (c *Cache) AddPod(pod *v1.Pod) error { return nil }

// UpdatePod is a fake method for testing.
func (c *Cache) UpdatePod(oldPod, newPod *v1.Pod) error { return nil }

// RemovePod is a fake method for testing.
func (c *Cache) RemovePod(pod *v1.Pod) error { return nil }

// IsAssumedPod is a fake method for testing.
func (c *Cache) IsAssumedPod(pod *v1.Pod) (bool, error) {
	return c.IsAssumedPodFunc(pod), nil
}

// GetPod is a fake method for testing.
func (c *Cache) GetPod(pod *v1.Pod) (*v1.Pod, error) {
	return c.GetPodFunc(pod), nil
}

// AddNode is a fake method for testing.
func (c *Cache) AddNode(node *v1.Node) error { return nil }

// UpdateNode is a fake method for testing.
func (c *Cache) UpdateNode(oldNode, newNode *v1.Node) error { return nil }

// RemoveNode is a fake method for testing.
func (c *Cache) RemoveNode(node *v1.Node) error { return nil }

// UpdateNodeInfoSnapshot is a fake method for testing.
func (c *Cache) UpdateNodeInfoSnapshot(poolName string) error {
	return nil
}

// List is a fake method for testing.
func (c *Cache) List(s labels.Selector) ([]*v1.Pod, error) { return nil, nil }

// FilteredList is a fake method for testing.
func (c *Cache) FilteredList(filter algorithm.PodFilter, selector labels.Selector) ([]*v1.Pod, error) {
	return nil, nil
}

// Snapshot is a fake method for testing
func (c *Cache) Snapshot() *info.Snapshot {
	return &info.Snapshot{}
}

// NodeTree is a fake method for testing.
func (c *Cache) NodeTree(poolName string) *info.NodeTree { return nil }

func (c *Cache) AddPool(pool *v1alpha1.Pool) error { return nil }

func (c *Cache) UpdatePool(oldPool, newPool *v1alpha1.Pool) error { return nil }

func (c *Cache) RemovePool(pool *v1alpha1.Pool) error { return nil }

func (c *Cache) Pools() map[string]*info.PoolInfo { return nil }

func (c *Cache) NumPools() int { return 0 }

func (c *Cache) NumNodes() int { return 0 }

func (c *Cache) GetPool(poolName string) (*info.PoolInfo, error) { return nil, nil }

func (c *Cache) DeserveAllPools() error { return nil }

func (c *Cache) TotalAllocatableResource() *info.Resource { return nil }

func (c *Cache) NodeInfoSnapshot(poolName string) *info.NodeInfoSnapshot { return nil }
