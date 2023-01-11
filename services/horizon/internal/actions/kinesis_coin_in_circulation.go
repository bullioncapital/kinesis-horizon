package actions

import (
	"net/http"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/services/horizon/internal/context"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/ledger"
	horizonProblem "github.com/stellar/go/services/horizon/internal/render/problem"
)

type KinesisCoinInCirculationQuery struct {
	From string `schema:"from"`
}

// URITemplate returns a rfc6570 URI template the query struct
func (q KinesisCoinInCirculationQuery) URITemplate() string {
	return getURITemplate(&q, "coin_in_circulation", true)
}

type KinesisCoinInCirculationHandler struct {
	LedgerState       *ledger.State
	NetworkPassphrase string
}

func (handler KinesisCoinInCirculationHandler) GetResource(w HeaderWriter, r *http.Request) (interface{}, error) {
	ctx := r.Context()
	cic := horizon.KinesisCoinInCirculation{}

	if handler.LedgerState.CurrentStatus().HorizonStatus.HistoryElder > 2 {
		return nil, horizonProblem.PartialLedgerIngested
	}

	historyQ, err := context.HistoryQFromRequest(r)
	if err != nil {
		return nil, err
	}

	criteria := history.KinesisCoinInCirculationQuery{}
	// known accounts
	criteria.RootAccount = getPublicKeyFromSeedPhrase(handler.NetworkPassphrase)
	criteria.EmissionAccount = getPublicKeyFromSeedPhrase(handler.NetworkPassphrase + "emission")
	criteria.HotWalletAccount = getPublicKeyFromSeedPhrase(handler.NetworkPassphrase + "exchange")
	criteria.FeepoolAccount = getPublicKeyFromSeedPhrase(handler.NetworkPassphrase + "feepool")

	records, _ := historyQ.KinesisCoinInCirculation(ctx, criteria)

	for _, record := range records {
		var daily horizon.KinesisDailyCoinInCirculation
		daily.Date = record.TxDate
		daily.Circulation = record.Circulation
		daily.Mint = record.Mint
		daily.Redemption = record.Redemption
		cic.Records = append(cic.Records, daily)
	}

	cic.State = "Partial"

	return cic, nil
}

func getPublicKeyFromSeedPhrase(seedPhrase string) string {
	kp := keypair.Root(seedPhrase)
	return kp.Address()
}
