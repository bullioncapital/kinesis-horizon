package actions

import (
	"net/http"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
)

type KinesisCoinInCirculationQuery struct {
	From string `schema:"from"`
}

// URITemplate returns a rfc6570 URI template the query struct
func (q KinesisCoinInCirculationQuery) URITemplate() string {
	return getURITemplate(&q, "coin_in_circulation", true)
}

type KinesisCoinInCirculationHandler struct {
	NetworkPassphrase string
}

func (handler KinesisCoinInCirculationHandler) GetResource(w HeaderWriter, r *http.Request) (interface{}, error) {
	cic := horizon.KinesisCoinInCirculation{}

	// known accounts
	rootAccount := getPublicKeyFromSeedPhrase(handler.NetworkPassphrase)
	emissionAccount := getPublicKeyFromSeedPhrase(handler.NetworkPassphrase + "emission")
	hotWalletAccount := getPublicKeyFromSeedPhrase(handler.NetworkPassphrase + "exchange")
	feepoolAccount := getPublicKeyFromSeedPhrase(handler.NetworkPassphrase + "feepool")

	cic.Echo = "seed: " + handler.NetworkPassphrase + "root: " + rootAccount + " emission: " + emissionAccount + " hot: " + hotWalletAccount + " fee: " + feepoolAccount
	cic.State = "Partial"
	return cic, nil
}

func getPublicKeyFromSeedPhrase(seedPhrase string) string {
	kp := keypair.Root(seedPhrase)
	return kp.Address()
}
