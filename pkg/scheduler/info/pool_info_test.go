package info

import (
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
)

const ErrorF = "\nexpected: %#v, \n     got: %#v"

func TestNewPoolInfo(t *testing.T) {
	poolName := ""
	expected := &PoolInfo{
		pool: 		 nil,
		nodes:       map[string]*NodeInfo{},
		nodeTree:    newNodeTree(nil),

		capacity: 	 &Resource{},
		allocatable: &Resource{},
		used:        &Resource{},
		shared:      &Resource{},
	}

	poolInfo := NewPoolInfo()
	if poolName != expected.Name() {
		t.Errorf(ErrorF, expected.Name(), poolName)
	}
	if !reflect.DeepEqual(poolInfo, expected) {
		t.Errorf(ErrorF, expected, poolInfo)
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

		nodes: map[string]*NodeInfo{},
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

		nodes: map[string]*NodeInfo{},
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

		nodes: map[string]*NodeInfo{},
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

		nodes: map[string]*NodeInfo{},
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

		nodes: map[string]*NodeInfo{},
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

		nodes: map[string]*NodeInfo{},
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

		nodes: map[string]*NodeInfo{},
	}
	if !reflect.DeepEqual(testPool, expected) {
		t.Errorf(ErrorF, expected, testPool)
	}
}

func TestPoolInfoAddNodeInfo(t *testing.T) {
	testPoolInfo := NewPoolInfo()
	nodeName := "test-node"
	testNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Status: v1.NodeStatus{
			Capacity: map[v1.ResourceName]resource.Quantity{

			},
		},
	}
	nodeInfo := &NodeInfo{
		node: testNode,
		allocatableResource: &Resource{
			MilliCPU: 30000,
			Memory: 64000,
			AllowedPodNumber: 100,
			EphemeralStorage: 1000,
			ScalarResources: map[v1.ResourceName]int64{ResourceGPU: 8},
		},
	}

	expected := &PoolInfo{
		pool: 		 nil,
		nodes:       map[string]*NodeInfo{
			nodeName: nodeInfo,
		},
		nodeTree:    newNodeTree([]*v1.Node{testNode}),

		capacity: 	 &Resource{},
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
	testNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Status: v1.NodeStatus{
			Capacity: map[v1.ResourceName]resource.Quantity{

			},
		},
	}
	nodeInfo := &NodeInfo{
		node: testNode,
		allocatableResource: &Resource{
			MilliCPU: 30000,
			Memory: 64000,
			AllowedPodNumber: 100,
			EphemeralStorage: 1000,
			ScalarResources: map[v1.ResourceName]int64{ResourceGPU: 8},
		},
	}
	testPoolInfo := &PoolInfo{
		pool: 		 nil,
		nodes:       map[string]*NodeInfo{nodeName: nodeInfo},
		nodeTree:    newNodeTree([]*v1.Node{testNode}),

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

	nodeTree := newNodeTree([]*v1.Node{testNode})
	nodeTree.removeNode(testNode)
	expected := &PoolInfo{
		pool: 		 nil,
		nodes:       map[string]*NodeInfo{},
		nodeTree:    nodeTree,

		allocatable: &Resource{
			MilliCPU: 0,
			Memory: 0,
			AllowedPodNumber: 0,
			EphemeralStorage: 0,
			ScalarResources: map[v1.ResourceName]int64{ResourceGPU: 0},
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

