package allocate

import (
	"github.com/kubernetes-sigs/kube-batch/pkg/scheduler/api"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/framework"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/nodeinfo"
	"k8s.io/klog"
)

type allocationAction struct {
	name string
}

var _ = framework.Action(&allocationAction{})

// New
func New() *allocationAction {
	return &allocationAction{
		name: "allocate",
	}
}

// Name return the unique name of Action
func (a *allocationAction) Name() string {
	return a.name
}

// Execute execute the main logic of action
func (a *allocationAction) Execute(ssn *framework.Session) {
	klog.V(3).Infof("Enter allocate ...")
	defer klog.V(3).Infof("Exit allocate ...")

	//poolQ := util.NewPriorityQueue(ssn.PoolOrderFunc)
	//jobQueues := map[string]*util.PriorityQueue{}
	//
	//for _, job := range ssn.Jobs {
	//	if pool, found := ssn.Pools[job.Pool.Name]; found {
	//		poolQ.Push(pool)
	//	} else {
	//		klog.Warningf("Skip adding Job %s/%s because its pool %s is not found",
	//			job.Job.Namespace, job.Job.Name, job.Pool.Name)
	//		continue
	//	}
	//
	//	if _, found := jobQueues[job.Pool.Name]; !found {
	//		jobQueues[job.Pool.Name] = util.NewPriorityQueue(ssn.JobOrderFn)
	//	}
	//
	//	klog.V(4).Infof("Added Job <%s/%s> into Pool <%s>", job.Job.Namespace, job.Job.Name, job.Pool.Name)
	//	jobQueues[job.Pool.Name].Push(job)
	//}
	//
	//klog.V(3).Infof("Try to allocate resource to %d Pools", len(jobQueues))
	//
	//pendingTasks := map[string]*util.PriorityQueue{}
	//allNodes := GetNodeList(ssn.Nodes)
	//
	//predicateFn := func(task *info.TaskInfo, node *api.NodeInfo) error {
	//	// Check for Resource Predicate
	//	// TODO: We could not allocate resource to task from both node.Idle and node.Releasing now,
	//	// after it is done, we could change the following compare to:
	//	// clonedNode := node.Idle.Clone()
	//	// if !task.InitResreq.LessEqual(clonedNode.Add(node.Releasing)) {
	//	//    ...
	//	// }
	//	if !task.InitResreq.LessEqual(node.Idle) && !task.InitResreq.LessEqual(node.Releasing) {
	//		return fmt.Errorf("task <%s/%s> ResourceFit failed on node <%s>",
	//			task.Namespace, task.Name, node.Name)
	//	}
	//
	//	return ssn.PredicateFn(task, node)
	//}
	//
	//for {
	//	if queues.Empty() {
	//		break
	//	}
	//
	//	queue := queues.Pop().(*api.QueueInfo)
	//	if ssn.Overused(queue) {
	//		glog.V(3).Infof("Queue <%s> is overused, ignore it.", queue.Name)
	//		continue
	//	}
	//
	//	jobs, found := jobsMap[queue.UID]
	//
	//	glog.V(3).Infof("Try to allocate resource to Jobs in Queue <%v>", queue.Name)
	//
	//	if !found || jobs.Empty() {
	//		glog.V(4).Infof("Can not find jobs for queue %s.", queue.Name)
	//		continue
	//	}
	//
	//	job := jobs.Pop().(*api.JobInfo)
	//	if _, found := pendingTasks[job.UID]; !found {
	//		tasks := util.NewPriorityQueue(ssn.TaskOrderFn)
	//		for _, task := range job.TaskStatusIndex[api.Pending] {
	//			// Skip BestEffort task in 'allocate' action.
	//			if task.Resreq.IsEmpty() {
	//				glog.V(4).Infof("Task <%v/%v> is BestEffort task, skip it.",
	//					task.Namespace, task.Name)
	//				continue
	//			}
	//
	//			tasks.Push(task)
	//		}
	//		pendingTasks[job.UID] = tasks
	//	}
	//	tasks := pendingTasks[job.UID]
	//
	//	glog.V(3).Infof("Try to allocate resource to %d tasks of Job <%v/%v>",
	//		tasks.Len(), job.Namespace, job.Name)
	//
	//	for !tasks.Empty() {
	//		task := tasks.Pop().(*api.TaskInfo)
	//
	//		glog.V(3).Infof("There are <%d> nodes for Job <%v/%v>",
	//			len(ssn.Nodes), job.Namespace, job.Name)
	//
	//		//any task that doesn't fit will be the last processed
	//		//within this loop context so any existing contents of
	//		//NodesFitDelta are for tasks that eventually did fit on a
	//		//node
	//		if len(job.NodesFitDelta) > 0 {
	//			job.NodesFitDelta = make(api.NodeResourceMap)
	//		}
	//
	//		predicateNodes := util.PredicateNodes(task, allNodes, predicateFn)
	//		if len(predicateNodes) == 0 {
	//			break
	//		}
	//
	//		nodeScores := util.PrioritizeNodes(task, predicateNodes, ssn.NodeOrderFn)
	//
	//		node := util.SelectBestNode(nodeScores)
	//		// Allocate idle resource to the task.
	//		if task.InitResreq.LessEqual(node.Idle) {
	//			glog.V(3).Infof("Binding Task <%v/%v> to node <%v>",
	//				task.Namespace, task.Name, node.Name)
	//			if err := ssn.Allocate(task, node.Name); err != nil {
	//				glog.Errorf("Failed to bind Task %v on %v in Session %v, err: %v",
	//					task.UID, node.Name, ssn.UID, err)
	//			}
	//		} else {
	//			//store information about missing resources
	//			job.NodesFitDelta[node.Name] = node.Idle.Clone()
	//			job.NodesFitDelta[node.Name].FitDelta(task.InitResreq)
	//			glog.V(3).Infof("Predicates failed for task <%s/%s> on node <%s> with limited resources",
	//				task.Namespace, task.Name, node.Name)
	//
	//			// Allocate releasing resource to the task if any.
	//			if task.InitResreq.LessEqual(node.Releasing) {
	//				glog.V(3).Infof("Pipelining Task <%v/%v> to node <%v> for <%v> on <%v>",
	//					task.Namespace, task.Name, node.Name, task.InitResreq, node.Releasing)
	//				if err := ssn.Pipeline(task, node.Name); err != nil {
	//					glog.Errorf("Failed to pipeline Task %v on %v in Session %v",
	//						task.UID, node.Name, ssn.UID)
	//				}
	//			}
	//		}
	//
	//		if ssn.JobReady(job) {
	//			jobs.Push(job)
	//			break
	//		}
	//	}
	//
	//	// Added Queue back until no job in Queue.
	//	queues.Push(queue)
	//}
}

// GetNodeList returns values of the map 'nodes'
func GetNodeList(nodes map[string]*nodeinfo.NodeInfo) []*api.NodeInfo {
	result := make([]*api.NodeInfo, 0, len(nodes))
	for _, v := range nodes {
		result = append(result, v)
	}
	return result
}
