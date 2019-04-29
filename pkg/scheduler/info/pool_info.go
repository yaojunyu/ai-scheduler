package info

import (
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

type PoolID types.UID

type PoolInfo struct {
	UID PoolID

	Pool *v1alpha1.Pool

	Deserved *Resource
	Used *Resource
	Borrowed *Resource
}

func NewPoolInfo(pool *v1alpha1.Pool) *PoolInfo {
	poolInfo := &PoolInfo{
		UID: PoolID(pool.Name),
		Pool: pool,
	}

	poolInfo.Deserved = NewResource(pool.Status.Deserved)
	poolInfo.Used = NewResource(pool.Status.Used)
	poolInfo.Borrowed = NewResource(pool.Status.Borrowed)

	return poolInfo
}
