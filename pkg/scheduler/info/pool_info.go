package info

import (
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
	DefaultPool = "default"
)
type PoolInfo struct {
	pool *v1alpha1.Pool

	// All nodes matched to this pool
	nodes []*v1.Node
	// All pods consume the pool
	pods  []*v1.Pod

	// Resources divided to the pool
	deserved *Resource
	// All resources used by pods
	used 	 *Resource
	// Resources borrowed by other task from other pool
	shared   *Resource
}

func NewPoolInfo() *PoolInfo {
	pi := &PoolInfo{
		deserved: &Resource{},
		used: &Resource{},
		shared: &Resource{},
	}
	return pi
}

func (p *PoolInfo) AddNode(node *v1.Node) {
	if p == nil {
		return
	}
	p.nodes = append(p.nodes, node)
}

func (p *PoolInfo) AddPod(pod *v1.Pod) error {
	//res, non0CPU, non0Mem := calculateResource(pod)
	//p.used.MilliCPU += res.MilliCPU
	//p.used.
	// TODO add task to pool
	return nil
}

func (p *PoolInfo) GetPool() *v1alpha1.Pool {
	if p == nil {
		return nil
	}
	return p.pool
}

// SetPool sets the overall pool information
func (p *PoolInfo) SetPool(pool *v1alpha1.Pool, nodes []*v1.Node) error {
	if p == nil {
		return nil
	}

	p.pool = pool

	// TODO set other fields

	//_, err := p.MatchPoolNodes(nodes)
	return nil
}

// RemovePool removes the overall information about the pool
func (p *PoolInfo) RemovePool() error {
	if p == nil {
		return nil
	}
	p.pool = nil
	p.nodes = nil
	p.pods = nil
	p.deserved = &Resource{}
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

func (p *PoolInfo) GetNodeSize() int {
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

func (p *PoolInfo) Deserved() *Resource {
	if p == nil {
		return &Resource{}
	}
	return p.deserved
}

func (p *PoolInfo) SetDeserved(resource *Resource) {
	if p == nil {
		return
	}
	p.deserved = resource
}

func (p *PoolInfo) SetDeservedResource(name v1.ResourceName, value int64) {
	if p == nil || value < 0 {
		return
	}
	switch name {
	case v1.ResourceCPU:
		p.deserved.MilliCPU = value
	case v1.ResourceMemory:
		p.deserved.Memory = value
	case v1.ResourceEphemeralStorage:
		p.deserved.EphemeralStorage = value
	case ResourceGPU:
		p.deserved.SetScalar(name, value)
	default:
		p.deserved.SetScalar(name, value)
	}
}

func (p *PoolInfo) MatchNode(node *v1.Node) bool {
	if p == nil || node == nil {
		return false
	}
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

func (p *PoolInfo) MatchPoolNodes(nodes []*v1.Node) (*Resource, error) {
	if nodes == nil || len(nodes) == 0 {
		klog.Warning("Not a node cached, will not filter nodes for Pool")
		return &Resource{}, nil
	}

	if p == nil || p.pool.Name == DefaultPool ||
		(p.pool.Spec.NodeSelector == nil &&
			p.pool.Spec.SupportResources == nil) {
		return &Resource{}, nil
	}

	matchedNodes := make([]*v1.Node, 0, len(nodes))
	totalResource := &Resource{}
	for _, n := range nodes {
		if p.MatchNode(n) {
			matchedNodes = append(matchedNodes, n)
			totalResource.Add(n.Status.Allocatable)
		}
	}

	p.nodes = matchedNodes

	return totalResource, nil
}

func (p *PoolInfo) IsDefaultPool() bool {
	return p != nil && p.Name() == DefaultPool
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