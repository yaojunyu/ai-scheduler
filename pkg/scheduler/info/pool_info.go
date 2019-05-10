package info

import (
	"fmt"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
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
	nodes     map[string]*NodeInfoListItem
	// headNode points to the most recently updated NodeInfo in "nodes". It is the
	// head of the linked list.
	headNode *NodeInfoListItem

	// node trees
	nodeTree *NodeTree
	// A map from image name to its imageState.
	imageStates map[string]*imageState

	// All nodes' capacity sum
	capacity *Resource
	// Resources divided to the pool
	allocatable *Resource
	// All resources used by pods
	used 	 *Resource
	// Resources borrowed by other task from other pool
	shared   *Resource

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
func (n *PoolInfo) HeadNode() *NodeInfoListItem {
	return n.headNode
}

func NewPoolInfo() *PoolInfo {
	pi := &PoolInfo{
		pool: 		 nil,
		//nodes:       map[string]*NodeInfo{},
		nodes:       make(map[string]*NodeInfoListItem),
		nodeTree:    newNodeTree(nil),

		capacity: 	 &Resource{},
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

func (p *PoolInfo) UpdateNodeFromPool(oldP *PoolInfo, oldNode, newNode *v1.Node) error {
	if p == oldP {
		return p.updateNode(oldNode, newNode)
	} else {
		ni, ok := oldP.nodes[oldNode.Name]
		if ok {
			oldP.removeNodeInfoFromList(oldNode.Name)
			oldP.nodeTree.RemoveNode(oldNode)
			oldP.capacity.Sub(NewResource(oldNode.Status.Capacity))
			oldP.allocatable.Sub(NewResource(oldNode.Status.Allocatable))
			for _, pod := range ni.info.pods {
				oldP.reducePodResource(pod)
			}
		}
		return p.AddNodeInfo(ni)
	}
}

func (p *PoolInfo) updateNode(oldNode, newNode *v1.Node) error {
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
	if p == nil || item == nil || item.info == nil {
		return nil
	}
	if item.info.node == nil {
		klog.Warningf("node in nodeinfo is nil")
		return nil
	}
	if _, ok := p.nodes[item.info.node.Name]; ok {
		return fmt.Errorf("nodeinfo %v already exist in pool %v", item.info.node.Name, p.Name())
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
	if p == nil || item == nil || item.info == nil {
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

//func (p *PoolInfo) UpdateNodeInfo(oldNi *NodeInfo, newNi *NodeInfo) {
//
//	p.nodeTree.UpdateNode(oldNi.node, newNi.node)
//}

// SetPool sets the overall pool information
func (p *PoolInfo) SetPool(pool *v1alpha1.Pool) error {
	p.pool = pool
	return nil
}

// RemovePool removes the overall information about the pool
func (p *PoolInfo) ClearPool() error {
	if p == nil {
		return nil
	}
	p.pool = nil
	p.nodes = nil
	p.allocatable = nil
	p.used = nil
	p.shared = nil

	return nil
}

func (p *PoolInfo) GetPoolWeight() map[v1.ResourceName]int32 {
	if p == nil || p.pool == nil {
		return nil
	}
	return p.pool.Spec.Weight
}

func (p *PoolInfo) GetQuota() v1.ResourceList {
	if p == nil || p.pool == nil {
		return nil
	}
	return p.pool.Spec.Quota
}

func (p *PoolInfo) GetQuotaValue(name v1.ResourceName) int64 {
	if p == nil || p.pool == nil || name == "" {
		return 0
	}
	quota, ok := p.pool.Spec.Quota[name]
	if !ok {
		return 0
	}
	if name == v1.ResourceCPU {
		return quota.MilliValue()
	} else {
		return quota.Value()
	}
}

func (p *PoolInfo) NumNodes() int {
	if p == nil || p.nodeTree == nil {
		return 0
	}
	num :=len(p.nodes)
	if num != p.nodeTree.numNodes {
		klog.Errorf("Error: nodeTree size %d not equals nodes size %d in pool %v", num, p.nodeTree.numNodes, p.Name())
	}
	return num
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

	if p.pool.Spec.NodeSelector == nil && p.pool.Spec.SupportResources == nil {
		return false
	}

	if p.pool.Spec.NodeSelector != nil {
		if selector, err := metav1.LabelSelectorAsSelector(p.pool.Spec.NodeSelector);
		    err == nil && !selector.Matches(labels.Set(node.Labels)) {
			return false
		}
	}

	if p.pool.Spec.SupportResources != nil && len(p.pool.Spec.SupportResources) > 0 {
		for _, rn := range p.pool.Spec.SupportResources {
			if _, ok := node.Status.Allocatable[rn]; !ok {
				return false
			}
		}
	}

	return true
}

// TODO consider move nodeTree to pool struct the uncomment bellow function
//func (p *PoolInfo) MatchPoolNodes(nodes []*v1.Node) (*Resource, error) {
//	if nodes == nil || len(nodes) == 0 {
//		klog.Warning("Not any nodes cached, will not filter nodes for Pool")
//		return &Resource{}, nil
//	}
//
//	if p == nil ||
//		(p.pool.Spec.NodeSelector == nil &&
//			p.pool.Spec.SupportResources == nil) {
//		return &Resource{}, nil
//	}
//
//	totalResource := &Resource{}
//	for _, n := range nodes {
//		if p.matchNode(n) {
//			p.nodeTree.AddNode(n)
//			totalResource.Add(n.Status.Allocatable)
//		}
//	}
//
//	return totalResource, nil
//}

func (p *PoolInfo) IsDefaultPool() bool {
	if p == nil {
		return false
	}
	return p.Name() == DefaultPoolName
}

func (p *PoolInfo) Weighted(name v1.ResourceName) bool {
	if p == nil || p.pool == nil || p.pool.Spec.Weight == nil {
		return false
	}
	_, ok := p.pool.Spec.Weight[name]

	return ok
}

func (p *PoolInfo) HasQuota(name v1.ResourceName) bool {
	if p == nil || p.pool == nil || p.pool.Spec.Quota == nil {
		return false
	}

	_, ok := p.pool.Spec.Quota[name]
	return ok
}

func (p *PoolInfo) NeedMatchNodes(name v1.ResourceName) bool {
	if p == nil || p.pool == nil {
		return false
	}

	return p.pool.Spec.NodeSelector != nil ||
		(p.pool.Spec.SupportResources != nil &&
		len(p.pool.Spec.SupportResources) > 0)
}

func (p *PoolInfo) NeedMatchNodeLabel() bool {
	if p == nil || p.pool == nil {
		return false
	}

	return p.pool.Spec.NodeSelector != nil
}

func (p *PoolInfo) NeedMatchNodeResource(name v1.ResourceName) bool {
	if p == nil || p.pool == nil {
		return false
	}

	return p.pool.Spec.SupportResources != nil &&
			len(p.pool.Spec.SupportResources) > 0
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
	for node := p.headNode; node != nil; node = node.next {
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