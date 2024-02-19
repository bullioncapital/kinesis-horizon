package actions

import (
	"net/http"
	"time"

	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/services/horizon/internal/context"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/ledger"
	horizonProblem "github.com/stellar/go/services/horizon/internal/render/problem"
	"github.com/stellar/go/support/errors"
	supportProblem "github.com/stellar/go/support/render/problem"
)

type KinesisCoinInCirculationByLedgerIDQuery struct {
	LedgerID uint64 `schema:"ledger_id" valid:"required"`
}

// KinesisCoinInCirculationByLedger is the action handler for the /ledger/{ledger_id} endpoint
type GetKinesisCoinInCirculationByLedgerHandler struct {
	LedgerState       *ledger.State
	NetworkPassphrase string
}

// GetResource returns a kinesis coin in circulation by ledger id.
func (handler GetKinesisCoinInCirculationByLedgerHandler) GetResource(w HeaderWriter, r *http.Request) (interface{}, error) {
	ctx := r.Context()

	qp := KinesisCoinInCirculationByLedgerIDQuery{}
	if err := getParams(&qp, r); err != nil {
		return nil, err
	}

	criteria := history.KinesisCoinInCirculationByLedgerQuery{}
	criteria.PopulateAccounts(handler.NetworkPassphrase)

	// get data
	historyQ, err := context.HistoryQFromRequest(r)
	if err != nil {
		return nil, err
	}

	cic := horizon.KinesisDailyCoinInCirculationByLedger{}
	records, _ := historyQ.KinesisCoinInCirculationByLedger(ctx, criteria)

	if len(records) > 0 {
		cic.Timestamp = records[0].TxDate
		cic.Circulation = records[0].Circulation
		cic.Mint = records[0].Mint
		cic.Redemption = records[0].Redemption
		cic.Ledger = records[0].Ledger
	}

	return cic, nil
}

type KinesisCoinInCirculationQuery struct {
	From string `schema:"from"`
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
	}
	criteria.PopulateAccounts(handler.NetworkPassphrase)

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
		cic.Records[i].Ledger = record.Ledger
	}

	ledgerState := handler.LedgerState.CurrentStatus()
	cic.IngestSequence = ledgerState.ExpHistoryLatest
	cic.HorizonSequence = ledgerState.HistoryLatest
	cic.HorizonLatestClosedAt = ledgerState.HistoryLatestClosedAt
	cic.HistoryElderSequence = ledgerState.HistoryElder

	return cic, nil
}
