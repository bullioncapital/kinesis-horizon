package history

import (
	"context"

	"github.com/stellar/go/support/db"
	"github.com/stellar/go/xdr"
)

// ClaimableBalanceClaimantBatchInsertBuilder is used to insert transactions into the
// history_transactions table
type ClaimableBalanceClaimantBatchInsertBuilder interface {
	Add(ctx context.Context, claimableBalanceClaimant ClaimableBalanceClaimant) error
	Exec(ctx context.Context) error
}

// ClaimableBalanceClaimantBatchInsertBuilder is a simple wrapper around db.BatchInsertBuilder
type claimableBalanceClaimantBatchInsertBuilder struct {
	encodingBuffer *xdr.EncodingBuffer
	builder        db.BatchInsertBuilder
}

// NewClaimableBalanceClaimantBatchInsertBuilder constructs a new ClaimableBalanceClaimantBatchInsertBuilder instance
func (q *Q) NewClaimableBalanceClaimantBatchInsertBuilder(maxBatchSize int) ClaimableBalanceClaimantBatchInsertBuilder {
	return &claimableBalanceClaimantBatchInsertBuilder{
		encodingBuffer: xdr.NewEncodingBuffer(),
		builder: db.BatchInsertBuilder{
			Table:        q.GetTable("claimable_balance_claimants"),
			MaxBatchSize: maxBatchSize,
			Suffix:       "ON CONFLICT (id, destination) DO UPDATE SET last_modified_ledger=EXCLUDED.last_modified_ledger",
		},
	}
}

// Add adds a new transaction to the batch
func (i *claimableBalanceClaimantBatchInsertBuilder) Add(ctx context.Context, claimableBalanceClaimant ClaimableBalanceClaimant) error {
	return i.builder.RowStruct(ctx, claimableBalanceClaimant)
}

func (i *claimableBalanceClaimantBatchInsertBuilder) Exec(ctx context.Context) error {
	return i.builder.Exec(ctx)
}
