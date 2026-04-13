package processors

import (
	"context"
	"time"

	"github.com/stellar/go/amount"
	"github.com/stellar/go/ingest"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

// KinesisOpsRollupProcessor ingests mint and redemption operations into
// the kinesis_ledger_ops_rollup table. It filters operations at the Go
// ingestion level so that only mint and redemption rows are stored.
//
// Mint: create_account or native-asset payment from EmissionAccount to non-RootAccount
// Redemption: native-asset payment from HotWalletAccount to EmissionAccount or RootAccount
type KinesisOpsRollupProcessor struct {
	rollupQ  history.QKinesisOpsRollup
	batch    history.KinesisOpsRollupBatchInsertBuilder
	ledger   xdr.LedgerHeaderHistoryEntry
	sequence uint32

	rootAccount      string
	emissionAccount  string
	hotWalletAccount string
	feepoolAccount   string
}

// NewKinesisOpsRollupProcessor creates a new processor. The networkPassphrase is
// used to derive the well-known Kinesis accounts (root, emission, hot-wallet,
// fee-pool) following the same logic as KinesisCoinInCirculationBaseQuery.PopulateAccounts.
func NewKinesisOpsRollupProcessor(
	rollupQ history.QKinesisOpsRollup,
	ledger xdr.LedgerHeaderHistoryEntry,
	networkPassphrase string,
) *KinesisOpsRollupProcessor {
	sequence := uint32(ledger.Header.LedgerSeq)
	return &KinesisOpsRollupProcessor{
		rollupQ:          rollupQ,
		batch:            rollupQ.NewKinesisOpsRollupBatchInsertBuilder(maxBatchSize),
		ledger:           ledger,
		sequence:         sequence,
		rootAccount:      keypair.Root(networkPassphrase).Address(),
		emissionAccount:  keypair.Root(networkPassphrase + "emission").Address(),
		hotWalletAccount: keypair.Root(networkPassphrase + "exchange").Address(),
		feepoolAccount:   keypair.Root(networkPassphrase + "feepool").Address(),
	}
}

// ProcessTransaction examines each operation in a successful transaction and,
// if it matches the mint or redemption criteria, adds it to the batch.
func (p *KinesisOpsRollupProcessor) ProcessTransaction(ctx context.Context, transaction ingest.LedgerTransaction) error {
	// Only successful transactions produce real effects.
	if !transaction.Result.Successful() {
		return nil
	}

	closedAt := time.Unix(int64(p.ledger.Header.ScpValue.CloseTime), 0).UTC()
	txDate := closedAt.Truncate(24 * time.Hour) // tx_date is DATE (no time component)

	for i, op := range transaction.Envelope.Operations() {
		opType := op.Body.Type

		// We only care about create_account (0) and payment (1).
		if opType != xdr.OperationTypeCreateAccount && opType != xdr.OperationTypePayment {
			continue
		}

		var sourceAccount, destAccount, operationTypeName string
		var totalAmount string

		switch opType {
		case xdr.OperationTypeCreateAccount:
			createOp := op.Body.MustCreateAccountOp()
			sourceAccount = p.resolveSourceAccount(&op, &transaction)
			destAccount = createOp.Destination.Address()
			totalAmount = amount.String(xdr.Int64(createOp.StartingBalance))
			operationTypeName = "create_account"

		case xdr.OperationTypePayment:
			payOp := op.Body.MustPaymentOp()
			// Only native-asset payments qualify as mint or redemption.
			if payOp.Asset.Type != xdr.AssetTypeAssetTypeNative {
				continue
			}
			sourceAccount = p.resolveSourceAccount(&op, &transaction)
			destAccount = payOp.Destination.ToAccountId().Address()
			totalAmount = amount.String(payOp.Amount)
			operationTypeName = "payment"
		}

		// Apply the mint/redemption filter.
		if !p.isMintOrRedemption(sourceAccount, destAccount) {
			continue
		}

		opID := toid.New(
			int32(p.sequence),
			int32(transaction.Index),
			int32(i+1),
		).ToInt64()

		entry := history.KinesisOpsRollup{
			ID:            opID,
			Ledger:        int32(p.sequence),
			TxDate:        txDate,
			ClosedAt:      closedAt,
			OperationType: operationTypeName,
			SourceAccount: sourceAccount,
			DestAccount:   destAccount,
			TotalAmount:   totalAmount,
		}

		if err := p.batch.Add(ctx, entry); err != nil {
			return errors.Wrap(err, "error adding kinesis ops rollup entry")
		}
	}

	return nil
}

// Commit flushes the remaining rows to the database.
func (p *KinesisOpsRollupProcessor) Commit(ctx context.Context) error {
	return p.batch.Exec(ctx)
}

// isMintOrRedemption returns true if the operation matches mint or redemption
// criteria. The caller is responsible for ensuring the operation involves only
// native assets before calling this function.
//
//   - Mint: source is EmissionAccount AND dest is NOT RootAccount
//   - Redemption: source is HotWalletAccount AND dest is EmissionAccount OR RootAccount
func (p *KinesisOpsRollupProcessor) isMintOrRedemption(sourceAccount, destAccount string) bool {
	// Exclude any operations involving the feepool account.
	if sourceAccount == p.feepoolAccount || destAccount == p.feepoolAccount {
		return false
	}

	// Mint: emission -> non-root
	if sourceAccount == p.emissionAccount && destAccount != p.rootAccount {
		return true
	}

	// Redemption: hot-wallet -> emission or root
	if sourceAccount == p.hotWalletAccount &&
		(destAccount == p.emissionAccount || destAccount == p.rootAccount) {
		return true
	}

	return false
}

// resolveSourceAccount resolves the effective source account for an operation,
// falling back to the transaction source if the operation doesn't override it.
func (p *KinesisOpsRollupProcessor) resolveSourceAccount(op *xdr.Operation, tx *ingest.LedgerTransaction) string {
	if op.SourceAccount != nil {
		return op.SourceAccount.ToAccountId().Address()
	}
	return tx.Envelope.SourceAccount().ToAccountId().Address()
}
