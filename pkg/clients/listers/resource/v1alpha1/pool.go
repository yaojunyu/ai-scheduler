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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "code.aibee.cn/platform/ai-scheduler/pkg/apis/resource/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// PoolLister helps list Pools.
type PoolLister interface {
	// List lists all Pools in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.Pool, err error)
	// Get retrieves the Pool from the index for a given name.
	Get(name string) (*v1alpha1.Pool, error)
	PoolListerExpansion
}

// poolLister implements the PoolLister interface.
type poolLister struct {
	indexer cache.Indexer
}

// NewPoolLister returns a new PoolLister.
func NewPoolLister(indexer cache.Indexer) PoolLister {
	return &poolLister{indexer: indexer}
}

// List lists all Pools in the indexer.
func (s *poolLister) List(selector labels.Selector) (ret []*v1alpha1.Pool, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.Pool))
	})
	return ret, err
}

// Get retrieves the Pool from the index for a given name.
func (s *poolLister) Get(name string) (*v1alpha1.Pool, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("pool"), name)
	}
	return obj.(*v1alpha1.Pool), nil
}
