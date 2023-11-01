package internal

import (
	"sync"
	"testing"

	"github.com/stellar/go/txnbuild"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stretchr/testify/assert"
)

func TestFriendbot_Pay(t *testing.T) {
	mockSubmitTransaction := func(minion *Minion, hclient horizonclient.ClientInterface, tx string) (*hProtocol.Transaction, error) {
		// Instead of submitting the tx, we emulate a success.
		txSuccess := hProtocol.Transaction{EnvelopeXdr: tx, Successful: true}
		return &txSuccess, nil
	}

	// Public key: GD25B4QI6KWVDWXDW25CIM7EKR6A6PBSWE2RCNSAC4NJQDQJXZJYMMKR
	botSeed := "SCWNLYELENPBXN46FHYXETT5LJCYBZD5VUQQVW4KZPHFO2YTQJUWT4D5"
	botKeypair, err := keypair.Parse(botSeed)
	if !assert.NoError(t, err) {
		return
	}
	botAccount := Account{AccountID: botKeypair.Address()}

	// Public key: GD4AGPPDFFHKK3Z2X4XZDRXX6GZQKP4FMLVQ5T55NDEYGG3GIP7BQUHM
	minionSeed := "SDTNSEERJPJFUE2LSDNYBFHYGVTPIWY7TU2IOJZQQGLWO2THTGB7NU5A"
	minionKeypair, err := keypair.Parse(minionSeed)
	if !assert.NoError(t, err) {
		return
	}

	minion := Minion{
		Account: Account{
			AccountID: minionKeypair.Address(),
			Sequence:  1,
		},
		Keypair:              minionKeypair.(*keypair.Full),
		BotAccount:           botAccount,
		BotKeypair:           botKeypair.(*keypair.Full),
		Network:              "Test SDF Network ; September 2015",
		StartingBalance:      "10000.00",
		SubmitTransaction:    mockSubmitTransaction,
		CheckSequenceRefresh: CheckSequenceRefresh,
		BaseFee:              txnbuild.MinBaseFee,
	}
	fb := &Bot{Minions: []Minion{minion}}

	recipientAddress := "GDJIN6W6PLTPKLLM57UW65ZH4BITUXUMYQHIMAZFYXF45PZVAWDBI77Z"
	txSuccess, err := fb.Pay(recipientAddress)
	if !assert.NoError(t, err) {
		return
	}
	expectedTxn := "AAAAAgAAAAD4Az3jKU6lbzq/L5HG9/GzBT+FYusOz71oyYMbZkP+GAAAAAAAAABkAAAAAAAAAAIAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAQAAAAD10PII8q1R2uO2uiQz5FR8DzwysTURNkAXGpgOCb5ThgAAAAAAAAAA0ob63nrm9S1s7+lvdyfgUTpejMQOhgMlxcvOvzUFhhQAAAAXSHboAAAAAAAAAAACZkP+GAAAAED7KA5G/ZwSRwCBwcBRQNkno54AvojlVKQ9QwFKJmKl3059no42s7XYBrHZI6Lcs1Tfc/HJDF/BsktzoDszS6QACb5ThgAAAEAqaP8+FB5TTXK47fPlxc9fJ1fU/VF4vk6kYSMa1Oy6ZIlFXms12WW2Wk9oXJbLEwwurysset9blslHxmUb3B0C"
	assert.Equal(t, expectedTxn, txSuccess.EnvelopeXdr)

	// Don't assert on tx values below, since the completion order is unknown.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		_, err := fb.Pay(recipientAddress)
		assert.NoError(t, err)
		wg.Done()
	}()
	go func() {
		_, err := fb.Pay(recipientAddress)
		assert.NoError(t, err)
		wg.Done()
	}()
	wg.Wait()
}
