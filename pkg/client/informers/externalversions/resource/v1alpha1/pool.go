/*
Copyright 2019 The Ai-Scheduler Authors.

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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	time "time"

	resourcev1alpha1 "gitlab.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	versioned "gitlab.aibee.cn/platform/ai-scheduler/pkg/client/clientset/versioned"
	internalinterfaces "gitlab.aibee.cn/platform/ai-scheduler/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha1 "gitlab.aibee.cn/platform/ai-scheduler/pkg/client/listers/resource/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// PoolInformer provides access to a shared informer and lister for
// Pools.
type PoolInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.PoolLister
}

type poolInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewPoolInformer constructs a new informer for Pool type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewPoolInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredPoolInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredPoolInformer constructs a new informer for Pool type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredPoolInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ResourceV1alpha1().Pools().List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ResourceV1alpha1().Pools().Watch(options)
			},
		},
		&resourcev1alpha1.Pool{},
		resyncPeriod,
		indexers,
	)
}

func (f *poolInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredPoolInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *poolInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&resourcev1alpha1.Pool{}, f.defaultInformer)
}

func (f *poolInformer) Lister() v1alpha1.PoolLister {
	return v1alpha1.NewPoolLister(f.Informer().GetIndexer())
}