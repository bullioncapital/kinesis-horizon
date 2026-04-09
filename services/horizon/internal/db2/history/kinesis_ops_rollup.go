package history

import (
	"context"
	"time"

	"github.com/stellar/go/support/db"
)

// KinesisOpsRollup represents a row in the kinesis_ledger_ops_rollup table.
type KinesisOpsRollup struct {
	ID            int64     `db:"id"`
	Ledger        int32     `db:"ledger"`
	TxDate        time.Time `db:"tx_date"`
	ClosedAt      time.Time `db:"closed_at"`
	OperationType string    `db:"operation_type"`
	SourceAccount string    `db:"source_account"`
	DestAccount   string    `db:"dest_account"`
	TotalAmount   string    `db:"total_amount"`
}

// QKinesisOpsRollup defines the interface for creating a KinesisOpsRollupBatchInsertBuilder.
type QKinesisOpsRollup interface {
	NewKinesisOpsRollupBatchInsertBuilder(maxBatchSize int) KinesisOpsRollupBatchInsertBuilder
}

// KinesisOpsRollupBatchInsertBuilder is used to insert rows into the
// kinesis_ledger_ops_rollup table.
type KinesisOpsRollupBatchInsertBuilder interface {
	Add(ctx context.Context, entry KinesisOpsRollup) error
	Exec(ctx context.Context) error
}

// kinesisOpsRollupBatchInsertBuilder is a simple wrapper around db.BatchInsertBuilder.
type kinesisOpsRollupBatchInsertBuilder struct {
	builder db.BatchInsertBuilder
}

// NewKinesisOpsRollupBatchInsertBuilder constructs a new KinesisOpsRollupBatchInsertBuilder instance.
func (q *Q) NewKinesisOpsRollupBatchInsertBuilder(maxBatchSize int) KinesisOpsRollupBatchInsertBuilder {
	return &kinesisOpsRollupBatchInsertBuilder{
		builder: db.BatchInsertBuilder{
			Table:        q.GetTable("kinesis_ledger_ops_rollup"),
			MaxBatchSize: maxBatchSize,
			Suffix:       "ON CONFLICT (id) DO NOTHING",
		},
	}
}

// Add adds a new entry to the batch.
func (i *kinesisOpsRollupBatchInsertBuilder) Add(ctx context.Context, entry KinesisOpsRollup) error {
	return i.builder.RowStruct(ctx, entry)
}

// Exec flushes all remaining items in the batch.
func (i *kinesisOpsRollupBatchInsertBuilder) Exec(ctx context.Context) error {
	return i.builder.Exec(ctx)
}
