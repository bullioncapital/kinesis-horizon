package sequence

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test the Push method
func TestManager_Push(t *testing.T) {
	mgr := NewManager()

	minSeq := uint64(1)
	mgr.Push("1", 2, nil)
	mgr.Push("1", 2, nil)
	mgr.Push("1", 3, &minSeq)
	mgr.Push("2", 2, nil)

	assert.Equal(t, 4, mgr.Size())
	assert.Equal(t, 3, mgr.queues["1"].Size())
	assert.Equal(t, 1, mgr.queues["2"].Size())
}

// Test the NotifyLastAccountSequences method
func TestManager_NotifyLastAccountSequences(t *testing.T) {
	mgr := NewManager()
	minSeq := uint64(1)
	results := []<-chan error{
		mgr.Push("1", 4, &minSeq),
		mgr.Push("1", 3, nil),
		mgr.Push("2", 2, nil),
	}

	mgr.NotifyLastAccountSequences(map[string]uint64{
		"1": 1,
		"2": 1,
	})

	assert.Equal(t, 1, mgr.Size())
	_, ok := mgr.queues["2"]
	assert.False(t, ok)

	assert.Equal(t, nil, <-results[0])
	assert.Equal(t, nil, <-results[2])
	assert.Equal(t, 0, len(results[1]))
}

// Push until maximum queue size is reached and check that another push results in ErrNoMoreRoom
func TestManager_PushNoMoreRoom(t *testing.T) {
	mgr := NewManager()
	for i := 0; i < mgr.MaxSize; i++ {
		mgr.Push("1", 2, nil)
	}

	assert.Equal(t, 1024, mgr.Size())
	assert.Equal(t, ErrNoMoreRoom, <-mgr.Push("1", 2, nil))
}
