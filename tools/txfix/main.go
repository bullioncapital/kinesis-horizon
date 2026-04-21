package main

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/stellar/go/network"
	"github.com/stellar/go/xdr"
)

func fixEnvelope(oldB64 string) (newB64 string, newHash string, err error) {
	oldBytes, err := base64.StdEncoding.DecodeString(oldB64)
	if err != nil {
		return "", "", fmt.Errorf("base64 decode: %w", err)
	}

	// V0 envelope layout (old, uint32 fee):
	// bytes 0-3:   envelope type (00000000 = EnvelopeTypeTxV0)
	// bytes 4-35:  SourceAccountEd25519 (32 bytes)
	// bytes 36-39: Fee (uint32 big-endian)
	// bytes 40+:   rest of transaction
	//
	// To convert to uint64 fee, insert 4 zero bytes at offset 36
	if len(oldBytes) < 40 {
		return "", "", fmt.Errorf("envelope too short: %d bytes", len(oldBytes))
	}

	// Verify it's a V0 envelope (type = 0)
	if oldBytes[0] != 0 || oldBytes[1] != 0 || oldBytes[2] != 0 || oldBytes[3] != 0 {
		return "", "", fmt.Errorf("not a V0 envelope, type bytes: %x", oldBytes[0:4])
	}

	newBytes := make([]byte, len(oldBytes)+4)
	copy(newBytes[:36], oldBytes[:36])
	// insert 4 zero bytes (high word of uint64 fee)
	newBytes[36] = 0
	newBytes[37] = 0
	newBytes[38] = 0
	newBytes[39] = 0
	// copy fee low word and rest
	copy(newBytes[40:], oldBytes[36:])

	newB64 = base64.StdEncoding.EncodeToString(newBytes)

	// Decode and compute hash
	var env xdr.TransactionEnvelope
	if err = xdr.SafeUnmarshalBase64(newB64, &env); err != nil {
		return "", "", fmt.Errorf("unmarshal new envelope: %w", err)
	}
	if env.Type != xdr.EnvelopeTypeEnvelopeTypeTxV0 {
		return "", "", fmt.Errorf("unexpected envelope type: %v", env.Type)
	}

	hashBytes, err := network.HashTransactionInEnvelope(env, network.TestNetworkPassphrase)
	if err != nil {
		return "", "", fmt.Errorf("hash: %w", err)
	}
	newHash = hex.EncodeToString(hashBytes[:])

	// Print decoded fields
	tx := env.V0.Tx
	fmt.Printf("  Fee:    %d\n", uint64(tx.Fee))
	fmt.Printf("  SeqNum: %d\n", int64(tx.SeqNum))
	if tx.TimeBounds != nil {
		fmt.Printf("  TimeBounds.MinTime: %d\n", uint64(tx.TimeBounds.MinTime))
		fmt.Printf("  TimeBounds.MaxTime: %d\n", uint64(tx.TimeBounds.MaxTime))
	}
	sa := xdr.AccountId(xdr.PublicKey{Type: xdr.PublicKeyTypePublicKeyTypeEd25519, Ed25519: &tx.SourceAccountEd25519})
	addr, _ := sa.GetAddress()
	fmt.Printf("  Account: %s\n", addr)

	return newB64, newHash, nil
}

func main() {
	// Old V0 envelopes to fix (from SQL fixtures and test file)
	envelopes := map[string]string{
		// transaction_with_max_time_bound (test case in transaction_test.go)
		"tx_max_time_bound": "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAZAAAAAAAAeJAAAAAAQAAAAAAAAAAAAAAAF3y1xsAAAAAAAAAAQAAAAAAAAALAAAAAAAS1ocAAAAAAAAAAA==",

		// base-core.sql txhistory entries (all V0)
		"base_tx1": "AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAAZAAAAAAAAAABAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAArqN6LeOagjxMaUP96Bzfs9e0corNZXzBWJkFoK7kvkwAAAAAO5rKAAAAAAAAAAABVvwF9wAAAECDzqvkQBQoNAJifPRXDoLhvtycT3lFPCQ51gkdsFHaBNWw05S/VhW0Xgkr0CBPE4NaFV2Kmcs3ZwLmib4TRrML",
		"base_tx2": "AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAAZAAAAAAAAAACAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAO2C/AO45YBD3tHVFO1R3A0MekP8JR6nN1A9eWidyItUAAAAAO5rKAAAAAAAAAAABVvwF9wAAAEASEZiZbeFwCsrKBnKIus/05VtJDBrgosuhLQ/U6XUj4twWyhs7UtS4CMexOM6JqcfqJK10WlBkkwn4g8PIfjIG",
		"base_tx3": "AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAAZAAAAAAAAAADAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAbmgm1V2dg5V1mq1elMcG1txjSYKZ9wEgoSBaeW8UiFoAAAAAO5rKAAAAAAAAAAABVvwF9wAAAEDJul1tLGLF4Vxwt0dDCVEf6tb5l4byMrGgCp+lVZMmxct54iNf2mxtjx6Md5ZJ4E4Dlcsf46EAhBGSUPsn8fYD",
		"base_tx4": "AAAAAK6jei3jmoI8TGlD/egc37PXtHKKzWV8wViZBaCu5L5MAAAAZAAAAAIAAAABAAAAAAAAAAAAAAABAAAAAAAAAAEAAAAAbmgm1V2dg5V1mq1elMcG1txjSYKZ9wEgoSBaeW8UiFoAAAAAAAAAAAL68IAAAAAAAAAAAa7kvkwAAABA9Pu9pjykcRS60lqOLqN8FHz244QP8baYNeTTJZIlr3SbRC13qEr9uP4ORDgyCB/gcug2GKrDMuK0ST3QOaKUBw==",

		// failed_transactions-core.sql txhistory entries
		"failed_tx1": "AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAAZAAAAAAAAAABAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAArqN6LeOagjxMaUP96Bzfs9e0corNZXzBWJkFoK7kvkwAAAACVAvkAAAAAAAAAAABVvwF9wAAAEDt3KwmaPuPdFSUxdAFeb6OQetyQKIWazlbSMMhmHKNLD4sqhEqUZcQP0l+X/Op+osWmN6+FUYbsz75Q2jG4vMM",
		"failed_tx2": "AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAAZAAAAAAAAAACAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAbmgm1V2dg5V1mq1elMcG1txjSYKZ9wEgoSBaeW8UiFoAAAACVAvkAAAAAAAAAAABVvwF9wAAAECdDtG2xmgQ/MAtqqffgBM+UfZVHz9oDxtzFNd58k/m2blPGnIbbueamtpQvC94rRhaw/HsBEfaa9qjZw7YpVkG",
		"failed_tx3": "AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAAZAAAAAAAAAADAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAO2C/AO45YBD3tHVFO1R3A0MekP8JR6nN1A9eWidyItUAAAACVAvkAAAAAAAAAAABVvwF9wAAAEBj4gBQ/BAbgqf7qOotatgZUHjDlsOtDNdp7alZR5/Fk9fGj+lxEygAZWzY7/LY1Z3SF6c0qs172LhAkkvV8p0M",
		"failed_tx4": "AAAAADtgvwDuOWAQ97R1RTtUdwNDHpD/CUepzdQPXlonciLVAAAAZAAAAAIAAAABAAAAAAAAAAAAAAABAAAAAAAAAAYAAAABVVNEAAAAAACuo3ot45qCPExpQ/3oHN+z17Ryis1lfMFYmQWgruS+TH//////////AAAAAAAAAAEnciLVAAAAQHE1p+5tBPq8pUoGAXqO9S7aw5O9bn87RyPw0X1dK0d7hSR67uG/khAyC3o9TrPT6z9dZkhmX/NAk8nxm9hlYQE=",
		"failed_tx5": "AAAAAG5oJtVdnYOVdZqtXpTHBtbcY0mCmfcBIKEgWnlvFIhaAAAAZAAAAAIAAAABAAAAAAAAAAAAAAABAAAAAAAAAAYAAAABVVNEAAAAAACuo3ot45qCPExpQ/3oHN+z17Ryis1lfMFYmQWgruS+TH//////////AAAAAAAAAAFvFIhaAAAAQHiLpENW73jcT1Sdkf/eaxjSLGTQCgIne0t34aIeydhplVtW9xDQ6hAT38G9kirKKRIyoKukoUNNhAwdWy/PjQc=",
		"failed_tx6": "AAAAAK6jei3jmoI8TGlD/egc37PXtHKKzWV8wViZBaCu5L5MAAAAZAAAAAIAAAABAAAAAAAAAAAAAAABAAAAAAAAAAEAAAAAbmgm1V2dg5V1mq1elMcG1txjSYKZ9wEgoSBaeW8UiFoAAAABVVNEAAAAAACuo3ot45qCPExpQ/3oHN+z17Ryis1lfMFYmQWgruS+TAAAAAA7msoAAAAAAAAAAAGu5L5MAAAAQEnKDbDYvKkJjYK0arvhFln+GK0+7Ay6g0a+1hjRRelEAe4wmjeqNcRg2m4Cn7t4AjJzAsDQI0iXahGboJPINAw=",
		"failed_tx7": "AAAAAK6jei3jmoI8TGlD/egc37PXtHKKzWV8wViZBaCu5L5MAAAAZAAAAAIAAAACAAAAAAAAAAAAAAABAAAAAAAAAAEAAAAAO2C/AO45YBD3tHVFO1R3A0MekP8JR6nN1A9eWidyItUAAAABVVNEAAAAAACuo3ot45qCPExpQ/3oHN+z17Ryis1lfMFYmQWgruS+TAAAAAA7msoAAAAAAAAAAAGu5L5MAAAAQDpIk9q30tzfQkpQuCwF7iaP3bN6DRCk+wU3V867tqkLQV3Id452WsKUYpPQrN8ej6fk0uxeemBNsz1N5VMs9gY=",
		"failed_tx8": "AAAAAK6jei3jmoI8TGlD/egc37PXtHKKzWV8wViZBaCu5L5MAAAAZAAAAAIAAAADAAAAAAAAAAAAAAABAAAAAAAAAAMAAAAAAAAAAVVTRAAAAAAArqN6LeOagjxMaUP96Bzfs9e0corNZXzBWJkFoK7kvkwAAAAA7msoAAAAAAEAAAACAAAAAAAAAAAAAAAAAAAAAa7kvkwAAABAvsu5f+v7VrJDHKu28WwE2zwDQ5lMWnC7FogSlT/NjxgHxD7kkZHMW2lkjYx/9S45sIJGCO4vj6+gIvxHrw6lBA==",
		"failed_tx9": "AAAAAG5oJtVdnYOVdZqtXpTHBtbcY0mCmfcBIKEgWnlvFIhaAAAAZAAAAAIAAAACAAAAAAAAAAAAAAABAAAAAAAAAAEAAAAAO2C/AO45YBD3tHVFO1R3A0MekP8JR6nN1A9eWidyItUAAAABVVNEAAAAAACuo3ot45qCPExpQ/3oHN+z17Ryis1lfMFYmQWgruS+TAAAAAB3NZQAAAAAAAAAAAFvFIhaAAAAQKcGS9OsVnVHCVIH04C9ZKzzKYBRdCmy+Jwmzld7QcALOxZUcAgkuGfoSdvXpH38mNvrqQiaMsSNmTJWYRzHvgo=",
	}

	for key, oldB64 := range envelopes {
		fmt.Printf("=== %s ===\n", key)
		fmt.Printf("  OldB64: %s\n", oldB64)
		newB64, newHash, err := fixEnvelope(oldB64)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			continue
		}
		fmt.Printf("  NewB64: %s\n", newB64)
		fmt.Printf("  NewHash: %s\n", newHash)
		fmt.Println()
	}
}
