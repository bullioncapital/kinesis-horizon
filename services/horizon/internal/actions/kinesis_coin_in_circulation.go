package actions

import (
	"net/http"

	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/services/horizon/internal/context"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/ledger"
	horizonProblem "github.com/stellar/go/services/horizon/internal/render/problem"
	"github.com/stellar/go/support/errors"
	supportProblem "github.com/stellar/go/support/render/problem"
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

	criteria := history.KinesisCoinInCirculationQuery{}
	fromDate := r.URL.Query().Get("from")
	if fromDate != "" {
		date, err := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
		if err != nil {
			return nil, supportProblem.MakeInvalidFieldProblem(
				"from",
				errors.New("`from` is not a valid date format. Please use ISO8061 format e.g 2020-04-30T04:00:00.000Z or remove the value to get records from last 7 days."),
			)
		}
		criteria.FromDate = date.Format("2006-01-02")
	} else {
		// 7 days
		date := time.Now().AddDate(0, 0, -7)
		criteria.FromDate = date.Format("2006-01-02")
	}

	// known accounts
	criteria.RootAccount = getPublicKeyFromSeedPhrase(handler.NetworkPassphrase)
	criteria.EmissionAccount = getPublicKeyFromSeedPhrase(handler.NetworkPassphrase + "emission")
	criteria.HotWalletAccount = getPublicKeyFromSeedPhrase(handler.NetworkPassphrase + "exchange")
	criteria.FeepoolAccount = getPublicKeyFromSeedPhrase(handler.NetworkPassphrase + "feepool")

	// get data
	historyQ, err := context.HistoryQFromRequest(r)
	if err != nil {
		return nil, err
	}

	records, _ := historyQ.KinesisCoinInCirculation(ctx, criteria)
	cic.Records = make([]horizon.KinesisDailyCoinInCirculation, len(records))
	for i, record := range records {
		cic.Records[i].Date = record.TxDate
		cic.Records[i].Circulation = record.Circulation
		cic.Records[i].Mint = record.Mint
		cic.Records[i].Redemption = record.Redemption
	}

	ledgerState := handler.LedgerState.CurrentStatus()
	cic.IngestSequence = ledgerState.ExpHistoryLatest
	cic.HorizonSequence = ledgerState.HistoryLatest
	cic.HorizonLatestClosedAt = ledgerState.HistoryLatestClosedAt
	cic.HistoryElderSequence = ledgerState.HistoryElder

	return cic, nil
}

func getPublicKeyFromSeedPhrase(seedPhrase string) string {
	kp := keypair.Root(seedPhrase)
	return kp.Address()
}
