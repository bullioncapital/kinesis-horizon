package history

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/stellar/go/support/errors"
)

type KinesisCoinInCirculationQuery struct {
	RootAccount      string
	EmissionAccount  string
	HotWalletAccount string
	FeepoolAccount   string
	FromDate         string
}

func (q *Q) KinesisCoinInCirculation(ctx context.Context, criteria KinesisCoinInCirculationQuery) ([]KinesisCoinInCirculation, error) {
	fn := fmt.Sprintf("kinesis_coin_in_circulation('%s', '%s', '%s', '%s')",
		criteria.RootAccount,
		criteria.EmissionAccount,
		criteria.HotWalletAccount,
		criteria.FeepoolAccount)

	selectQuery := sq.Select(`
		tx_date,
		circulation,
		mint,
		redemption
	`).From(fn).Where(sq.GtOrEq{"tx_date": criteria.FromDate})

	var results []KinesisCoinInCirculation
	if err := q.Select(ctx, &results, selectQuery); err != nil {
		return nil, errors.Wrap(err, "could not run select query")
	}

	return results, nil
}
