package ledgerbackend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go/xdr"
)

func TestGetLedgerSucceeds(t *testing.T) {
	expectedLedger := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: 64,
				},
			},
		},
	}
	called := 0
	var encodeFailed int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		if nil != json.NewEncoder(w).Encode(LedgerResponse{
			Ledger: Base64Ledger(expectedLedger),
		}) {
			atomic.AddInt64(&encodeFailed, 1)
		}
	}))
	defer server.Close()

	client, err := NewRemoteCaptive(server.URL)
	require.NoError(t, err)

	ledger, err := client.GetLedger(context.Background(), 64)
	require.NoError(t, err)
	require.Equal(t, 1, called)
	require.Equal(t, expectedLedger, ledger)
	require.Equal(t, int64(0), atomic.LoadInt64(&encodeFailed))
}

func TestGetLedgerTakesAWhile(t *testing.T) {
	expectedLedger := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: 64,
				},
			},
		},
	}
	called := 0
	var encodeFailed int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		if called == 1 {
			// TODO: Check this is what the server really does.
			w.WriteHeader(http.StatusRequestTimeout)
			return
		}
		if nil != json.NewEncoder(w).Encode(LedgerResponse{
			Ledger: Base64Ledger(expectedLedger),
		}) {
			atomic.AddInt64(&encodeFailed, 1)
		}
	}))
	defer server.Close()

	client, err := NewRemoteCaptive(server.URL)
	require.NoError(t, err)

	ledger, err := client.GetLedger(context.Background(), 64)
	require.NoError(t, err)
	require.Equal(t, 2, called)
	require.Equal(t, expectedLedger, ledger)
	require.Equal(t, int64(0), atomic.LoadInt64(&encodeFailed))
}
