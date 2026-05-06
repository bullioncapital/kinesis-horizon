package history

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/guregu/null"
	"github.com/stellar/go/services/horizon/internal/db2"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

// ClaimableBalancesQuery is a helper struct to configure queries to claimable balances
type ClaimableBalancesQuery struct {
	PageQuery db2.PageQuery
	Asset     *xdr.Asset
	Sponsor   *xdr.AccountId
	Claimant  *xdr.AccountId
}

// Cursor validates and returns the query page cursor
func (cbq ClaimableBalancesQuery) Cursor() (int64, string, error) {
	p := cbq.PageQuery
	var l int64
	var r string
	var err error

	if p.Cursor != "" {
		parts := strings.SplitN(p.Cursor, "-", 2)
		if len(parts) != 2 {
			return l, r, errors.New("Invalid cursor")
		}

		l, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return l, r, errors.Wrap(err, "Invalid cursor - first value should be higher than 0")
		}

		var balanceID xdr.ClaimableBalanceId
		if err = xdr.SafeUnmarshalHex(parts[1], &balanceID); err != nil {
			return l, r, errors.Wrap(err, "Invalid cursor - second value should be a valid claimable balance id")
		}
		r = parts[1]
		if l < 0 {
			return l, r, errors.Wrap(err, "Invalid cursor - first value should be higher than 0")
		}
	}

	return l, r, nil
}

// ApplyCursor applies cursor to the given sql. For performance reason the limit
// is not applied here. This allows us to hint the planner later to use the right
// indexes.
func applyClaimableBalancesQueriesCursor(sql sq.SelectBuilder, lCursor int64, rCursor string, order string) (sq.SelectBuilder, error) {
	hasPagedLimit := false
	if lCursor > 0 && rCursor != "" {
		hasPagedLimit = true
	}

	switch order {
	case db2.OrderAscending:
		if hasPagedLimit {
			sql = sql.
				Where(sq.Expr("(last_modified_ledger, id) > (?, ?)", lCursor, rCursor))

		}
		sql = sql.OrderBy("last_modified_ledger asc, id asc")
	case db2.OrderDescending:
		if hasPagedLimit {
			sql = sql.
				Where(sq.Expr("(last_modified_ledger, id) < (?, ?)", lCursor, rCursor))
		}

		sql = sql.OrderBy("last_modified_ledger desc, id desc")
	default:
		return sql, errors.Errorf("invalid order: %s", order)
	}

	return sql, nil
}

// ClaimableBalanceClaimant is a row of data from the `claimable_balances_claimants` table.
// This table exists to allow faster querying for claimable balances for a specific claimant.
type ClaimableBalanceClaimant struct {
	BalanceID          string `db:"id"`
	Destination        string `db:"destination"`
	LastModifiedLedger uint32 `db:"last_modified_ledger"`
}

// ClaimableBalance is a row of data from the `claimable_balances` table.
type ClaimableBalance struct {
	BalanceID          string      `db:"id"`
	Claimants          Claimants   `db:"claimants"`
	Asset              xdr.Asset   `db:"asset"`
	Amount             xdr.Int64   `db:"amount"`
	Sponsor            null.String `db:"sponsor"`
	LastModifiedLedger uint32      `db:"last_modified_ledger"`
	Flags              uint32      `db:"flags"`
}

type Claimants []Claimant

func (c Claimants) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *Claimants) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &c)
}

type Claimant struct {
	Destination string             `json:"destination"`
	Predicate   xdr.ClaimPredicate `json:"predicate"`
}

// QClaimableBalances defines claimable-balance-related related queries.
type QClaimableBalances interface {
	UpsertClaimableBalances(ctx context.Context, cb []ClaimableBalance) error
	RemoveClaimableBalances(ctx context.Context, ids []string) (int64, error)
	RemoveClaimableBalanceClaimants(ctx context.Context, ids []string) (int64, error)
	GetClaimableBalancesByID(ctx context.Context, ids []string) ([]ClaimableBalance, error)
	CountClaimableBalances(ctx context.Context) (int, error)
	NewClaimableBalanceClaimantBatchInsertBuilder(maxBatchSize int) ClaimableBalanceClaimantBatchInsertBuilder
	GetClaimantsByClaimableBalances(ctx context.Context, ids []string) (map[string][]ClaimableBalanceClaimant, error)
}

// CountClaimableBalances returns the total number of claimable balances in the DB
func (q *Q) CountClaimableBalances(ctx context.Context) (int, error) {
	sql := sq.Select("count(*)").From("claimable_balances")

	var count int
	if err := q.Get(ctx, &count, sql); err != nil {
		return 0, errors.Wrap(err, "could not run select query")
	}

	return count, nil
}

// GetClaimableBalancesByID finds all claimable balances by ClaimableBalanceId
func (q *Q) GetClaimableBalancesByID(ctx context.Context, ids []string) ([]ClaimableBalance, error) {
	var cBalances []ClaimableBalance
	sql := selectClaimableBalances.Where(map[string]interface{}{"cb.id": ids})
	err := q.Select(ctx, &cBalances, sql)
	return cBalances, err
}

// GetClaimantsByClaimableBalances finds all claimants for ClaimableBalanceIds.
// The returned list is sorted by ids and then destination ids for each balance id.
func (q *Q) GetClaimantsByClaimableBalances(ctx context.Context, ids []string) (map[string][]ClaimableBalanceClaimant, error) {
	var claimants []ClaimableBalanceClaimant
	sql := sq.Select("*").From("claimable_balance_claimants cbc").
		Where(map[string]interface{}{"cbc.id": ids}).
		OrderBy("id asc, destination asc")
	err := q.Select(ctx, &claimants, sql)

	claimantsMap := make(map[string][]ClaimableBalanceClaimant)
	for _, claimant := range claimants {
		claimantsMap[claimant.BalanceID] = append(claimantsMap[claimant.BalanceID], claimant)
	}
	return claimantsMap, err
}

// UpsertClaimableBalances upserts a batch of claimable balances in the claimable_balances table.
// There's currently no limit of the number of offers this method can
// accept other than 2GB limit of the query string length what should be enough
// for each ledger with the current limits.
func (q *Q) UpsertClaimableBalances(ctx context.Context, cbs []ClaimableBalance) error {
	var id, claimants, asset, amount, sponsor, lastModifiedLedger, flags []interface{}

	for _, cb := range cbs {
		id = append(id, cb.BalanceID)
		claimants = append(claimants, cb.Claimants)
		asset = append(asset, cb.Asset)
		amount = append(amount, cb.Amount)
		sponsor = append(sponsor, cb.Sponsor)
		lastModifiedLedger = append(lastModifiedLedger, cb.LastModifiedLedger)
		flags = append(flags, cb.Flags)
	}

	upsertFields := []upsertField{
		{"id", "text", id},
		{"claimants", "jsonb", claimants},
		{"asset", "text", asset},
		{"amount", "bigint", amount},
		{"sponsor", "text", sponsor},
		{"last_modified_ledger", "integer", lastModifiedLedger},
		{"flags", "int", flags},
	}

	return q.upsertRows(ctx, "claimable_balances", "id", upsertFields)
}

// RemoveClaimableBalances deletes claimable balances table.
// Returns number of rows affected and error.
func (q *Q) RemoveClaimableBalances(ctx context.Context, ids []string) (int64, error) {
	sql := sq.Delete("claimable_balances").
		Where(sq.Eq{"id": ids})
	result, err := q.Exec(ctx, sql)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// RemoveClaimableBalanceClaimants deletes claimable balance claimants.
// Returns number of rows affected and error.
func (q *Q) RemoveClaimableBalanceClaimants(ctx context.Context, ids []string) (int64, error) {
	sql := sq.Delete("claimable_balance_claimants").
		Where(sq.Eq{"id": ids})
	result, err := q.Exec(ctx, sql)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// FindClaimableBalanceByID returns a claimable balance.
func (q *Q) FindClaimableBalanceByID(ctx context.Context, balanceID string) (ClaimableBalance, error) {
	var claimableBalance ClaimableBalance
	sql := selectClaimableBalances.Limit(1).Where("cb.id = ?", balanceID)
	err := q.Get(ctx, &claimableBalance, sql)
	return claimableBalance, err
}

// GetClaimableBalances finds all claimable balances where accountID is one of the claimants
func (q *Q) GetClaimableBalances(ctx context.Context, query ClaimableBalancesQuery) ([]ClaimableBalance, error) {
	l, r, err := query.Cursor()
	if err != nil {
		return nil, errors.Wrap(err, "error getting cursor")
	}

	sql, err := applyClaimableBalancesQueriesCursor(selectClaimableBalances, l, r, query.PageQuery.Order)
	if err != nil {
		return nil, errors.Wrap(err, "could not apply query to page")
	}

	if query.Asset != nil {
		// when search by asset, profiling has shown best performance to have the LIMIT on inner query
		sql = sql.Where("cb.asset = ?", query.Asset)
	}

	if query.Sponsor != nil {
		sql = sql.Where("cb.sponsor = ?", query.Sponsor.Address())
	}

	if query.Claimant != nil {
		var selectClaimableBalanceClaimants = sq.Select("id").From("claimable_balance_claimants").
			Where("destination = ?", query.Claimant.Address()).
			// Given that each destination can be a claimant for each balance maximum once
			// we can LIMIT the subquery.
			Limit(query.PageQuery.Limit)
		subSql, err := applyClaimableBalancesQueriesCursor(selectClaimableBalanceClaimants, l, r, query.PageQuery.Order)
		if err != nil {
			return nil, errors.Wrap(err, "could not apply subquery to page")
		}

		subSqlString, subSqlArgs, err := subSql.ToSql()
		if err != nil {
			return nil, errors.Wrap(err, "could not build subquery")
		}

		sql = sql.
			Where(fmt.Sprintf("cb.id IN (%s)", subSqlString), subSqlArgs...)
	}

	sql = sql.Limit(query.PageQuery.Limit)

	var results []ClaimableBalance
	if err := q.Select(ctx, &results, sql); err != nil {
		return nil, errors.Wrap(err, "could not run select query")
	}

	return results, nil
}

var claimableBalancesSelectStatement = "cb.id, " +
	"cb.claimants, " +
	"cb.asset, " +
	"cb.amount, " +
	"cb.sponsor, " +
	"cb.last_modified_ledger, " +
	"cb.flags"

var selectClaimableBalances = sq.Select(claimableBalancesSelectStatement).From("claimable_balances cb")
