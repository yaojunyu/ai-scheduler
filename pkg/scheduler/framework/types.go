package framework


// LessFn is the func declaration used by sort or priority queue.
type LessFn func(interface{}, interface{}) bool

// CompareFn is the func declaration used by sort or priority queue.
type CompareFn func(interface{}, interface{}) int

// ValidateFn is the func declaration used to check object's status.
type ValidateFn func(interface{}) bool

// ValidateResult is struct to which can used to determine the result
type ValidateResult struct {
	Pass    bool
	Reason  string
	Message string
}

// ValidateExFn is the func declaration used to validate the result
type ValidateExFn func(interface{}) *ValidateResult

