package test

import (
	"context"
	"github.com/creachadair/jrpc2/code"
	"github.com/stellar/go/keypair"
	"testing"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/jhttp"
	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/exp/services/soroban-rpc/internal/methods"
)

func TestAccount(t *testing.T) {
	test := NewTest(t)

	ch := jhttp.NewChannel(test.server.URL, nil)
	cli := jrpc2.NewClient(ch, nil)

	request := methods.AccountRequest{
		Address: keypair.Master(StandaloneNetworkPassphrase).Address(),
	}
	var result methods.AccountInfo
	if err := cli.CallResult(context.Background(), "getAccount", request, &result); err != nil {
		t.Fatalf("rpc call failed: %v", err)
	}
	assert.Equal(t, methods.AccountInfo{ID: request.Address, Sequence: 0}, result)

	request.Address = "invalid"
	err := cli.CallResult(context.Background(), "getAccount", request, &result).(*jrpc2.Error)
	assert.Equal(t, "Bad Request", err.Message)
	assert.Equal(t, code.InvalidRequest, err.Code)
	assert.Equal(
		t,
		"{\"invalid_field\":\"account_id\",\"reason\":\"Account ID must start with `G` and contain 56 alphanum characters\"}",
		string(err.Data),
	)
}
