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

