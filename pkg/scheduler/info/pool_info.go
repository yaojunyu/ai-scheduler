package info

import (
	"fmt"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
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
	nodes map[string]*NodeInfo
	// node trees
	nodeTree *NodeTree

	// All nodes' capacity sum
	capacity *Resource
	// Resources divided to the pool
	allocatable *Resource
	// All resources used by pods
	used 	 *Resource
	// Resources borrowed by other task from other pool
	shared   *Resource

}

func NewPoolInfo() *PoolInfo {
	pi := &PoolInfo{
		pool: 		 nil,
		nodes:       map[string]*NodeInfo{},
		nodeTree:    newNodeTree(nil),

		capacity: 	 &Resource{},
		allocatable: &Resource{},
		used:        &Resource{},
		shared:      &Resource{},
	}
	return pi
}

// AddPod compute used and shared
func (p *PoolInfo) AddPod(pod *v1.Pod) error {
	res := GetResourceRequestForPool(pod)
	p.used.Plus(res)
	if !p.MatchPod(pod) {
		p.shared.Plus(res)
	}
	return nil
}

func (p *PoolInfo) RemovePod(pod *v1.Pod) error {
	res := GetResourceRequestForPool(pod)
	p.used.Sub(res)
	if !p.MatchPod(pod) {
		p.shared.Sub(res)
	}
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

// AddNode add node to tool, and compute all resources value
func (p *PoolInfo) AddNodeInfo(ni *NodeInfo) error {
	if p == nil || ni == nil {
		return nil
	}
	if ni.node == nil {
		klog.Warningf("node in nodeinfo is nil")
		return nil
	}
	if _, ok := p.nodes[ni.node.Name]; ok {
		return fmt.Errorf("nodeinf %v already exist in pool %v", ni.node.Name, p.Name())
	}
	p.nodes[ni.node.Name] = ni
	p.nodeTree.AddNode(ni.node)
	p.capacity.Add(ni.node.Status.Capacity)
	p.allocatable.Plus(ni.allocatableResource)
	for _, pod := range ni.pods {
		p.AddPod(pod)
	}
	return nil
}

// RemoveNode remove node from pool
func (p *PoolInfo) RemoveNodeInfo(ni *NodeInfo) error {
	if p == nil || ni == nil {
		return nil
	}
	if _, ok := p.nodes[ni.node.Name]; !ok {
		return fmt.Errorf("nodeinfo %v not exists in pool %v", ni.Node().Name, p.Name())
	}
	delete(p.nodes, ni.node.Name)
	if err := p.nodeTree.RemoveNode(ni.node); err != nil {
		return err
	}
	p.capacity.Sub(NewResource(ni.node.Status.Capacity))
	p.allocatable.Sub(ni.allocatableResource)
	for _, pod := range ni.pods {
		p.RemovePod(pod)
	}
	return nil
}

func (p *PoolInfo) UpdateNodeInfo(oldNi *NodeInfo, newNi *NodeInfo) {

	p.nodeTree.UpdateNode(oldNi.node, newNi.node)
}

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
	if p == nil || p.nodes == nil {
		return 0
	}
	return p.nodeTree.numNodes
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

func (p *PoolInfo) Nodes() map[string]*NodeInfo {
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

func (p *PoolInfo) ContainsNode(nodeName string) bool {
	if p == nil || nodeName == "" {
		return false
	}
	for nn := range p.nodes {
		if nn == nodeName {
			return true
		}
	}
	return false
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