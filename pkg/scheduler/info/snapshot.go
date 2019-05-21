package info

import v1 "k8s.io/api/core/v1"

// Snapshot is a snapshot of cache state
type Snapshot struct {
	AssumedPods map[string]bool
	Nodes       map[string]*NodeInfo
}

// NodeInfoSnapshot is a snapshot of cache NodeInfo. The scheduler takes a
// snapshot at the beginning of each scheduling cycle and uses it for its
// operations in that cycle.
type NodeInfoSnapshot struct {
	NodeInfoMap map[string]*NodeInfo
	Generation  int64
}

func (ns *NodeInfoSnapshot) Nodes() []*v1.Node {
	result := make([]*v1.Node, 0, len(ns.NodeInfoMap))
	for _, n := range ns.NodeInfoMap {
		if n.node != nil {
			result = append(result, n.node)
		}
	}
	return result
}
