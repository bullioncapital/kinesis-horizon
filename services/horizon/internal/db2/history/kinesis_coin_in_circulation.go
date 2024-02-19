package history

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/support/errors"
)

type KinesisCoinInCirculationBaseQuery struct {
	RootAccount      string
	EmissionAccount  string
	HotWalletAccount string
	FeepoolAccount   string
}

type KinesisCoinInCirculationQuery struct {
	KinesisCoinInCirculationBaseQuery
	FromDate string
}

func (q *Q) KinesisCoinInCirculation(ctx context.Context, criteria KinesisCoinInCirculationQuery) ([]KinesisCoinInCirculation, error) {
	fn := fmt.Sprintf("kinesis_coin_in_circulation('%s', '%s', '%s', '%s')",
		criteria.RootAccount,
		criteria.EmissionAccount,
		criteria.HotWalletAccount,
		criteria.FeepoolAccount)

	selectQuery := sq.Select(`
		tx_date,
		ledger,
		circulation,
		mint,
		redemption
	`).From(fn)

	if criteria.FromDate != "" {
		selectFromDate := selectQuery.Where(sq.GtOrEq{"tx_date": criteria.FromDate})
		return q.queryKinesisCoinInCirculation(ctx, selectFromDate)
	}

	// last 7 records
	subQ := selectQuery.Limit(7).OrderBy("tx_date DESC")
	reverseOrderQuery := sq.Select("*").FromSelect(subQ, "cic").OrderBy("tx_date ASC")

	return q.queryKinesisCoinInCirculation(ctx, reverseOrderQuery)
}

type KinesisCoinInCirculationByLedgerQuery struct {
	KinesisCoinInCirculationBaseQuery
	LedgerID uint64
}

func (q *Q) KinesisCoinInCirculationByLedger(ctx context.Context, criteria KinesisCoinInCirculationByLedgerQuery) ([]KinesisCoinInCirculationByLedger, error) {
	fn := fmt.Sprintf("kinesis_coin_in_circulation_at_ledger('%s', '%s', '%s', '%s', %d)",
		criteria.RootAccount,
		criteria.EmissionAccount,
		criteria.HotWalletAccount,
		criteria.FeepoolAccount,
		criteria.LedgerID)

	selectQuery := sq.Select(`
		last_ledger_timestamp,
		last_ledger,
		circulation,
		mint,
		redemption
	`).From(fn)

	return q.queryKinesisCoinInCirculationByLeader(ctx, selectQuery.Limit(1))
}

func (q *Q) queryKinesisCoinInCirculationByLeader(ctx context.Context, selectQuery sq.SelectBuilder) ([]KinesisCoinInCirculationByLedger, error) {
	var results []KinesisCoinInCirculationByLedger
	if err := q.Select(ctx, &results, selectQuery); err != nil {
		return nil, errors.Wrap(err, "could not run select query")
	}

	return results, nil
}

func (q *Q) queryKinesisCoinInCirculation(ctx context.Context, selectQuery sq.SelectBuilder) ([]KinesisCoinInCirculation, error) {
	var results []KinesisCoinInCirculation
	if err := q.Select(ctx, &results, selectQuery); err != nil {
		return nil, errors.Wrap(err, "could not run select query")
	}

	return results, nil
}

func getPublicKeyFromSeedPhrase(seedPhrase string) string {
	kp := keypair.Root(seedPhrase)
	return kp.Address()
}

func (t *KinesisCoinInCirculationBaseQuery) PopulateAccounts(networkPassphrase string) {
	t.RootAccount = getPublicKeyFromSeedPhrase(networkPassphrase)
	t.EmissionAccount = getPublicKeyFromSeedPhrase(networkPassphrase + "emission")
	t.HotWalletAccount = getPublicKeyFromSeedPhrase(networkPassphrase + "exchange")
	t.FeepoolAccount = getPublicKeyFromSeedPhrase(networkPassphrase + "feepool")
}
