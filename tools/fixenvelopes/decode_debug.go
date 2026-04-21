//go:build ignore
// +build ignore

package main

import (
	"encoding/base64"
	"fmt"
	"github.com/stellar/go/xdr"
)

func inspect(label, b64 string) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		fmt.Printf("[%s] base64 decode error: %v\n", label, err)
		return
	}
	fmt.Printf("[%s] raw len=%d\n", label, len(raw))
	if len(raw) >= 8 {
		fmt.Printf("  bytes 0-7: %02x %02x %02x %02x  %02x %02x %02x %02x\n",
			raw[0],raw[1],raw[2],raw[3],raw[4],raw[5],raw[6],raw[7])
	}
	var env xdr.TransactionEnvelope
	if err := xdr.SafeUnmarshalBase64(b64, &env); err != nil {
		fmt.Printf("  envelope decode FAILED: %v\n", err)
	} else {
		fmt.Printf("  envelope decode OK: type=%v\n", env.Type)
		// Try to re-encode
		reenc, err2 := xdr.MarshalBase64(env)
		if err2 != nil {
			fmt.Printf("  re-encode FAILED: %v\n", err2)
		} else {
			fmt.Printf("  re-encode OK (len=%d)\n", len(reenc))
		}
	}
}

func main() {
	cases := []struct{ label, b64 string }{
		{"multi-op env", "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAAAAAADIAAAAAAAB4kAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAsAAAAAABLWhwAAAAAAAAALAAAAAAAS1ogAAAAAAAAAAA=="},
		{"v2-timebounds env", "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAAAAABkAAAAAAAAAAEAAAACAAAAAQAAAAAAAAAAAAAAAGI81AkAAAABAAAAAAAAAAEAAAAAAAAAAAAAAAoAAAACAAAAAAAAAAAAAAABAAAAAAAAAAsAAAAAAAAAAAAAAAAAAAAA"},
		{"okk0 env", "AAAAAgAAAAAokk0ZqR+mxwuhJJ2uXvNqIhmObygxBFIJKvQgf/7fqwAAAACVAvkAARdSGwAAMNEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAJdGVzdCBtZW1vAAAAAAAAAQAAAAAAAAALARdSGwAAV+EAAAAAAAAAAA=="},
	}
	for _, c := range cases {
		inspect(c.label, c.b64)
	}
}
