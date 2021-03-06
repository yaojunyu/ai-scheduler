/*
Copyright 2015 The Kubernetes Authors.

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

package cache

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"code.aibee.cn/platform/ai-scheduler/pkg/scheduler/metrics"
	"reflect"
	"sync"
	"time"

	"code.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	"code.aibee.cn/platform/ai-scheduler/pkg/scheduler/algorithm"
	"code.aibee.cn/platform/ai-scheduler/pkg/scheduler/algorithm/predicates"
	schedulerinfo "code.aibee.cn/platform/ai-scheduler/pkg/scheduler/info"
	"code.aibee.cn/platform/ai-scheduler/pkg/scheduler/util"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
)

var (
	cleanAssumedPeriod = 1 * time.Second
)

// New returns a Cache implementation.
// It automatically starts a go routine that manages expiration of assumed pods.
// "ttl" is how long the assumed pod will get expired.
// "stop" is the channel that would close the background goroutine.
func New(ttl time.Duration, stop <-chan struct{}) Cache {
	cache := newSchedulerCache(ttl, cleanAssumedPeriod, stop)
	cache.run()
	return cache
}

type schedulerCache struct {
	stop   <-chan struct{}
	ttl    time.Duration
	period time.Duration

	// This mutex guards all fields within this cache struct.
	mu sync.RWMutex
	// a set of assumed pod keys.
	// The key could further be used to get an entry in podStates.
	assumedPods map[string]bool
	// a map from pod key to podState.
	podStates map[string]*podState

	// A map from image name to its imageState.
	imageStates map[string]*imageState

	// resource pools
	pools map[string]*schedulerinfo.PoolInfo
	//nodeTrees map[string]*NodeTree
}

type podState struct {
	pod *v1.Pod
	// Used by assumedPod to determinate expiration.
	deadline *time.Time
	// Used to block cache from expiring assumedPod if binding still runs
	bindingFinished bool
}

type imageState struct {
	// Size of the image
	size int64
	// A set of node names for nodes having this image present
	nodes sets.String
}

// createImageStateSummary returns a summarizing snapshot of the given image's state.
func (cache *schedulerCache) createImageStateSummary(state *imageState) *schedulerinfo.ImageStateSummary {
	return &schedulerinfo.ImageStateSummary{
		Size:     state.size,
		NumNodes: len(state.nodes),
	}
}

func newSchedulerCache(ttl, period time.Duration, stop <-chan struct{}) *schedulerCache {
	cache := &schedulerCache{
		ttl:    ttl,
		period: period,
		stop:   stop,

		assumedPods: make(map[string]bool),
		podStates:   make(map[string]*podState),
		imageStates: make(map[string]*imageState),

		pools: make(map[string]*schedulerinfo.PoolInfo),
	}
	// add nodeTree for default pool
	cache.pools[schedulerinfo.DefaultPoolName] = schedulerinfo.NewPoolInfo()
	return cache
}

// Snapshot takes a snapshot of the current scheduler cache. This is used for
// debugging purposes only and shouldn't be confused with UpdateNodeInfoSnapshot
// function.
// This method is expensive, and should be only used in non-critical path.
func (cache *schedulerCache) Snapshot() *schedulerinfo.Snapshot {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	nds := cache.nodes()
	nodes := make(map[string]*schedulerinfo.NodeInfo, len(nds))
	for k, v := range nds {
		nodes[k] = v.Info().Clone()
	}

	assumedPods := make(map[string]bool, len(cache.assumedPods))
	for k, v := range cache.assumedPods {
		assumedPods[k] = v
	}

	return &schedulerinfo.Snapshot{
		Nodes:       nodes,
		AssumedPods: assumedPods,
	}
}

// DON NOT add any locks
func (cache *schedulerCache) NodeInfoSnapshot(poolName string) *schedulerinfo.NodeInfoSnapshot {
	pi, ok := cache.pools[poolName]
	if !ok {
		return nil
	}
	return pi.NodeInfoSnapshot()
}

// UpdateNodeInfoSnapshot takes a snapshot of cached NodeInfo map. This is called at
// beginning of every scheduling cycle.
// This function tracks generation number of NodeInfo and updates only the
// entries of an existing snapshot that have changed after the snapshot was taken.
func (cache *schedulerCache) UpdateNodeInfoSnapshot(poolName string) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if pi, ok := cache.pools[poolName]; ok {
		return pi.UpdateNodeInfoSnapshot()
	}
	return fmt.Errorf("pool %v not exists in cache", poolName)
}

func (cache *schedulerCache) List(selector labels.Selector) ([]*v1.Pod, error) {
	alwaysTrue := func(p *v1.Pod) bool { return true }
	return cache.FilteredList(alwaysTrue, selector)
}

func (cache *schedulerCache) FilteredList(podFilter algorithm.PodFilter, selector labels.Selector) ([]*v1.Pod, error) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	// FIXME REVIEW if possible
	// podFilter is expected to return true for most or all of the pods. We
	// can avoid expensive array growth without wasting too much memory by
	// pre-allocating capacity.
	maxSize := 0
	nodes := cache.nodes()
	for _, n := range nodes {
		maxSize += len(n.Info().Pods())
	}
	pods := make([]*v1.Pod, 0, maxSize)
	for _, n := range nodes {
		for _, pod := range n.Info().Pods() {
			if podFilter(pod) && selector.Matches(labels.Set(pod.Labels)) {
				pods = append(pods, pod)
			}
		}
	}
	return pods, nil
}

func (cache *schedulerCache) AssumePod(pod *v1.Pod) error {
	key, err := schedulerinfo.GetPodKey(pod)
	if err != nil {
		return err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()
	if _, ok := cache.podStates[key]; ok {
		return fmt.Errorf("pod %v is in the cache, so can't be assumed", key)
	}

	cache.addPod(pod)
	ps := &podState{
		pod: pod,
	}
	cache.podStates[key] = ps
	cache.assumedPods[key] = true
	return nil
}

func (cache *schedulerCache) FinishBinding(pod *v1.Pod) error {
	return cache.finishBinding(pod, time.Now())
}

// finishBinding exists to make tests determinitistic by injecting now as an argument
func (cache *schedulerCache) finishBinding(pod *v1.Pod, now time.Time) error {
	key, err := schedulerinfo.GetPodKey(pod)
	if err != nil {
		return err
	}

	cache.mu.RLock()
	defer cache.mu.RUnlock()

	klog.V(5).Infof("Finished binding for pod %v. Can be expired.", key)
	currState, ok := cache.podStates[key]
	if ok && cache.assumedPods[key] {
		dl := now.Add(cache.ttl)
		currState.bindingFinished = true
		currState.deadline = &dl
	}
	return nil
}

func (cache *schedulerCache) ForgetPod(pod *v1.Pod) error {
	key, err := schedulerinfo.GetPodKey(pod)
	if err != nil {
		return err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	currState, ok := cache.podStates[key]
	if ok && currState.pod.Spec.NodeName != pod.Spec.NodeName {
		return fmt.Errorf("pod %v was assumed on %v but assigned to %v", key, pod.Spec.NodeName, currState.pod.Spec.NodeName)
	}

	switch {
	// Only assumed pod can be forgotten.
	case ok && cache.assumedPods[key]:
		err := cache.removePod(pod)
		if err != nil {
			return err
		}
		delete(cache.assumedPods, key)
		delete(cache.podStates, key)
	default:
		return fmt.Errorf("pod %v wasn't assumed so cannot be forgotten", key)
	}
	return nil
}

// Assumes that lock is already acquired.
func (cache *schedulerCache) addPod(pod *v1.Pod) {
	pi := cache.matchPoolForPod(pod)
	if err := pi.AddPod(pod); err != nil {
		klog.Errorf("add pod to pool %v failed: %v", pi.Name(), err)
	}
}

// Assumes that lock is already acquired.
func (cache *schedulerCache) updatePod(oldPod, newPod *v1.Pod) error {
	if err := cache.removePod(oldPod); err != nil {
		return err
	}
	cache.addPod(newPod)
	return nil
}

// Assumes that lock is already acquired.
func (cache *schedulerCache) removePod(pod *v1.Pod) error {
	pi := cache.matchPoolForPod(pod)
	return pi.RemovePod(pod)
}

func (cache *schedulerCache) AddPod(pod *v1.Pod) error {
	key, err := schedulerinfo.GetPodKey(pod)
	if err != nil {
		return err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	currState, ok := cache.podStates[key]
	switch {
	case ok && cache.assumedPods[key]:
		if currState.pod.Spec.NodeName != pod.Spec.NodeName {
			// The pod was added to a different node than it was assumed to.
			klog.Warningf("Pod %v was assumed to be on %v but got added to %v", key, pod.Spec.NodeName, currState.pod.Spec.NodeName)
			// Clean this up.
			cache.removePod(currState.pod)
			cache.addPod(pod)
		}
		delete(cache.assumedPods, key)
		cache.podStates[key].deadline = nil
		cache.podStates[key].pod = pod
	case !ok:
		// Pod was expired. We should add it back.
		cache.addPod(pod)
		ps := &podState{
			pod: pod,
		}
		cache.podStates[key] = ps
	default:
		return fmt.Errorf("pod %v was already in added state", key)
	}
	return nil
}

func (cache *schedulerCache) UpdatePod(oldPod, newPod *v1.Pod) error {
	key, err := schedulerinfo.GetPodKey(oldPod)
	if err != nil {
		return err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	currState, ok := cache.podStates[key]
	switch {
	// An assumed pod won't have Update/Remove event. It needs to have Add event
	// before Update event, in which case the state would change from Assumed to Added.
	case ok && !cache.assumedPods[key]:
		if currState.pod.Spec.NodeName != newPod.Spec.NodeName {
			klog.Errorf("Pod %v updated on a different node than previously added to.", key)
			klog.Fatalf("Schedulercache is corrupted and can badly affect scheduling decisions")
		}
		if err := cache.updatePod(oldPod, newPod); err != nil {
			return err
		}
		currState.pod = newPod
	default:
		return fmt.Errorf("pod %v is not added to scheduler cache, so cannot be updated", key)
	}
	return nil
}

func (cache *schedulerCache) RemovePod(pod *v1.Pod) error {
	key, err := schedulerinfo.GetPodKey(pod)
	if err != nil {
		return err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	currState, ok := cache.podStates[key]
	switch {
	// An assumed pod won't have Delete/Remove event. It needs to have Add event
	// before Remove event, in which case the state would change from Assumed to Added.
	case ok && !cache.assumedPods[key]:
		if currState.pod.Spec.NodeName != pod.Spec.NodeName {
			klog.Errorf("Pod %v was assumed to be on %v but got added to %v", key, pod.Spec.NodeName, currState.pod.Spec.NodeName)
			klog.Fatalf("Schedulercache is corrupted and can badly affect scheduling decisions")
		}
		err := cache.removePod(currState.pod)
		if err != nil {
			return err
		}
		delete(cache.podStates, key)
	default:
		return fmt.Errorf("pod %v is not found in scheduler cache, so cannot be removed from it", key)
	}
	return nil
}

func (cache *schedulerCache) IsAssumedPod(pod *v1.Pod) (bool, error) {
	key, err := schedulerinfo.GetPodKey(pod)
	if err != nil {
		return false, err
	}

	cache.mu.RLock()
	defer cache.mu.RUnlock()

	b, found := cache.assumedPods[key]
	if !found {
		return false, nil
	}
	return b, nil
}

func (cache *schedulerCache) GetPod(pod *v1.Pod) (*v1.Pod, error) {
	key, err := schedulerinfo.GetPodKey(pod)
	if err != nil {
		return nil, err
	}

	cache.mu.RLock()
	defer cache.mu.RUnlock()

	podState, ok := cache.podStates[key]
	if !ok {
		return nil, fmt.Errorf("pod %v does not exist in scheduler cache", key)
	}

	return podState.pod, nil
}

func (cache *schedulerCache) AddPool(pool *v1alpha1.Pool) error {
	// check if is pool name is DefaultPoolName,
	// if so don't add any nodes to it
	// as default pool create by system, not conflict with manual created pool
	// for preventing hold all nodes unexpect
	if pool == nil || pool.Name == schedulerinfo.DefaultPoolName {
		return fmt.Errorf("forbidden add pool which name is ''")
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	pi, ok := cache.pools[pool.Name]
	if !ok {
		pi = schedulerinfo.NewPoolInfo()
		cache.pools[pool.Name] = pi
	}
	pi.SetPool(pool)
	return cache.moveAllNodeInfosFromMatchedPoolsTo(pi)
}

func (cache *schedulerCache) UpdatePool(oldPool, newPool *v1alpha1.Pool) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	pi, ok := cache.pools[newPool.Name]
	if !ok {
		pi = schedulerinfo.NewPoolInfo()
		cache.pools[newPool.Name] = pi
		pi.SetPool(newPool)
		return cache.moveAllNodeInfosFromMatchedPoolsTo(pi)
	}
	pi.SetPool(newPool)
	// check if need change
	if PoolResourcePropertiesChanged(oldPool, newPool) {
		if err := cache.moveNodeInfosToMatchedPoolFrom(pi); err != nil {
			return err
		}
		return cache.moveAllNodeInfosFromMatchedPoolsTo(pi)
	}
	return nil
}

func PoolResourcePropertiesChanged(oldPool, newPool *v1alpha1.Pool) bool {
	if (oldPool == nil && newPool == nil) || newPool == nil {
		return false
	}
	if poolNodeSelectorChanged(oldPool, newPool) {
		return true
	}
	return false
}
func poolNodeSelectorChanged(oldPool, newPool *v1alpha1.Pool) bool {
	return !reflect.DeepEqual(oldPool.Spec.NodeSelector, newPool.Spec.NodeSelector)
}


func (cache *schedulerCache) RemovePool(pool *v1alpha1.Pool) error {
	if pool.Name == "" {
		return fmt.Errorf("forbiden remove build-in default pool in scheduler cache")
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	pi, ok := cache.pools[pool.Name]
	if !ok {
		return nil
	}
	// gc resources to exists pool
	err := cache.moveNodeInfosToMatchedPoolFrom(pi)
	if err == nil {
		delete(cache.pools, pool.Name)
		pi.ClearPool()
	}
	return err
}

func (cache *schedulerCache) GetPool(poolName string) (*schedulerinfo.PoolInfo, error) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	pool, ok := cache.pools[poolName]
	if !ok {
		return nil, fmt.Errorf("pool %v does not exist in scheduler cache", poolName)
	}
	return pool, nil
}

func (cache *schedulerCache) Pools() map[string]*schedulerinfo.PoolInfo {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.pools
}

func (cache *schedulerCache) moveNodeInfosToMatchedPoolFrom(p *schedulerinfo.PoolInfo) error {
	if p == nil {
		return nil
	}
	df := cache.defaultPool()
	// move all conflict nodes from default pool to its matched pool when delete p
	for _, item := range df.Nodes() {
		var matches []*schedulerinfo.PoolInfo
		for n, pi := range cache.pools {
			if n == p.Name() {
				continue
			}
			if pi.MatchNode(item.Info().Node()) {
				matches = append(matches, pi)
			}
		}
		if len(matches) == 1 {
			// conflict node has not conflicts now
			df.RemoveNodeInfo(item)
			matches[0].AddNodeInfo(item)
		}
	}

	// move all nodes from p to default pool or matched pools
	for _, item := range p.Nodes() {
		var matches []*schedulerinfo.PoolInfo
		for n, pi := range cache.pools {
			if n == p.Name() {
				continue
			}
			if pi.MatchNode(item.Info().Node()) {
				matches = append(matches, pi)
			}
		}
		p.RemoveNodeInfo(item)
		if len(matches) == 1 {
			// ensure node not conflict in pools
			matches[0].AddNodeInfo(item)
		} else {
			df.AddNodeInfo(item)
		}
	}
	return nil
}

func (cache *schedulerCache) moveAllNodeInfosFromMatchedPoolsTo(p *schedulerinfo.PoolInfo) error {
	if p == nil {
		return nil
	}
	df := cache.defaultPool()
	for _, item := range df.Nodes() {
		var matches []*schedulerinfo.PoolInfo
		for _, pi := range cache.pools {
			if pi.IsDefaultPool() {
				continue
			}
			if pi.MatchNode(item.Info().Node()) {
				matches = append(matches, pi)
			}
		}
		if len(matches) == 1 {
			if matches[0] != p {
				klog.Warningf("matches error pool: %v", matches[0].Name())
				continue
			}
			df.RemoveNodeInfo(item)
			p.AddNodeInfo(item)
		}
	}
	for n, pi := range cache.pools {
		if pi.IsDefaultPool() || n == p.Name() {
			continue
		}
		for _, item := range pi.Nodes() {
			if p.MatchNode(item.Info().Node()) {
				pi.RemoveNodeInfo(item)
				df.AddNodeInfo(item)
			}
		}
	}
	return nil
}

func (cache *schedulerCache) AddNode(node *v1.Node) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	p := cache.matchPoolForNode(node)
	if _, ok := p.ContainsNode(node.Name); ok {
		cache.removeNodeImageStates(node)
	}
	if err := p.AddNode(node); err != nil {
		return err
	}
	cache.addNodeImageStates(node, p.Nodes()[node.Name].Info())
	return nil
}

func (cache *schedulerCache) UpdateNode(oldNode, newNode *v1.Node) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	oldPool := cache.matchPoolForNode(oldNode)
	newPool := cache.matchPoolForNode(newNode)
	if oldPool == newPool {
		if n, ok := newPool.ContainsNode(newNode.Name); ok {
			cache.removeNodeImageStates(n.Info().Node())
		}
		if err := newPool.UpdateNode(oldNode, newNode); err != nil {
			return err
		}
	} else {
		oldItem, ok := oldPool.ContainsNode(newNode.Name)
		if ok {
			cache.removeNodeImageStates(oldItem.Info().Node())
			if err := oldPool.RemoveNodeInfo(oldItem); err != nil {
				return err
			}
		}
		if newItem, ok := newPool.ContainsNode(newNode.Name); ok {
			cache.removeNodeImageStates(newItem.Info().Node())
			if err := newPool.AddNodeInfo(newItem); err != nil {
				return err
			}
			if err := newItem.Info().SetNode(newNode); err != nil {
				return err
			}
		} else {
			if err := newPool.AddNodeInfo(oldItem); err != nil {
				return err
			}
			if err := oldItem.Info().SetNode(newNode); err != nil {
				return err
			}
		}
	}
	cache.addNodeImageStates(newNode, newPool.Nodes()[newNode.Name].Info())
	return nil
}

func (cache *schedulerCache) RemoveNode(node *v1.Node) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	p := cache.matchPoolForNode(node)
	if err := p.RemoveNode(node); err != nil {
		return err
	}
	cache.removeNodeImageStates(node)
	return nil
}

func (cache *schedulerCache) matchPoolForNode(node *v1.Node) *schedulerinfo.PoolInfo {
	if node == nil {
		return cache.defaultPool()
	}
	var matches []*schedulerinfo.PoolInfo
	for _, p := range cache.pools {
		if p.MatchNode(node) {
			matches = append(matches, p)
		}
	}
	klog.V(4).Infof("matched pools for node %v: %v", node.Name, matches)

	if len(matches) == 1 {
		return matches[0]
	}
	return cache.defaultPool()
}

// matchPoolForPod
func (cache *schedulerCache) matchPoolForPod(pod *v1.Pod) *schedulerinfo.PoolInfo {
	// scheduled pod
	if nodeName := pod.Spec.NodeName; nodeName != "" {
		for _, pi := range cache.pools {
			if _, ok := pi.ContainsNode(nodeName); ok {
				return pi
			}
		}
		klog.Warningf("Warning not any pool matched for pod %v", pod.Name)
		return cache.defaultPool()
	} else {
		poolName := schedulerinfo.GetPodAnnotationsPoolName(pod)
		if p, ok := cache.pools[poolName]; ok {
			return p
		} else {
			return cache.defaultPool()
		}
	}
}

func (cache *schedulerCache) defaultPool() *schedulerinfo.PoolInfo {
	p, ok := cache.pools[schedulerinfo.DefaultPoolName]
	if !ok {
		klog.Errorf("Error: default pool not exists!!!")
	}
	return p

}

// addNodeImageStates adds states of the images on given node to the given nodeInfo and update the imageStates in
// scheduler cache. This function assumes the lock to scheduler cache has been acquired.
func (cache *schedulerCache) addNodeImageStates(node *v1.Node, nodeInfo *schedulerinfo.NodeInfo) {
	newSum := make(map[string]*schedulerinfo.ImageStateSummary)

	for _, image := range node.Status.Images {
		for _, name := range image.Names {
			// update the entry in imageStates
			state, ok := cache.imageStates[name]
			if !ok {
				state = &imageState{
					size:  image.SizeBytes,
					nodes: sets.NewString(node.Name),
				}
				cache.imageStates[name] = state
			} else {
				state.nodes.Insert(node.Name)
			}
			// create the imageStateSummary for this image
			if _, ok := newSum[name]; !ok {
				newSum[name] = cache.createImageStateSummary(state)
			}
		}
	}
	nodeInfo.SetImageStates(newSum)
}

// removeNodeImageStates removes the given node record from image entries having the node
// in imageStates cache. After the removal, if any image becomes free, i.e., the image
// is no longer available on any node, the image entry will be removed from imageStates.
func (cache *schedulerCache) removeNodeImageStates(node *v1.Node) {
	if node == nil {
		return
	}

	for _, image := range node.Status.Images {
		for _, name := range image.Names {
			state, ok := cache.imageStates[name]
			if ok {
				state.nodes.Delete(node.Name)
				if len(state.nodes) == 0 {
					// Remove the unused image to make sure the length of
					// imageStates represents the total number of different
					// images on all nodes
					delete(cache.imageStates, name)
				}
			}
		}
	}
}

func (cache *schedulerCache) run() {
	go wait.Until(cache.cleanupExpiredAssumedPods, cache.period, cache.stop)
}

func (cache *schedulerCache) cleanupExpiredAssumedPods() {
	cache.cleanupAssumedPods(time.Now())
}

// cleanupAssumedPods exists for making test deterministic by taking time as input argument.
func (cache *schedulerCache) cleanupAssumedPods(now time.Time) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	// The size of assumedPods should be small
	for key := range cache.assumedPods {
		ps, ok := cache.podStates[key]
		if !ok {
			panic("Key found in assumed set but not in podStates. Potentially a logical error.")
		}
		if !ps.bindingFinished {
			klog.V(3).Infof("Couldn't expire cache for pod %v/%v. Binding is still in progress.",
				ps.pod.Namespace, ps.pod.Name)
			continue
		}
		if now.After(*ps.deadline) {
			klog.Warningf("Pod %s/%s expired", ps.pod.Namespace, ps.pod.Name)
			if err := cache.expirePod(key, ps); err != nil {
				klog.Errorf("ExpirePod failed for %s: %v", key, err)
			}
		}
	}
}

func (cache *schedulerCache) expirePod(key string, ps *podState) error {
	if err := cache.removePod(ps.pod); err != nil {
		return err
	}
	delete(cache.assumedPods, key)
	delete(cache.podStates, key)
	return nil
}

func (cache *schedulerCache) NodeTree(poolName string) *schedulerinfo.NodeTree {
	return cache.pools[poolName].NodeTree()
}

// DeserveAllPools compute all pools resources deserved quota
// Compute all quotas sum, if sum great than remain find max quotas set to 0
// or reduce it to satisfy remain.
// Quota will set first and then compute resource by weight.
// When quota deserved must skip weighted deserve.
// weight pool deserved only when has non-zero remain resources.
// deserve weighted resources only total weight great than 0.
// DEPRECATED
func (cache *schedulerCache) DeserveAllPools() error {
	// Deserve default pool first
	//ds := cache.deserveDefaultPool()
	// Compute all allocatable resources of all nodes
	//totalRes := cache.calculateTotalResource()
	//remainRes := totalRes.Clone().Sub(ds)
	//remainRes := totalRes.Clone()

	// Deserve all pools' quota
	//cache.deserveQuotaPools(remainRes)

	// Deserve all pools need match nodes
	//cache.deserveNeedMatchNodePools(remainRes)

	// Deserve weighted pools
	//cache.deserveWeightedPools(remainRes)

	return nil
}

func (cache *schedulerCache) calculateTotalResource() *schedulerinfo.Resource {
	total := &schedulerinfo.Resource{}
	for _, node := range cache.nodes() {
		rs := node.Info().AllocatableResource()
		total.Plus(&rs)
	}
	return total
}

func (cache *schedulerCache) nodes() map[string]*schedulerinfo.NodeInfoListItem {
	// FIXME REVIEW if possible
	// Get all nodes in all pools
	maxNodeSize := 0
	for _, pi := range cache.pools {
		maxNodeSize += pi.NumNodes()
	}
	nodes := make(map[string]*schedulerinfo.NodeInfoListItem, maxNodeSize)
	for _, pi := range cache.pools {
		for name, n := range pi.Nodes() {
			nodes[name] = n
		}
	}

	return nodes
}

func (cache *schedulerCache) getPools() []interface{} {
	pools := make([]interface{}, len(cache.pools))
	for _, p := range cache.pools {
		pools = append(pools, p.GetPool())
	}

	return pools
}

func (cache *schedulerCache) NumPools() int {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return len(cache.pools)
}

func (cache *schedulerCache) NumNodes() int {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return len(cache.nodes())
}

func (cache *schedulerCache) TotalAllocatableResource() *schedulerinfo.Resource {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.calculateTotalResource()
}

func (cache *schedulerCache) GetPoolContainsNode(nodeName string) *schedulerinfo.PoolInfo {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	for _, p := range cache.pools {
		if _, ok := p.ContainsNode(nodeName); ok {
			return p
		}
	}
	return nil
}

func (cache *schedulerCache) BorrowPool(fromPoolName string, pod *v1.Pod, predicateFuncs map[string]predicates.FitPredicate) string {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	selfPoolInfo := cache.matchPoolForPod(pod)
	selfPoolName := selfPoolInfo.Name()
	if !selfPoolInfo.DisableBorrowing() {
		higherIdleFunc := func(pool1, pool2 interface{}) bool {
			p1 := pool1.(*schedulerinfo.PoolInfo)
			p2 := pool2.(*schedulerinfo.PoolInfo)
			// self pool has highest priority
			if selfPoolName == p1.Name() && selfPoolName != p2.Name() {
				return true
			}
			if selfPoolName != p1.Name() && selfPoolName == p2.Name() {
				return false
			}
			return !p1.Idle().LessOrEqual(p2.Idle())
		}
		sharingPools := util.SortableList{CompFunc: higherIdleFunc}
		for _, p := range cache.pools {
			// skip other pools that set disableSharing=true and current fromPool
			if fromPoolName == p.Name() || !selfPoolInfo.CanBorrowPool(p) {
				continue
			}
			sharingPools.Items = append(sharingPools.Items, p)
		}
		sharingPools.Sort()
		for _, p := range sharingPools.Items {
			pi := p.(*schedulerinfo.PoolInfo)
			klog.V(4).Infof("Attempt to borrow %q for pod %v/%v@%v in queue %q", pi.Name(), pod.Namespace, pod.Name, selfPoolName, fromPoolName)
			// predicate for nodes of pool
			for _, ni := range pi.Nodes() {
				cache.mu.RUnlock()
				fit, reasons, err := predicateCheck(pod, ni.Info().Clone(), predicateFuncs)
				cache.mu.RLock()
				if !fit {
					nodeName := "NOT FOUND"
					if ni.Info().Node() != nil {
						nodeName = ni.Info().Node().Name
					}
					klog.V(4).Infof("Node %v in %v predicates fit failed: %v:%v", nodeName, pi.Name(), reasons, err)
					continue
				}
				klog.V(4).Infof("Node %v in %v pool predicates succeed.", ni.Info().Node().Name, pi.Name())
				return pi.Name()
			}
		}

	}
	klog.V(4).Infof("pool %q disabled borrowing: %v, or Not any pools fit pod %v/%v", selfPoolName, selfPoolInfo.DisableBorrowing(), pod.Namespace, pod.Name)
	return selfPoolName
}

func predicateCheck(pod *v1.Pod, nodeInfo *schedulerinfo.NodeInfo, predicateFuncs map[string]predicates.FitPredicate) (bool,[]predicates.PredicateFailureReason, error) {
	var failedPredicates []predicates.PredicateFailureReason
	for _, predicateKey := range predicates.Ordering() {
		var (
			fit     bool
			reasons []predicates.PredicateFailureReason
			err     error
		)
		if predicate, exist := predicateFuncs[predicateKey]; exist {
			fit, reasons, err = predicate(pod, nil, nodeInfo)
			if err != nil {
				return false, []predicates.PredicateFailureReason{}, err
			}
			if !fit {
				// eCache is available and valid, and predicates result is unfit, record the fail reasons
				failedPredicates = append(failedPredicates, reasons...)
				break
			}
		}
	}
	return len(failedPredicates) == 0, failedPredicates, nil
}

func (cache *schedulerCache) Metrics() {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	metrics.PoolFeatures.Reset()
	metrics.PoolNodes.Reset()
	for poolName, poolInfo := range cache.pools {
		if poolName == schedulerinfo.DefaultPoolName {
			poolName = "default"
		}
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceCpu, "type": metrics.PoolDetailTypeCapacity}).Set(float64(poolInfo.Capacity().MilliCPU))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceGpu, "type": metrics.PoolDetailTypeCapacity}).Set(float64(poolInfo.Capacity().ScalarResources[schedulerinfo.ResourceGPU]))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceMem, "type": metrics.PoolDetailTypeCapacity}).Set(float64(poolInfo.Capacity().Memory))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceEStorage, "type": metrics.PoolDetailTypeCapacity}).Set(float64(poolInfo.Capacity().EphemeralStorage))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceNode, "type": metrics.PoolDetailTypeCapacity}).Set(float64(poolInfo.NumNodes()))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourcePods, "type": metrics.PoolDetailTypeCapacity}).Set(float64(poolInfo.Capacity().AllowedPodNumber))

		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceCpu, "type": metrics.PoolDetailTypeAllocatable}).Set(float64(poolInfo.Allocatable().MilliCPU))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceGpu, "type": metrics.PoolDetailTypeAllocatable}).Set(float64(poolInfo.Allocatable().ScalarResources[schedulerinfo.ResourceGPU]))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceMem, "type": metrics.PoolDetailTypeAllocatable}).Set(float64(poolInfo.Allocatable().Memory))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceEStorage, "type": metrics.PoolDetailTypeAllocatable}).Set(float64(poolInfo.Allocatable().EphemeralStorage))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceNode, "type": metrics.PoolDetailTypeAllocatable}).Set(float64(poolInfo.NodeTree().NumNodes()))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourcePods, "type": metrics.PoolDetailTypeAllocatable}).Set(float64(poolInfo.Allocatable().AllowedPodNumber))

		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceCpu, "type": metrics.PoolDetailTypeUsed}).Set(float64(poolInfo.Used().MilliCPU))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceGpu, "type": metrics.PoolDetailTypeUsed}).Set(float64(poolInfo.Used().ScalarResources[schedulerinfo.ResourceGPU]))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceMem, "type": metrics.PoolDetailTypeUsed}).Set(float64(poolInfo.Used().Memory))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceEStorage, "type": metrics.PoolDetailTypeUsed}).Set(float64(poolInfo.Used().EphemeralStorage))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourcePods, "type": metrics.PoolDetailTypeUsed}).Set(float64(poolInfo.Used().AllowedPodNumber))

		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceCpu, "type": metrics.PoolDetailTypeShared}).Set(float64(poolInfo.Shared().MilliCPU))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceGpu, "type": metrics.PoolDetailTypeShared}).Set(float64(poolInfo.Shared().ScalarResources[schedulerinfo.ResourceGPU]))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceMem, "type": metrics.PoolDetailTypeShared}).Set(float64(poolInfo.Shared().Memory))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourceEStorage, "type": metrics.PoolDetailTypeShared}).Set(float64(poolInfo.Shared().EphemeralStorage))
		metrics.PoolResourceDetails.With(prometheus.Labels{"pool": poolName, "resource": metrics.PoolResourcePods, "type": metrics.PoolDetailTypeShared}).Set(float64(poolInfo.Shared().AllowedPodNumber))

		var preemption = float64(0)
		var borrowing = float64(0)
		var sharing = float64(0)
		if !poolInfo.DisablePreemption() {
			preemption = float64(1)
		}
		if !poolInfo.DisableBorrowing() {
			borrowing = float64(1)
		}
		if !poolInfo.DisableSharing() {
			sharing = float64(1)
		}
		metrics.PoolFeatures.With(prometheus.Labels{"pool": poolName, "feature": metrics.PoolFeaturePreemption}).Set(preemption)
		metrics.PoolFeatures.With(prometheus.Labels{"pool": poolName, "feature": metrics.PoolFeatureBorrowing}).Set(borrowing)
		metrics.PoolFeatures.With(prometheus.Labels{"pool": poolName, "feature": metrics.PoolFeatureSharing}).Set(sharing)

		for _, nodeItem := range poolInfo.Nodes() {
			if nodeItem.Info().Node() != nil {
				metrics.PoolNodes.With(prometheus.Labels{"pool": poolName, "node": nodeItem.Info().Node().Name}).Set(1)
			}
		}
	}
}
