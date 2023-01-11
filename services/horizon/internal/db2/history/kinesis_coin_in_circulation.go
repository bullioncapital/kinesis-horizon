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
}

func (q *Q) KinesisCoinInCirculation(ctx context.Context, criteria KinesisCoinInCirculationQuery) ([]KinesisCoinInCirculation, error) {
	fn := fmt.Sprintf("kinesis_coin_in_circulation('%s', '%s', '%s', '%s')",
		criteria.RootAccount,
		criteria.EmissionAccount,
		criteria.HotWalletAccount,
		criteria.FeepoolAccount)
	sql := sq.Select(`
		tx_date,
		circulation,
		mint,
		redemption
	`).From(fn)

	raw, _, _ := sql.ToSql()

	fmt.Printf("%s\n", raw)
	var results []KinesisCoinInCirculation
	if err := q.Select(ctx, &results, sql); err != nil {
		return nil, errors.Wrap(err, "could not run select query")
	}
	fmt.Printf("%+v\n", results)
	return results, nil
}
