package network

import (
	"testing"

	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashTransaction(t *testing.T) {
	// txdummy := xdr.TransactionEnvelope{
	// 	Type: xdr.EnvelopeTypeEnvelopeTypeTxV0,
	// 	V0: &xdr.TransactionV0Envelope{
	// 		Tx: xdr.TransactionV0{
	// 			SourceAccountEd25519: *xdr.MustAddress("GBXGQJWVLWOYHFLVTKWV5FGHA3LNYY2JQKM7OAJAUEQFU6LPCSEFVXON").Ed25519,
	// 			Fee:                  100,
	// 			SeqNum:               8589934594,
	// 			Operations: []xdr.Operation{
	// 				{
	// 					Body: xdr.OperationBody{
	// 						Type: xdr.OperationTypeCreateAccount,
	// 						CreateAccountOp: &xdr.CreateAccountOp{
	// 							Destination:     xdr.MustAddress("GCXKG6RN4ONIEPCMNFB732A436Z5PNDSRLGWK7GBLCMQLIFO4S7EYWVU"),
	// 							StartingBalance: 1000000000,
	// 						},
	// 					},
	// 				},
	// 			},
	// 		},
	// 		Signatures: []xdr.DecoratedSignature{
	// 			{
	// 				Hint:      xdr.SignatureHint{86, 252, 5, 247},
	// 				Signature: xdr.Signature{131, 206, 171, 228, 64, 20, 40, 52, 2, 98, 124, 244, 87, 14, 130, 225, 190, 220, 156, 79, 121, 69, 60, 36, 57, 214, 9, 29, 176, 81, 218, 4, 213, 176, 211, 148, 191, 86, 21, 180, 94, 9, 43, 208, 32, 79, 19, 131, 90, 21, 93, 138, 153, 203, 55, 103, 2, 230, 137, 190, 19, 70, 179, 11},
	// 			},
	// 		},
	// 	},
	// }
	// env, _ := xdr.MarshalBase64(txdummy)
	// fmt.Println("Envelope:", env)

	var txe xdr.TransactionEnvelope
	err := xdr.SafeUnmarshalBase64("AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAK6jei3jmoI8TGlD/egc37PXtHKKzWV8wViZBaCu5L5MAAAAADuaygAAAAAAAAAAAVb8BfcAAABAg86r5EAUKDQCYnz0Vw6C4b7cnE95RTwkOdYJHbBR2gTVsNOUv1YVtF4JK9AgTxODWhVdipnLN2cC5om+E0azCw==", &txe)

	require.NoError(t, err)

	expected := [32]byte{
		0xff, 0x5c, 0xba, 0x32, 0xe8, 0x91, 0x83, 0x27,
		0xf1, 0xd5, 0x63, 0xf5, 0x7c, 0xd5, 0x4d, 0xc5,
		0xf5, 0x90, 0x6f, 0x33, 0xce, 0x53, 0xae, 0xb1,
		0x19, 0xdf, 0x6, 0xa1, 0x6f, 0x79, 0x73, 0x87,
	}
	actual, err := HashTransactionV0(txe.V0.Tx, TestNetworkPassphrase)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	actual, err = HashTransactionInEnvelope(txe, TestNetworkPassphrase)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	_, err = HashTransactionV0(txe.V0.Tx, "")
	assert.Contains(t, err.Error(), "empty network passphrase")
	_, err = HashTransactionInEnvelope(txe, "")
	assert.Contains(t, err.Error(), "empty network passphrase")

	tx := xdr.Transaction{
		SourceAccount: txe.SourceAccount(),
		Fee:           xdr.Uint64(txe.Fee()),
		Memo:          txe.Memo(),
		Operations:    txe.Operations(),
		SeqNum:        xdr.SequenceNumber(txe.SeqNum()),
		Cond:          xdr.NewPreconditionsWithTimeBounds(txe.TimeBounds()),
	}

	actual, err = HashTransaction(tx, TestNetworkPassphrase)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	txe.Type = xdr.EnvelopeTypeEnvelopeTypeTx
	txe.V0 = nil
	txe.V1 = &xdr.TransactionV1Envelope{
		Tx: tx,
	}
	actual, err = HashTransactionInEnvelope(txe, TestNetworkPassphrase)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	// sadpath: empty passphrase
	_, err = HashTransaction(tx, "")
	assert.Contains(t, err.Error(), "empty network passphrase")
	_, err = HashTransactionInEnvelope(txe, "")
	assert.Contains(t, err.Error(), "empty network passphrase")

	sourceAID := xdr.MustAddress("GCLOMB72ODBFUGK4E2BK7VMR3RNZ5WSTMEOGNA2YUVHFR3WMH2XBAB6H")
	feeBumpTx := xdr.FeeBumpTransaction{
		Fee:       123456,
		FeeSource: sourceAID.ToMuxedAccount(),
		InnerTx: xdr.FeeBumpTransactionInnerTx{
			Type: xdr.EnvelopeTypeEnvelopeTypeTx,
			V1: &xdr.TransactionV1Envelope{
				Tx:         tx,
				Signatures: []xdr.DecoratedSignature{},
			},
		},
	}

	expected = [32]uint8{
		0x7d, 0xd4, 0x6c, 0x67, 0xc7, 0xe9, 0x87, 0x5f,
		0xdb, 0x3e, 0xdb, 0xf2, 0x38, 0x95, 0xf5, 0x5,
		0xa2, 0x1e, 0xe7, 0xff, 0xf7, 0x23, 0xbd, 0x7c,
		0x2c, 0x7a, 0x63, 0xc2, 0x71, 0xa1, 0xb1, 0x38,
	}
	actual, err = HashFeeBumpTransaction(feeBumpTx, TestNetworkPassphrase)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	txe.Type = xdr.EnvelopeTypeEnvelopeTypeTxFeeBump
	txe.V1 = nil
	txe.FeeBump = &xdr.FeeBumpTransactionEnvelope{
		Tx: feeBumpTx,
	}
	actual, err = HashTransactionInEnvelope(txe, TestNetworkPassphrase)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	_, err = HashFeeBumpTransaction(feeBumpTx, "")
	assert.Contains(t, err.Error(), "empty network passphrase")
	_, err = HashTransactionInEnvelope(txe, "")
	assert.Contains(t, err.Error(), "empty network passphrase")
}
