package info

import (
	"fmt"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sort"
	"strings"

	policyv1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JobID is the type of JobInfo's ID.
type JobID types.UID

type tasksMap map[TaskID]*TaskInfo

// NodeResourceMap stores resource in a node
type NodeResourceMap map[string]*Resource

// JobInfo will have all info of a Job
type JobInfo struct {
	UID JobID

	Name      string
	Namespace string

	//Queue QueueID
	PoolID PoolID

	Priority int32

	NodeSelector map[string]string
	MinAvailable int32

	NodesFitDelta NodeResourceMap

	// All tasks of the Job.
	TaskStatusIndex map[TaskStatus]tasksMap
	Tasks           tasksMap

	Allocated    *Resource
	TotalRequest *Resource

	CreationTimestamp metav1.Time
	//PodGroup          *v1alpha1.PodGroup
	Pool 			  *v1alpha1.Pool

	// TODO(k82cn): keep backward compatibility, removed it when v1alpha1 finalized.
	PDB *policyv1.PodDisruptionBudget
}


// NewJobInfo creates a new jobInfo for set of tasks
func NewJobInfo(uid JobID, tasks ...*TaskInfo) *JobInfo {
	job := &JobInfo{
		UID: uid,

		MinAvailable:  0,
		NodeSelector:  make(map[string]string),
		NodesFitDelta: make(NodeResourceMap),
		Allocated:     EmptyResource(),
		TotalRequest:  EmptyResource(),

		TaskStatusIndex: map[TaskStatus]tasksMap{},
		Tasks:           tasksMap{},
	}

	for _, task := range tasks {
		job.AddTaskInfo(task)
	}

	return job
}

// AddTaskInfo is used to add a task to a job
func (ji *JobInfo) AddTaskInfo(ti *TaskInfo) {
	ji.Tasks[ti.UID] = ti
	ji.addTaskIndex(ti)

	ji.TotalRequest.Add(ti.Resreq)

	if AllocatedStatus(ti.Status) {
		ji.Allocated.Add(ti.Resreq)
	}
}

// UnsetPodGroup removes podGroup details from a job
func (ji *JobInfo) UnsetPodGroup() {
	//ji.PodGroup = nil
}

// SetPodGroup sets podGroup details to a job
//func (ji *JobInfo) SetPodGroup(pg *v1alpha1.PodGroup) {
//	ji.Name = pg.Name
//	ji.Namespace = pg.Namespace
//	ji.MinAvailable = pg.Spec.MinMember
//	ji.Queue = QueueID(pg.Spec.Queue)
//	ji.CreationTimestamp = pg.GetCreationTimestamp()
//
//	ji.PodGroup = pg
//}

// SetPDB sets PDB to a job
func (ji *JobInfo) SetPDB(pdb *policyv1.PodDisruptionBudget) {
	ji.Name = pdb.Name
	ji.MinAvailable = pdb.Spec.MinAvailable.IntVal
	ji.Namespace = pdb.Namespace

	ji.CreationTimestamp = pdb.GetCreationTimestamp()
	ji.PDB = pdb
}

// UnsetPDB removes PDB info of a job
func (ji *JobInfo) UnsetPDB() {
	ji.PDB = nil
}

// GetTasks gets all tasks with the taskStatus
func (ji *JobInfo) GetTasks(statuses ...TaskStatus) []*TaskInfo {
	var res []*TaskInfo

	for _, status := range statuses {
		if tasks, found := ji.TaskStatusIndex[status]; found {
			for _, task := range tasks {
				res = append(res, task.Clone())
			}
		}
	}

	return res
}


// UpdateTaskStatus is used to update task's status in a job
func (ji *JobInfo) UpdateTaskStatus(task *TaskInfo, status TaskStatus) error {
	if err := validateStatusUpdate(task.Status, status); err != nil {
		return err
	}

	// Remove the task from the task list firstly
	ji.DeleteTaskInfo(task)

	// Update task's status to the target status
	task.Status = status
	ji.AddTaskInfo(task)

	return nil
}


// DeleteTaskInfo is used to delete a task from a job
func (ji *JobInfo) DeleteTaskInfo(ti *TaskInfo) error {
	if task, found := ji.Tasks[ti.UID]; found {
		ji.TotalRequest.Sub(task.Resreq)

		if AllocatedStatus(task.Status) {
			ji.Allocated.Sub(task.Resreq)
		}

		delete(ji.Tasks, task.UID)

		ji.deleteTaskIndex(task)
		return nil
	}

	return fmt.Errorf("failed to find task <%v/%v> in job <%v/%v>",
		ti.Namespace, ti.Name, ji.Namespace, ji.Name)
}


// Clone is used to clone a jobInfo object
func (ji *JobInfo) Clone() *JobInfo {
	info := &JobInfo{
		UID:       ji.UID,
		Name:      ji.Name,
		Namespace: ji.Namespace,
		//Queue:     ji.Queue,
		PoolID:    ji.PoolID,
		Priority:  ji.Priority,

		MinAvailable:  ji.MinAvailable,
		NodeSelector:  map[string]string{},
		Allocated:     EmptyResource(),
		TotalRequest:  EmptyResource(),
		NodesFitDelta: make(NodeResourceMap),

		PDB:      ji.PDB,
		//PodGroup: ji.PodGroup,

		TaskStatusIndex: map[TaskStatus]tasksMap{},
		Tasks:           tasksMap{},
	}

	ji.CreationTimestamp.DeepCopyInto(&info.CreationTimestamp)

	for k, v := range ji.NodeSelector {
		info.NodeSelector[k] = v
	}

	for _, task := range ji.Tasks {
		info.AddTaskInfo(task.Clone())
	}

	return info
}

// String returns a jobInfo object in string format
func (ji JobInfo) String() string {
	res := ""

	i := 0
	for _, task := range ji.Tasks {
		res = res + fmt.Sprintf("\n\t %d: %v", i, task)
		i++
	}

	return fmt.Sprintf("Job (%v): namespace %v (%v), name %v, minAvailable %d",
		ji.UID, ji.Namespace, ji.PoolID, ji.Name, ji.MinAvailable, /*ji.PodGroup*/) + res
}


// FitError returns detailed information on why a job's task failed to fit on
// each available node
func (ji *JobInfo) FitError() string {
	if len(ji.NodesFitDelta) == 0 {
		reasonMsg := fmt.Sprintf("0 nodes are available")
		return reasonMsg
	}

	reasons := make(map[string]int)
	for _, v := range ji.NodesFitDelta {
		if v.Get(v1.ResourceCPU) < 0 {
			reasons["cpu"]++
		}
		if v.Get(v1.ResourceMemory) < 0 {
			reasons["memory"]++
		}

		for rName, rQuant := range v.ScalarResources {
			if rQuant < 0 {
				reasons[string(rName)]++
			}
		}
	}

	sortReasonsHistogram := func() []string {
		reasonStrings := []string{}
		for k, v := range reasons {
			reasonStrings = append(reasonStrings, fmt.Sprintf("%v insufficient %v", v, k))
		}
		sort.Strings(reasonStrings)
		return reasonStrings
	}
	reasonMsg := fmt.Sprintf("0/%v nodes are available, %v.", len(ji.NodesFitDelta), strings.Join(sortReasonsHistogram(), ", "))
	return reasonMsg
}

// ReadyTaskNum returns the number of tasks that are ready.
func (ji *JobInfo) ReadyTaskNum() int32 {
	occupid := 0
	for status, tasks := range ji.TaskStatusIndex {
		if AllocatedStatus(status) ||
			status == Succeeded {
			occupid = occupid + len(tasks)
		}
	}

	return int32(occupid)
}

// WaitingTaskNum returns the number of tasks that are pipelined.
func (ji *JobInfo) WaitingTaskNum() int32 {
	occupid := 0
	for status, tasks := range ji.TaskStatusIndex {
		if status == Pipelined {
			occupid = occupid + len(tasks)
		}
	}

	return int32(occupid)
}

// ValidTaskNum returns the number of tasks that are valid.
func (ji *JobInfo) ValidTaskNum() int32 {
	occupied := 0
	for status, tasks := range ji.TaskStatusIndex {
		if AllocatedStatus(status) ||
			status == Succeeded ||
			status == Pipelined ||
			status == Pending {
			occupied = occupied + len(tasks)
		}
	}

	return int32(occupied)
}

// Ready returns whether job is ready for run
func (ji *JobInfo) Ready() bool {
	occupied := ji.ReadyTaskNum()

	return occupied >= ji.MinAvailable
}

// Pipelined returns whether the number of ready and pipelined task is enough
func (ji *JobInfo) Pipelined() bool {
	occupied := ji.WaitingTaskNum() + ji.ReadyTaskNum()

	return occupied >= ji.MinAvailable
}

func (ji *JobInfo) addTaskIndex(ti *TaskInfo) {
	if _, found := ji.TaskStatusIndex[ti.Status]; !found {
		ji.TaskStatusIndex[ti.Status] = tasksMap{}
	}

	ji.TaskStatusIndex[ti.Status][ti.UID] = ti
}

func (ji *JobInfo) deleteTaskIndex(ti *TaskInfo) {
	if tasks, found := ji.TaskStatusIndex[ti.Status]; found {
		delete(tasks, ti.UID)

		if len(tasks) == 0 {
			delete(ji.TaskStatusIndex, ti.Status)
		}
	}
}

// validateStatusUpdate validates whether the status transfer is valid.
func validateStatusUpdate(oldStatus, newStatus TaskStatus) error {
	return nil
}

// AllocatedStatus checks whether the tasks has AllocatedStatus
func AllocatedStatus(status TaskStatus) bool {
	switch status {
	case Bound, Binding, Running, Allocated:
		return true
	default:
		return false
	}
}


