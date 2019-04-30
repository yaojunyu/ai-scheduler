package info

import (
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

type PoolInfo struct {
	pool *v1alpha1.Pool

	// All nodes allocated to this pool
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

func NewPoolInfo(nodes ...*v1.Node) *PoolInfo {
	pi := &PoolInfo{
		deserved: &Resource{},
		used: &Resource{},
		shared: &Resource{},
	}
	for _, node := range nodes {
		pi.AddNode(node)
	}
	return pi
}

func (p *PoolInfo) AddNode(node *v1.Node) {
	res := NewResource(node.Status.Allocatable)
	p.deserved.MilliCPU += res.MilliCPU
	p.deserved.Memory += res.Memory
	p.deserved.EphemeralStorage += res.EphemeralStorage
	if p.deserved.ScalarResources == nil && len(res.ScalarResources) > 0 {
		p.deserved.ScalarResources = map[v1.ResourceName]int64{}
	}
	for rName, rQuant := range res.ScalarResources {
		p.deserved.ScalarResources[rName] += rQuant
	}
	// TODO non-zero resources
	p.nodes = append(p.nodes, node)
}

func (p *PoolInfo) AddPod(pod *v1.Pod) error {
	//res, non0CPU, non0Mem := calculateResource(pod)
	//p.used.MilliCPU += res.MilliCPU
	//p.used.
	// TODO add task to pool
	return nil
}

// SetPool sets the overall pool information
func (p *PoolInfo) SetPool(pool *v1alpha1.Pool) error {
	p.pool = pool

	// FIXME
	p.deserved = NewResource(pool.Status.Deserved)
	p.used = NewResource(pool.Status.Used)
	p.shared = NewResource(pool.Status.Shared)

	return nil
}

// RemovePool removes the overall information about the pool
func (p *PoolInfo) RemovePool(pool *v1alpha1.Pool) error {
	p.pool = nil
	p.nodes = nil
	p.pods = nil
	p.deserved = &Resource{}
	p.used = &Resource{}
	p.shared = &Resource{}

	return nil
}
