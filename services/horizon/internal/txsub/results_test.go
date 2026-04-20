package txsub

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stretchr/testify/mock"
)

func TestGetIngestedTx(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("base")
	defer tt.Finish()
	q := &history.Q{SessionInterface: tt.HorizonSession()}
	hash := "ff5cba32e8918327f1d563f57cd54dc5f5906f33ce53aeb119df06a16f797387"
	tx, err := txResultByHash(tt.Ctx, q, hash)
	tt.Assert.NoError(err)
	tt.Assert.Equal(hash, tx.TransactionHash)
}

func TestGetIngestedTxHashes(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("base")
	defer tt.Finish()
	q := &history.Q{SessionInterface: tt.HorizonSession()}
	hashes := []string{"ff5cba32e8918327f1d563f57cd54dc5f5906f33ce53aeb119df06a16f797387"}
	txs, err := q.AllTransactionsByHashesSinceLedger(tt.Ctx, hashes, 0)
	tt.Assert.NoError(err)
	tt.Assert.Equal(hashes[0], txs[0].TransactionHash)
}

func TestGetMissingTx(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("base")
	defer tt.Finish()
	q := &history.Q{SessionInterface: tt.HorizonSession()}
	hash := "adf1efb9fd253f53cbbe6230c131d2af19830328e52b610464652d67d2fb7195"

	_, err := txResultByHash(tt.Ctx, q, hash)
	tt.Assert.Equal(ErrNoResults, err)
}

func TestGetFailedTx(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("failed_transactions")
	defer tt.Finish()
	q := &history.Q{SessionInterface: tt.HorizonSession()}
	hash := "e34941080e33bf0ce90c7fac31ec13a0f7e9e5489204e766c3def374164aa3fa"

	_, err := txResultByHash(tt.Ctx, q, hash)
	tt.Assert.Equal("AAAAAAAAAGT/////AAAAAQAAAAAAAAAB/////gAAAAA=", err.(*FailedTransactionError).ResultXDR)
}

func TestFilteredQueryErrs(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()

	q := &mockDBQ{}
	hash := "e34941080e33bf0ce90c7fac31ec13a0f7e9e5489204e766c3def374164aa3fa"

	q.On("PreFilteredTransactionByHash", tt.Ctx, mock.Anything, hash).Return(sql.ErrConnDone).Once()
	q.On("NoRows", sql.ErrConnDone).Return(false).Once()
	_, err := txResultByHash(tt.Ctx, q, hash)
	tt.Assert.True(errors.Is(err, sql.ErrConnDone))
}

func TestHistoryQueryErrs(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()

	q := &mockDBQ{}
	hash := "e34941080e33bf0ce90c7fac31ec13a0f7e9e5489204e766c3def374164aa3fa"

	q.On("PreFilteredTransactionByHash", tt.Ctx, mock.Anything, hash).Return(sql.ErrNoRows).Once()
	q.On("NoRows", sql.ErrNoRows).Return(true).Once()

	q.On("TransactionByHash", tt.Ctx, mock.Anything, hash).Return(sql.ErrConnDone).Once()
	q.On("NoRows", sql.ErrConnDone).Return(false).Once()

	_, err := txResultByHash(tt.Ctx, q, hash)
	tt.Assert.True(errors.Is(err, sql.ErrConnDone))
}
