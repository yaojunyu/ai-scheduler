package queue

import (
	"fmt"
	"sync"

	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/info"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/util"
	"k8s.io/api/core/v1"
	"k8s.io/klog"
)

type SchedulingPoolQueue interface {
	GetQueue(poolName string) (SchedulingQueue, error)
	AddQueue(poolName string, stopCh <-chan struct{}) (SchedulingQueue, error)
	RemoveQueue(poolName string) error
	Add(pod *v1.Pod) error
	AddIfNotPresent(pod *v1.Pod) error
	Delete(pod *v1.Pod) error
	Update(oldPod, newPod *v1.Pod) error
	NumQueues() int
	Queues() map[string]SchedulingQueue
	NumUnschedulablePods() int
	MoveAllToActiveQueue()
	AssignedPodAdded(pod *v1.Pod)
	AssignedPodUpdated(pod *v1.Pod)
	PendingPods() []*v1.Pod
	NominatedPodsForNode(nodeName string) []*v1.Pod

	GetPoolQueueNameIfNotPresent(pod *v1.Pod) string
	Close()
}

var _ = SchedulingPoolQueue(&PoolQueue{})

type PoolQueue struct {
	lock sync.RWMutex
	cond sync.Cond

	// queues is queue map for pool name as key and podInfo heap as value
	queues map[string]SchedulingQueue

	// startCh to start a goroutine for pool
	//startCh chan string

	//predicates               map[string]predicates.FitPredicate
	//priorityMetaProducer     priorities.PriorityMetadataProducer
	//predicateMetaProducer    predicates.PredicateMetadataProducer
	//prioritizers             []priorities.PriorityConfig
	//lastNodeIndex            uint64
	//alwaysCheckAllPredicates bool
	//disablePreemption        bool
	//percentageOfNodesToScore int32
}

// NewPoolQueue
func NewPoolQueue(stopCh <-chan struct{}) *PoolQueue {
	pq := &PoolQueue{
		queues: make(map[string]SchedulingQueue),
	}
	pq.cond.L = &pq.lock

	// init queue for pod that not belong to any pool
	pq.queues[info.DefaultPoolName] = NewSchedulingQueue(pq.poolQueuePodPriorityComp(info.DefaultPoolName), stopCh)

	return pq
}

func (pq *PoolQueue) GetQueue(poolName string) (SchedulingQueue, error) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	if q, ok := pq.queues[poolName]; ok {
		return q, nil
	}
	return nil, fmt.Errorf("queue for pool %s not found", poolName)
}

// AddPoolQ
func (pq *PoolQueue) AddQueue(poolName string, stopCh <-chan struct{}) (SchedulingQueue, error) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	q, ok := pq.queues[poolName]
	if !ok {
		q = NewSchedulingQueue(pq.poolQueuePodPriorityComp(poolName), stopCh)
		pq.queues[poolName] = q
	}
	return q, nil
}

func (pq *PoolQueue) RemoveQueue(poolName string) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	if _, ok := pq.queues[poolName]; !ok {
		return fmt.Errorf("pool queue %v not found", poolName)
	}
	delete(pq.queues, poolName)
	return nil
}

func(pq *PoolQueue) Add(pod *v1.Pod) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodAnnotationsPoolName(pod)
	q := pq.getPriorityQueue(poolName)
	klog.Infof("add pod %v/%v to pool queue %v", pod.Namespace, pod.Name, poolName)
	return q.Add(pod)
}

func (pq *PoolQueue) AddIfNotPresent(pod *v1.Pod) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodAnnotationsPoolName(pod)
	q := pq.getPriorityQueue(poolName)
	return q.AddIfNotPresent(pod)
}

// Delete deletes pod
func (pq *PoolQueue) Delete(pod *v1.Pod) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodAnnotationsPoolName(pod)
	q := pq.getPriorityQueue(poolName)
	return q.Delete(pod)
}

// Update
func (pq *PoolQueue) Update(oldPod, newPod *v1.Pod) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodAnnotationsPoolName(oldPod)
	q := pq.getPriorityQueue(poolName)
	return q.Update(oldPod, newPod)
}

// NumQueues return the len of queues
func (pq *PoolQueue) NumQueues() int {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	return len(pq.queues)
}

func (pq *PoolQueue) Queues() map[string]SchedulingQueue {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	return pq.queues
}

func (pq *PoolQueue) NumUnschedulablePods() int {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	num := 0
	for _, q := range pq.queues {
		num += q.NumUnschedulablePods()
	}
	return num
}

func (pq *PoolQueue) MoveAllToActiveQueue() {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	for _, q := range pq.queues {
		q.MoveAllToActiveQueue()
	}
}

func (pq *PoolQueue) AssignedPodAdded(pod *v1.Pod) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodAnnotationsPoolName(pod)
	q := pq.getPriorityQueue(poolName)
	q.AssignedPodAdded(pod)
}

func (pq *PoolQueue) AssignedPodUpdated(pod *v1.Pod) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodAnnotationsPoolName(pod)
	q := pq.getPriorityQueue(poolName)
	q.AssignedPodUpdated(pod)
}

func (pq *PoolQueue) PendingPods() []*v1.Pod {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	result := []*v1.Pod{}
	for _, q := range pq.queues {
		result = append(result, q.PendingPods()...)
	}
	return result
}

func (pq *PoolQueue) NominatedPodsForNode(nodeName string) []*v1.Pod {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	result := []*v1.Pod{}
	for _, q := range pq.queues {
		// TODO remove the same pods
		result = append(result, q.NominatedPodsForNode(nodeName)...)
	}
	return result
}

func (pq *PoolQueue) Close() {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	for _, q := range pq.queues {
		q.Close()
	}
}

func (pq *PoolQueue) CloseQ(poolName string) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	q, ok := pq.queues[poolName]
	if !ok {
		klog.Errorf("Error: pool queue %v not exists, close failed", poolName)
		return
	}
	q.Close()
}

func (pq *PoolQueue) getPriorityQueue(poolName string) SchedulingQueue {
	if q, ok := pq.queues[poolName]; ok {
		return q
	}
	// if not found return default queue
	klog.Warningf("Warning: queue for pool %v not found, use default queue", poolName)
	return pq.queues[info.DefaultPoolName]
}

// poolQueuePodPriorityComp task has same pool name will get higher priority
func (pq *PoolQueue) poolQueuePodPriorityComp(poolName string) util.LessFunc {
	return func(podInfo1, podInfo2 interface{}) bool {
		pInfo1 := podInfo1.(*podInfo)
		pInfo2 := podInfo2.(*podInfo)
		prio1 := util.GetPodPriority(pInfo1.pod)
		prio2 := util.GetPodPriority(pInfo2.pod)

		pn1 := pq.GetPoolQueueNameIfNotPresent(pInfo1.pod)
		pn2 := pq.GetPoolQueueNameIfNotPresent(pInfo2.pod)

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

// getPoolQueueNameIfNotPresent return pool name by pod annotations, if not found return default name
func (pq *PoolQueue) GetPoolQueueNameIfNotPresent(pod *v1.Pod) string {
	poolName := info.GetPodAnnotationsPoolName(pod)
	if _, ok := pq.queues[poolName]; ok {
		return poolName
	}
	return info.DefaultPoolName
}
