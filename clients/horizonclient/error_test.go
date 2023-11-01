package horizonclient

import (
	"testing"

	"github.com/stellar/go/support/render/problem"
	"github.com/stretchr/testify/assert"
)

func TestError_Error(t *testing.T) {
	var herr Error

	// transaction failed happy path: with the appropriate extra fields
	herr = Error{
		Problem: problem.P{
			Title: "Transaction Failed",
			Type:  "transaction_failed",
			Extras: map[string]interface{}{
				"result_codes": map[string]interface{}{
					"transaction": "tx_failed",
					"operations":  []string{"op_underfunded", "op_already_exists"},
				},
			},
		},
	}
	assert.Equal(t, `horizon error: "Transaction Failed" (tx_failed, op_underfunded, op_already_exists) - check horizon.Error.Problem for more information`, herr.Error())

	// transaction failed sad path: missing result_codes extra
	herr = Error{
		Problem: problem.P{
			Title:  "Transaction Failed",
			Type:   "transaction_failed",
			Extras: map[string]interface{}{},
		},
	}
	assert.Equal(t, `horizon error: "Transaction Failed" - check horizon.Error.Problem for more information`, herr.Error())

	// transaction failed sad path: unparseable result_codes extra
	herr = Error{
		Problem: problem.P{
			Title: "Transaction Failed",
			Type:  "transaction_failed",
			Extras: map[string]interface{}{
				"result_codes": "kaboom",
			},
		},
	}
	assert.Equal(t, `horizon error: "Transaction Failed" - check horizon.Error.Problem for more information`, herr.Error())

	// non-transaction errors
	herr = Error{
		Problem: problem.P{
			Type:   "https://stellar.org/horizon-errors/not_found",
			Title:  "Resource Missing",
			Status: 404,
		},
	}
	assert.Equal(t, `horizon error: "Resource Missing" - check horizon.Error.Problem for more information`, herr.Error())
}

func TestError_ResultCodes(t *testing.T) {
	var herr Error

	// happy path: transaction_failed with the appropriate extra fields
	herr.Problem.Type = "transaction_failed"
	herr.Problem.Extras = make(map[string]interface{})
	herr.Problem.Extras["result_codes"] = map[string]interface{}{
		"transaction": "tx_failed",
		"operations":  []string{"op_underfunded", "op_already_exists"},
	}

	trc, err := herr.ResultCodes()
	if assert.NoError(t, err) {
		assert.Equal(t, "tx_failed", trc.TransactionCode)

		if assert.Len(t, trc.OperationCodes, 2) {
			assert.Equal(t, "op_underfunded", trc.OperationCodes[0])
			assert.Equal(t, "op_already_exists", trc.OperationCodes[1])
		}
	}

	// sad path: missing result_codes extra
	herr.Problem.Type = "transaction_failed"
	herr.Problem.Extras = make(map[string]interface{})
	_, err = herr.ResultCodes()
	assert.Equal(t, ErrResultCodesNotPopulated, err)

	// sad path: unparseable result_codes extra
	herr.Problem.Type = "transaction_failed"
	herr.Problem.Extras = make(map[string]interface{})
	herr.Problem.Extras["result_codes"] = "kaboom"
	_, err = herr.ResultCodes()
	assert.Error(t, err)
}

func TestError_ResultString(t *testing.T) {
	var herr Error

	// happy path: transaction_failed with the appropriate extra fields
	herr.Problem.Type = "transaction_failed"
	herr.Problem.Extras = make(map[string]interface{})
	herr.Problem.Extras["result_xdr"] = "AAAAAAAAAMj/////AAAAAgAAAAAAAAAA/////wAAAAAAAAAAAAAAAAAAAAA="

	trs, err := herr.ResultString()
	if assert.NoError(t, err) {
		assert.Equal(t, "AAAAAAAAAMj/////AAAAAgAAAAAAAAAA/////wAAAAAAAAAAAAAAAAAAAAA=", trs)
	}

	// sad path: missing result_xdr extra
	herr.Problem.Type = "transaction_failed"
	herr.Problem.Extras = make(map[string]interface{})
	_, err = herr.ResultString()
	assert.Equal(t, ErrResultNotPopulated, err)

	// sad path: unparseable result_xdr extra
	herr.Problem.Type = "transaction_failed"
	herr.Problem.Extras = make(map[string]interface{})
	herr.Problem.Extras["result_xdr"] = 1234
	_, err = herr.ResultString()
	assert.Error(t, err)
}

func TestError_Envelope(t *testing.T) {
	var herr Error

	// happy path: transaction_failed with the appropriate extra fields
	herr.Problem.Type = "transaction_failed"
	herr.Problem.Extras = make(map[string]interface{})
	herr.Problem.Extras["envelope_xdr"] = `AAAAAEtl2k+Vx6bLH0iiP9boT+j4e7m/uApHLEaX9zulHmVBAAAAAB2BGiQAAAAAAAAAAQAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAEahxdpkSFx+yLaWqZI2+YaIsdmw4ruszEbDYiccAQ20AAAAAEGQqwAAAAAAAAAAAaUeZUEAAABAVrCFJvyzHb+YicyrvIo0axh61qaXapPTQxmraykhg8APE3TVTQyS+t8SR0LF2CfDKjLk4Xl2GRhIztXZlEYqBw==`

	_, err := herr.Envelope()
	assert.NoError(t, err)

	// sad path: missing envelope_xdr extra
	herr.Problem.Extras = make(map[string]interface{})
	_, err = herr.Envelope()
	assert.Equal(t, ErrEnvelopeNotPopulated, err)

	// sad path: unparseable envelope_xdr extra
	herr.Problem.Extras = make(map[string]interface{})
	herr.Problem.Extras["envelope_xdr"] = "AAAAADSMMRmQGDH6EJzkgi"
	_, err = herr.Envelope()
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "xdr decode")
	}
}
