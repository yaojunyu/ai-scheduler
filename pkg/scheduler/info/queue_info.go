/*
Copyright 2018 The Kubernetes Authors.

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

package info

//// QueueID is UID type, serves as unique ID for each queue
//type QueueID types.UID
//
//// QueueInfo will have all details about queue
//type QueueInfo struct {
//	UID  QueueID
//	Name string
//
//	Weight int32
//
//	Queue *arbcorev1.Queue
//}
//
//// NewQueueInfo creates new queueInfo object
//func NewQueueInfo(queue *arbcorev1.Queue) *QueueInfo {
//	return &QueueInfo{
//		UID:  QueueID(queue.Name),
//		Name: queue.Name,
//
//		Weight: queue.Spec.Weight,
//
//		Queue: queue,
//	}
//}
//
//// Clone is used to clone queueInfo object
//func (q *QueueInfo) Clone() *QueueInfo {
//	return &QueueInfo{
//		UID:    q.UID,
//		Name:   q.Name,
//		Weight: q.Weight,
//		Queue:  q.Queue,
//	}
//}
