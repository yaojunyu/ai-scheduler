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
	RemoveQueue(poolName string)
	Add(pod *v1.Pod) error
	AddIfNotPresent(pod *v1.Pod) error
	Delete(pod *v1.Pod) error
	Update(oldPod, newPod *v1.Pod) error
	NumQueues() int
	Queues() map[string]SchedulingQueue
	NumUnschedulablePods() int
	NumUnschedulablePodsIn(poolName string) int
	MoveAllToActiveQueue()
	MoveAllToActiveQueueIn(poolName string)
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
	pq.queues[info.DefaultPoolName] = NewSchedulingQueueWithLessFunc(pq.poolQueuePodPriorityComp(info.DefaultPoolName), stopCh)

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

// JUST for test
func (pq *PoolQueue) SetQueue(poolName string, queue SchedulingQueue) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	pq.queues[poolName] = queue
}

// AddPoolQ
func (pq *PoolQueue) AddQueue(poolName string, stopCh <-chan struct{}) (SchedulingQueue, error) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	q, ok := pq.queues[poolName]
	if !ok {
		q = NewSchedulingQueueWithLessFunc(pq.poolQueuePodPriorityComp(poolName), stopCh)
		pq.queues[poolName] = q
	}
	return q, nil
}

func (pq *PoolQueue) RemoveQueue(poolName string) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	q, ok := pq.queues[poolName]
	if !ok {
		return
	}
	delete(pq.queues, poolName)
	if df, ok := pq.queues[info.DefaultPoolName]; !ok {
		klog.Errorf("default pool queue not exists")
	} else {
		// remove all pending pods to default pool queue
		for _, pod := range q.PendingPods() {
			if err := df.AddIfNotPresent(pod); err != nil {
				klog.Errorf("Error move pod %v/%v to default pool queue failed: %v", pod.Namespace, pod.Name, err)
			} else {
				klog.V(4).Infof("moved pod %v/%v from pool queue %v to default pool queue", pod.Namespace, pod.Name, poolName)
			}
		}
	}
	q.Close()
}

func (pq *PoolQueue) Add(pod *v1.Pod) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodAnnotationsPoolName(pod)
	q, n, err := pq.getPriorityQueue(poolName)
	if err != nil {
		return err
	}
	klog.V(4).Infof("Add pod %v/%v to pool queue '%v'", pod.Namespace, pod.Name, n)
	return q.Add(pod)
}

func (pq *PoolQueue) AddIfNotPresent(pod *v1.Pod) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodAnnotationsPoolName(pod)
	q, n, err := pq.getPriorityQueue(poolName)
	if err != nil {
		return err
	}
	klog.V(4).Infof("AddIfNotPresent pod %v/%v to pool queue '%v'", pod.Namespace, pod.Name, n)
	return q.AddIfNotPresent(pod)
}

// Delete deletes pod
func (pq *PoolQueue) Delete(pod *v1.Pod) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodAnnotationsPoolName(pod)
	q, n, err := pq.getPriorityQueue(poolName)
	if err != nil {
		return err
	}
	klog.V(4).Infof("Delete pod %v/%v from pool queue '%v'", pod.Namespace, pod.Name, n)
	return q.Delete(pod)
}

// Update
func (pq *PoolQueue) Update(oldPod, newPod *v1.Pod) error {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodAnnotationsPoolName(oldPod)
	q, n, err := pq.getPriorityQueue(poolName)
	if err != nil {
		return err
	}
	klog.V(4).Infof("Update pod from %v/%v to %v/%v in pool queue '%v'",
		oldPod.Namespace, oldPod.Name, newPod.Namespace, newPod.Name, n)
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

func (pq *PoolQueue) NumUnschedulablePodsIn(poolName string) int {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	if q, ok := pq.queues[poolName]; ok {
		return q.NumUnschedulablePods()
	}
	return 0
}

func (pq *PoolQueue) MoveAllToActiveQueue() {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	for _, q := range pq.queues {
		q.MoveAllToActiveQueue()
	}
}

func (pq *PoolQueue) MoveAllToActiveQueueIn(poolName string) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	if q, ok := pq.queues[poolName]; ok {
		q.MoveAllToActiveQueue()
	}
}

func (pq *PoolQueue) AssignedPodAdded(pod *v1.Pod) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodAnnotationsPoolName(pod)
	q, _, err := pq.getPriorityQueue(poolName)
	if err != nil {
		klog.Errorf("Failed getting queue %v when AssignedPodAdded", poolName)
		return
	}
	q.AssignedPodAdded(pod)
}

func (pq *PoolQueue) AssignedPodUpdated(pod *v1.Pod) {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	poolName := info.GetPodAnnotationsPoolName(pod)
	q, _, err := pq.getPriorityQueue(poolName)
	if err != nil {
		klog.Errorf("Failed getting queue %v when AssignedPodUpdated", poolName)
		return
	}
	q.AssignedPodUpdated(pod)
}

func (pq *PoolQueue) PendingPods() []*v1.Pod {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	var result []*v1.Pod
	for _, q := range pq.queues {
		result = append(result, q.PendingPods()...)
	}
	return result
}

func (pq *PoolQueue) NominatedPodsForNode(nodeName string) []*v1.Pod {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	var result []*v1.Pod
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

func (pq *PoolQueue) getPriorityQueue(poolName string) (SchedulingQueue, string, error) {
	if q, ok := pq.queues[poolName]; ok {
		return q, poolName, nil
	}
	// if not found return default queue
	klog.Warningf("Warning: queue for pool %v not found, use default queue", poolName)
	q, ok := pq.queues[info.DefaultPoolName]
	if ok {
		return q, info.DefaultPoolName, nil
	}
	return nil, "", fmt.Errorf("build-in default pool queue not found")
}

// poolQueuePodPriorityComp task has same pool name will get higher priority
func (pq *PoolQueue) poolQueuePodPriorityComp(poolName string) util.LessFunc {
	return func(podInfo1, podInfo2 interface{}) bool {
		pInfo1 := podInfo1.(*podInfo)
		pInfo2 := podInfo2.(*podInfo)
		prio1 := util.GetPodPriority(pInfo1.pod)
		prio2 := util.GetPodPriority(pInfo2.pod)

		// must not lock
		pn1 := pq.matchPoolQueueNameForPod(pInfo1.pod)
		pn2 := pq.matchPoolQueueNameForPod(pInfo2.pod)

		if pn1 == poolName && pn2 == poolName {
			// self pool task
			return (prio1 > prio2) || (prio1 == prio2 &&
				pInfo1.timestamp.Before(pInfo2.timestamp))
		} else if pn1 == poolName && pn2 != poolName {
			return true
		} else if pn1 != poolName && pn2 == poolName {
			return false
		} else {
			// both are jobs that need borrowing
			// TODO consider pool priority
			return (prio1 > prio2) || (prio1 == prio2 &&
				pInfo1.timestamp.Before(pInfo2.timestamp))
		}
	}
}

// getPoolQueueNameIfNotPresent return pool name by pod annotations, if not found return default name
func (pq *PoolQueue) GetPoolQueueNameIfNotPresent(pod *v1.Pod) string {
	pq.lock.Lock()
	defer pq.lock.Unlock()
	return pq.matchPoolQueueNameForPod(pod)
}

func (pq *PoolQueue) matchPoolQueueNameForPod(pod *v1.Pod) string {
	poolName := info.GetPodAnnotationsPoolName(pod)
	if _, ok := pq.queues[poolName]; ok {
		return poolName
	}
	return info.DefaultPoolName
}
