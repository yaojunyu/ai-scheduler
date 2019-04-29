package info

import (
	"fmt"

	"gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)


const (
	// Pending means the task is pending in the apiserver.
	Pending TaskStatus = 1 << iota

	// Allocated means the scheduler assigns a host to it.
	Allocated

	// Pipelined means the scheduler assigns a host to wait for releasing resource.
	Pipelined

	// Binding means the scheduler send Bind request to apiserver.
	Binding

	// Bound means the task/Pod bounds to a host.
	Bound

	// Running means a task is running on the host.
	Running

	// Releasing means a task/pod is deleted.
	Releasing

	// Succeeded means that all containers in the pod have voluntarily terminated
	// with a container exit code of 0, and the system is not going to restart any of these containers.
	Succeeded

	// Failed means that all containers in the pod have terminated, and at least one container has
	// terminated in a failure (exited with a non-zero exit code or was stopped by the system).
	Failed

	// Unknown means the status of task/pod is unknown to the scheduler.
	Unknown
)

// TaskID is UID type for Task
type TaskID types.UID

// TaskStatus defines the status of a task/pod.
type TaskStatus int

// TaskInfo will have all infos about the task
type TaskInfo struct {
	UID TaskID
	Job JobID

	Name      string
	Namespace string

	// Resreq is the resource that used when task running.
	Resreq *Resource
	// InitResreq is the resource that used to launch a task.
	InitResreq *Resource

	NodeName    string
	Status      TaskStatus
	Priority    int32
	VolumeReady bool

	Pod *v1.Pod
}

// NewTaskInfo creates new taskInfo object for a Pod
func NewTaskInfo(pod *v1.Pod) *TaskInfo {
	req := GetPodResourceWithoutInitContainers(pod)
	initResreq := GetPodResourceRequest(pod)

	jobID := getJobID(pod)

	ti := &TaskInfo{
		UID:        TaskID(pod.UID),
		Job:        jobID,
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		NodeName:   pod.Spec.NodeName,
		Status:     getTaskStatus(pod),
		Priority:   1,
		Pod:        pod,
		Resreq:     req,
		InitResreq: initResreq,
	}

	if pod.Spec.Priority != nil {
		ti.Priority = *pod.Spec.Priority
	}

	return ti
}

// Clone is used for cloning a task
func (ti *TaskInfo) Clone() *TaskInfo {
	return &TaskInfo{
		UID:         ti.UID,
		Job:         ti.Job,
		Name:        ti.Name,
		Namespace:   ti.Namespace,
		NodeName:    ti.NodeName,
		Status:      ti.Status,
		Priority:    ti.Priority,
		Pod:         ti.Pod,
		Resreq:      ti.Resreq.Clone(),
		InitResreq:  ti.InitResreq.Clone(),
		VolumeReady: ti.VolumeReady,
	}
}

// String returns the taskInfo details in a string
func (ti TaskInfo) String() string {
	return fmt.Sprintf("Task (%v:%v/%v): job %v, status %v, pri %v, resreq %v",
		ti.UID, ti.Namespace, ti.Name, ti.Job, ti.Status, ti.Priority, ti.Resreq)
}

func getJobID(pod *v1.Pod) JobID {
	if len(pod.Annotations) != 0 {
		if pn, found := pod.Annotations[v1alpha1.GroupNameAnnotationKey]; found && len(pn) != 0 {
			// Make sure Pod and PodGroup belong to the same namespace.
			jobID := fmt.Sprintf("%s/%s", pod.Namespace, pn)
			return JobID(jobID)
		}
	}

	return ""
}


func (ts TaskStatus) String() string {
	switch ts {
	case Pending:
		return "Pending"
	case Binding:
		return "Binding"
	case Bound:
		return "Bound"
	case Running:
		return "Running"
	case Releasing:
		return "Releasing"
	case Succeeded:
		return "Succeeded"
	case Failed:
		return "Failed"
	default:
		return "Unknown"
	}
}

func getTaskStatus(pod *v1.Pod) TaskStatus {
	switch pod.Status.Phase {
	case v1.PodRunning:
		if pod.DeletionTimestamp != nil {
			return Releasing
		}

		return Running
	case v1.PodPending:
		if pod.DeletionTimestamp != nil {
			return Releasing
		}

		if len(pod.Spec.NodeName) == 0 {
			return Pending
		}
		return Bound
	case v1.PodUnknown:
		return Unknown
	case v1.PodSucceeded:
		return Succeeded
	case v1.PodFailed:
		return Failed
	}

	return Unknown
}