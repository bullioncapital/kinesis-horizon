//go:build ignore
// +build ignore

package main

import (
	"encoding/base64"
	"fmt"
	"github.com/stellar/go/xdr"
)

func tryDecode(label, b64 string) {
	raw, _ := base64.StdEncoding.DecodeString(b64)
	fmt.Printf("\n=== %s ===\n", label)
	fmt.Printf("Length: %d bytes (base64: %d chars)\n", len(raw), len(b64))
	if len(raw) >= 8 {
		fmt.Printf("Bytes 0-7: %02x %02x %02x %02x  %02x %02x %02x %02x\n",
			raw[0],raw[1],raw[2],raw[3],raw[4],raw[5],raw[6],raw[7])
	}

	var env xdr.TransactionEnvelope
	errEnv := xdr.SafeUnmarshalBase64(b64, &env)
	fmt.Printf("Decode as TransactionEnvelope: %v\n", errEnv)

	var res xdr.TransactionResult
	errRes := xdr.SafeUnmarshalBase64(b64, &res)
	fmt.Printf("Decode as TransactionResult:   %v\n", errRes)
	if errRes == nil {
		fmt.Printf("  feeCharged=%d, code=%v\n", res.FeeCharged, res.Result.Code)
	}
}

func tryfixResult(b64 string) {
	raw, _ := base64.StdEncoding.DecodeString(b64)
	if len(raw) < 12 {
		fmt.Println("  too short")
		return
	}
	// Insert 4 zeros at offset 0
	fixed := make([]byte, len(raw)+4)
	copy(fixed[4:], raw)
	fixedB64 := base64.StdEncoding.EncodeToString(fixed)
	var res xdr.TransactionResult
	err := xdr.SafeUnmarshalBase64(fixedB64, &res)
	fmt.Printf("  After +4-zeros at 0: decode=%v", err)
	if err == nil {
		fmt.Printf("  feeCharged=%d code=%v", res.FeeCharged, res.Result.Code)
	}
	fmt.Println()
}

func mainDecode() {
	broken := "AAAAAgAAAAAAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA="
	valid  := "AAAAAAAAASwAAAAAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAFAAAAAAAAAAA="
	feebump := "AAAAAAAAAHsAAAAB6Yhpu6i84IwQt4QGICEn84iMJUVM03sCYAhiRSdR9SYAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsAAAAAAAAAAAAAAAA="

	tryDecode("broken resultXDR (transaction_test)", broken)
	fmt.Print("  TryFix: ")
	tryfixResult(broken)

	tryDecode("valid resultXDR (many tests)", valid)
	tryDecode("fee-bump TxResult (system_test)", feebump)
}
