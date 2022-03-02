package ingest

import (
	"context"
	"io"
	"reflect"
	"testing"

	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/network"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/ingest/processors"
	"github.com/stellar/go/xdr"
)

func TestProcessorRunnerRunHistoryArchiveIngestionGenesis(t *testing.T) {
	ctx := context.Background()
	maxBatchSize := 100000

	q := &mockDBQ{}

	// Batches
	mockAccountDataBatchInsertBuilder := &history.MockAccountDataBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockAccountDataBatchInsertBuilder)
	mockAccountDataBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	q.MockQData.On("NewAccountDataBatchInsertBuilder", maxBatchSize).
		Return(mockAccountDataBatchInsertBuilder).Once()

	mockClaimableBalancesBatchInsertBuilder := &history.MockClaimableBalancesBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockClaimableBalancesBatchInsertBuilder)
	mockClaimableBalancesBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	q.MockQClaimableBalances.On("NewClaimableBalancesBatchInsertBuilder", maxBatchSize).
		Return(mockClaimableBalancesBatchInsertBuilder).Once()

	q.MockQAccounts.On("UpsertAccounts", ctx, []xdr.LedgerEntry{
		{
			LastModifiedLedgerSeq: 1,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId:  xdr.MustAddress("GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7"),
					Balance:    xdr.Int64(1000000000000000000),
					SeqNum:     xdr.SequenceNumber(0),
					Thresholds: [4]byte{1, 0, 0, 0},
				},
			},
		},
	}).Return(nil).Once()

	mockAccountSignersBatchInsertBuilder := &history.MockAccountSignersBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockAccountSignersBatchInsertBuilder)
	mockAccountSignersBatchInsertBuilder.On("Add", ctx, history.AccountSigner{
		Account: "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7",
		Signer:  "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7",
		Weight:  1,
		Sponsor: null.String{},
	}).Return(nil).Once()
	mockAccountSignersBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	q.MockQSigners.On("NewAccountSignersBatchInsertBuilder", maxBatchSize).
		Return(mockAccountSignersBatchInsertBuilder).Once()

	q.MockQAssetStats.On("InsertAssetStats", ctx, []history.ExpAssetStat{}, 100000).
		Return(nil)

	runner := ProcessorRunner{
		ctx: ctx,
		config: Config{
			NetworkPassphrase: network.PublicNetworkPassphrase,
		},
		historyQ: q,
	}

	_, err := runner.RunGenesisStateIngestion()
	assert.NoError(t, err)
}

func TestProcessorRunnerRunHistoryArchiveIngestionHistoryArchive(t *testing.T) {
	ctx := context.Background()
	maxBatchSize := 100000

	config := Config{
		NetworkPassphrase: network.PublicNetworkPassphrase,
	}

	q := &mockDBQ{}
	defer mock.AssertExpectationsForObjects(t, q)
	historyAdapter := &mockHistoryArchiveAdapter{}
	defer mock.AssertExpectationsForObjects(t, historyAdapter)

	bucketListHash := xdr.Hash([32]byte{0, 1, 2})
	historyAdapter.On("BucketListHash", uint32(63)).Return(bucketListHash, nil).Once()

	m := &ingest.MockChangeReader{}
	m.On("Read").Return(ingest.GenesisChange(network.PublicNetworkPassphrase), nil).Once()
	m.On("Read").Return(ingest.Change{}, io.EOF).Once()
	m.On("Close").Return(nil).Once()

	historyAdapter.
		On("GetState", ctx, uint32(63)).
		Return(
			m,
			nil,
		).Once()

	// Batches
	mockAccountDataBatchInsertBuilder := &history.MockAccountDataBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockAccountDataBatchInsertBuilder)
	mockAccountDataBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	q.MockQData.On("NewAccountDataBatchInsertBuilder", maxBatchSize).
		Return(mockAccountDataBatchInsertBuilder).Once()

	mockClaimableBalancesBatchInsertBuilder := &history.MockClaimableBalancesBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockClaimableBalancesBatchInsertBuilder)
	mockClaimableBalancesBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	q.MockQClaimableBalances.On("NewClaimableBalancesBatchInsertBuilder", maxBatchSize).
		Return(mockClaimableBalancesBatchInsertBuilder).Once()

	q.MockQAccounts.On("UpsertAccounts", ctx, []xdr.LedgerEntry{
		{
			LastModifiedLedgerSeq: 1,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId:  xdr.MustAddress("GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7"),
					Balance:    xdr.Int64(1000000000000000000),
					SeqNum:     xdr.SequenceNumber(0),
					Thresholds: [4]byte{1, 0, 0, 0},
				},
			},
		},
	}).Return(nil).Once()

	mockAccountSignersBatchInsertBuilder := &history.MockAccountSignersBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockAccountSignersBatchInsertBuilder)
	mockAccountSignersBatchInsertBuilder.On("Add", ctx, history.AccountSigner{
		Account: "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7",
		Signer:  "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7",
		Weight:  1,
	}).Return(nil).Once()
	mockAccountSignersBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	q.MockQSigners.On("NewAccountSignersBatchInsertBuilder", maxBatchSize).
		Return(mockAccountSignersBatchInsertBuilder).Once()

	q.MockQAssetStats.On("InsertAssetStats", ctx, []history.ExpAssetStat{}, 100000).
		Return(nil)

	runner := ProcessorRunner{
		ctx:            ctx,
		config:         config,
		historyQ:       q,
		historyAdapter: historyAdapter,
	}

	_, err := runner.RunHistoryArchiveIngestion(63, MaxSupportedProtocolVersion, bucketListHash)
	assert.NoError(t, err)
}

func TestProcessorRunnerRunHistoryArchiveIngestionProtocolVersionNotSupported(t *testing.T) {
	ctx := context.Background()
	maxBatchSize := 100000

	config := Config{
		NetworkPassphrase: network.PublicNetworkPassphrase,
	}

	q := &mockDBQ{}
	defer mock.AssertExpectationsForObjects(t, q)
	historyAdapter := &mockHistoryArchiveAdapter{}
	defer mock.AssertExpectationsForObjects(t, historyAdapter)

	// Batches
	mockAccountDataBatchInsertBuilder := &history.MockAccountDataBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockAccountDataBatchInsertBuilder)
	q.MockQData.On("NewAccountDataBatchInsertBuilder", maxBatchSize).
		Return(mockAccountDataBatchInsertBuilder).Once()

	mockClaimableBalancesBatchInsertBuilder := &history.MockClaimableBalancesBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockClaimableBalancesBatchInsertBuilder)
	q.MockQClaimableBalances.On("NewClaimableBalancesBatchInsertBuilder", maxBatchSize).
		Return(mockClaimableBalancesBatchInsertBuilder).Once()

	mockAccountSignersBatchInsertBuilder := &history.MockAccountSignersBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockAccountSignersBatchInsertBuilder)
	q.MockQSigners.On("NewAccountSignersBatchInsertBuilder", maxBatchSize).
		Return(mockAccountSignersBatchInsertBuilder).Once()

	q.MockQAssetStats.On("InsertAssetStats", ctx, []history.ExpAssetStat{}, 100000).
		Return(nil)

	runner := ProcessorRunner{
		ctx:            ctx,
		config:         config,
		historyQ:       q,
		historyAdapter: historyAdapter,
	}

	_, err := runner.RunHistoryArchiveIngestion(100, 200, xdr.Hash{})
	assert.EqualError(t, err, "Error while checking for supported protocol version: This Horizon version does not support protocol version 200. The latest supported protocol version is 17. Please upgrade to the latest Horizon version.")
}

func TestProcessorRunnerBuildChangeProcessor(t *testing.T) {
	ctx := context.Background()
	maxBatchSize := 100000

	q := &mockDBQ{}
	defer mock.AssertExpectationsForObjects(t, q)

	// Twice = checking ledgerSource and historyArchiveSource
	q.MockQData.On("NewAccountDataBatchInsertBuilder", maxBatchSize).
		Return(&history.MockAccountDataBatchInsertBuilder{}).Twice()
	q.MockQSigners.On("NewAccountSignersBatchInsertBuilder", maxBatchSize).
		Return(&history.MockAccountSignersBatchInsertBuilder{}).Twice()

	runner := ProcessorRunner{
		ctx:      ctx,
		historyQ: q,
	}

	stats := &ingest.StatsChangeProcessor{}
	processor := runner.buildChangeProcessor(stats, ledgerSource, 123)
	assert.IsType(t, &groupChangeProcessors{}, processor)

	assert.IsType(t, &statsChangeProcessor{}, processor.processors[0])
	assert.IsType(t, &processors.AccountDataProcessor{}, processor.processors[1])
	assert.IsType(t, &processors.AccountsProcessor{}, processor.processors[2])
	assert.IsType(t, &processors.OffersProcessor{}, processor.processors[3])
	assert.IsType(t, &processors.AssetStatsProcessor{}, processor.processors[4])
	assert.True(t, reflect.ValueOf(processor.processors[4]).
		Elem().FieldByName("useLedgerEntryCache").Bool())
	assert.IsType(t, &processors.SignersProcessor{}, processor.processors[5])
	assert.True(t, reflect.ValueOf(processor.processors[5]).
		Elem().FieldByName("useLedgerEntryCache").Bool())
	assert.IsType(t, &processors.TrustLinesProcessor{}, processor.processors[6])

	runner = ProcessorRunner{
		ctx:      ctx,
		historyQ: q,
	}

	processor = runner.buildChangeProcessor(stats, historyArchiveSource, 456)
	assert.IsType(t, &groupChangeProcessors{}, processor)

	assert.IsType(t, &statsChangeProcessor{}, processor.processors[0])
	assert.IsType(t, &processors.AccountDataProcessor{}, processor.processors[1])
	assert.IsType(t, &processors.AccountsProcessor{}, processor.processors[2])
	assert.IsType(t, &processors.OffersProcessor{}, processor.processors[3])
	assert.IsType(t, &processors.AssetStatsProcessor{}, processor.processors[4])
	assert.False(t, reflect.ValueOf(processor.processors[4]).
		Elem().FieldByName("useLedgerEntryCache").Bool())
	assert.IsType(t, &processors.SignersProcessor{}, processor.processors[5])
	assert.False(t, reflect.ValueOf(processor.processors[5]).
		Elem().FieldByName("useLedgerEntryCache").Bool())
	assert.IsType(t, &processors.TrustLinesProcessor{}, processor.processors[6])
}

func TestProcessorRunnerBuildTransactionProcessor(t *testing.T) {
	ctx := context.Background()
	maxBatchSize := 100000

	q := &mockDBQ{}
	defer mock.AssertExpectationsForObjects(t, q)

	q.MockQOperations.On("NewOperationBatchInsertBuilder", maxBatchSize).
		Return(&history.MockOperationsBatchInsertBuilder{}).Twice() // Twice = with/without failed
	q.MockQTransactions.On("NewTransactionBatchInsertBuilder", maxBatchSize).
		Return(&history.MockTransactionsBatchInsertBuilder{}).Twice()

	runner := ProcessorRunner{
		ctx:      ctx,
		config:   Config{},
		historyQ: q,
	}

	stats := &processors.StatsLedgerTransactionProcessor{}
	ledger := xdr.LedgerHeaderHistoryEntry{}
	processor := runner.buildTransactionProcessor(stats, ledger)
	assert.IsType(t, &groupTransactionProcessors{}, processor)

	assert.IsType(t, &statsLedgerTransactionProcessor{}, processor.processors[0])
	assert.IsType(t, &processors.EffectProcessor{}, processor.processors[1])
	assert.IsType(t, &processors.LedgersProcessor{}, processor.processors[2])
	assert.IsType(t, &processors.OperationProcessor{}, processor.processors[3])
	assert.IsType(t, &processors.TradeProcessor{}, processor.processors[4])
	assert.IsType(t, &processors.ParticipantsProcessor{}, processor.processors[5])
	assert.IsType(t, &processors.TransactionProcessor{}, processor.processors[6])
}

func TestProcessorRunnerRunAllProcessorsOnLedger(t *testing.T) {
	ctx := context.Background()
	maxBatchSize := 100000

	config := Config{
		NetworkPassphrase: network.PublicNetworkPassphrase,
	}

	q := &mockDBQ{}
	defer mock.AssertExpectationsForObjects(t, q)

	ledger := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					BucketListHash: xdr.Hash([32]byte{0, 1, 2}),
				},
			},
		},
	}

	// Batches
	mockAccountDataBatchInsertBuilder := &history.MockAccountDataBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockAccountDataBatchInsertBuilder)
	mockAccountDataBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	q.MockQData.On("NewAccountDataBatchInsertBuilder", maxBatchSize).
		Return(mockAccountDataBatchInsertBuilder).Once()

	mockAccountSignersBatchInsertBuilder := &history.MockAccountSignersBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockAccountSignersBatchInsertBuilder)
	q.MockQSigners.On("NewAccountSignersBatchInsertBuilder", maxBatchSize).
		Return(mockAccountSignersBatchInsertBuilder).Once()

	mockOperationsBatchInsertBuilder := &history.MockOperationsBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockOperationsBatchInsertBuilder)
	mockOperationsBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	q.MockQOperations.On("NewOperationBatchInsertBuilder", maxBatchSize).
		Return(mockOperationsBatchInsertBuilder).Twice()

	mockTransactionsBatchInsertBuilder := &history.MockTransactionsBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockTransactionsBatchInsertBuilder)
	mockTransactionsBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	q.MockQTransactions.On("NewTransactionBatchInsertBuilder", maxBatchSize).
		Return(mockTransactionsBatchInsertBuilder).Twice()

	mockClaimableBalancesBatchInsertBuilder := &history.MockClaimableBalancesBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockClaimableBalancesBatchInsertBuilder)
	mockClaimableBalancesBatchInsertBuilder.On("Exec", ctx).Return(nil).Once()
	q.MockQClaimableBalances.On("NewClaimableBalancesBatchInsertBuilder", maxBatchSize).
		Return(mockClaimableBalancesBatchInsertBuilder).Once()

	q.MockQLedgers.On("InsertLedger", ctx, ledger.V0.LedgerHeader, 0, 0, 0, 0, CurrentVersion).
		Return(int64(1), nil).Once()

	runner := ProcessorRunner{
		ctx:      ctx,
		config:   config,
		historyQ: q,
	}

	_, _, _, _, err := runner.RunAllProcessorsOnLedger(ledger)
	assert.NoError(t, err)
}

func TestProcessorRunnerRunAllProcessorsOnLedgerProtocolVersionNotSupported(t *testing.T) {
	ctx := context.Background()
	maxBatchSize := 100000

	config := Config{
		NetworkPassphrase: network.PublicNetworkPassphrase,
	}

	q := &mockDBQ{}
	defer mock.AssertExpectationsForObjects(t, q)

	ledger := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerVersion: 200,
				},
			},
		},
	}

	// Batches
	mockAccountDataBatchInsertBuilder := &history.MockAccountDataBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockAccountDataBatchInsertBuilder)
	q.MockQData.On("NewAccountDataBatchInsertBuilder", maxBatchSize).
		Return(mockAccountDataBatchInsertBuilder).Once()

	mockAccountSignersBatchInsertBuilder := &history.MockAccountSignersBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockAccountSignersBatchInsertBuilder)
	q.MockQSigners.On("NewAccountSignersBatchInsertBuilder", maxBatchSize).
		Return(mockAccountSignersBatchInsertBuilder).Once()

	mockOperationsBatchInsertBuilder := &history.MockOperationsBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockOperationsBatchInsertBuilder)
	q.MockQOperations.On("NewOperationBatchInsertBuilder", maxBatchSize).
		Return(mockOperationsBatchInsertBuilder).Twice()

	mockTransactionsBatchInsertBuilder := &history.MockTransactionsBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockTransactionsBatchInsertBuilder)
	q.MockQTransactions.On("NewTransactionBatchInsertBuilder", maxBatchSize).
		Return(mockTransactionsBatchInsertBuilder).Twice()

	mockClaimableBalancesBatchInsertBuilder := &history.MockClaimableBalancesBatchInsertBuilder{}
	defer mock.AssertExpectationsForObjects(t, mockClaimableBalancesBatchInsertBuilder)
	q.MockQClaimableBalances.On("NewClaimableBalancesBatchInsertBuilder", maxBatchSize).
		Return(mockClaimableBalancesBatchInsertBuilder).Once()

	runner := ProcessorRunner{
		ctx:      ctx,
		config:   config,
		historyQ: q,
	}

	_, _, _, _, err := runner.RunAllProcessorsOnLedger(ledger)
	assert.EqualError(t, err, "Error while checking for supported protocol version: This Horizon version does not support protocol version 200. The latest supported protocol version is 17. Please upgrade to the latest Horizon version.")
}
