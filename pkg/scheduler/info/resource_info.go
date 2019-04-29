package info

import (
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/apis/core/v1/helper"
	"math"
)

type Resource struct {
	MilliCPU float64
	Memory 	 float64

	// ScalarResources
	ScalarResources map[v1.ResourceName]float64
	
	// MaxTaskNum is only used by predicates; it should NOT
	// be accounted in other operators, e.g. Add.
	MaxTaskNum int
}

const (
	// GPUResourceName need to follow https://github.com/NVIDIA/k8s-device-plugin/blob/66a35b71ac4b5cbfb04714678b548bd77e5ba719/server.go#L20
	GPUResourceName = "nvidia.com/gpu"

	MinMilliCPU float64 = 10
	MinMilliScalarResources float64 = 10
	MinMemory float64 = 10 * 1024 * 1024
)


// EmptyResource creates a empty resource object and returns
func EmptyResource() *Resource {
	return &Resource{}
}

// Clone is used to clone a resource type
func (r *Resource) Clone() *Resource {
	clone := &Resource{
		MilliCPU: r.MilliCPU,
		Memory: r.Memory,
		MaxTaskNum: r.MaxTaskNum,
	}

	if r.ScalarResources != nil {
		clone.ScalarResources = make(map[v1.ResourceName]float64)
		for k, v := range r.ScalarResources {
			clone.ScalarResources[k] = v
		}
	}

	return clone
}

// NewResource create a new resource object from resource list
func NewResource(rl v1.ResourceList) *Resource {
	r := EmptyResource()
	for rName, rQuant := range rl {
		switch rName {
		case v1.ResourceCPU:
			r.MilliCPU += float64(rQuant.MilliValue())
		case v1.ResourceMemory:
			r.Memory += float64(rQuant.Value())
		case v1.ResourcePods:
			r.MaxTaskNum += int(rQuant.Value())
		default:
			if helper.IsScalarResourceName(rName) {
				r.AddScalar(rName, float64(rQuant.MilliValue()))
			}
		}
	}

	return r
}

// IsEmpty returns bool after checking any of resource is less than min possible value
func (r *Resource) IsEmpty() bool {
	if !(r.MilliCPU < MinMilliCPU && r.Memory < MinMemory) {
		return false
	}
	for _, rQuant := range r.ScalarResources {
		if rQuant >= MinMilliScalarResources {
			return false
		}
	}
	return true
}


// IsZero checks whether that resource is less than min possible value
func (r *Resource) IsZero(rn v1.ResourceName) bool {
	switch rn {
	case v1.ResourceCPU:
		return r.MilliCPU < MinMilliCPU
	case v1.ResourceMemory:
		return r.Memory < MinMemory
	default:
		if r.ScalarResources == nil {
			return true
		}

		if _, ok := r.ScalarResources[rn]; !ok {
			panic("unknown resource")
		}

		return r.ScalarResources[rn] < MinMilliScalarResources
	}
}

// Add is used to add the two resources
func (r *Resource) Add(rr *Resource) *Resource {
	r.MilliCPU += rr.MilliCPU
	r.Memory += rr.Memory

	for rName, rQuant := range rr.ScalarResources {
		if r.ScalarResources == nil {
			r.ScalarResources = map[v1.ResourceName]float64{}
		}
		r.ScalarResources[rName] += rQuant
	}

	return r
}

//Sub subtracts two Resource objects.
func (r *Resource) Sub(rr *Resource) *Resource {
	if rr.LessEqual(r) {
		r.MilliCPU -= rr.MilliCPU
		r.Memory -= rr.Memory

		for rrName, rrQuant := range rr.ScalarResources {
			if r.ScalarResources == nil {
				return r
			}
			r.ScalarResources[rrName] -= rrQuant
		}

		return r
	}

	panic(fmt.Errorf("Resource is not sufficient to do operation: %v sub %v", r, rr))
}

// Multi multiples the resource with ratio provided
func (r *Resource) Multi(ratio float64) *Resource {
	r.MilliCPU = r.MilliCPU * ratio
	r.Memory = r.Memory * ratio
	for rName, rQuant := range r.ScalarResources {
		r.ScalarResources[rName] = rQuant * ratio
	}
	return r
}

// Less checks whether a resource is less than other
func (r *Resource) Less(rr *Resource) bool {
	if !(r.MilliCPU < rr.MilliCPU && r.Memory < rr.Memory) {
		return false
	}

	if r.ScalarResources == nil {
		if rr.ScalarResources == nil {
			return false
		}
		return true
	}

	for rName, rQuant := range r.ScalarResources {
		if rr.ScalarResources == nil {
			return false
		}

		rrQuant := rr.ScalarResources[rName]
		if rQuant >= rrQuant {
			return false
		}
	}

	return true
}

// LessEqual checks whether a resource is less than other resource
func (r *Resource) LessEqual(rr *Resource) bool {
	isLess := (r.MilliCPU < rr.MilliCPU || math.Abs(rr.MilliCPU-r.MilliCPU) < MinMilliCPU) &&
		(r.Memory < rr.Memory || math.Abs(rr.Memory-r.Memory) < MinMemory)
	if !isLess {
		return false
	}

	if r.ScalarResources == nil {
		return true
	}

	for rName, rQuant := range r.ScalarResources {
		if rr.ScalarResources == nil {
			return false
		}

		rrQuant := rr.ScalarResources[rName]
		if !(rQuant < rrQuant || math.Abs(rrQuant-rQuant) < MinMilliScalarResources) {
			return false
		}
	}

	return true
}


// SetMaxResource compares with ResourceList and takes max value for each Resource.
func (r *Resource) SetMaxResource(rr *Resource) {
	if r == nil || rr == nil {
		return
	}

	if rr.MilliCPU > r.MilliCPU {
		r.MilliCPU = rr.MilliCPU
	}
	if rr.Memory > r.Memory {
		r.Memory = rr.Memory
	}

	for rrName, rrQuant := range rr.ScalarResources {
		if r.ScalarResources == nil {
			r.ScalarResources = make(map[v1.ResourceName]float64)
			for k, v := range rr.ScalarResources {
				r.ScalarResources[k] = v
			}
			return
		}

		if rrQuant > r.ScalarResources[rrName] {
			r.ScalarResources[rrName] = rrQuant
		}
	}
}

//FitDelta Computes the delta between a resource oject representing available
//resources an operand representing resources being requested.  Any
//field that is less than 0 after the operation represents an
//insufficient resource.
func (r *Resource) FitDelta(rr *Resource) *Resource {
	if rr.MilliCPU > 0 {
		r.MilliCPU -= rr.MilliCPU + MinMilliCPU
	}

	if rr.Memory > 0 {
		r.Memory -= rr.Memory + MinMemory
	}

	for rrName, rrQuant := range rr.ScalarResources {
		if r.ScalarResources == nil {
			r.ScalarResources = map[v1.ResourceName]float64{}
		}

		if rrQuant > 0 {
			r.ScalarResources[rrName] -= rrQuant + MinMilliScalarResources
		}
	}

	return r
}

// AddScalar adds a resource by a scalar value of this resource.
func (r *Resource) AddScalar(name v1.ResourceName, quantity float64) {
	r.SetScalar(name, r.ScalarResources[name]+quantity)
}

// SetScalar sets a resource by a scalar value of this resource.
func (r *Resource) SetScalar(name v1.ResourceName, quantity float64) {
	// Lazily allocate scalar resource map.
	if r.ScalarResources == nil {
		r.ScalarResources = map[v1.ResourceName]float64{}
	}
	r.ScalarResources[name] = quantity
}

// Get returns the resource value for that particular resource type
func (r *Resource) Get(rn v1.ResourceName) float64 {
	switch rn {
	case v1.ResourceCPU:
		return r.MilliCPU
	case v1.ResourceMemory:
		return r.Memory
	default:
		if r.ScalarResources == nil {
			return 0
		}
		return r.ScalarResources[rn]
	}
}

// ResourceNames returns all resource types
func (r *Resource) ResourceNames() []v1.ResourceName {
	resNames := []v1.ResourceName{v1.ResourceCPU, v1.ResourceMemory}

	for rName := range r.ScalarResources {
		resNames = append(resNames, rName)
	}

	return resNames
}

// String returns resource details in string format
func (r *Resource) String() string {
	str := fmt.Sprintf("cpu %0.2f, memory %0.2f", r.MilliCPU, r.Memory)
	for rName, rQuant := range r.ScalarResources {
		str = fmt.Sprintf("%s, %s %0.2f", str, rName, rQuant)
	}
	return str
}