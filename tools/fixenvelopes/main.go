//go:build ignore
// +build ignore

// fixenvelopes repairs broken TransactionEnvelope and TransactionResult XDR base64
// strings in test files. A previous agent session removed 4 bytes from the fee
// position instead of inserting 4 zero bytes (to upgrade uint32 fee to uint64 fee).
// This tool detects such broken XDR blobs and re-inserts the 4 zero bytes.
//
// Usage (from project root):
//   go run tools/fixenvelopes/main.go
package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/stellar/go/xdr"
)

var testFiles = []string{
	"services/horizon/internal/ingest/processors/effects_processor_test.go",
	"services/horizon/internal/ingest/processors/transaction_operation_wrapper_test.go",
	"services/horizon/internal/ingest/processors/participants_test.go",
	"services/horizon/internal/txsub/system_test.go",
	"services/horizon/internal/db2/history/operation_test.go",
	"services/horizon/internal/db2/history/transaction_test.go",
}

// base64Pattern matches quoted base64 strings of reasonable length (40+ chars)
var base64Pattern = regexp.MustCompile(`"([A-Za-z0-9+/]{40,}={0,2})"`)

func main() {
	fmt.Println("=== XDR Envelope / Result Fixer ===")
	for _, file := range testFiles {
		fixFile(file)
	}
	fmt.Println("=== Done ===")
}

// tryDecodeEnvelope tries to decode a base64 string as a TransactionEnvelope.
func tryDecodeEnvelope(b64 string) error {
	var envelope xdr.TransactionEnvelope
	return xdr.SafeUnmarshalBase64(b64, &envelope)
}

// tryDecodeResult tries to decode a base64 string as a TransactionResult.
func tryDecodeResult(b64 string) error {
	var result xdr.TransactionResult
	return xdr.SafeUnmarshalBase64(b64, &result)
}

// getEnvelopeFeeOffset returns the byte offset of the fee field for a
// TransactionEnvelope. Returns (offset, true) if the envelope type is
// recognized, (0, false) otherwise.
func getEnvelopeFeeOffset(rawBytes []byte) (int, bool) {
	if len(rawBytes) < 8 {
		return 0, false
	}

	// Read envelope type discriminant (4 bytes, big-endian signed int32 in XDR)
	envelopeType := int32(rawBytes[0])<<24 | int32(rawBytes[1])<<16 | int32(rawBytes[2])<<8 | int32(rawBytes[3])

	switch envelopeType {
	case 0: // ENVELOPE_TYPE_TX_V0
		// TransactionV0Envelope layout:
		//   [4 bytes type discriminant]
		//   TransactionV0:
		//     [32 bytes sourceAccountEd25519]  <- bytes 4-35
		//     [8 bytes fee uint64]             <- bytes 36-43  (fee offset = 36)
		return 36, true

	case 2: // ENVELOPE_TYPE_TX (V1)
		// Transaction:
		//   MuxedAccount sourceAccount (union):
		//     [4 bytes type]
		//     if KEY_TYPE_ED25519 (0):         [32 bytes key] -> fee at offset 4+4+32 = 40
		//     if KEY_TYPE_MUXED_ED25519 (256): [8 bytes id][32 bytes key] -> fee at offset 4+4+8+32 = 48
		sourceType := int32(rawBytes[4])<<24 | int32(rawBytes[5])<<16 | int32(rawBytes[6])<<8 | int32(rawBytes[7])
		switch sourceType {
		case 0: // KEY_TYPE_ED25519
			return 40, true
		case 256: // KEY_TYPE_MUXED_ED25519
			return 48, true
		}
	}

	return 0, false
}

// tryRepairEnvelope inserts 4 zero bytes at the fee offset of a broken
// TransactionEnvelope. Returns (fixedBytes, true) if repair is possible.
func tryRepairEnvelope(rawBytes []byte) ([]byte, bool) {
	feeOffset, ok := getEnvelopeFeeOffset(rawBytes)
	if !ok {
		return nil, false
	}
	if len(rawBytes) < feeOffset {
		return nil, false
	}
	return insertZeros(rawBytes, feeOffset), true
}

// tryRepairResult inserts 4 zero bytes at offset 0 of a broken TransactionResult.
// The feeCharged (Int64) is at the start; the previous agent removed the high
// 4 zero bytes.  Candidate broken results have bytes 0-3 non-zero (the low bytes
// of a small fee ended up at the beginning after deletion).
func tryRepairResult(rawBytes []byte) ([]byte, bool) {
	if len(rawBytes) < 12 {
		return nil, false
	}
	// Only attempt repair if the first 4 bytes look like they could be the
	// low-half of a fee (i.e., bytes 0-3 are NOT all zero).  When the high
	// 4 zero bytes are present the first 4 bytes of a valid result are all 0.
	if rawBytes[0] == 0 && rawBytes[1] == 0 && rawBytes[2] == 0 && rawBytes[3] == 0 {
		return nil, false // already has the zero high-bytes; not a broken result
	}
	return insertZeros(rawBytes, 0), true
}

// insertZeros returns a new byte slice with 4 zero bytes inserted at offset.
func insertZeros(src []byte, offset int) []byte {
	dst := make([]byte, len(src)+4)
	copy(dst[:offset], src[:offset])
	// dst[offset:offset+4] is already zero
	copy(dst[offset+4:], src[offset:])
	return dst
}

// repairCandidate holds a discovered (broken b64, repaired b64) pair.
type repairCandidate struct {
	oldB64 string
	newB64 string
	kind   string // "envelope" or "result"
}

// findRepairs scans all base64 strings in content for broken XDR and returns
// a list of (old, new) pairs for strings that need replacement.
func findRepairs(content string) []repairCandidate {
	matches := base64Pattern.FindAllStringSubmatch(content, -1)

	seen := map[string]bool{}
	var repairs []repairCandidate

	for _, match := range matches {
		b64str := match[1]
		if seen[b64str] {
			continue
		}
		seen[b64str] = true

		// ── 1. Try TransactionEnvelope ──────────────────────────────────────
		if tryDecodeEnvelope(b64str) == nil {
			continue // valid as-is
		}

		raw, err := base64.StdEncoding.DecodeString(b64str)
		if err != nil {
			continue
		}

		if fixedBytes, ok := tryRepairEnvelope(raw); ok {
			fixedB64 := base64.StdEncoding.EncodeToString(fixedBytes)
			if tryDecodeEnvelope(fixedB64) == nil {
				repairs = append(repairs, repairCandidate{b64str, fixedB64, "envelope"})
				continue
			}
			// envelope repair didn't help
		}

		// ── 2. Try TransactionResult ────────────────────────────────────────
		if tryDecodeResult(b64str) == nil {
			continue // valid result as-is
		}

		if fixedBytes, ok := tryRepairResult(raw); ok {
			fixedB64 := base64.StdEncoding.EncodeToString(fixedBytes)
			if tryDecodeResult(fixedB64) == nil {
				repairs = append(repairs, repairCandidate{b64str, fixedB64, "result"})
				continue
			}
		}

		// neither strategy worked – leave it alone
	}

	return repairs
}

func fixFile(filename string) {
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("[SKIP] Cannot read %s: %v\n", filename, err)
		return
	}

	repairs := findRepairs(string(content))

	if len(repairs) == 0 {
		fmt.Printf("[OK]   %s  (nothing to fix)\n", filename)
		return
	}

	modified := string(content)
	for _, r := range repairs {
		before := fmt.Sprintf("%q", r.oldB64)
		after := fmt.Sprintf("%q", r.newB64)
		modified = strings.ReplaceAll(modified, before, after)
		fmt.Printf("  [FIXED-%s] %s\n    old: %.60s...\n    new: %.60s...\n",
			r.kind, filename, r.oldB64, r.newB64)
	}

	if err := os.WriteFile(filename, []byte(modified), 0644); err != nil {
		fmt.Printf("  [ERROR] Writing %s: %v\n", filename, err)
	} else {
		fmt.Printf("[SAVED] %s  (%d fixes)\n", filename, len(repairs))
	}
}
