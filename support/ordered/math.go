package ordered

import (
	"golang.org/x/exp/constraints"
)

// Min returns the smaller of the given items.
func Min[T constraints.Ordered](a, b T) T {
	if a <= b {
		return a
	}
	return b
}

// Min returns the larger of the given items.
func Max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}
