package main

import (
	"encoding/base64"
	"fmt"
	"github.com/stellar/go/xdr"
)

func decodeEnv(label, b64 string) {
	fmt.Printf("=== %s ===\n", label)
	if b64 == "" {
		fmt.Println("  (empty)")
		return
	}
	raw, _ := base64.StdEncoding.DecodeString(b64)
	_ = raw
	var env xdr.TransactionEnvelope
	err := xdr.SafeUnmarshalBase64(b64, &env)
	if err != nil {
		fmt.Printf("  xdr error: %v\n", err)
		return
	}
	fmt.Printf("  type: %v\n", env.Type)
	switch env.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		v0 := env.V0.Tx
		fmt.Printf("  V0 fee: %v\n", v0.Fee)
		fmt.Printf("  V0 seqNum: %v\n", v0.SeqNum)
		if v0.TimeBounds != nil {
			fmt.Printf("  V0 timeBounds: minTime=%v maxTime=%v\n", v0.TimeBounds.MinTime, v0.TimeBounds.MaxTime)
		} else {
			fmt.Printf("  V0 timeBounds: nil\n")
		}
		fmt.Printf("  V0 memo type: %v\n", v0.Memo.Type)
		if v0.Memo.Type == xdr.MemoTypeMemoId { fmt.Printf("  V0 memo id: %v\n", *v0.Memo.Id) }
		if v0.Memo.Type == xdr.MemoTypeMemoText { fmt.Printf("  V0 memo text: %v\n", *v0.Memo.Text) }
		fmt.Printf("  V0 ops: %v\n", len(v0.Operations))
		fmt.Printf("  V0 sigs: %v\n", len(env.V0.Signatures))
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		tx := env.V1.Tx
		fmt.Printf("  sourceAccount type: %v\n", tx.SourceAccount.Type)
		fmt.Printf("  fee: %v\n", tx.Fee)
		fmt.Printf("  seqNum: %v\n", tx.SeqNum)
		cond := tx.Cond
		fmt.Printf("  preconditions type: %v\n", cond.Type)
		switch cond.Type {
		case xdr.PreconditionTypePrecondTime:
			if cond.TimeBounds != nil {
				fmt.Printf("  timeBounds: minTime=%v maxTime=%v\n", cond.TimeBounds.MinTime, cond.TimeBounds.MaxTime)
			}
		case xdr.PreconditionTypePrecondNone:
			fmt.Println("  no timeBounds (PRECOND_NONE)")
		case xdr.PreconditionTypePrecondV2:
			v2 := cond.V2
			fmt.Printf("  v2 timeBounds: %v\n", v2.TimeBounds)
		}
		fmt.Printf("  memo type: %v\n", tx.Memo.Type)
		if tx.Memo.Type == xdr.MemoTypeMemoId { fmt.Printf("  memo id: %v\n", *tx.Memo.Id) }
		if tx.Memo.Type == xdr.MemoTypeMemoText { fmt.Printf("  memo text: %v\n", *tx.Memo.Text) }
		fmt.Printf("  ops count: %v\n", len(tx.Operations))
		fmt.Printf("  sigs count: %v\n", len(env.V1.Signatures))
		for i, s := range env.V1.Signatures {
			fmt.Printf("  sig[%d] hint=%x sig=%s\n", i, s.Hint, base64.StdEncoding.EncodeToString(s.Signature))
		}
	}
}

func main() {
	envelopes := []struct{ label, b64 string }{
		{"[successful/failed/id_memo shared] BROKEN envelope", "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsBF1IbAABX4QAAAAAAAAAA"},
		{"64bit fee charged", "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAACVAvkAARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAJdGVzdCBtZW1vAAAAAAAAAQAAAAAAAAALARdSGwAAV+EAAAAAAAAAAA=="},
		{"text memo", "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAJdGVzdCBtZW1vAAAAAAAAAQAAAAAAAAALARdSGwAAV+EAAAAAAAAAAA=="},
		{"hash memo V0", "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAAAAAAMgBF1IbAAAw0QAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAA34t7yDVohpWvipFe2SPcC7hr4idPfZXkOkqBQgen6vxAAAAAQAAAAAAAAALARdSGwAAV+EAAAAAAAAAAA=="},
		{"return memo V0", "AAAAACiSTRmpH6bHC6Ekna5e82oiGY5vKDEEUgkq9CB//t+rAAAAAAAAAMgBF1IbAAAw0QAAAAEAAAAAAAAAAAAAAAAAAAAAAAAABM3YwK5SC2vyzb+6O5aP6r+fn1VTzZdMj4GUFgervoC+AAAAAQAAAAAAAAALARdSGwAAV+EAAAAAAAAAAA=="},
		{"min time bound V1", "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAABkAAAAAAAB4kAAAAABAAAAAF3y1xsAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAsAAAAAABLWhwAAAAAAAAAA"},
		{"max time bound (empty)", ""},
		{"min+max time bound V1", "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAABkAAAAAAAB4kAAAAABAAAAAF3xUHsAAAAAXfLXGwAAAAAAAAABAAAAAAAAAAsAAAAAABLWhwAAAAAAAAAA"},
		{"v2 preconditions", "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAAAAABkAAAAAAAAAAEAAAACAAAAAQAAAAAAAAAAAAAAAGI81AkAAAABAAAAAAAAAAEAAAAAAAAAAAAAAAoAAAACAAAAAAAAAAAAAAABAAAAAAAAAAsAAAAAAAAAAAAAAAAAAAAA"},
		{"multiple operations", "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIAAAAAAAB4kAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAsAAAAAABLWhwAAAAAAAAALAAAAAAAS1ogAAAAAAAAAAA=="},
	}
	for _, e := range envelopes {
		decodeEnv(e.label, e.b64)
	}
}
