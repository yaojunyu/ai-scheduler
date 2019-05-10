package info

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

