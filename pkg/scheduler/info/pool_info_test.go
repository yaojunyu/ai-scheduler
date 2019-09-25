package info

import (
	"code.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
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
		pool: nil,
		//nodes:       map[string]*NodeInfo{},
		nodes:    make(map[string]*NodeInfoListItem),
		nodeTree: newNodeTree(nil),

		capacity:    &Resource{},
		allocatable: &Resource{},
		used:        &Resource{},
		shared:      &Resource{},

		nodeInfoSnapshot: NewNodeInfoSnapshot(),
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
		nodes:    make(map[string]*NodeInfoListItem),
		nodeTree: newNodeTree(nil),

		capacity:    &Resource{},
		allocatable: &Resource{},
		used:        &Resource{},
		shared:      &Resource{},

		nodeInfoSnapshot: NewNodeInfoSnapshot(),
	}

	head := &NodeInfoListItem{
		info: NewNodeInfo(),
		prev: nil,
		next: nil,
	}
	expected := &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		},
		allocatable: &Resource{},
		used: &Resource{
			MilliCPU:         500,
			Memory:           2548,
			AllowedPodNumber: 3,
		},
		shared: &Resource{
			MilliCPU:         400,
			Memory:           2048,
			AllowedPodNumber: 2,
		},

		nodes: map[string]*NodeInfoListItem{
			nodeName: head,
		},
		headNode:         head,
		nodeTree:         newNodeTree(nil),
		capacity:         &Resource{},
		nodeInfoSnapshot: NewNodeInfoSnapshot(),
	}
	for _, pod := range pods {
		testPool.AddPod(pod)
	}

	if !reflect.DeepEqual(testPool.allocatable, expected.allocatable) {
		t.Errorf(ErrorF, expected.allocatable, testPool.allocatable)
	}
	if !reflect.DeepEqual(testPool.capacity, expected.capacity) {
		t.Errorf(ErrorF, expected.capacity, testPool.capacity)
	}
	if !reflect.DeepEqual(testPool.used, expected.used) {
		t.Errorf(ErrorF, expected.used, testPool.used)
	}
	if !reflect.DeepEqual(testPool.shared, expected.shared) {
		t.Errorf(ErrorF, expected.shared, testPool.shared)
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
		used: &Resource{
			MilliCPU:         500,
			Memory:           2548,
			AllowedPodNumber: 3,
		},
		shared: &Resource{
			MilliCPU:         400,
			Memory:           2048,
			AllowedPodNumber: 2,
		},

		nodes: map[string]*NodeInfoListItem{
			nodeName: {
				info: NewNodeInfo(pods...),
			},
		},
		nodeTree: newNodeTree(nil),
		capacity: &Resource{},

		nodeInfoSnapshot: NewNodeInfoSnapshot(),
	}

	expected := &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		},
		allocatable: &Resource{},
		used: &Resource{
			MilliCPU:         400,
			Memory:           2048,
			AllowedPodNumber: 2,
		},
		shared: &Resource{
			MilliCPU:         400,
			Memory:           2048,
			AllowedPodNumber: 2,
		},

		nodes:            map[string]*NodeInfoListItem{},
		nodeTree:         newNodeTree(nil),
		capacity:         &Resource{},
		nodeInfoSnapshot: NewNodeInfoSnapshot(),
	}
	for _, pod := range pods {
		testPool.RemovePod(pod)
	}

	if !reflect.DeepEqual(testPool.allocatable, expected.allocatable) {
		t.Errorf(ErrorF, expected.allocatable, testPool.allocatable)
	}
	if !reflect.DeepEqual(testPool.capacity, expected.capacity) {
		t.Errorf(ErrorF, expected.capacity, testPool.capacity)
	}
	if !reflect.DeepEqual(testPool.used, expected.used) {
		t.Errorf(ErrorF, expected.used, testPool.used)
	}
	if !reflect.DeepEqual(testPool.shared, expected.shared) {
		t.Errorf(ErrorF, expected.shared, testPool.shared)
	}
}

func TestPoolInfoUpdatePod(t *testing.T) {
	poolName := "test-pool"
	nodeName := "test-node"
	oldPod := basePod(t, nodeName, "test-1", "100m", "500", "", map[string]string{v1alpha1.GroupNameAnnotationKey: poolName}, []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 80, Protocol: "TCP"}})
	newPod := basePod(t, nodeName, "test-1", "200m", "800", "", map[string]string{v1alpha1.GroupNameAnnotationKey: poolName}, []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 80, Protocol: "TCP"}})
	newPod2 := basePod(t, nodeName, "test-1", "100m", "500", "", map[string]string{v1alpha1.GroupNameAnnotationKey: ""}, []v1.ContainerPort{{HostIP: "127.0.0.1", HostPort: 80, Protocol: "TCP"}})

	testPool := &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		},
		allocatable: &Resource{},
		used: &Resource{
			MilliCPU:         500,
			Memory:           2548,
			AllowedPodNumber: 2,
		},
		shared: &Resource{
			MilliCPU:         400,
			Memory:           2048,
			AllowedPodNumber: 2,
		},

		nodes: map[string]*NodeInfoListItem{
			nodeName: {
				info: NewNodeInfo(oldPod),
			},
		},
		nodeTree:         newNodeTree(nil),
		capacity:         &Resource{},
		nodeInfoSnapshot: NewNodeInfoSnapshot(),
	}

	expected := &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		},
		allocatable: &Resource{},
		used: &Resource{
			MilliCPU:         600,
			Memory:           2848,
			AllowedPodNumber: 2,
		},
		shared: &Resource{
			MilliCPU:         400,
			Memory:           2048,
			AllowedPodNumber: 2,
		},

		nodes: map[string]*NodeInfoListItem{
			nodeName: {
				info: NewNodeInfo(newPod),
			},
		},
		nodeTree:         newNodeTree(nil),
		capacity:         &Resource{},
		nodeInfoSnapshot: NewNodeInfoSnapshot(),
	}

	testPool.UpdatePod(oldPod, newPod)
	if !reflect.DeepEqual(testPool.nodes[nodeName].info.pods[0], newPod) {
		t.Errorf("pod not same")
	}
	if !reflect.DeepEqual(testPool.allocatable, expected.allocatable) {
		t.Errorf(ErrorF, expected.allocatable, testPool.allocatable)
	}
	if !reflect.DeepEqual(testPool.capacity, expected.capacity) {
		t.Errorf(ErrorF, expected.capacity, testPool.capacity)
	}
	if !reflect.DeepEqual(testPool.used, expected.used) {
		t.Errorf(ErrorF, expected.used, testPool.used)
	}
	if !reflect.DeepEqual(testPool.shared, expected.shared) {
		t.Errorf(ErrorF, expected.shared, testPool.shared)
	}
	testPool.UpdatePod(newPod, newPod2)
	expected = &PoolInfo{
		pool: &v1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		},
		allocatable: &Resource{},
		used: &Resource{
			MilliCPU:         500,
			Memory:           2548,
			AllowedPodNumber: 2,
		},
		shared: &Resource{
			MilliCPU:         500,
			Memory:           2548,
			AllowedPodNumber: 3,
		},

		nodes: map[string]*NodeInfoListItem{
			nodeName: {
				info: NewNodeInfo(newPod2),
			},
		},
		nodeTree:         newNodeTree(nil),
		capacity:         &Resource{},
		nodeInfoSnapshot: NewNodeInfoSnapshot(),
	}
	if !reflect.DeepEqual(testPool.nodes[nodeName].info.pods[0], newPod2) {
		t.Errorf("pod not same")
	}
	if !reflect.DeepEqual(testPool.allocatable, expected.allocatable) {
		t.Errorf(ErrorF, expected.allocatable, testPool.allocatable)
	}
	if !reflect.DeepEqual(testPool.capacity, expected.capacity) {
		t.Errorf(ErrorF, expected.capacity, testPool.capacity)
	}
	if !reflect.DeepEqual(testPool.used, expected.used) {
		t.Errorf(ErrorF, expected.used, testPool.used)
	}
	if !reflect.DeepEqual(testPool.shared, expected.shared) {
		t.Errorf(ErrorF, expected.shared, testPool.shared)
	}
}

func TestPoolInfoAddNodeInfo(t *testing.T) {
	testPool := NewPoolInfo()
	nodeName := "test-node"
	testNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Status: v1.NodeStatus{
			Capacity: map[v1.ResourceName]resource.Quantity{},
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(1000, resource.BinarySI),
				"pods":            *resource.NewQuantity(100, resource.DecimalSI),
			},
		},
	}
	nodeInfo := &NodeInfoListItem{
		info: &NodeInfo{
			node: testNode,
			allocatableResource: &Resource{
				MilliCPU:         1000,
				Memory:           1000,
				AllowedPodNumber: 100,
				EphemeralStorage: 0,
			},
		},
	}

	expected := &PoolInfo{
		pool: nil,
		nodes: map[string]*NodeInfoListItem{
			nodeName: nodeInfo,
		},
		nodeTree: newNodeTree([]*v1.Node{testNode}),

		capacity: &Resource{},
		allocatable: &Resource{
			MilliCPU:         1000,
			Memory:           1000,
			AllowedPodNumber: 100,
			EphemeralStorage: 0,
		},
		used:             &Resource{},
		shared:           &Resource{},
		nodeInfoSnapshot: NewNodeInfoSnapshot(),
	}

	testPool.AddNodeInfo(nodeInfo)
	if !reflect.DeepEqual(testPool.allocatable, expected.allocatable) {
		t.Errorf(ErrorF, expected.allocatable, testPool.allocatable)
	}
	if !reflect.DeepEqual(testPool.capacity, expected.capacity) {
		t.Errorf(ErrorF, expected.capacity, testPool.capacity)
	}
	if !reflect.DeepEqual(testPool.used, expected.used) {
		t.Errorf(ErrorF, expected.used, testPool.used)
	}
	if !reflect.DeepEqual(testPool.shared, expected.shared) {
		t.Errorf(ErrorF, expected.shared, testPool.shared)
	}
}

func TestPoolInfoRemoveNodeInfo(t *testing.T) {

	nodeName := "test-node"
	testNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Status: v1.NodeStatus{
			Capacity: map[v1.ResourceName]resource.Quantity{},
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(1000, resource.BinarySI),
				"pods":            *resource.NewQuantity(100, resource.DecimalSI),
			},
		},
	}
	nodeInfo := &NodeInfoListItem{
		info: &NodeInfo{
			node: testNode,
			allocatableResource: &Resource{
				MilliCPU:         30000,
				Memory:           64000,
				AllowedPodNumber: 100,
				EphemeralStorage: 1000,
				ScalarResources:  map[v1.ResourceName]int64{ResourceGPU: 8},
			},
		},
	}
	testPool := &PoolInfo{
		pool:     nil,
		nodes:    map[string]*NodeInfoListItem{nodeName: nodeInfo},
		nodeTree: newNodeTree([]*v1.Node{testNode}),

		allocatable: &Resource{
			MilliCPU:         1000,
			Memory:           1000,
			AllowedPodNumber: 100,
			EphemeralStorage: 0,
		},
		used:   &Resource{},
		shared: &Resource{},

		capacity:         &Resource{},
		nodeInfoSnapshot: NewNodeInfoSnapshot(),
	}

	nodeTree := newNodeTree([]*v1.Node{testNode})
	nodeTree.removeNode(testNode)
	expected := &PoolInfo{
		pool:     nil,
		nodes:    map[string]*NodeInfoListItem{},
		nodeTree: nodeTree,

		allocatable: &Resource{
			MilliCPU:         0,
			Memory:           0,
			AllowedPodNumber: 0,
			EphemeralStorage: 0,
		},
		used:   &Resource{},
		shared: &Resource{},

		capacity:         &Resource{},
		nodeInfoSnapshot: NewNodeInfoSnapshot(),
	}

	testPool.RemoveNodeInfo(nodeInfo)
	if !reflect.DeepEqual(testPool.allocatable, expected.allocatable) {
		t.Errorf(ErrorF, expected.allocatable, testPool.allocatable)
	}
	if !reflect.DeepEqual(testPool.capacity, expected.capacity) {
		t.Errorf(ErrorF, expected.capacity, testPool.capacity)
	}
	if !reflect.DeepEqual(testPool.used, expected.used) {
		t.Errorf(ErrorF, expected.used, testPool.used)
	}
	if !reflect.DeepEqual(testPool.shared, expected.shared) {
		t.Errorf(ErrorF, expected.shared, testPool.shared)
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
	testPoolInfos := []*PoolInfo{
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

func TestPoolInfoNodesHasCircle(t *testing.T) {
	headerNode := "node-1"
	tailNode := "node-4"
	header := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: headerNode,
		},
	}
	tests := []struct {
		node *v1.Node
	}{
		{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: tailNode,
				},
			},
		},
		{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node3",
				},
			},
		},
		{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
				},
			},
		},
		{
			node: header,
		},
	}

	pool := NewPoolInfo()
	for _, test := range tests {
		pool.AddNode(test.node)
	}

	err := pool.UpdateNodeInfoSnapshot()
	if err != nil {
		t.Errorf("pool update nodeInfoSnapshot not right: %v", err)
	}
	// Case 1: build a circle nodeInfoMap
	// update generation to check doubly link circle
	for _, test := range tests {
		pool.UpdateNode(nil, test.node)
	}
	pool.nodes[tailNode].next = pool.nodes[headerNode]
	pool.UpdateNodeInfoSnapshot()
	if pool.nodes[tailNode].next == pool.nodes[headerNode] {
		t.Errorf("circle not be broken")
	}
	err = pool.UpdateNodeInfoSnapshot()
	if err != nil {
		t.Errorf("pool update nodeInfoSnapshot not right: %v", err)
	}
}
