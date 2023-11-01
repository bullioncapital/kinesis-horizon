//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package sequence

import (
	"context"
	"testing"
	"time"

	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type QueueTestSuite struct {
	suite.Suite
	ctx   context.Context
	queue *AccountTxSubmissionQueue
}

func (suite *QueueTestSuite) SetupTest() {
	suite.ctx = test.Context()
	suite.queue = NewAccountTxSubmissionQueue()
}

// Tests the NotifyLastAccountSequence method
func (suite *QueueTestSuite) TestQueue_NotifyLastAccountSequence() {
	// NotifyLastAccountSequence removes sequences that are submittable or in the past
	lowMin := uint64(1)
	results := []<-chan error{
		suite.queue.Push(1, nil),
		suite.queue.Push(2, nil),
		suite.queue.Push(3, nil),
		suite.queue.Push(4, nil),
		suite.queue.Push(4, &lowMin),
	}

	suite.queue.NotifyLastAccountSequence(2)

	// the update above signifies that 2 is the accounts current sequence,
	// meaning that 3 is submittable, and so only 4 (Min/maxAccSeqNum=3) should remain
	assert.Equal(suite.T(), 1, suite.queue.Size())
	entry := suite.queue.transactions[0]
	assert.Equal(suite.T(), uint64(3), entry.minAccSeqNum)
	assert.Equal(suite.T(), uint64(3), entry.maxAccSeqNum)

	suite.queue.NotifyLastAccountSequence(4)
	assert.Equal(suite.T(), 0, suite.queue.Size())

	assert.Equal(suite.T(), ErrBadSequence, <-results[0])
	assert.Equal(suite.T(), ErrBadSequence, <-results[1])
	assert.Equal(suite.T(), nil, <-results[2])
	assert.Equal(suite.T(), ErrBadSequence, <-results[3])
	assert.Equal(suite.T(), nil, <-results[4])

	// NotifyLastAccountSequence clears the queue if the head has not been released within the time limit
	suite.queue.timeout = 1 * time.Millisecond
	result := suite.queue.Push(2, nil)
	<-time.After(10 * time.Millisecond)
	suite.queue.NotifyLastAccountSequence(0)

	assert.Equal(suite.T(), 0, suite.queue.Size())
	assert.Equal(suite.T(), ErrBadSequence, <-result)
}

func TestQueueTestSuite(t *testing.T) {
	suite.Run(t, new(QueueTestSuite))
}
