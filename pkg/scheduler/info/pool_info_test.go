package info

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"reflect"
	"testing"
)

func TestNewPoolInfo(t *testing.T) {
	poolName := ""
	expected := &PoolInfo{
		name: "",
		allocatable: &Resource{},
		used:        &Resource{},
		shared:      &Resource{},

		nodes: sets.NewString(),
	}

	poolInfo := NewPoolInfo()
	if poolName != expected.name {
		t.Errorf("expected: %#v, got: %#v", expected.name, poolName)
	}
	if !reflect.DeepEqual(poolInfo, expected) {
		t.Errorf("expected: %#v, got: %#v", expected, poolInfo)
	}
}

func TestPoolInfoAddNode(t *testing.T) {
	nodeName := "test-node"
	testNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(1000, resource.BinarySI),
				"pods":            *resource.NewQuantity(100, resource.DecimalSI),
			},
		},
	}
	expected := &PoolInfo{
		name:        "",
		nodes: sets.NewString(nodeName),

		allocatable: &Resource{
			MilliCPU: 1000,
			Memory: 1000,
			AllowedPodNumber: 100,
		},
		used:        &Resource{},
		shared:      &Resource{},

	}

	testPoolInfo := NewPoolInfo()
	testPoolInfo.AddNode(testNode)
	if !testPoolInfo.nodes.Has(nodeName) {
		t.Errorf("nodes not added the node name")
	}

	if !reflect.DeepEqual(testPoolInfo, expected) {
		t.Errorf("expected: %#v, \ngot: %#v", testPoolInfo, expected)
	}
}

func TestPoolInfoRemoveNode(t *testing.T) {
	nodeName := "test-node"
	testNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(1000, resource.BinarySI),
				"pods":            *resource.NewQuantity(100, resource.DecimalSI),
			},
		},
	}
	testHasNodePoolInfoExpected := &PoolInfo{
		name:        "",
		nodes: sets.NewString(),

		allocatable: &Resource{
			MilliCPU: -1000,
			Memory: -1000,
			AllowedPodNumber: -100,
		},
		used:        &Resource{},
		shared:      &Resource{},

	}

	testHasNodePoolInfo := &PoolInfo{
		name:        "",
		allocatable: &Resource{},
		used:        &Resource{},
		shared:      &Resource{},

		nodes: sets.NewString(nodeName),
	}


	testHasNodePoolInfo.RemoveNode(testNode)
	if testHasNodePoolInfo.nodes.Has(nodeName) {
		t.Errorf("nodes not remove the node name")
	}
	if !reflect.DeepEqual(testHasNodePoolInfo, testHasNodePoolInfoExpected) {
		t.Errorf("expected: %#v, \ngot: %#v", testHasNodePoolInfoExpected, testHasNodePoolInfo)
	}

	testNoNodePoolInfo := &PoolInfo{
		name:        "",
		allocatable: &Resource{},
		used:        &Resource{},
		shared:      &Resource{},

		nodes: sets.NewString(),
	}
	testNoNodePoolInfoExpected := &PoolInfo{
		name:        "",
		allocatable: &Resource{},
		used:        &Resource{},
		shared:      &Resource{},

		nodes: sets.NewString(),
	}
	err := testNoNodePoolInfo.RemoveNode(testNode)
	if err == nil {
		t.Errorf("remove node not exist not return error")
	}
	if !reflect.DeepEqual(testNoNodePoolInfo, testNoNodePoolInfoExpected) {
		t.Errorf("expected: %#v, \ngot: %#v", testNoNodePoolInfoExpected, testNoNodePoolInfo)
	}
}

func TestPoolInfoUpdateNode(t *testing.T) {
	nodeName := "test-node"
	oldNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(1000, resource.BinarySI),
				"pods":            *resource.NewQuantity(100, resource.DecimalSI),
			},
		},
	}
	newNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(800, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(800, resource.BinarySI),
				"pods":            *resource.NewQuantity(80, resource.DecimalSI),
			},
		},
	}
	expected := &PoolInfo{
		name:        "",
		allocatable: &Resource{
			MilliCPU: 800,
			Memory: 800,
			AllowedPodNumber: 80,
		},
		used:        &Resource{},
		shared:      &Resource{},

		nodes: sets.NewString(nodeName),
	}

	testHasNodePoolInfo := NewPoolInfo()
	testHasNodePoolInfo.AddNode(oldNode)
	testHasNodePoolInfo.UpdateNode(oldNode, newNode)
	if !reflect.DeepEqual(testHasNodePoolInfo, expected) {
		t.Errorf("expected: %#v, \ngot: %#v", testHasNodePoolInfo, expected)
	}

	testNoNodePoolInfo := NewPoolInfo()
	testNoNodePoolInfo.AddNode(oldNode)

}

