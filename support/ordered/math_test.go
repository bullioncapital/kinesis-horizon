package ordered

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMinMax(t *testing.T) {
	t.Run("int", func(tt *testing.T) {
		a, b := -5, 10
		require.Equal(tt, Min(a, b), a)
		require.Equal(tt, Max(a, b), b)
	})

	t.Run("float", func(tt *testing.T) {
		a, b := -5.0, 10.0
		require.Equal(tt, Min(a, b), a)
		require.Equal(tt, Max(a, b), b)
	})

	t.Run("unsigned", func(tt *testing.T) {
		a, b := uint(5), uint(10)
		require.Equal(tt, Min(a, b), a)
		require.Equal(tt, Max(a, b), b)
	})
}
