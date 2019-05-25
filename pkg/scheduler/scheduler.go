/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scheduler

import (
	"errors"
	"fmt"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/info"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/internal/queue"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/util/wait"
	"os"
	"sort"
	"strings"
	"time"

	"k8s.io/klog"

	resourceinformers "gitlab.aibee.cn/platform/ai-scheduler/pkg/client/informers/externalversions/resource/v1alpha1"
	schedulerapi "gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/api"
	latestschedulerapi "gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/api/latest"
	kubeschedulerconfig "gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/apis/config"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/core"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/factory"
	schedulerinternalcache "gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/internal/cache"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/metrics"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/util"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	policyinformers "k8s.io/client-go/informers/policy/v1beta1"
	storageinformers "k8s.io/client-go/informers/storage/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	// BindTimeoutSeconds defines the default bind timeout
	BindTimeoutSeconds = 100
	// SchedulerError is the reason recorded for events when an error occurs during scheduling a pod.
	SchedulerError = "SchedulerError"
)

// Scheduler watches for new unscheduled pods. It attempts to find
// nodes that they fit on and writes bindings back to the api server.
type Scheduler struct {
	config *factory.Config
}

// Cache returns the cache in scheduler for test to check the data in scheduler.
func (sched *Scheduler) Cache() schedulerinternalcache.Cache {
	return sched.config.SchedulerCache
}

type schedulerOptions struct {
	schedulerName                  string
	hardPodAffinitySymmetricWeight int32
	disablePreemption              bool
	percentageOfNodesToScore       int32
	bindTimeoutSeconds             int64
}

// Option configures a Scheduler
type Option func(*schedulerOptions)

// WithName sets schedulerName for Scheduler, the default schedulerName is default-scheduler
func WithName(schedulerName string) Option {
	return func(o *schedulerOptions) {
		o.schedulerName = schedulerName
	}
}

// WithHardPodAffinitySymmetricWeight sets hardPodAffinitySymmetricWeight for Scheduler, the default value is 1
func WithHardPodAffinitySymmetricWeight(hardPodAffinitySymmetricWeight int32) Option {
	return func(o *schedulerOptions) {
		o.hardPodAffinitySymmetricWeight = hardPodAffinitySymmetricWeight
	}
}

// WithPreemptionDisabled sets disablePreemption for Scheduler, the default value is false
func WithPreemptionDisabled(disablePreemption bool) Option {
	return func(o *schedulerOptions) {
		o.disablePreemption = disablePreemption
	}
}

// WithPercentageOfNodesToScore sets percentageOfNodesToScore for Scheduler, the default value is 50
func WithPercentageOfNodesToScore(percentageOfNodesToScore int32) Option {
	return func(o *schedulerOptions) {
		o.percentageOfNodesToScore = percentageOfNodesToScore
	}
}

// WithBindTimeoutSeconds sets bindTimeoutSeconds for Scheduler, the default value is 100
func WithBindTimeoutSeconds(bindTimeoutSeconds int64) Option {
	return func(o *schedulerOptions) {
		o.bindTimeoutSeconds = bindTimeoutSeconds
	}
}

var defaultSchedulerOptions = schedulerOptions{
	schedulerName:                  v1.DefaultSchedulerName,
	hardPodAffinitySymmetricWeight: v1.DefaultHardPodAffinitySymmetricWeight,
	disablePreemption:              false,
	percentageOfNodesToScore:       schedulerapi.DefaultPercentageOfNodesToScore,
	bindTimeoutSeconds:             BindTimeoutSeconds,
}

// New returns a Scheduler
func New(client clientset.Interface,
	poolInformer resourceinformers.PoolInformer,
	nodeInformer coreinformers.NodeInformer,
	podInformer coreinformers.PodInformer,
	pvInformer coreinformers.PersistentVolumeInformer,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
	replicationControllerInformer coreinformers.ReplicationControllerInformer,
	replicaSetInformer appsinformers.ReplicaSetInformer,
	statefulSetInformer appsinformers.StatefulSetInformer,
	serviceInformer coreinformers.ServiceInformer,
	pdbInformer policyinformers.PodDisruptionBudgetInformer,
	storageClassInformer storageinformers.StorageClassInformer,
	recorder record.EventRecorder,
	schedulerAlgorithmSource kubeschedulerconfig.SchedulerAlgorithmSource,
	stopCh <-chan struct{},
	opts ...func(o *schedulerOptions)) (*Scheduler, error) {

	options := defaultSchedulerOptions
	for _, opt := range opts {
		opt(&options)
	}
	// Set up the configurator which can create schedulers from configs.
	configurator := factory.NewConfigFactory(&factory.ConfigFactoryArgs{
		SchedulerName:                  options.schedulerName,
		Client:                         client,
		PoolInformer:                   poolInformer,
		NodeInformer:                   nodeInformer,
		PodInformer:                    podInformer,
		PvInformer:                     pvInformer,
		PvcInformer:                    pvcInformer,
		ReplicationControllerInformer:  replicationControllerInformer,
		ReplicaSetInformer:             replicaSetInformer,
		StatefulSetInformer:            statefulSetInformer,
		ServiceInformer:                serviceInformer,
		PdbInformer:                    pdbInformer,
		StorageClassInformer:           storageClassInformer,
		HardPodAffinitySymmetricWeight: options.hardPodAffinitySymmetricWeight,
		DisablePreemption:              options.disablePreemption,
		PercentageOfNodesToScore:       options.percentageOfNodesToScore,
		BindTimeoutSeconds:             options.bindTimeoutSeconds,
	})
	var config *factory.Config
	source := schedulerAlgorithmSource
	switch {
	case source.Provider != nil:
		// Create the config from a named algorithm provider.
		sc, err := configurator.CreateFromProvider(*source.Provider)
		if err != nil {
			return nil, fmt.Errorf("couldn't create scheduler using provider %q: %v", *source.Provider, err)
		}
		config = sc
	case source.Policy != nil:
		// Create the config from a user specified policy source.
		policy := &schedulerapi.Policy{}
		switch {
		case source.Policy.File != nil:
			if err := initPolicyFromFile(source.Policy.File.Path, policy); err != nil {
				return nil, err
			}
		case source.Policy.ConfigMap != nil:
			if err := initPolicyFromConfigMap(client, source.Policy.ConfigMap, policy); err != nil {
				return nil, err
			}
		}
		sc, err := configurator.CreateFromConfig(*policy)
		if err != nil {
			return nil, fmt.Errorf("couldn't create scheduler from policy: %v", err)
		}
		config = sc
	default:
		return nil, fmt.Errorf("unsupported algorithm source: %v", source)
	}
	// Additional tweaks to the config produced by the configurator.
	config.Recorder = recorder
	config.DisablePreemption = options.disablePreemption
	config.StopEverything = stopCh

	// Create the scheduler.
	sched := NewFromConfig(config)

	AddAllEventHandlers(sched, options.schedulerName, poolInformer, nodeInformer, podInformer, pvInformer, pvcInformer, replicationControllerInformer, replicaSetInformer, statefulSetInformer, serviceInformer, pdbInformer, storageClassInformer)
	return sched, nil
}

// initPolicyFromFile initialize policy from file
func initPolicyFromFile(policyFile string, policy *schedulerapi.Policy) error {
	// Use a policy serialized in a file.
	_, err := os.Stat(policyFile)
	if err != nil {
		return fmt.Errorf("missing policy config file %s", policyFile)
	}
	data, err := ioutil.ReadFile(policyFile)
	if err != nil {
		return fmt.Errorf("couldn't read policy config: %v", err)
	}
	err = runtime.DecodeInto(latestschedulerapi.Codec, []byte(data), policy)
	if err != nil {
		return fmt.Errorf("invalid policy: %v", err)
	}
	return nil
}

// initPolicyFromConfigMap initialize policy from configMap
func initPolicyFromConfigMap(client clientset.Interface, policyRef *kubeschedulerconfig.SchedulerPolicyConfigMapSource, policy *schedulerapi.Policy) error {
	// Use a policy serialized in a config map value.
	policyConfigMap, err := client.CoreV1().ConfigMaps(policyRef.Namespace).Get(policyRef.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("couldn't get policy config map %s/%s: %v", policyRef.Namespace, policyRef.Name, err)
	}
	data, found := policyConfigMap.Data[kubeschedulerconfig.SchedulerPolicyConfigMapKey]
	if !found {
		return fmt.Errorf("missing policy config map value at key %q", kubeschedulerconfig.SchedulerPolicyConfigMapKey)
	}
	err = runtime.DecodeInto(latestschedulerapi.Codec, []byte(data), policy)
	if err != nil {
		return fmt.Errorf("invalid policy: %v", err)
	}
	return nil
}

// NewFromConfig returns a new scheduler using the provided Config.
func NewFromConfig(config *factory.Config) *Scheduler {
	metrics.Register()
	return &Scheduler{
		config: config,
	}
}

// Run begins watching and scheduling. It waits for cache to be synced, then starts a goroutine and returns immediately.
func (sched *Scheduler) Run() {
	if !sched.config.WaitForCacheSync() {
		return
	}
	go wait.Until(sched.schedulePools, 0, sched.config.StopEverything)
}

// Config returns scheduler's config pointer. It is exposed for testing purposes.
func (sched *Scheduler) Config() *factory.Config {
	return sched.config
}

// recordFailedSchedulingEvent records an event for the pod that indicates the
// pod has failed to schedule.
// NOTE: This function modifies "pod". "pod" should be copied before being passed.
func (sched *Scheduler) recordSchedulingFailure(poolName string, pod *v1.Pod, err error, reason string, message string, needBorrow bool) {
	sched.config.Error(poolName, pod, err, needBorrow)
	sched.config.Recorder.Event(pod, v1.EventTypeWarning, "FailedScheduling", message)
	sched.config.PodConditionUpdater.Update(pod, &v1.PodCondition{
		Type:    v1.PodScheduled,
		Status:  v1.ConditionFalse,
		Reason:  reason,
		Message: err.Error(),
	})
}

// schedule implements the scheduling algorithm and returns the suggested result(host,
// evaluated nodes number,feasible nodes number).
func (sched *Scheduler) schedule(poolName string, pod *v1.Pod) (core.ScheduleResult, error) {
	result, err := sched.config.Algorithm.Schedule(poolName, pod, sched.config.NodeLister)
	if err != nil {
		//pod = pod.DeepCopy()
		//sched.recordSchedulingFailure(poolName, pod, err, v1.PodReasonUnschedulable, err.Error())
		return core.ScheduleResult{}, err
	}
	return result, err
}

// preempt tries to create room for a pod that has failed to schedule, by preempting lower priority pods if possible.
// If it succeeds, it adds the name of the node where preemption has happened to the pod annotations.
// It returns the node name and an error if any.
func (sched *Scheduler) preempt(poolName string, preemptor *v1.Pod, scheduleErr error) (string, error, bool) {
	preemptor, err := sched.config.PodPreemptor.GetUpdatedPod(preemptor)
	if err != nil {
		klog.Errorf("Error getting the updated preemptor pod object: %v", err)
		return "", err, false
	}

	node, victims, nominatedPodsToClear, err, needBorrow := sched.config.Algorithm.Preempt(poolName, preemptor, sched.config.NodeLister, scheduleErr)
	if err != nil {
		klog.Errorf("Error preempting victims to make room for %v/%v.", preemptor.Namespace, preemptor.Name)
		return "", err, needBorrow
	}
	var nodeName = ""
	if node != nil {
		nodeName = node.Name
		// Update the scheduling queue with the nominated pod information. Without
		// this, there would be a race condition between the next scheduling cycle
		// and the time the scheduler receives a Pod Update for the nominated pod.
		q, err := sched.config.PoolQueue.GetQueue(poolName)
		if err != nil {
			return "", err, false
		}
		q.UpdateNominatedPodForNode(preemptor, nodeName)

		// Make a call to update nominated node name of the pod on the API server.
		err = sched.config.PodPreemptor.SetNominatedNodeName(preemptor, nodeName)
		if err != nil {
			klog.Errorf("Error in preemption process. Cannot update pod %v/%v annotations: %v", preemptor.Namespace, preemptor.Name, err)
			q, err := sched.config.PoolQueue.GetQueue(poolName)
			if err != nil {
				return "", err, false
			}
			q.DeleteNominatedPodIfExists(preemptor)
			return "", err, false
		}

		for _, victim := range victims {
			if err := sched.config.PodPreemptor.DeletePod(victim); err != nil {
				klog.Errorf("Error preempting pod %v/%v: %v", victim.Namespace, victim.Name, err)
				return "", err, false
			}
			klog.V(4).Infof("Preempted pod %v/%v by %v/%v on node %v", victim.Namespace, victim.Name, preemptor.Namespace, preemptor.Name, nodeName)
			sched.config.Recorder.Eventf(victim, v1.EventTypeNormal, "Preempted", "by %v/%v on node %v", preemptor.Namespace, preemptor.Name, nodeName)
		}
		metrics.PreemptionVictims.Set(float64(len(victims)))
	}
	// Clearing nominated pods should happen outside of "if node != nil". Node could
	// be nil when a pod with nominated node name is eligible to preempt again,
	// but preemption logic does not find any node for it. In that case Preempt()
	// function of generic_scheduler.go returns the pod itself for removal of the annotation.
	for _, p := range nominatedPodsToClear {
		rErr := sched.config.PodPreemptor.RemoveNominatedNodeName(p)
		if rErr != nil {
			klog.Errorf("Cannot remove nominated node annotation of pod: %v", rErr)
			// We do not return as this error is not critical.
		}
	}
	return nodeName, err, needBorrow
}

// assumeVolumes will update the volume cache with the chosen bindings
//
// This function modifies assumed if volume binding is required.
func (sched *Scheduler) assumeVolumes(poolName string, assumed *v1.Pod, host string) (allBound bool, err error) {
	allBound, err = sched.config.VolumeBinder.Binder.AssumePodVolumes(assumed, host)
	if err != nil {
		sched.recordSchedulingFailure(poolName, assumed, err, SchedulerError,
			fmt.Sprintf("AssumePodVolumes failed: %v", err), false)
	}
	return
}

// bindVolumes will make the API update with the assumed bindings and wait until
// the PV controller has completely finished the binding operation.
//
// If binding errors, times out or gets undone, then an error will be returned to
// retry scheduling.
func (sched *Scheduler) bindVolumes(poolName string, assumed *v1.Pod) error {
	klog.V(5).Infof("Trying to bind volumes for pod \"%v/%v\"", assumed.Namespace, assumed.Name)
	err := sched.config.VolumeBinder.Binder.BindPodVolumes(assumed)
	if err != nil {
		klog.V(1).Infof("Failed to bind volumes for pod \"%v/%v\": %v", assumed.Namespace, assumed.Name, err)

		// Unassume the Pod and retry scheduling
		if forgetErr := sched.config.SchedulerCache.ForgetPod(assumed); forgetErr != nil {
			klog.Errorf("scheduler cache ForgetPod failed: %v", forgetErr)
		}

		sched.recordSchedulingFailure(poolName, assumed, err, "VolumeBindingFailed", err.Error(), false)
		return err
	}

	klog.V(5).Infof("Success binding volumes for pod \"%v/%v\"", assumed.Namespace, assumed.Name)
	return nil
}

// assume signals to the cache that a pod is already in the cache, so that binding can be asynchronous.
// assume modifies `assumed`.
func (sched *Scheduler) assume(poolName string, assumed *v1.Pod, host string) error {
	// Optimistically assume that the binding will succeed and send it to apiserver
	// in the background.
	// If the binding fails, scheduler will release resources allocated to assumed pod
	// immediately.
	assumed.Spec.NodeName = host

	if err := sched.config.SchedulerCache.AssumePod(assumed); err != nil {
		klog.Errorf("scheduler cache AssumePod failed: %v", err)

		// This is most probably result of a BUG in retrying logic.
		// We report an error here so that pod scheduling can be retried.
		// This relies on the fact that Error will check if the pod has been bound
		// to a node and if so will not add it back to the unscheduled pods queue
		// (otherwise this would cause an infinite loop).
		sched.recordSchedulingFailure(poolName, assumed, err, SchedulerError,
			fmt.Sprintf("AssumePod failed: %v", err), false)
		return err
	}
	// if "assumed" is a nominated pod, we should remove it from internal cache
	if sched.config.PoolQueue != nil {
		q, err := sched.config.PoolQueue.GetQueue(poolName)
		if err != nil {
			return err
		}
		q.DeleteNominatedPodIfExists(assumed)
	}

	return nil
}

// bind binds a pod to a given node defined in a binding object.  We expect this to run asynchronously, so we
// handle binding metrics internally.
func (sched *Scheduler) bind(poolName string, assumed *v1.Pod, b *v1.Binding) error {
	bindingStart := time.Now()
	// If binding succeeded then PodScheduled condition will be updated in apiserver so that
	// it's atomic with setting host.
	err := sched.config.GetBinder(assumed).Bind(b)
	if finErr := sched.config.SchedulerCache.FinishBinding(assumed); finErr != nil {
		klog.Errorf("scheduler cache FinishBinding failed: %v", finErr)
	}
	if err != nil {
		klog.V(1).Infof("Failed to bind pod: %v/%v", assumed.Namespace, assumed.Name)
		if err := sched.config.SchedulerCache.ForgetPod(assumed); err != nil {
			klog.Errorf("scheduler cache ForgetPod failed: %v", err)
		}
		sched.recordSchedulingFailure(poolName, assumed, err, SchedulerError,
			fmt.Sprintf("Binding rejected: %v", err), false)
		return err
	}

	metrics.BindingLatency.Observe(metrics.SinceInSeconds(bindingStart))
	metrics.DeprecatedBindingLatency.Observe(metrics.SinceInMicroseconds(bindingStart))
	metrics.SchedulingLatency.WithLabelValues(metrics.Binding).Observe(metrics.SinceInSeconds(bindingStart))
	metrics.DeprecatedSchedulingLatency.WithLabelValues(metrics.Binding).Observe(metrics.SinceInSeconds(bindingStart))
	sched.config.Recorder.Eventf(assumed, v1.EventTypeNormal, "Scheduled", "Successfully assigned %v/%v to %v", assumed.Namespace, assumed.Name, b.Target.Name)
	return nil
}

// scheduleOne does the entire scheduling workflow for a single pod.  It is serialized on the scheduling algorithm's host fitting.
func (sched *Scheduler) scheduleOne(poolName string) error {
	plugins := sched.config.PluginSet
	// Remove all plugin context data at the beginning of a scheduling cycle.
	if plugins.Data().Ctx != nil {
		plugins.Data().Ctx.Reset()
	}

	pod, err := sched.config.NextPod(poolName)
	// pod could be nil when schedulerQueue is closed
	if pod == nil {
		return err
	}
	if pod.DeletionTimestamp != nil {
		sched.config.Recorder.Eventf(pod, v1.EventTypeWarning, "FailedScheduling", "skip schedule deleting pod: %v/%v", pod.Namespace, pod.Name)
		klog.V(3).Infof("Skip schedule deleting pod: %v/%v", pod.Namespace, pod.Name)
		return err
	}

	klog.V(3).Infof("Attempting to schedule pod: %v/%v", pod.Namespace, pod.Name)

	// Synchronously attempt to find a fit for the pod.
	start := time.Now()
	scheduleResult, err := sched.schedule(poolName, pod)
	if err != nil {
		// schedule() may have failed because the pod would not fit on any host, so we try to
		// preempt, with the expectation that the next time the pod is tried for scheduling it
		// will fit due to the preemption. It is also possible that a different pod will schedule
		// into the resources that were preempted, but this is harmless.
		needBorrow := true
		var nodeName string
		if fitError, ok := err.(*core.FitError); ok {
			if !util.PodPriorityEnabled() || sched.config.DisablePreemption {
				klog.V(3).Infof("Pod priority feature is not enabled or preemption is disabled by scheduler configuration." +
					" No preemption is performed.")
			} else {
				selfPoolName := sched.config.PoolQueue.GetPoolQueueNameIfNotPresent(pod)
				// only self pool will perform preemption
				if selfPoolName == poolName {
					if p, e := sched.config.SchedulerCache.GetPool(poolName); e != nil {
						klog.Errorf("Error get pool failed: %v", e)
					} else {
						if p.DisablePreemption() {
							klog.V(3).Infof("Pool %v preemption disabled. No preemption is performed", poolName)
						} else {
							preemptionStartTime := time.Now()
							nodeName, _, needBorrow = sched.preempt(poolName, pod, fitError)
							if nodeName != "" {
								klog.V(4).Infof("Preempt for %v/%v in pool %v at node %v succeed", pod.Namespace, pod.Name, poolName, nodeName)
							} else {
								klog.V(4).Infof("Preempt for %v/%v in pool %v not feasible or failed: %v", pod.Namespace, pod.Name, poolName, e)
							}
							metrics.PreemptionAttempts.Inc()
							metrics.SchedulingAlgorithmPremptionEvaluationDuration.Observe(metrics.SinceInSeconds(preemptionStartTime))
							metrics.DeprecatedSchedulingAlgorithmPremptionEvaluationDuration.Observe(metrics.SinceInMicroseconds(preemptionStartTime))
							metrics.SchedulingLatency.WithLabelValues(metrics.PreemptionEvaluation).Observe(metrics.SinceInSeconds(preemptionStartTime))
							metrics.DeprecatedSchedulingLatency.WithLabelValues(metrics.PreemptionEvaluation).Observe(metrics.SinceInSeconds(preemptionStartTime))
						}
					}

					// when come many pods from self pool, to avoid pod from other pool has no chance to schedule,
					// move all pods from other pools to their self pool queues
					sched.config.PoolQueue.MoveAllBorrowingPodsToSelfQueue(poolName)
				}
			}
			// Pod did not fit anywhere, so it is counted as a failure. If preemption
			// succeeds, the pod should get counted as a success the next time we try to
			// schedule it. (hopefully)
			metrics.PodScheduleFailures.Inc()
		} else {
			klog.Errorf("error selecting node for pod: %v", err)
			metrics.PodScheduleErrors.Inc()
		}
		// call Error() to borrow or reschedule pod
		pod = pod.DeepCopy()
		sched.recordSchedulingFailure(poolName, pod, err, v1.PodReasonUnschedulable, err.Error(), needBorrow)
		return err
	}
	metrics.SchedulingAlgorithmLatency.Observe(metrics.SinceInSeconds(start))
	metrics.DeprecatedSchedulingAlgorithmLatency.Observe(metrics.SinceInMicroseconds(start))
	// Tell the cache to assume that a pod now is running on a given node, even though it hasn't been bound yet.
	// This allows us to keep scheduling without waiting on binding to occur.
	assumedPod := pod.DeepCopy()

	// Assume volumes first before assuming the pod.
	//
	// If all volumes are completely bound, then allBound is true and binding will be skipped.
	//
	// Otherwise, binding of volumes is started after the pod is assumed, but before pod binding.
	//
	// This function modifies 'assumedPod' if volume binding is required.
	allBound, err := sched.assumeVolumes(poolName, assumedPod, scheduleResult.SuggestedHost)
	if err != nil {
		klog.Errorf("error assuming volumes: %v", err)
		metrics.PodScheduleErrors.Inc()
		return err
	}

	// Run "reserve" plugins.
	for _, pl := range plugins.ReservePlugins() {
		if err := pl.Reserve(plugins, assumedPod, scheduleResult.SuggestedHost); err != nil {
			klog.Errorf("error while running %v reserve plugin for pod %v: %v", pl.Name(), assumedPod.Name, err)
			sched.recordSchedulingFailure(poolName, assumedPod, err, SchedulerError,
				fmt.Sprintf("reserve plugin %v failed", pl.Name()), false)
			metrics.PodScheduleErrors.Inc()
			return err
		}
	}
	// assume modifies `assumedPod` by setting NodeName=scheduleResult.SuggestedHost
	err = sched.assume(poolName, assumedPod, scheduleResult.SuggestedHost)
	if err != nil {
		klog.Errorf("error assuming pod: %v", err)
		metrics.PodScheduleErrors.Inc()
		return err
	}
	// bind the pod to its host asynchronously (we can do this b/c of the assumption step above).
	go func() {
		// Bind volumes first before Pod
		if !allBound {
			err := sched.bindVolumes(poolName, assumedPod)
			if err != nil {
				klog.Errorf("error binding volumes: %v", err)
				metrics.PodScheduleErrors.Inc()
				return
			}
		}

		// Run "prebind" plugins.
		for _, pl := range plugins.PrebindPlugins() {
			approved, err := pl.Prebind(plugins, assumedPod, scheduleResult.SuggestedHost)
			if err != nil {
				approved = false
				klog.Errorf("error while running %v prebind plugin for pod %v: %v", pl.Name(), assumedPod.Name, err)
				metrics.PodScheduleErrors.Inc()
			}
			if !approved {
				sched.Cache().ForgetPod(assumedPod)
				var reason string
				if err == nil {
					msg := fmt.Sprintf("prebind plugin %v rejected pod %v.", pl.Name(), assumedPod.Name)
					klog.V(4).Infof(msg)
					err = errors.New(msg)
					reason = v1.PodReasonUnschedulable
				} else {
					reason = SchedulerError
				}
				sched.recordSchedulingFailure(poolName, assumedPod, err, reason, err.Error(), false)
				return
			}
		}

		err := sched.bind(poolName, assumedPod, &v1.Binding{
			ObjectMeta: metav1.ObjectMeta{Namespace: assumedPod.Namespace, Name: assumedPod.Name, UID: assumedPod.UID},
			Target: v1.ObjectReference{
				Kind: "Node",
				Name: scheduleResult.SuggestedHost,
			},
		})
		metrics.E2eSchedulingLatency.Observe(metrics.SinceInSeconds(start))
		metrics.DeprecatedE2eSchedulingLatency.Observe(metrics.SinceInMicroseconds(start))
		if err != nil {
			klog.Errorf("error binding pod: %v", err)
			metrics.PodScheduleErrors.Inc()
		} else {
			klog.V(2).Infof("pod %v/%v is bound successfully on node %v, %d nodes evaluated, %d nodes were found feasible", assumedPod.Namespace, assumedPod.Name, scheduleResult.SuggestedHost, scheduleResult.EvaluatedNodes, scheduleResult.FeasibleNodes)
			metrics.PodScheduleSuccesses.Inc()
		}
	}()
	return nil
}

func (sched *Scheduler) schedulePools() {
	stopCh := sched.config.StopEverything
	// start default pool queue first
	go scheduleOnePool(sched.scheduleOne, info.DefaultPoolName, stopCh)
	// start other pool queues
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		func() {
			defer runtimeutil.HandleCrash()
			poolName := <-sched.config.StartSchedulingQueue
			go scheduleOnePool(sched.scheduleOne, poolName, stopCh)
		}()

		select {
		case <-stopCh:
			return
		default:
		}
	}
}

func scheduleOnePool(f func(string) error, poolName string, stopCh <-chan struct{}) {
	klog.V(3).Infof("Starting scheduling for pool %s", poolName)
	for {
		select {
		case <-stopCh:
			klog.V(3).Infof("Stopping scheduling for pool %s", poolName)
			return
		default:
		}

		var err error
		func() {
			defer runtimeutil.HandleCrash()
			err = f(poolName)
		}()
		if err == queue.PriorityQueueClosedError || err == queue.PriorityQueueDeletedError {
			klog.V(3).Infof("Stopping scheduling for pool %s as Pool Deleted", poolName)
			return
		}

		// NOTE: b/c there is no priority selection in golang
		// it is possible for this to race, meaning we could
		// trigger t.C and stopCh, and t.C select falls through.
		// In order to mitigate we re-check stopCh at the beginning
		// of every loop to prevent extra executions of f().
		select {
		case <-stopCh:
			klog.V(3).Infof("Stopping scheduling for pool %s", poolName)
			return
		default:
		}
	}
}

// printScheduler print all pools cache and pool queue detail
func (sched *Scheduler) PrintPools() {
	cache := sched.config.SchedulerCache
	lineWidth := 160

	totalRes := cache.TotalAllocatableResource()
	var log = fmt.Sprintf(`All Pools Detail:
%-20s%-20s%-20s%-20s%-20s%-20s%-20s%-20s
%s`,
		"Pools", "Resource(w)", "Capacity", "Allocatable", "Used", "Shared", "Pending", "Total",
		strings.Repeat("-", lineWidth),
	)
	keys := make([]string, 0, cache.NumPools())
	for k := range cache.Pools() {
		keys = append(keys, k)
	}
	sort.Sort(sort.StringSlice(keys))

	for _, key := range keys {
		p, err := cache.GetPool(key)
		if err != nil {
			continue
		}
		q, err := sched.config.PoolQueue.GetQueue(key)
		if err != nil {
			continue
		}
		detail := `
%-20s%-20s%-20d%-20d%-20d%-20d%-20d%-20d
%-20s%-20s%-20d%-20d%-20d%-20d%-20d%-20d
%-20s%-20s%-20d%-20d%-20d%-20d%-20d%-20d
%-20s%-20s%-20d%-20d%-20d%-20d%-20d%-20d
%-20s%-20s%-20d%-20d%-20v%-20v%-20v%-20d
%-20s%-20s%-20d%-20d%-20d%-20d%-20d%-20d
%s`
		poolName := p.Name()

		capacity := p.Capacity()
		allocatable := p.Allocatable()
		used := p.Used()
		shared := p.Shared()
		disablePreemption := "√"
		disableBorrowing := "√"
		disableSharing := "√"
		if p.DisablePreemption() {
			disablePreemption = "x"
		}
		if p.DisableBorrowing() {
			disableBorrowing = "x"
		}
		if p.DisableSharing() {
			disableSharing = "x"
		}

		pendingRes := info.CalculateSumPodsRequestResource(q.PendingPods())
		if p.Name() == "" {
			poolName = "Default"
		}
		log += fmt.Sprintf(detail,
			"", fmt.Sprintf("cpu(%d)", p.GetPoolWeight()[v1.ResourceCPU]), capacity.MilliCPU,
			allocatable.MilliCPU, used.MilliCPU,
			shared.MilliCPU, pendingRes.MilliCPU, totalRes.MilliCPU,

			"", fmt.Sprintf("gpu(%d)", p.GetPoolWeight()[info.ResourceGPU]),
			capacity.ScalarResources[info.ResourceGPU], allocatable.ScalarResources[info.ResourceGPU],
			used.ScalarResources[info.ResourceGPU], shared.ScalarResources[info.ResourceGPU],
			pendingRes.ScalarResources[info.ResourceGPU],
			totalRes.ScalarResources[info.ResourceGPU],

			poolName, fmt.Sprintf("mem(%d)", p.GetPoolWeight()[v1.ResourceMemory]), capacity.Memory,
			allocatable.Memory, used.Memory, shared.Memory,
			pendingRes.Memory, totalRes.Memory,

			fmt.Sprintf("(p=%v,b=%v,s=%v)", disablePreemption, disableBorrowing, disableSharing),
			fmt.Sprintf("storage(%d)", p.GetPoolWeight()[v1.ResourceEphemeralStorage]),
			capacity.EphemeralStorage, allocatable.EphemeralStorage,
			used.EphemeralStorage, shared.EphemeralStorage,
			pendingRes.EphemeralStorage, totalRes.EphemeralStorage,

			"", "nodes" /*cache.pools[p.Name()].NumNodes()*/, p.NumNodes(),
			cache.NodeTree(p.Name()).NumNodes(), "-", "-", "-", cache.NumNodes(),

			"", fmt.Sprintf("pods(%d)", p.GetPoolWeight()[v1.ResourcePods]),
			capacity.AllowedPodNumber, allocatable.AllowedPodNumber,
			used.AllowedPodNumber, shared.AllowedPodNumber,
			pendingRes.AllowedPodNumber, totalRes.AllowedPodNumber,

			strings.Repeat("-", lineWidth),
		)

	}
	klog.V(4).Infoln(log)
}
