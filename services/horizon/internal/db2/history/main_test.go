package history

import (
	"testing"
	"time"

	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLatestLedger(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("base")
	defer tt.Finish()
	q := &Q{tt.HorizonSession()}

	var seq int
	err := q.LatestLedger(tt.Ctx, &seq)

	if tt.Assert.NoError(err) {
		tt.Assert.Equal(3, seq)
	}
}

func TestLatestLedgerSequenceClosedAt(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("base")
	defer tt.Finish()
	q := &Q{tt.HorizonSession()}

	sequence, closedAt, err := q.LatestLedgerSequenceClosedAt(tt.Ctx)
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(int32(3), sequence)
		tt.Assert.Equal("2019-10-31T13:19:46Z", closedAt.Format(time.RFC3339))
	}

	test.ResetHorizonDB(t, tt.HorizonDB)

	sequence, closedAt, err = q.LatestLedgerSequenceClosedAt(tt.Ctx)
	if tt.Assert.NoError(err) {
		tt.Assert.Equal(int32(0), sequence)
		tt.Assert.Equal("0001-01-01T00:00:00Z", closedAt.Format(time.RFC3339))
	}
}

func TestGetLatestHistoryLedgerEmptyDB(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &Q{tt.HorizonSession()}

	value, err := q.GetLatestHistoryLedger(tt.Ctx)
	tt.Assert.NoError(err)
	tt.Assert.Equal(uint32(0), value)
}

func TestElderLedger(t *testing.T) {
	tt := test.Start(t)
	tt.Scenario("base")
	defer tt.Finish()
	q := &Q{tt.HorizonSession()}

	var seq int
	err := q.ElderLedger(tt.Ctx, &seq)

	if tt.Assert.NoError(err) {
		tt.Assert.Equal(1, seq)
	}
}

func TestConstructReapLookupTablesQuery(t *testing.T) {
	query, err := constructReapLookupTablesQuery(
		"history_accounts",
		[]tableObjectFieldPair{
			{
				name:        "history_effects",
				objectField: "history_account_id",
			},
			{
				name:        "history_operation_participants",
				objectField: "history_account_id",
			},
			{
				name:        "history_trades",
				objectField: "base_account_id",
			},
			{
				name:        "history_trades",
				objectField: "counter_account_id",
			},
			{
				name:        "history_transaction_participants",
				objectField: "history_account_id",
			},
		},
		10,
		0,
	)

	require.NoError(t, err)
	assert.Equal(t,
		"delete from history_accounts where id IN "+
			"(select id from "+
			"(select id, (select 1 from history_effects where history_account_id = hcb.id limit 1) as c0, "+
			"(select 1 from history_operation_participants where history_account_id = hcb.id limit 1) as c1, "+
			"(select 1 from history_trades where base_account_id = hcb.id limit 1) as c2, "+
			"(select 1 from history_trades where counter_account_id = hcb.id limit 1) as c3, "+
			"(select 1 from history_transaction_participants where history_account_id = hcb.id limit 1) as c4, "+
			"1 as cx from history_accounts hcb where id >= 0 order by id limit 10) as sub "+
			"where c0 IS NULL and c1 IS NULL and c2 IS NULL and c3 IS NULL and c4 IS NULL and 1=1);", query)
}
