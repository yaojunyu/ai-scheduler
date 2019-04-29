package framework

import (
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/info"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/internal/cache"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/nodeinfo"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/klog"
)

type Session struct {
	// UID the unique id of session
	UID types.UID

	// cache the scheduling cache
	cache cache.Cache

	Jobs map[info.JobID]*info.JobInfo
	Nodes map[string]*nodeinfo.NodeInfo
	Pools map[string]*v1alpha1.Pool

	Plugins map[string]Plugin


	jobValidFns map[string]ValidateExFn

}

// openSession init session
func openSession(cache cache.Cache) *Session {
	ssn := &Session {
		UID: uuid.NewUUID(),
		cache: cache,

		Jobs: map[string]*info.JobInfo{},
		Nodes: map[string]*nodeinfo.NodeInfo{},
		Pools: map[string]*v1alpha1.Pool{},

		Plugins: map[string]Plugin{},

	}

	// snapshot
	snapshot := cache.Snapshot()
	ssn.Jobs = snapshot.Jobs
	for _, job := range ssn.Jobs {
		if result := ssn.JobValid(job); result != nil {
			if !result.Pass {
				// TODO record condition
			}

			delete(ssn.Jobs, job.UID)
		}
	}
	ssn.Pools = snapshot.Pools
	ssn.Nodes = snapshot.Nodes

	klog.V(3).Infof("Open Session %v with %d Job(s) and %d Pool(s)",
		ssn.UID, len(ssn.Jobs), len(ssn.Pools))

	return ssn
}

func closeSession(ssn *Session) {
	//for _, job := range snn.Jobs {
	//
	//}

	ssn.Jobs = nil
	ssn.Nodes = nil
	ssn.Pools = nil
	ssn.Plugins = nil

	klog.V(3).Infof("Close Session %v", ssn.UID)
}

// JobValid invoke jobValid function of all plugins
func (ssn *Session) JobValid(obj interface{}) *ValidateResult {
	for _, plugin := range ssn.Plugins {
		validFn, found := ssn.jobValidFns[plugin.Name()]
		if !found {
			continue
		}

		if vr := validFn(obj); vr != nil && !vr.Pass {
			return vr
		}
	}

	return nil
}

// PoolOrderFn
func (ssn *Session) PoolOrderFunc(l, r interface{}) bool {
	// If no queue order funcs, order queue by CreationTimestamp first, then by UID.
	lv := l.(*v1alpha1.Pool)
	rv := r.(*v1alpha1.Pool)
	if lv.CreationTimestamp.Equal(&rv.CreationTimestamp) {
		return lv.UID < rv.UID
	}
	return lv.CreationTimestamp.Before(&rv.CreationTimestamp)
}

// JobOrderFn invoke joborder function of the plugins
func (ssn *Session) JobOrderFn(l, r interface{}) bool {

	// If no job order funcs, order job by CreationTimestamp first, then by UID.
	lv := l.(*info.JobInfo)
	rv := r.(*info.JobInfo)
	if lv.Job.CreationTimestamp.Equal(&rv.Job.CreationTimestamp) {
		return lv.Job.UID < rv.Job.UID
	}
	return lv.Job.CreationTimestamp.Before(&rv.Job.CreationTimestamp)

}