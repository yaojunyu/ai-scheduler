package queue

import (
	"sync"
	"time"

	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/info"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

type PoolQueue struct {
	lock sync.RWMutex
	cond sync.Cond

	queues map[string]*util.Heap
}

// NewPoolQueue
func NewPoolQueue() *PoolQueue {
	pq := &PoolQueue{
		queues: make(map[string]*util.Heap),
	}
	pq.cond.L = &pq.lock

	return pq
}

// AddPoolQ
func (pq *PoolQueue) AddPoolQ(poolName string, q *util.Heap) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	if _, ok := pq.queues[poolName]; ok {
		klog.Warning("Pool queue %s already exist!")
	}
	pq.queues[poolName] = q
}

func (pq *PoolQueue) AddIfNotPresent(poolName string, pod *v1.Pod) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()

	pInfo := newPodInfoWithTimestamp(pod)
	for _, q := range pq.queues {
		if _, exists, _ := q.Get(pInfo); exists {
			return nil
		}
	}

	err := pq.getOrCreate(poolName).Add(pInfo)
	if err == nil {
		pq.cond.Broadcast()
	}
	return err
}

// Delete deletes pod
func (pq *PoolQueue) Delete(pod *v1.Pod) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	for _, q := range pq.queues {
		if err := q.Delete(newPodInfoNoTimestamp(pod)); err == nil {
			break
		}
	}
}

// Update
func (pq *PoolQueue) Update(oldPod, newPod *v1.Pod) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()

	// if oldPod not nil, exists
	if oldPod != nil {
		oldPodInfo := newPodInfoNoTimestamp(oldPod)
		for _, q := range pq.queues {
			if oldPodInfo, exists, _ := q.Get(oldPodInfo); exists {
				newPodInfo := newPodInfoNoTimestamp(newPod)
				newPodInfo.timestamp = oldPodInfo.(*podInfo).timestamp
				err := q.Update(newPodInfo)
				return err
			}
		}
	}

	// if oldPod is not any of the queues
	poolName := info.GetPodPoolName(newPod)
	err := pq.getOrCreate(poolName).Add(newPodInfoWithTimestamp(newPod))
	if err == nil {
		pq.cond.Broadcast()
	}
	return err
}

func newPodInfoWithTimestamp(pod *v1.Pod) *podInfo {
	return &podInfo{
		pod: pod,
		timestamp: time.Now(),
	}
}

func (pq *PoolQueue) getOrCreate(poolName string) *util.Heap {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	q, ok := pq.queues[poolName]
	if !ok {
		q = util.NewHeap(podInfoKeyFunc, poolQueuePodPriorityComp(poolName))
		pq.queues[poolName] = q
	}
	return q
}

// NumQueues return the len of queues
func (pq *PoolQueue) NumQueues() int {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	return len(pq.queues)
}

func poolQueuePodPriorityComp(poolName string) util.LessFunc {
	return func(podInfo1, podInfo2 interface{}) bool {
		pInfo1 := podInfo1.(*podInfo)
		pInfo2 := podInfo2.(*podInfo)
		prio1 := util.GetPodPriority(pInfo1.pod)
		prio2 := util.GetPodPriority(pInfo2.pod)

		pn1 := info.GetPodPoolName(pInfo1.pod)
		pn2 := info.GetPodPoolName(pInfo2.pod)

		if pn1 == poolName && pn2 == poolName {
			return (prio1 > prio2) || (prio1 == prio2 &&
				pInfo1.timestamp.Before(pInfo2.timestamp))
		} else if pn1 == poolName && pn2 != poolName {
			return true
		} else if pn1 != poolName && pn2 == poolName {
			return false
		} else {
			// TODO consider pool priority
			return (prio1 > prio2) || (prio1 == prio2 &&
				pInfo1.timestamp.Before(pInfo2.timestamp))
		}
	}
}
