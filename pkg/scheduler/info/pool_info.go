package info

import (
	"fmt"
	"code.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/features"
)

const (
	// ResourceGPU need to follow https://github.com/NVIDIA/k8s-device-plugin/blob/66a35b71ac4b5cbfb04714678b548bd77e5ba719/server.go#L20
	ResourceGPU = "nvidia.com/gpu"

	// DefaultPool name of Default pool, DefaultPool collect all pods not belong
	// to any pools.
	DefaultPoolName = ""
)

type PoolInfo struct {
	pool *v1alpha1.Pool

	// All nodes matched to this pool, key as node name
	//nodes map[string]*NodeInfo
	nodes map[string]*NodeInfoListItem
	// headNode points to the most recently updated NodeInfo in "nodes". It is the
	// head of the linked list.
	headNode *NodeInfoListItem

	// node trees
	nodeTree *NodeTree

	// All nodes' capacity sum
	capacity *Resource
	// Resources divided to the pool
	allocatable *Resource
	// All resources used by pods
	used *Resource
	// Resources borrowed by other task from other pool
	shared *Resource

	// nodeInfoSnapshot
	nodeInfoSnapshot *NodeInfoSnapshot
}

type imageState struct {
	// Size of the image
	size int64
	// A set of node names for nodes having this image present
	nodes sets.String
}

// nodeInfoListItem holds a NodeInfo pointer and acts as an item in a doubly
// linked list. When a NodeInfo is updated, it goes to the head of the list.
// The items closer to the head are the most recently updated items.
type NodeInfoListItem struct {
	info *NodeInfo
	next *NodeInfoListItem
	prev *NodeInfoListItem
}

func (n *NodeInfoListItem) Info() *NodeInfo {
	return n.info
}
func (n *NodeInfoListItem) Next() *NodeInfoListItem {
	return n.next
}
func (n *NodeInfoListItem) Prev() *NodeInfoListItem {
	return n.prev
}
func (p *PoolInfo) HeadNode() *NodeInfoListItem {
	return p.headNode
}

func NewPoolInfo() *PoolInfo {
	pi := &PoolInfo{
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
	return pi
}

func NewNodeInfoSnapshot() *NodeInfoSnapshot {
	return &NodeInfoSnapshot{
		NodeInfoMap: make(map[string]*NodeInfo),
	}
}

// AddPod compute used and shared
func (p *PoolInfo) AddPod(pod *v1.Pod) error {
	// add pod to pool
	n, ok := p.nodes[pod.Spec.NodeName]
	if !ok {
		n = newNodeInfoListItem(NewNodeInfo())
		p.nodes[pod.Spec.NodeName] = n
	}
	n.info.AddPod(pod)
	p.moveNodeInfoToHead(pod.Spec.NodeName)

	// compute resources for pool
	p.plusPodResource(pod)
	return nil
}

func (p *PoolInfo) RemovePod(pod *v1.Pod) error {
	// remove pod from pool
	n, ok := p.nodes[pod.Spec.NodeName]
	if !ok {
		return fmt.Errorf("node %v is not found", pod.Spec.NodeName)
	}
	if err := n.info.RemovePod(pod); err != nil {
		return err
	}
	if len(n.info.Pods()) == 0 && n.info.Node() == nil {
		p.removeNodeInfoFromList(pod.Spec.NodeName)
	} else {
		p.moveNodeInfoToHead(pod.Spec.NodeName)
	}

	// reduce resources from pool
	p.reducePodResource(pod)
	return nil
}

func (p *PoolInfo) UpdatePod(oldPod, newPod *v1.Pod) error {
	if p == nil || oldPod == nil || newPod == nil || oldPod == newPod {
		return nil
	}
	// TODO check if the pod is in pool
	p.RemovePod(oldPod)
	p.AddPod(newPod)
	return nil
}

func (p *PoolInfo) GetPool() *v1alpha1.Pool {
	return p.pool
}

func (p *PoolInfo) AddNode(node *v1.Node) error {
	n, ok := p.nodes[node.Name]
	if !ok {
		n = newNodeInfoListItem(NewNodeInfo())
		p.nodes[node.Name] = n
	}
	p.moveNodeInfoToHead(node.Name)

	p.nodeTree.AddNode(node)
	p.capacity.Add(node.Status.Capacity)
	p.allocatable.Add(node.Status.Allocatable)
	return n.info.SetNode(node)
}

func (p *PoolInfo) RemoveNode(node *v1.Node) error {
	n, ok := p.nodes[node.Name]
	if !ok {
		return fmt.Errorf("node %v is not found", node.Name)
	}
	if err := n.info.RemoveNode(node); err != nil {
		return err
	}
	// We remove NodeInfo for this node only if there aren't any pods on this node.
	// We can't do it unconditionally, because notifications about pods are delivered
	// in a different watch, and thus can potentially be observed later, even though
	// they happened before node removal.
	if len(n.info.Pods()) == 0 && n.info.Node() == nil {
		p.removeNodeInfoFromList(node.Name)
	} else {
		p.moveNodeInfoToHead(node.Name)
	}

	p.nodeTree.RemoveNode(node)
	p.capacity.Sub(NewResource(node.Status.Capacity))
	p.allocatable.Sub(NewResource(node.Status.Allocatable))
	return nil
}

func (p *PoolInfo) UpdateNode(oldNode, newNode *v1.Node) error {
	n, ok := p.nodes[newNode.Name]
	if !ok {
		n = newNodeInfoListItem(NewNodeInfo())
		p.nodes[newNode.Name] = n
	}
	p.moveNodeInfoToHead(newNode.Name)
	p.nodeTree.UpdateNode(oldNode, newNode)

	cdelta := NewResource(newNode.Status.Capacity)
	adelta := NewResource(newNode.Status.Allocatable)
	if oldNode != nil {
		cdelta = cdelta.Sub(NewResource(oldNode.Status.Capacity))
		adelta = adelta.Sub(NewResource(oldNode.Status.Allocatable))
	}
	p.capacity.Plus(cdelta)
	p.allocatable.Plus(adelta)

	return n.info.SetNode(newNode)
}

// AddNode add node to tool, and compute all resources value
func (p *PoolInfo) AddNodeInfo(item *NodeInfoListItem) error {
	if p == nil || item == nil || item.info == nil || item.info.node == nil {
		return nil
	}
	if item.info.node == nil {
		klog.Warningf("node in nodeinfo is nil")
		return nil
	}
	if _, ok := p.nodes[item.info.node.Name]; ok {
		klog.Warningf("nodeinfo %v already exist in pool %v", item.info.node.Name, p.Name())
	}
	p.nodes[item.info.node.Name] = item
	p.moveNodeInfoToHead(item.info.node.Name)
	p.nodeTree.AddNode(item.info.node)

	p.capacity.Add(item.info.node.Status.Capacity)
	p.allocatable.Add(item.info.node.Status.Allocatable)
	for _, pod := range item.info.pods {
		p.plusPodResource(pod)
	}
	return nil
}

// RemoveNode remove node from pool
func (p *PoolInfo) RemoveNodeInfo(item *NodeInfoListItem) error {
	if p == nil || item == nil || item.info == nil || item.info.node == nil {
		return nil
	}
	if _, ok := p.nodes[item.info.node.Name]; !ok {
		return fmt.Errorf("nodeinfo %v not exists in pool %v", item.info.node.Name, p.Name())
	}
	p.removeNodeInfoFromList(item.info.node.Name)
	if err := p.nodeTree.RemoveNode(item.info.node); err != nil {
		return err
	}
	p.capacity.Sub(NewResource(item.info.node.Status.Capacity))
	p.allocatable.Sub(NewResource(item.info.node.Status.Allocatable))
	for _, pod := range item.info.pods {
		p.reducePodResource(pod)
	}
	return nil
}

// SetPool sets the overall pool information
func (p *PoolInfo) SetPool(pool *v1alpha1.Pool) error {
	p.pool = pool
	return nil
}

// RemovePool removes the overall information about the pool
func (p *PoolInfo) ClearPool() {
	if p == nil {
		return
	}
	p.pool = nil
	p.nodes = nil
	p.headNode = nil
	p.capacity = nil
	p.allocatable = nil
	p.used = nil
	p.shared = nil
	p.nodeInfoSnapshot = nil

	return
}

func (p *PoolInfo) NumNodes() int {
	if p == nil {
		return 0
	}
	return len(p.nodes)
}

func (p *PoolInfo) Name() string {
	if p == nil || p.pool == nil {
		return ""
	}
	return p.pool.Name
}

func (p *PoolInfo) Capacity() *Resource {
	if p == nil {
		return nil
	}
	return p.capacity
}

func (p *PoolInfo) Allocatable() *Resource {
	if p == nil {
		return nil
	}
	return p.allocatable
}

// Add idle filed if possible
func (p *PoolInfo) Idle() *Resource {
	if p == nil {
		return nil
	}
	return p.allocatable.Clone().Sub(p.used)
}

func (p *PoolInfo) Used() *Resource {
	if p == nil {
		return nil
	}
	return p.used
}

func (p *PoolInfo) Shared() *Resource {
	if p == nil {
		return nil
	}
	return p.shared
}

func (p *PoolInfo) Nodes() map[string]*NodeInfoListItem {
	if p == nil {
		return nil
	}
	return p.nodes
}

func (p *PoolInfo) NodeTree() *NodeTree {
	if p == nil {
		return nil
	}
	return p.nodeTree
}

func (p *PoolInfo) DisableSharing() bool {
	if p == nil || p.pool == nil {
		return false
	}
	return p.pool.Spec.DisableSharing
}

func (p *PoolInfo) DisableBorrowing() bool {
	if p == nil || p.pool == nil {
		return true
	}
	return p.pool.Spec.DisableBorrowing
}

func (p *PoolInfo) DisablePreemption() bool {
	if p == nil || p.pool == nil {
		return true
	}
	return p.pool.Spec.DisablePreemption
}

func (p *PoolInfo) SetAllocatable(resource *Resource) {
	if p == nil {
		return
	}
	p.allocatable = resource
}

func (p *PoolInfo) SetUsed(resource *Resource) {
	if p == nil {
		return
	}
	p.used = resource
}

func (p *PoolInfo) SetShared(resource *Resource) {
	if p == nil {
		return
	}
	p.shared = resource
}

func (p *PoolInfo) SetAllocatableResource(name v1.ResourceName, value int64) {
	if p == nil {
		return
	}
	p.allocatable.SetValue(name, value)
}

func (p *PoolInfo) ContainsNode(nodeName string) (*NodeInfoListItem, bool) {
	if p == nil || nodeName == "" {
		return nil, false
	}
	item, ok := p.nodes[nodeName]
	return item, ok
}

func (p *PoolInfo) MatchPod(pod *v1.Pod) bool {
	if p == nil || pod == nil {
		return false
	}
	return p.Name() == GetPodAnnotationsPoolName(pod)
}

func (p *PoolInfo) MatchNode(node *v1.Node) bool {
	if p == nil || node == nil || p.IsDefaultPool() {
		return false
	}
	// if build-in default pool always return false

	if p.pool.Spec.NodeSelector == nil /* && p.pool.Spec.SupportResources == nil*/ {
		return false
	}

	if p.pool.Spec.NodeSelector != nil {
		if selector, err := metav1.LabelSelectorAsSelector(p.pool.Spec.NodeSelector); err == nil && !selector.Matches(labels.Set(node.Labels)) {
			return false
		}
	}

	//if p.pool.Spec.SupportResources != nil && len(p.pool.Spec.SupportResources) > 0 {
	//	for _, rn := range p.pool.Spec.SupportResources {
	//		if _, ok := node.Status.Allocatable[rn]; !ok {
	//			return false
	//		}
	//	}
	//}

	return true
}

func (p *PoolInfo) IsDefaultPool() bool {
	if p == nil {
		return false
	}
	return p.Name() == DefaultPoolName
}

func (p *PoolInfo) NeedMatchNodeLabel() bool {
	if p == nil || p.pool == nil {
		return false
	}

	return p.pool.Spec.NodeSelector != nil
}

// newNodeInfoListItem initializes a new nodeInfoListItem.
func newNodeInfoListItem(ni *NodeInfo) *NodeInfoListItem {
	return &NodeInfoListItem{
		info: ni,
	}
}

// moveNodeInfoToHead moves a NodeInfo to the head of "cache.nodes" doubly
// linked list. The head is the most recently updated NodeInfo.
// We assume cache lock is already acquired.
func (p *PoolInfo) moveNodeInfoToHead(name string) {
	ni, ok := p.nodes[name]
	if !ok {
		klog.Errorf("No NodeInfo with name %v found in the cache", name)
		return
	}
	// if the node info list item is already at the head, we are done.
	if ni == p.headNode {
		return
	}

	if ni.prev != nil {
		ni.prev.next = ni.next
	}
	if ni.next != nil {
		ni.next.prev = ni.prev
	}
	if p.headNode != nil {
		p.headNode.prev = ni
	}
	ni.next = p.headNode
	ni.prev = nil
	p.headNode = ni
}

// removeNodeInfoFromList removes a NodeInfo from the "cache.nodes" doubly
// linked list.
// We assume cache lock is already acquired.
func (p *PoolInfo) removeNodeInfoFromList(name string) {
	ni, ok := p.nodes[name]
	if !ok {
		klog.Errorf("No NodeInfo with name %v found in the cache", name)
		return
	}

	if ni.prev != nil {
		ni.prev.next = ni.next
	}
	if ni.next != nil {
		ni.next.prev = ni.prev
	}
	// if the removed item was at the head, we must update the head.
	if ni == p.headNode {
		p.headNode = ni.next
	}
	delete(p.nodes, name)
}

func (p *PoolInfo) UpdateNodeInfoSnapshot() error {
	//cache.mu.Lock()
	//defer cache.mu.Unlock()
	balancedVolumesEnabled := utilfeature.DefaultFeatureGate.Enabled(features.BalanceAttachedNodeVolumes)

	nodeSnapshot := p.nodeInfoSnapshot
	// Get the last generation of the the snapshot.
	snapshotGeneration := nodeSnapshot.Generation

	// Start from the head of the NodeInfo doubly linked list and update snapshot
	// of NodeInfos updated after the last snapshot.
	// fast used to check doubly link if has circle
	fast := p.headNode
	node := p.headNode
	prev := p.headNode
	hasCircle := false
	for node != nil {
		if node.info.GetGeneration() <= snapshotGeneration {
			// all the nodes are updated before the existing snapshot. We are done.
			break
		}
		if balancedVolumesEnabled && node.info.TransientInfo != nil {
			// Transient scheduler info is reset here.
			node.info.TransientInfo.ResetTransientSchedulerInfo()
		}
		if np := node.info.Node(); np != nil {
			nodeSnapshot.NodeInfoMap[np.Name] = node.info.Clone()
		}
		prev = node
		node = node.next
		if fast != nil {
			if fast = fast.next; fast != nil {
				fast = fast.next
			}
		}
		if node != nil && fast != nil && node == fast {
			hasCircle = true
			klog.Warningf("Warning nodes doubly link has circle!!!")
			break
		}
	}
	// find circle entry point and break the circle
	if hasCircle {
		for fast = p.headNode; node != fast; fast = fast.next {
			prev = node
			node = node.next
		}
		klog.Info("Break nodes doubly link circle")
		prev.next = nil
	}
	// Update the snapshot generation with the latest NodeInfo generation.
	if p.headNode != nil {
		nodeSnapshot.Generation = p.headNode.info.GetGeneration()
	}

	if len(nodeSnapshot.NodeInfoMap) > len(p.nodes) {
		for name := range nodeSnapshot.NodeInfoMap {
			if _, ok := p.nodes[name]; !ok {
				delete(nodeSnapshot.NodeInfoMap, name)
			}
		}
	}
	return nil
}

// not add lock
func (p *PoolInfo) NodeInfoSnapshot() *NodeInfoSnapshot {
	return p.nodeInfoSnapshot
}

func (p *PoolInfo) plusPodResource(pod *v1.Pod) {
	res := GetResourceRequestForPool(pod)
	p.used.Plus(res)
	if !p.MatchPod(pod) {
		p.shared.Plus(res)
	}
}
func (p *PoolInfo) reducePodResource(pod *v1.Pod) {
	res := GetResourceRequestForPool(pod)
	p.used.Sub(res)
	if !p.MatchPod(pod) {
		p.shared.Sub(res)
	}
}

func (p *PoolInfo) String() string {
	return p.Name()
}

func (p *PoolInfo) CanBorrowPool(pool *PoolInfo) bool {
	if p.Name() == pool.Name() {
		return true
	}
	if p.DisableBorrowing() || pool.DisableSharing() {
		return false
	}
	if p.pool.Spec.BorrowingPools == nil ||
		len(p.pool.Spec.BorrowingPools) == 0 ||
		SliceContainsString(p.pool.Spec.BorrowingPools, pool.Name()) {
		return true
	}
	return false
}
