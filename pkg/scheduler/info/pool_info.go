package info

import (
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
)
const (
	// ResourceGPU need to follow https://github.com/NVIDIA/k8s-device-plugin/blob/66a35b71ac4b5cbfb04714678b548bd77e5ba719/server.go#L20
	ResourceGPU = "nvidia.com/gpu"

	// DefaultPool name of Default pool, DefaultPool collect all pods not belong
	// to any pools.
	DefaultPoolName = ""
)
type PoolInfo struct {
	name string
	pool *v1alpha1.Pool

	// All nodes matched to this pool, it's a node name set
	nodes sets.String

	// Resources divided to the pool
	capacity *Resource
	// All resources used by pods
	used 	 *Resource
	// Resources borrowed by other task from other pool
	shared   *Resource
}

func NewPoolInfo() *PoolInfo {
	pi := &PoolInfo{
		name:     "",
		capacity: &Resource{},
		used:     &Resource{},
		shared:   &Resource{},

		//pods: make(map[types.UID]*v1.Pod),
		nodes: sets.NewString(),
	}
	return pi
}

// AddNode add node to tool, and compute all resources value
func (p *PoolInfo) AddNode(node *v1.Node) error {
	if p == nil || node == nil {
		return nil
	}
	p.capacity.Add(node.Status.Allocatable)
	p.nodes.Insert(node.Name)
	return nil
}

// RemoveNode remove node from pool
func (p *PoolInfo) RemoveNode(node *v1.Node) error {
	if p == nil || node == nil {
		return nil
	}
	p.capacity.Sub(NewResource(node.Status.Allocatable))
	p.nodes.Delete(node.Name)
	return nil
}

func (p *PoolInfo) UpdateNode(oldNode *v1.Node, newNode *v1.Node) error {
	if p == nil || oldNode == nil || newNode == nil || oldNode == newNode {
		return nil
	}
	p.RemoveNode(oldNode)
	p.AddNode(newNode)
	p.nodes.Delete(oldNode.Name)
	p.nodes.Insert(newNode.Name)
	return nil
}

// AddPod compute used and shared
func (p *PoolInfo) AddPod(pod *v1.Pod) error {
	res := GetPodResourceRequestWithoutNonZeroContainer(pod)
	p.used.Plus(res)
	p.used.AllowedPodNumber += 1

	if !p.MatchPod(pod) {
		p.shared.Plus(res)
		p.shared.AllowedPodNumber += 1
	}

	return nil
}

func (p *PoolInfo) RemovePod(pod *v1.Pod) error {
	res := GetPodResourceRequestWithoutNonZeroContainer(pod)
	p.used.Sub(res)
	p.used.AllowedPodNumber -= 1

	if !p.MatchPod(pod) {
		p.shared.Sub(res)
		p.shared.AllowedPodNumber -= 1
	}
	// we ignore pod if nod exists
	return nil
}

func (p *PoolInfo) UpdatePod(oldPod, newPod *v1.Pod) error {
	if p == nil || oldPod == nil || newPod == nil || oldPod == newPod {
		return nil
	}
	p.RemovePod(oldPod)
	p.AddPod(newPod)
	return nil
}

func (p *PoolInfo) GetPool() *v1alpha1.Pool {
	if p == nil {
		return nil
	}
	return p.pool
}

// AddNode add node to tool, and compute all resources value
func (p *PoolInfo) AddNodeInfo(node *NodeInfo) error {
	if p == nil || node == nil {
		return nil
	}
	p.capacity.Plus(node.allocatableResource)
	p.nodes.Insert(node.Node().Name)
	for _, pod := range node.pods {
		p.AddPod(pod)
	}
	return nil
}

// RemoveNode remove node from pool
func (p *PoolInfo) RemoveNodeInfo(node *NodeInfo) error {
	if p == nil || node == nil {
		return nil
	}
	p.capacity.Sub(node.allocatableResource)
	p.nodes.Delete(node.Node().Name)
	for _, pod := range node.pods {
		p.RemovePod(pod)
	}
	return nil
}

//func (p *PoolInfo) AddNodeInfos(nodes []*NodeInfo) error {
//	if nodes == nil || len(nodes) == 0 {
//		return nil
//	}
//	for _, node := range nodes {
//		p.AddNodeInfo(node)
//	}
//	return nil
//}
//
//func (p *PoolInfo) RemoveNodeInfos(nodes []*NodeInfo) error {
//	if nodes == nil || len(nodes) == 0 {
//		return nil
//	}
//	for _, node := range nodes {
//		p.RemoveNodeInfo(node)
//	}
//	return nil
//}

// SetPool sets the overall pool information
func (p *PoolInfo) SetPool(pool *v1alpha1.Pool) error {
	if p == nil || pool == nil {
		return nil
	}

	p.name = pool.Name
	p.pool = pool

	// TODO set other fields

	//_, err := p.MatchPoolNodes(nodes)
	return nil
}

// RemovePool removes the overall information about the pool
func (p *PoolInfo) ClearPool() error {
	if p == nil {
		return nil
	}
	p.pool = nil
	p.capacity = &Resource{}
	p.used = &Resource{}
	p.shared = &Resource{}

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

//func (p *PoolInfo) GetNodes() []*v1.Node {
//	if p == nil {
//		return nil
//	}
//	return p.nodes
//}
func (p *PoolInfo) NumNodes() int {
	if p == nil {
		return 0
	}
	return p.nodes.Len()
}

func (p *PoolInfo) Name() string {
	if p == nil || p.pool == nil {
		return ""
	}
	return p.pool.Name
}

func (p *PoolInfo) Capacity() *Resource {
	if p == nil {
		return &Resource{}
	}
	return p.capacity
}

func (p *PoolInfo) Used() *Resource {
	if p == nil {
		return &Resource{}
	}
	return p.used
}

func (p *PoolInfo) Shared() *Resource {
	if p == nil {
		return &Resource{}
	}
	return p.shared
}

func (p *PoolInfo) SetCapacity(resource *Resource) {
	if p == nil {
		return
	}
	p.capacity = resource
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

func (p *PoolInfo) SetCapacityResource(name v1.ResourceName, value int64) {
	if p == nil || value < 0 {
		return
	}
	switch name {
	case v1.ResourceCPU:
		p.capacity.MilliCPU = value
	case v1.ResourceMemory:
		p.capacity.Memory = value
	case v1.ResourceEphemeralStorage:
		p.capacity.EphemeralStorage = value
	case ResourceGPU:
		p.capacity.SetScalar(name, value)
	default:
		p.capacity.SetScalar(name, value)
	}
}

func (p *PoolInfo) MatchPod(pod *v1.Pod) bool {
	if p == nil || pod == nil {
		return false
	}
	return p.Name() == GetPodAnnotationsPoolName(pod)
}

func (p *PoolInfo) MatchNode(node *v1.Node) bool {
	if p == nil || node == nil || p.name == DefaultPoolName || p.pool == nil {
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

//func (p *PoolInfo) IsDefaultPool() bool {
//	return p != nil && p.Name() == DefaultPool
//}

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