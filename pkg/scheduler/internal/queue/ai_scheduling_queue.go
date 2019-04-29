package queue

import (
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/info"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/util"
	v1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/tools/cache"
	"sync"
)

type AISchedulingQueue interface {
	GetJobQueues() map[string]*util.Heap
	AddJob(job *v1.Job) error
}

var _ = AISchedulingQueue(&AIQueue{})

func NewAISchedulingQueue() AISchedulingQueue {
	return NewAIQueue()
}

type AIQueue struct {
	lock sync.Mutex

	// queue for scheduling pool
	poolQueue *util.Heap

	// map key pool to value job queue
	jobQueues map[string]*util.Heap
}

func NewAIQueue() *AIQueue {
	aq := &AIQueue{
		poolQueue: util.NewHeap(poolKeyFunc, poolQueueComp),
		jobQueues: make(map[string]*util.Heap),
	}

	return aq
}

func (aq *AIQueue) AddJob(jobInfo *info.JobInfo) error {
	aq.lock.Lock()
	defer aq.lock.Unlock()

	// TODO get pool form job's annotation
	pool := &v1alpha1.Pool{}


	aq.poolQueue.Add(jobInfo)

	aq.jobQueues[pool.Name] = util.NewHeap(jobKeyFunc, jobQueueComp)
	aq.jobQueues[pool.Name].Add(jobInfo.Job)

	return nil
}

func (aq *AIQueue) GetJobQueues() map[string]*util.Heap {
	return aq.jobQueues
}

func jobKeyFunc(obj interface{}) (string, error) {
	return cache.MetaNamespaceKeyFunc(obj.(*v1.Job))
}

func jobQueueComp(j1, j2 interface{}) bool {
	jobinfo1 := j1.(*info.JobInfo)
	jobinfo2 := j2.(*info.JobInfo)

	// TODO set self pool has high priority
	return jobinfo1.Job.CreationTimestamp.Before(&jobinfo2.Job.CreationTimestamp)
}

func poolKeyFunc(obj interface{}) (string, error) {
	return cache.MetaNamespaceKeyFunc(obj.(*v1alpha1.Pool))
}

func poolQueueComp(pool1, pool2 interface{}) bool {
	p1 := pool1.(*v1alpha1.Pool)
	p2 := pool2.(*v1alpha1.Pool)

	if p1.CreationTimestamp.Equal(&p2.CreationTimestamp) {
		return p1.UID < p2.UID
	}
	return p1.CreationTimestamp.Before(&p2.CreationTimestamp)
}


