package info

import (
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"reflect"
	"testing"
)

const ErrorF = "\nexpected: %#v, \n     got: %#v"

func TestNewPoolInfo(t *testing.T) {
	poolName := ""
	expected := &PoolInfo{
		allocatable: &Resource{},
		used:        &Resource{},
		shared:      &Resource{},

		nodes: sets.NewString(),
	}

	poolInfo := NewPoolInfo()
	if poolName != expected.Name() {
		t.Errorf(ErrorF, expected.Name(), poolName)
	}
	if !reflect.DeepEqual(poolInfo, expected) {
		t.Errorf(ErrorF, expected, poolInfo)
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
		t.Errorf(ErrorF, testPoolInfo, expected)
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
		t.Errorf(ErrorF, testHasNodePoolInfoExpected, testHasNodePoolInfo)
	}

	testNoNodePoolInfo := &PoolInfo{
		allocatable: &Resource{},
		used:        &Resource{},
		shared:      &Resource{},

		nodes: sets.NewString(),
	}
	testNoNodePoolInfoExpected := &PoolInfo{
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
		t.Errorf(ErrorF, testNoNodePoolInfoExpected, testNoNodePoolInfo)
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
		t.Errorf(ErrorF, testHasNodePoolInfo, expected)
	}

	testNoNodePoolInfo := NewPoolInfo()
	testNoNodePoolInfo.UpdateNode(oldNode, newNode)
	expected = NewPoolInfo()
	if !reflect.DeepEqual(testNoNodePoolInfo, expected) {
		t.Errorf(ErrorF, expected, testNoNodePoolInfo)
	}
}

func TestPoolInfoAddPod(t *testing.T) {
	poolName := "test-pool"
	nodeName := "test-node"
	pods := []*v1.Pod{
		basePod(t, nodeName, "test-1", "100m", "500", "", map[string]string{v1alpha1.GroupNameAnnotationKey: poolName}, []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 80, Protocol: "TCP"}}),
		makeBasePod(t, nodeName, "test-2", "200m", "1Ki", "", []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 8080, Protocol: "TCP"}}),
		makeBasePod(t, nodeName, "test-3", "200m", "1Ki", "", []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 8080, Protocol: "TCP"}}),
	}
	testPool := &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		},
		allocatable: &Resource{},
		used:        &Resource{},
		shared:      &Resource{},

		nodes: sets.NewString(),
	}

	expected := &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		},
		allocatable: &Resource{},
		used:        &Resource{
			MilliCPU: 500,
			Memory: 2548,
			AllowedPodNumber: 3,
		},
		shared:      &Resource{
			MilliCPU: 400,
			Memory: 2048,
			AllowedPodNumber: 2,
		},

		nodes: sets.NewString(),
	}
	for _, pod := range pods {
		testPool.AddPod(pod)
	}

	if !reflect.DeepEqual(testPool, expected) {
		t.Errorf(ErrorF, expected, testPool)
	}
}

func TestPoolInfoRemovePod(t *testing.T) {
	poolName := "test-pool"
	nodeName := "test-node"
	pods := []*v1.Pod{
		basePod(t, nodeName, "test-1", "100m", "500", "", map[string]string{v1alpha1.GroupNameAnnotationKey: poolName}, []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 80, Protocol: "TCP"}}),
	}
	testPool := &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		},
		allocatable: &Resource{},
		used:        &Resource{
			MilliCPU: 500,
			Memory: 2548,
			AllowedPodNumber: 3,
		},
		shared:      &Resource{
			MilliCPU: 400,
			Memory: 2048,
			AllowedPodNumber: 2,
		},

		nodes: sets.NewString(),
	}

	expected := &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		},
		allocatable: &Resource{},
		used:        &Resource{
			MilliCPU: 400,
			Memory: 2048,
			AllowedPodNumber: 2,
		},
		shared:      &Resource{
			MilliCPU: 400,
			Memory: 2048,
			AllowedPodNumber: 2,
		},

		nodes: sets.NewString(),
	}
	for _, pod := range pods {
		testPool.RemovePod(pod)
	}

	if !reflect.DeepEqual(testPool, expected) {
		t.Errorf(ErrorF, expected, testPool)
	}
}

func TestPoolInfoUpdatePod(t *testing.T) {
	poolName := "test-pool"
	nodeName := "test-node"
	oldPod := basePod(t, nodeName, "test-1", "100m", "500", "", map[string]string{v1alpha1.GroupNameAnnotationKey: poolName}, []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 80, Protocol: "TCP"}})
	newPod := basePod(t, nodeName, "test-1", "200m", "800", "", map[string]string{v1alpha1.GroupNameAnnotationKey: poolName}, []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 80, Protocol: "TCP"}})
	newPod2 := basePod(t, nodeName, "test-1", "200m", "800", "", map[string]string{v1alpha1.GroupNameAnnotationKey: ""}, []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 80, Protocol: "TCP"}})

	testPool := &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		},
		allocatable: &Resource{},
		used:        &Resource{
			MilliCPU: 500,
			Memory: 2548,
			AllowedPodNumber: 2,
		},
		shared:      &Resource{
			MilliCPU: 400,
			Memory: 2048,
			AllowedPodNumber: 2,
		},

		nodes: sets.NewString(),
	}

	expected := &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		},
		allocatable: &Resource{},
		used:        &Resource{
			MilliCPU: 600,
			Memory: 2848,
			AllowedPodNumber: 2,
		},
		shared:      &Resource{
			MilliCPU: 400,
			Memory: 2048,
			AllowedPodNumber: 2,
		},

		nodes: sets.NewString(),
	}

	testPool.UpdatePod(oldPod, newPod)
	if !reflect.DeepEqual(testPool, expected) {
		t.Errorf(ErrorF, expected, testPool)
	}
	testPool.UpdatePod(oldPod, newPod2)
	expected = &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		},
		allocatable: &Resource{},
		used:        &Resource{
			MilliCPU: 700,
			Memory: 3148,
			AllowedPodNumber: 2,
		},
		shared:      &Resource{
			MilliCPU: 600,
			Memory: 2848,
			AllowedPodNumber: 3,
		},

		nodes: sets.NewString(),
	}
	if !reflect.DeepEqual(testPool, expected) {
		t.Errorf(ErrorF, expected, testPool)
	}
}

func TestPoolInfoAddNodeInfo(t *testing.T) {
	testPoolInfo := NewPoolInfo()
	nodeName := "test-node"
	nodeInfo := &NodeInfo{
		node: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
		},
		allocatableResource: &Resource{
			MilliCPU: 30000,
			Memory: 64000,
			AllowedPodNumber: 100,
			EphemeralStorage: 1000,
			ScalarResources: map[v1.ResourceName]int64{ResourceGPU: 8},
		},
	}

	expected := &PoolInfo{
		nodes: sets.NewString(nodeName),
		allocatable: &Resource{
			MilliCPU: 30000,
			Memory: 64000,
			AllowedPodNumber: 100,
			EphemeralStorage: 1000,
			ScalarResources: map[v1.ResourceName]int64{ResourceGPU: 8},
		},
		used: &Resource{},
		shared: &Resource{},
	}

	testPoolInfo.AddNodeInfo(nodeInfo)
	if !reflect.DeepEqual(testPoolInfo, expected) {
		t.Errorf(ErrorF, expected, testPoolInfo)
	}
}

func TestPoolInfoRemoveNodeInfo(t *testing.T) {
	nodeName := "test-node"
	testPoolInfo := &PoolInfo{
		nodes: sets.NewString(nodeName),

		allocatable: &Resource{},
		used:        &Resource{},
		shared:      &Resource{},
	}
	nodeInfo := &NodeInfo{
		node: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
		},
		allocatableResource: &Resource{
			MilliCPU: 30000,
			Memory: 64000,
			AllowedPodNumber: 100,
			EphemeralStorage: 1000,
			ScalarResources: map[v1.ResourceName]int64{ResourceGPU: 8},
		},
	}

	expected := &PoolInfo{
		nodes: sets.NewString(),
		allocatable: &Resource{
			MilliCPU: -30000,
			Memory: -64000,
			AllowedPodNumber: -100,
			EphemeralStorage: -1000,
			ScalarResources: map[v1.ResourceName]int64{ResourceGPU: -8},
		},
		used: &Resource{},
		shared: &Resource{},
	}

	testPoolInfo.RemoveNodeInfo(nodeInfo)
	if !reflect.DeepEqual(testPoolInfo, expected) {
		t.Errorf(ErrorF, expected, testPoolInfo)
	}
}

func TestPoolInfoAllocatable(t *testing.T) {
	testPoolInfo := NewPoolInfo()
	alloc := testPoolInfo.Allocatable()
	if alloc == nil {
		t.Errorf(ErrorF, nil, alloc)
	}
}

func TestPoolInfoMatchPod(t *testing.T) {
	nodeName := "test-node"
	poolName := "test-pool"
	pod := basePod(t, nodeName, "test-1", "100m", "500", "", map[string]string{v1alpha1.GroupNameAnnotationKey: poolName}, []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 80, Protocol: "TCP"}})
	testPoolInfo := &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		},
	}
	matched := testPoolInfo.MatchPod(pod)
	expected := true

	testPoolInfo.pool.Name = "dark"
	matched = testPoolInfo.MatchPod(pod)
	expected = false
	if matched != expected {
		t.Errorf(ErrorF, expected, matched)
	}

	pod = makeBasePod(t, nodeName, "test-2", "200m", "1Ki", "", []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 8080, Protocol: "TCP"}})
	testPoolInfo.pool = nil
	matched = testPoolInfo.MatchPod(pod)
	expected = true
	if matched != expected {
		t.Errorf(ErrorF, expected, matched)
	}
}

func TestPoolInfoMatchNode(t *testing.T) {
	nodeName := "test-node"
	poolName := "test-pool"
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

	defaultPoolInfo := NewPoolInfo()
	matched := defaultPoolInfo.MatchNode(testNode)
	// defaultPool will not match any nodes
	expected := false
	if matched != expected {
		t.Errorf(ErrorF, expected, matched)
	}

	testPoolInfo := &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
			Spec: v1alpha1.PoolSpec{
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"cpu": "true",
					},
				},
			},
		},
	}
	testNode.Labels = map[string]string{
		"cpu": "true",
	}
	matched = testPoolInfo.MatchNode(testNode)
	expected = true
	if matched != expected {
		t.Errorf(ErrorF, expected, matched)
	}

	 testNode.Labels["gpu"] = "true"
	 testNode.SetLabels(map[string]string{})
	 matched = testPoolInfo.MatchNode(testNode)
	 expected = false
	if matched != expected {
		t.Errorf(ErrorF, expected, matched)
	}
}

func TestPoolInfoIsDefaultPool(t *testing.T) {
	testPoolInfos := []*PoolInfo {
		NewPoolInfo(),
		{
			pool: &v1alpha1.Pool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
			},
		},
		{
			pool: &v1alpha1.Pool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
		},
	}
	expects := []bool{true, true, false}
	for i, pi := range testPoolInfos {
		act := pi.IsDefaultPool()
		expected := expects[i]
		if act != expected {
			t.Errorf(ErrorF, expected, act)
		}
	}
}

