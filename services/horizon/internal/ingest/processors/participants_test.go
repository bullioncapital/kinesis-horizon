package processors

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/xdr"
)

func TestParticipantsForTransaction(t *testing.T) {
	var envelope xdr.TransactionEnvelope
	var meta xdr.TransactionMeta
	var feeChanges xdr.LedgerEntryChanges
	assert.NoError(
		t,
		xdr.SafeUnmarshalBase64(
			"AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAAAAAAAGQAAAAAAAAAAQAAAAEAAAAAAAAAZAAAAABeC9LwAAAAAAAAAAEAAAAAAAAAAAAAAAAuje6CXCYlM01kQy/fAY931ayVAdfcBbwsxB4fJ5/yDgAAAAJUC+QAAAAAAAAAAAFW/AX3AAAAQPAso8xF11UzMJ2UwGx3yXqthwhcwFpFPfHbNmVxao10eBIjdGA0gG/6ScF3NE4zGv/0ol/N1L1BUqi2vITlyAk=",
			&envelope,
		),
	)
	assert.NoError(
		t,
		xdr.SafeUnmarshalBase64(
			"AAAAAQAAAAIAAAADAAAAAwAAAAAAAAAAYvwdC9CRsrYcDdZWNGsqaNfTR8bywsjubQRHAlb8BfcN4Lazp2P/nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAwAAAAAAAAAAYvwdC9CRsrYcDdZWNGsqaNfTR8bywsjubQRHAlb8BfcN4Lazp2P/nAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAwAAAAMAAAADAAAAAAAAAABi/B0L0JGythwN1lY0aypo19NHxvLCyO5tBEcCVvwF9w3gtrOnY/+cAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAAAADAAAAAAAAAABi/B0L0JGythwN1lY0aypo19NHxvLCyO5tBEcCVvwF9w3gtrFTWBucAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAADAAAAAAAAAAAuje6CXCYlM01kQy/fAY931ayVAdfcBbwsxB4fJ5/yDgAAAAJUC+QAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA==",
			&meta,
		),
	)
	assert.NoError(
		t,
		xdr.SafeUnmarshalBase64(
			"AAAAAgAAAAMAAAABAAAAAAAAAABi/B0L0JGythwN1lY0aypo19NHxvLCyO5tBEcCVvwF9w3gtrOnZAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAAAADAAAAAAAAAABi/B0L0JGythwN1lY0aypo19NHxvLCyO5tBEcCVvwF9w3gtrOnY/+cAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA==",
			&feeChanges,
		),
	)

	particpants, err := ParticipantsForTransaction(
		3,
		ingest.LedgerTransaction{
			Index:      1,
			Envelope:   envelope,
			FeeChanges: feeChanges,
			UnsafeMeta: meta,
			Result: xdr.TransactionResultPair{
				Result: xdr.TransactionResult{
					Result: xdr.TransactionResultResult{
						Code: xdr.TransactionResultCodeTxSuccess,
					},
				},
			},
		},
	)
	assert.NoError(t, err)
	assert.Len(t, particpants, 2)
	assert.Contains(
		t,
		particpants,
		xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"),
	)
	assert.Contains(
		t,
		particpants,
		xdr.MustAddress("GAXI33UCLQTCKM2NMRBS7XYBR535LLEVAHL5YBN4FTCB4HZHT7ZA5CVK"),
	)
}
