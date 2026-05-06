package sequence

import (
	"sort"
	"time"
)

// AccountTxSubmissionQueue manages the submission queue for a single source account. The
// transaction system uses Push to enqueue submissions for given sequence
// numbers.
//
// AccountTxSubmissionQueue maintains a priority queue of pending submissions, and when updated
// (via the NotifyLastAccountSequence() method) with the current sequence number of the account
// being managed, queued submissions that can be acted upon will be unblocked.
type AccountTxSubmissionQueue struct {
	lastActiveAt            time.Time
	timeout                 time.Duration
	lastSeenAccountSequence uint64
	transactions            []txToSubmit
}

// txToSubmit represents a transaction being tracked by the queue
type txToSubmit struct {
	minAccSeqNum   uint64     // minimum account sequence required to send the transaction
	maxAccSeqNum   uint64     // maximum account sequence required to send the transaction
	notifyBackChan chan error // submission notification channel
}

// NewAccountTxSubmissionQueue creates a new *AccountTxSubmissionQueue
func NewAccountTxSubmissionQueue() *AccountTxSubmissionQueue {
	result := &AccountTxSubmissionQueue{
		lastActiveAt: time.Now(),
		timeout:      10 * time.Second,
	}
	return result
}

// Size returns the count of currently buffered submissions in the queue.
func (q *AccountTxSubmissionQueue) Size() int {
	return len(q.transactions)
}

// Push enqueues the intent to submit a transaction at the provided sequence
// number and returns a channel that will emit when it is safe for the client
// to do so.
//
// Push does not perform any triggering (which
// occurs in NotifyLastAccountSequence(), even if the current sequence number for this queue is
// the same as the provided sequence, to keep internal complexity much lower.
// Given that, the recommended usage pattern is:
//
//  1. Push the submission onto the queue
//  2. Load the current sequence number for the source account from the DB
//  3. Call NotifyLastAccountSequence() with the result from step 2 to trigger the submission if
//     possible
func (q *AccountTxSubmissionQueue) Push(sequence uint64, minSeqNum *uint64) <-chan error {
	// From CAP 21: If minSeqNum is nil, the txToSubmit is only valid when sourceAccount's sequence number is seqNum - 1.
	// Otherwise, valid when sourceAccount's sequence number n satisfies minSeqNum <= n < txToSubmit.seqNum.
	effectiveMinSeqNum := sequence - 1
	if minSeqNum != nil {
		effectiveMinSeqNum = *minSeqNum
	}
	ch := make(chan error, 1)
	q.transactions = append(q.transactions, txToSubmit{
		minAccSeqNum:   effectiveMinSeqNum,
		maxAccSeqNum:   sequence - 1,
		notifyBackChan: ch,
	})
	return ch
}

// NotifyLastAccountSequence notifies the queue that the provided sequence number is the latest
// seen value for the account that this queue manages submissions for.
//
// This function is monotonic... calling it with a sequence number lower than
// the latest seen sequence number is a noop.
func (q *AccountTxSubmissionQueue) NotifyLastAccountSequence(sequence uint64) {
	if q.lastSeenAccountSequence < sequence {
		q.lastSeenAccountSequence = sequence
	}

	queueWasChanged := false

	txsToSubmit := make([]txToSubmit, 0, len(q.transactions))
	// Extract transactions ready to submit and notify those which are un-submittable.
	for i := 0; i < len(q.transactions); {
		candidate := q.transactions[i]
		removeCandidateFromQueue := false
		if q.lastSeenAccountSequence > candidate.maxAccSeqNum {
			// this transaction can never be submitted because account sequence numbers only grow
			candidate.notifyBackChan <- ErrBadSequence
			close(candidate.notifyBackChan)
			removeCandidateFromQueue = true
		} else if q.lastSeenAccountSequence >= candidate.minAccSeqNum {
			txsToSubmit = append(txsToSubmit, candidate)
			removeCandidateFromQueue = true
		}
		if removeCandidateFromQueue {
			q.transactions = append(q.transactions[:i], q.transactions[i+1:]...)
			queueWasChanged = true
		} else {
			// only increment the index if there was no removal
			i++
		}
	}

	// To maximize successful submission opportunity, submit transactions by the account sequence
	// which would result from a successful submission (i.e. maxAccSeqNum+1)
	sort.Slice(txsToSubmit, func(i, j int) bool {
		return txsToSubmit[i].maxAccSeqNum < txsToSubmit[j].maxAccSeqNum
	})
	for _, tx := range txsToSubmit {
		tx.notifyBackChan <- nil
		close(tx.notifyBackChan)
	}

	// if we modified the queue, bump the timeout for this queue
	if queueWasChanged {
		q.lastActiveAt = time.Now()
		return
	}

	// if the queue wasn't changed, see if it is too old, clear
	// it and make room for other submissions
	if time.Since(q.lastActiveAt) > q.timeout {
		for _, tx := range q.transactions {
			tx.notifyBackChan <- ErrBadSequence
			close(tx.notifyBackChan)
		}
		q.transactions = nil
	}
}
