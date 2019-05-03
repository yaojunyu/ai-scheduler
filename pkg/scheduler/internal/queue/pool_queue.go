package queue

import (
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/info"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/util"
	v1 "k8s.io/api/core/v1"
	"sync"
)

const (
	// DefaultPoolQueueName
	DefaultPoolQueueName = "default"
)
type PoolQueue struct {
	lock sync.RWMutex
	cond sync.Cond

	// queues is queue map for pool name as key and podInfo heap as value

	queues map[string]*PriorityQueue
}

// NewPoolQueue
func NewPoolQueue(stopCh <-chan struct{}) *PoolQueue {
	pq := &PoolQueue{
		queues: make(map[string]*PriorityQueue),
	}
	pq.cond.L = &pq.lock

	// init queue for pod that not belong to any pool
	pq.queues[DefaultPoolQueueName] = NewPriorityQueue(stopCh)

	return pq
}

// AddPoolQ
func (pq *PoolQueue) AddPoolQ(poolName string, q *PriorityQueue) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	if _, ok := pq.queues[poolName]; ok {
		return nil
	}

	pq.queues[poolName] = q
	return nil
}

func (pq *PoolQueue) AddIfNotPresent(pod *v1.Pod) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodPoolName(pod)
	q := pq.getPriorityQueue(poolName)
	return q.AddIfNotPresent(pod)
}

func (pq *PoolQueue) getPriorityQueue(poolName string) *PriorityQueue {
	if q, ok := pq.queues[poolName]; ok {
		return q
	}
	// if not found return default queue
	return pq.queues[DefaultPoolQueueName]
}

// Delete deletes pod
func (pq *PoolQueue) Delete(pod *v1.Pod) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodPoolName(pod)
	q := pq.getPriorityQueue(poolName)
	return q.Delete(pod)
}

// Update
func (pq *PoolQueue) Update(oldPod, newPod *v1.Pod) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodPoolName(oldPod)
	q := pq.getPriorityQueue(poolName)
	return q.Update(oldPod, newPod)
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
