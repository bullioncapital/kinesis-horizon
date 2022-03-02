package history

import (
	"database/sql"
	"testing"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/guregu/null"
	"github.com/stellar/go/xdr"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/toid"
)

func TestTransactionQueries(t *testing.T) {
	tt := test.Start(t)
	test.ResetHorizonDB(t, tt.HorizonDB)
	tt.Scenario("base")
	defer tt.Finish()
	q := &Q{tt.HorizonSession()}

	// Test TransactionByHash
	var tx Transaction
	real := "2374e99349b9ef7dba9a5db3339b78fda8f34777b1af33ba468ad5c0df946d4d"
	err := q.TransactionByHash(tt.Ctx, &tx, real)
	tt.Assert.NoError(err)

	fake := "not_real"
	err = q.TransactionByHash(tt.Ctx, &tx, fake)
	tt.Assert.Equal(err, sql.ErrNoRows)
}

func TestTransactionByLiquidityPool(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	txIndex := int32(1)
	sequence := int32(56)
	txID := toid.New(sequence, int32(1), 0).ToInt64()

	// Insert a phony ledger
	ledgerCloseTime := time.Now().Unix()
	_, err := q.InsertLedger(tt.Ctx, xdr.LedgerHeaderHistoryEntry{
		Header: xdr.LedgerHeader{
			LedgerSeq: xdr.Uint32(sequence),
			ScpValue: xdr.StellarValue{
				CloseTime: xdr.TimePoint(ledgerCloseTime),
			},
		},
	}, 0, 0, 0, 0, 0)
	tt.Assert.NoError(err)

	// Insert a phony transaction
	transactionBuilder := q.NewTransactionBatchInsertBuilder(2)
	firstTransaction := buildLedgerTransaction(tt.T, testTransaction{
		index:         uint32(txIndex),
		envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
		resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
		feeChangesXDR: "AAAAAA==",
		metaXDR:       "AAAAAQAAAAAAAAAA",
		hash:          "19aaa18db88605aedec04659fb45e06f240b022eb2d429e05133e4d53cd945ba",
	})
	err = transactionBuilder.Add(tt.Ctx, firstTransaction, uint32(sequence))
	tt.Assert.NoError(err)
	err = transactionBuilder.Exec(tt.Ctx)
	tt.Assert.NoError(err)

	// Insert Liquidity Pool history
	liquidityPoolID := "a2f38836a839de008cf1d782c81f45e1253cc5d3dad9110b872965484fec0a49"
	toInternalID, err := q.CreateHistoryLiquidityPools(tt.Ctx, []string{liquidityPoolID}, 2)
	tt.Assert.NoError(err)
	lpTransactionBuilder := q.NewTransactionLiquidityPoolBatchInsertBuilder(2)
	tt.Assert.NoError(err)
	internalID, ok := toInternalID[liquidityPoolID]
	tt.Assert.True(ok)
	err = lpTransactionBuilder.Add(tt.Ctx, txID, internalID)
	tt.Assert.NoError(err)
	err = lpTransactionBuilder.Exec(tt.Ctx)
	tt.Assert.NoError(err)

	var records []Transaction
	err = q.Transactions().ForLiquidityPool(tt.Ctx, liquidityPoolID).Select(tt.Ctx, &records)
	tt.Assert.NoError(err)
	tt.Assert.Len(records, 1)

}

// TestTransactionSuccessfulOnly tests if default query returns successful
// transactions only.
// If it's not enclosed in brackets, it may return incorrect result when mixed
// with `ForAccount` or `ForLedger` filters.
func TestTransactionSuccessfulOnly(t *testing.T) {
	tt := test.Start(t)
	test.ResetHorizonDB(t, tt.HorizonDB)
	tt.Scenario("failed_transactions")
	defer tt.Finish()

	var transactions []Transaction

	q := &Q{tt.HorizonSession()}
	query := q.Transactions().
		ForAccount(tt.Ctx, "GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2")

	err := query.Select(tt.Ctx, &transactions)
	tt.Assert.NoError(err)

	tt.Assert.Equal(3, len(transactions))

	for _, transaction := range transactions {
		tt.Assert.True(transaction.Successful)
	}

	sql, _, err := query.sql.ToSql()
	tt.Assert.NoError(err)
	// Note: brackets around `(ht.successful = true OR ht.successful IS NULL)` are critical!
	tt.Assert.Contains(sql, "WHERE htp.history_account_id = ? AND (ht.successful = true OR ht.successful IS NULL)")
}

// TestTransactionIncludeFailed tests `IncludeFailed` method.
func TestTransactionIncludeFailed(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("failed_transactions")
	defer tt.Finish()

	var transactions []Transaction

	q := &Q{tt.HorizonSession()}
	query := q.Transactions().
		ForAccount(tt.Ctx, "GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2").
		IncludeFailed()

	err := query.Select(tt.Ctx, &transactions)
	tt.Assert.NoError(err)

	var failed, successful int
	for _, transaction := range transactions {
		if transaction.Successful {
			successful++
		} else {
			failed++
		}
	}

	tt.Assert.Equal(3, successful)
	tt.Assert.Equal(1, failed)

	sql, _, err := query.sql.ToSql()
	tt.Assert.NoError(err)
	tt.Assert.Equal("SELECT ht.id, ht.transaction_hash, ht.ledger_sequence, ht.application_order, ht.account, ht.account_muxed, ht.account_sequence, ht.max_fee, COALESCE(ht.fee_charged, ht.max_fee) as fee_charged, ht.operation_count, ht.tx_envelope, ht.tx_result, ht.tx_meta, ht.tx_fee_meta, ht.created_at, ht.updated_at, COALESCE(ht.successful, true) as successful, ht.signatures, ht.memo_type, ht.memo, time_bounds, hl.closed_at AS ledger_close_time, ht.inner_transaction_hash, ht.fee_account, ht.fee_account_muxed, ht.new_max_fee, ht.inner_signatures FROM history_transactions ht LEFT JOIN history_ledgers hl ON ht.ledger_sequence = hl.sequence JOIN history_transaction_participants htp ON htp.history_transaction_id = ht.id WHERE htp.history_account_id = ?", sql)
}

func TestExtraChecksTransactionSuccessfulTrueResultFalse(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("failed_transactions")
	defer tt.Finish()

	// successful `true` but tx result `false`
	_, err := tt.HorizonDB.Exec(
		`UPDATE history_transactions SET successful = true WHERE transaction_hash = 'aa168f12124b7c196c0adaee7c73a64d37f99428cacb59a91ff389626845e7cf'`,
	)
	tt.Require.NoError(err)

	var transactions []Transaction

	q := &Q{tt.HorizonSession()}
	query := q.Transactions().
		ForAccount(tt.Ctx, "GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2").
		IncludeFailed()

	err = query.Select(tt.Ctx, &transactions)
	tt.Assert.Error(err)
	tt.Assert.Contains(err.Error(), "Corrupted data! `successful=true` but returned transaction is not success")
}

func TestExtraChecksTransactionSuccessfulFalseResultTrue(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("failed_transactions")
	defer tt.Finish()

	// successful `false` but tx result `true`
	_, err := tt.HorizonDB.Exec(
		`UPDATE history_transactions SET successful = false WHERE transaction_hash = 'a2dabf4e9d1642722602272e178a37c973c9177b957da86192a99b3e9f3a9aa4'`,
	)
	tt.Require.NoError(err)

	var transactions []Transaction

	q := &Q{tt.HorizonSession()}
	query := q.Transactions().
		ForAccount(tt.Ctx, "GBXGQJWVLWOYHFLVTKWV5FGHA3LNYY2JQKM7OAJAUEQFU6LPCSEFVXON").
		IncludeFailed()

	err = query.Select(tt.Ctx, &transactions)
	tt.Assert.Error(err)
	tt.Assert.Contains(err.Error(), "Corrupted data! `successful=false` but returned transaction is success")
}

func TestInsertTransactionDoesNotAllowDuplicateIndex(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	sequence := uint32(123)
	insertBuilder := q.NewTransactionBatchInsertBuilder(0)

	firstTransaction := buildLedgerTransaction(tt.T, testTransaction{
		index:         1,
		envelopeXDR:   "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
		resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
		feeChangesXDR: "AAAAAA==",
		metaXDR:       "AAAAAQAAAAAAAAAA",
		hash:          "be1a785b12bcdc20534857353e3fa0c39e85b69064850bdb2cdafcf45f8244e6",
	})
	secondTransaction := buildLedgerTransaction(tt.T, testTransaction{
		index:         1,
		envelopeXDR:   "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
		resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
		feeChangesXDR: "AAAAAA==",
		metaXDR:       "AAAAAQAAAAAAAAAA",
		hash:          "be1a785b12bcdc20534857353e3fa0c39e85b69064850bdb2cdafcf45f8244e6",
	})

	tt.Assert.NoError(insertBuilder.Add(tt.Ctx, firstTransaction, sequence))
	tt.Assert.NoError(insertBuilder.Exec(tt.Ctx))

	tt.Assert.NoError(insertBuilder.Add(tt.Ctx, secondTransaction, sequence))
	tt.Assert.EqualError(
		insertBuilder.Exec(tt.Ctx),
		"error adding values while inserting to history_transactions: "+
			"exec failed: pq: duplicate key value violates unique constraint "+
			"\"hs_transaction_by_id\"",
	)

	ledger := Ledger{
		Sequence:                   int32(sequence),
		LedgerHash:                 "4db1e4f145e9ee75162040d26284795e0697e2e84084624e7c6c723ebbf80118",
		PreviousLedgerHash:         null.NewString("4b0b8bace3b2438b2404776ce57643966855487ba6384724a3c664c7aa4cd9e4", true),
		TotalOrderID:               TotalOrderID{toid.New(int32(69859), 0, 0).ToInt64()},
		ImporterVersion:            321,
		TransactionCount:           12,
		SuccessfulTransactionCount: new(int32),
		FailedTransactionCount:     new(int32),
		OperationCount:             23,
		TotalCoins:                 23451,
		FeePool:                    213,
		BaseReserve:                687,
		MaxTxSetSize:               345,
		ProtocolVersion:            12,
		BaseFee:                    100,
		ClosedAt:                   time.Now().UTC().Truncate(time.Second),
		LedgerHeaderXDR:            null.NewString("temp", true),
	}
	*ledger.SuccessfulTransactionCount = 12
	*ledger.FailedTransactionCount = 3
	_, err := q.Exec(tt.Ctx, sq.Insert("history_ledgers").SetMap(ledgerToMap(ledger)))
	tt.Assert.NoError(err)

	var transactions []Transaction
	tt.Assert.NoError(q.Transactions().Select(tt.Ctx, &transactions))
	tt.Assert.Len(transactions, 1)
	tt.Assert.Equal(
		"be1a785b12bcdc20534857353e3fa0c39e85b69064850bdb2cdafcf45f8244e6",
		transactions[0].TransactionHash,
	)
}

func TestInsertTransaction(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	sequence := uint32(123)
	ledger := Ledger{
		Sequence:                   int32(sequence),
		LedgerHash:                 "4db1e4f145e9ee75162040d26284795e0697e2e84084624e7c6c723ebbf80118",
		PreviousLedgerHash:         null.NewString("4b0b8bace3b2438b2404776ce57643966855487ba6384724a3c664c7aa4cd9e4", true),
		TotalOrderID:               TotalOrderID{toid.New(int32(69859), 0, 0).ToInt64()},
		ImporterVersion:            321,
		TransactionCount:           12,
		SuccessfulTransactionCount: new(int32),
		FailedTransactionCount:     new(int32),
		OperationCount:             23,
		TotalCoins:                 23451,
		FeePool:                    213,
		BaseReserve:                687,
		MaxTxSetSize:               345,
		ProtocolVersion:            12,
		BaseFee:                    100,
		ClosedAt:                   time.Now().UTC().Truncate(time.Second),
		LedgerHeaderXDR:            null.NewString("temp", true),
	}
	*ledger.SuccessfulTransactionCount = 12
	*ledger.FailedTransactionCount = 3
	_, err := q.Exec(tt.Ctx, sq.Insert("history_ledgers").SetMap(ledgerToMap(ledger)))
	tt.Assert.NoError(err)

	insertBuilder := q.NewTransactionBatchInsertBuilder(0)

	success := true

	emptySignatures := []string{}
	var nullSignatures []string

	nullTimeBounds := TimeBounds{Null: true}

	infiniteTimeBounds := TimeBounds{Lower: null.IntFrom(0)}
	timeBoundWithMin := TimeBounds{Lower: null.IntFrom(1576195867)}
	timeBoundWithMax := TimeBounds{Lower: null.IntFrom(0), Upper: null.IntFrom(1576195867)}
	timeboundsWithMinAndMax := TimeBounds{Lower: null.IntFrom(1576095867), Upper: null.IntFrom(1576195867)}

	withMultipleSignatures := []string{
		"MID8kIOLP/yEymCyhU7A/YeVpnVTDzAqszWtv8c+/qAw542BaKWxCJxl/jsggY0mF+SR8X0bvWXvPBgyYcDZDw==",
		"J0J8qTsKREW29GAmZMXXBTVkYKkGbOk1AUPUalbIiDdDjd8mpIIdMStqo9w+k5A8UKRTm/iO2V/riQ14CF9IAg==",
	}

	withSingleSignature := []string{
		"MID8kIOLP/yEymCyhU7A/YeVpnVTDzAqszWtv8c+/qAw542BaKWxCJxl/jsggY0mF+SR8X0bvWXvPBgyYcDZDw==",
	}

	for _, testCase := range []struct {
		name     string
		toInsert ingest.LedgerTransaction
		expected Transaction
	}{
		{
			"successful transaction without signatures",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "be1a785b12bcdc20534857353e3fa0c39e85b69064850bdb2cdafcf45f8244e6",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "be1a785b12bcdc20534857353e3fa0c39e85b69064850bdb2cdafcf45f8244e6",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  "78621794419880145",
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					Successful:       success,
					TimeBounds:       nullTimeBounds,
				},
			},
		},
		{
			"successful transaction with multiple signatures",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "be1a785b12bcdc20534857353e3fa0c39e85b69064850bdb2cdafcf45f8244e6",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "be1a785b12bcdc20534857353e3fa0c39e85b69064850bdb2cdafcf45f8244e6",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  "78621794419880145",
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       withMultipleSignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					TimeBounds:       nullTimeBounds,
					Successful:       success,
				},
			},
		},
		{
			"failed transaction",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
				resultXDR:     "AAAAAAAAAHv////6AAAAAA==",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "be1a785b12bcdc20534857353e3fa0c39e85b69064850bdb2cdafcf45f8244e6",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "be1a785b12bcdc20534857353e3fa0c39e85b69064850bdb2cdafcf45f8244e6",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  "78621794419880145",
					MaxFee:           200,
					FeeCharged:       123,
					OperationCount:   1,
					TxEnvelope:       "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
					TxResult:         "AAAAAAAAAHv////6AAAAAA==",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       withSingleSignature,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					TimeBounds:       nullTimeBounds,
					Successful:       false,
				},
			},
		},
		{
			"transaction with 64 bit fee charged",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAACVAvkAARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAJdGVzdCBtZW1vAAAAAAAAAQAAAAAAAAALARdSGwAAV+EAAAAAAAAAAA==",
				resultXDR:     "AAAAAgAAAAAAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "7ad466666791eedcf9e31256c793632d832e9ff98aa22e0ae1f42d5377482330",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "7ad466666791eedcf9e31256c793632d832e9ff98aa22e0ae1f42d5377482330",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  "78621794419880145",
					// set max fee to a value larger than MAX_INT32 but less than or equal to MAX_UINT32
					MaxFee:          2500000000,
					FeeCharged:      int64(1 << 33),
					OperationCount:  1,
					TxEnvelope:      "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAACVAvkAARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAJdGVzdCBtZW1vAAAAAAAAAQAAAAAAAAALARdSGwAAV+EAAAAAAAAAAA==",
					TxResult:        "AAAAAgAAAAAAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:       "AAAAAA==",
					TxMeta:          "AAAAAQAAAAAAAAAA",
					Signatures:      emptySignatures,
					InnerSignatures: nullSignatures,
					MemoType:        "text",
					Memo:            null.NewString("test memo", true),
					TimeBounds:      infiniteTimeBounds,
					Successful:      success,
				},
			},
		},
		{
			"transaction with text memo",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAJdGVzdCBtZW1vAAAAAAAAAQAAAAAAAAALARdSGwAAV+EAAAAAAAAAAA==",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "e949d96bc5d720a43afa4a9d400ea23938d4fe31b30932fda46b4549fdb2e22a",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "e949d96bc5d720a43afa4a9d400ea23938d4fe31b30932fda46b4549fdb2e22a",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  "78621794419880145",
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAJdGVzdCBtZW1vAAAAAAAAAQAAAAAAAAALARdSGwAAV+EAAAAAAAAAAA==",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "text",
					Memo:             null.NewString("test memo", true),
					TimeBounds:       infiniteTimeBounds,
					Successful:       success,
				},
			},
		},
		{
			"transaction with id memo",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "be1a785b12bcdc20534857353e3fa0c39e85b69064850bdb2cdafcf45f8244e6",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "be1a785b12bcdc20534857353e3fa0c39e85b69064850bdb2cdafcf45f8244e6",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  "78621794419880145",
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "id",
					Memo:             null.NewString("123", true),
					TimeBounds:       nullTimeBounds,
					Successful:       success,
				},
			},
		},
		{
			"transaction with hash memo",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAADfi3vINWiGla+KkV7ZI9wLuGviJ099leQ6SoFCB6fq/EAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "8aba253c2adc4083f35830cec37d9c6226b757ab3a94f15a12d6c22973fd5f3f",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "8aba253c2adc4083f35830cec37d9c6226b757ab3a94f15a12d6c22973fd5f3f",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  "78621794419880145",
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAADfi3vINWiGla+KkV7ZI9wLuGviJ099leQ6SoFCB6fq/EAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "hash",
					Memo:             null.NewString("fi3vINWiGla+KkV7ZI9wLuGviJ099leQ6SoFCB6fq/E=", true),
					TimeBounds:       infiniteTimeBounds,
					Successful:       success,
				},
			},
		},
		{
			"transaction with return memo",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAEzdjArlILa/LNv7o7lo/qv5+fVVPNl0yPgZQWB6u+gL4AAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "9e932a86cea43239aed62d8cd3b6b5ad2d8eb0a63247355e4ab44f2994209f2a",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "9e932a86cea43239aed62d8cd3b6b5ad2d8eb0a63247355e4ab44f2994209f2a",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  "78621794419880145",
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAyAEXUhsAADDRAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAEzdjArlILa/LNv7o7lo/qv5+fVVPNl0yPgZQWB6u+gL4AAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "return",
					Memo:             null.NewString("zdjArlILa/LNv7o7lo/qv5+fVVPNl0yPgZQWB6u+gL4=", true),
					TimeBounds:       infiniteTimeBounds,
					Successful:       success,
				},
			},
		},
		{
			"transaction with min time bound",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAABkAAAAAAAB4kAAAAABAAAAAF3y1xsAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsAAAAAABLWhwAAAAAAAAAA",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "f8ebf7f84c63427fa945eccc5cf64462cfd0ec2d2e9de9ec65e1cf88f9a74a96",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{

					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "f8ebf7f84c63427fa945eccc5cf64462cfd0ec2d2e9de9ec65e1cf88f9a74a96",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  "123456",
					MaxFee:           100,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAABkAAAAAAAB4kAAAAABAAAAAF3y1xsAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsAAAAAABLWhwAAAAAAAAAA",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					TimeBounds:       timeBoundWithMin,
					Successful:       success,
				},
			},
		},
		{
			"transaction with max time bound",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "5858709ae02992431f98f7410be3d3586c5a83e9e7105d270ce1ddc2cf45358a",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "5858709ae02992431f98f7410be3d3586c5a83e9e7105d270ce1ddc2cf45358a",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  "123456",
					MaxFee:           100,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					TimeBounds:       timeBoundWithMax,
					Successful:       success,
				},
			},
		},
		{
			"transaction with min and max time bound",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAABkAAAAAAAB4kAAAAABAAAAAF3xUHsAAAAAXfLXGwAAAAAAAAABAAAAAAAAAAsAAAAAABLWhwAAAAAAAAAA",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "a1161d86cc77bb199e83c079373da58d5710c2ccc91c62e0dd26ad70f104f5cb",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "a1161d86cc77bb199e83c079373da58d5710c2ccc91c62e0dd26ad70f104f5cb",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  "123456",
					MaxFee:           100,
					FeeCharged:       300,
					OperationCount:   1,
					TxEnvelope:       "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAABkAAAAAAAB4kAAAAABAAAAAF3xUHsAAAAAXfLXGwAAAAAAAAABAAAAAAAAAAsAAAAAABLWhwAAAAAAAAAA",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					TimeBounds:       timeboundsWithMinAndMax,
					Successful:       success,
				},
			},
		},
		{
			"transaction with multiple operations",
			buildLedgerTransaction(tt.T, testTransaction{
				index:         1,
				envelopeXDR:   "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIAAAAAAAB4kAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAsAAAAAABLWhwAAAAAAAAALAAAAAAAS1ogAAAAAAAAAAA==",
				resultXDR:     "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
				feeChangesXDR: "AAAAAA==",
				metaXDR:       "AAAAAQAAAAAAAAAA",
				hash:          "71fea76a4f4c3a658fb757e62df2e4ffc6a997a82add53f439fbf8d3c99aabdd",
			}),
			Transaction{
				LedgerCloseTime: ledger.ClosedAt,
				TransactionWithoutLedger: TransactionWithoutLedger{
					TotalOrderID:     TotalOrderID{528280981504},
					TransactionHash:  "71fea76a4f4c3a658fb757e62df2e4ffc6a997a82add53f439fbf8d3c99aabdd",
					LedgerSequence:   ledger.Sequence,
					ApplicationOrder: 1,
					Account:          "GAUJETIZVEP2NRYLUESJ3LS66NVCEGMON4UDCBCSBEVPIID773P2W6AY",
					AccountSequence:  "123456",
					MaxFee:           200,
					FeeCharged:       300,
					OperationCount:   2,
					TxEnvelope:       "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIAAAAAAAB4kAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAsAAAAAABLWhwAAAAAAAAALAAAAAAAS1ogAAAAAAAAAAA==",
					TxResult:         "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA=",
					TxFeeMeta:        "AAAAAA==",
					TxMeta:           "AAAAAQAAAAAAAAAA",
					Signatures:       emptySignatures,
					InnerSignatures:  nullSignatures,
					MemoType:         "none",
					Memo:             null.NewString("", false),
					TimeBounds:       nullTimeBounds,
					Successful:       success,
				},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			tt.Assert.NoError(insertBuilder.Add(tt.Ctx, testCase.toInsert, sequence))
			tt.Assert.NoError(insertBuilder.Exec(tt.Ctx))

			var transactions []Transaction
			tt.Assert.NoError(q.Transactions().IncludeFailed().Select(tt.Ctx, &transactions))
			tt.Assert.Len(transactions, 1)

			transaction := transactions[0]

			// ignore created time and updated time
			transaction.CreatedAt = testCase.expected.CreatedAt
			transaction.UpdatedAt = testCase.expected.UpdatedAt

			// compare ClosedAt separately because reflect.DeepEqual does not handle time.Time
			closedAt := transaction.LedgerCloseTime
			transaction.LedgerCloseTime = testCase.expected.LedgerCloseTime

			tt.Assert.True(closedAt.Equal(testCase.expected.LedgerCloseTime))
			tt.Assert.Equal(transaction, testCase.expected)

			_, err = q.Exec(tt.Ctx, sq.Delete("history_transactions"))
			tt.Assert.NoError(err)
		})
	}
}

func TestFetchFeeBumpTransaction(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	fixture := FeeBumpScenario(tt, q, true)

	var byOuterhash, byInnerHash Transaction
	tt.Assert.NoError(q.TransactionByHash(tt.Ctx, &byOuterhash, fixture.OuterHash))
	tt.Assert.NoError(q.TransactionByHash(tt.Ctx, &byInnerHash, fixture.InnerHash))

	tt.Assert.Equal(byOuterhash, byInnerHash)
	tt.Assert.Equal(byOuterhash, fixture.Transaction)

	outerOps, outerTransactions, err := q.Operations().IncludeTransactions().
		ForTransaction(tt.Ctx, fixture.OuterHash).Fetch(tt.Ctx)
	tt.Assert.NoError(err)
	tt.Assert.Len(outerTransactions, 1)
	tt.Assert.Len(outerOps, 1)

	innerOps, innerTransactions, err := q.Operations().IncludeTransactions().
		ForTransaction(tt.Ctx, fixture.InnerHash).Fetch(tt.Ctx)
	tt.Assert.NoError(err)
	tt.Assert.Len(innerTransactions, 1)
	tt.Assert.Equal(innerOps, outerOps)

	for _, tx := range append(outerTransactions, innerTransactions...) {
		tt.Assert.True(byOuterhash.CreatedAt.Equal(tx.CreatedAt))
		tt.Assert.True(byOuterhash.LedgerCloseTime.Equal(tx.LedgerCloseTime))
		byOuterhash.CreatedAt = tx.CreatedAt
		byOuterhash.LedgerCloseTime = tx.LedgerCloseTime
		tt.Assert.Equal(byOuterhash, byInnerHash)
	}

	var outerEffects, innerEffects []Effect
	err = q.Effects().ForTransaction(tt.Ctx, fixture.OuterHash).Select(tt.Ctx, &outerEffects)
	tt.Assert.NoError(err)
	tt.Assert.Len(outerEffects, 1)

	err = q.Effects().ForTransaction(tt.Ctx, fixture.InnerHash).Select(tt.Ctx, &innerEffects)
	tt.Assert.NoError(err)
	tt.Assert.Equal(outerEffects, innerEffects)
}
