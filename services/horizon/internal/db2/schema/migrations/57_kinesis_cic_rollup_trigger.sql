-- +migrate Up
-- +migrate StatementBegin

-- 1. Create the Ledger-Level Rollup Table
-- Each row maps 1:1 to a history_operations row so that re-running the backfill
-- or a duplicate trigger fire is fully idempotent (ON CONFLICT DO NOTHING).
CREATE TABLE IF NOT EXISTS kinesis_ledger_ops_rollup (
    id BIGINT NOT NULL,
    ledger INT NOT NULL,
    tx_date DATE NOT NULL,
    closed_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    operation_type VARCHAR(20) NOT NULL,
    source_account VARCHAR(56) NOT NULL,
    dest_account VARCHAR(56) NOT NULL,
    total_amount NUMERIC(18, 7) NOT NULL DEFAULT 0,
    PRIMARY KEY (id)
);

-- Fast lookup indexes for API Window Queries
CREATE INDEX IF NOT EXISTS idx_klor_source_account ON kinesis_ledger_ops_rollup(source_account);
CREATE INDEX IF NOT EXISTS idx_klor_dest_account ON kinesis_ledger_ops_rollup(dest_account);
CREATE INDEX IF NOT EXISTS idx_klor_tx_date ON kinesis_ledger_ops_rollup(tx_date);

CREATE OR REPLACE FUNCTION cleanup_cic_rollup_table(
    IN root_account VARCHAR(56),
    IN emission_account VARCHAR(56),
    IN hot_account VARCHAR(56),
    IN inflation_account VARCHAR(56)
)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM kinesis_ledger_ops_rollup
    WHERE NOT (
        -- Keep Minting events
        (source_account = emission_account AND dest_account <> root_account)
        OR
        -- Keep Redemption events
        (source_account = hot_account AND (dest_account = emission_account OR dest_account = root_account))
    )
    -- Explicitly delete inflation accounts or anything else
    OR source_account = inflation_account
    OR dest_account = inflation_account;
END;
$$;

-- 2. Backfill Existing Data
-- This runs the heavy parsing one time to seed the rollup table.
-- Grouping by ops.id (the PK of history_operations) is sufficient: PostgreSQL infers
-- that ops.type and ops.details are functionally dependent on ops.id, so the CASE
-- expressions in SELECT do not need to be repeated in GROUP BY.
-- l.sequence and l.closed_at are included in GROUP BY explicitly because they come
-- from a separate table (history_ledgers).
-- ON CONFLICT DO NOTHING makes this statement fully safe to re-run.
INSERT INTO kinesis_ledger_ops_rollup (id, ledger, tx_date, closed_at, operation_type, source_account, dest_account, total_amount)
SELECT
    ops.id,
    l.sequence AS ledger,
    l.closed_at::date AS tx_date,
    l.closed_at,
    (CASE
        WHEN ops.type = 0 THEN 'create_account'
        WHEN ops.type = 1 THEN 'payment'
        WHEN ops.type = 8 THEN 'merge'
        ELSE 'N/A'
    END)::varchar(20) AS operation_type,
    (CASE
        WHEN ops.type = 0 THEN (ops.details::jsonb)->>'funder'
        WHEN ops.type = 1 THEN (ops.details::jsonb)->>'from'
        WHEN ops.type = 8 THEN (ops.details::jsonb)->>'account'
        ELSE 'N/A'
    END)::varchar(56) AS source_account,
    (CASE
        WHEN ops.type = 0 THEN (ops.details::jsonb)->>'account'
        WHEN ops.type = 1 THEN (ops.details::jsonb)->>'to'
        WHEN ops.type = 8 THEN (ops.details::jsonb)->>'into'
        ELSE 'N/A'
    END)::varchar(56) AS dest_account,
    (CASE
        WHEN ops.type = 0 THEN (ops.details::jsonb)->>'starting_balance'
        ELSE (ops.details::jsonb)->>'amount'
    END)::decimal(18,7) AS total_amount
FROM history_transactions tx
  INNER JOIN history_operations ops ON tx.id = ops.transaction_id
  INNER JOIN history_ledgers l ON tx.ledger_sequence = l.sequence
WHERE ops.type IN (0, 1) AND tx.successful = true
ON CONFLICT (id) DO NOTHING;

-- 3. Rewrite `kinesis_coin_in_circulation_raw` to dramatically speed up queries
CREATE OR REPLACE FUNCTION kinesis_coin_in_circulation_raw(
    IN root_account VARCHAR(56),
    IN emission_account VARCHAR(56),
	IN hot_account VARCHAR(56),
    IN inflation_account VARCHAR(56)
)
RETURNS TABLE(
    tx_date DATE,
    closed_at timestamp without time zone,
    ledger INT,
    operation_type VARCHAR(20),
    tx_type VARCHAR(20),
    source_account VARCHAR(56),
    dest_account VARCHAR(56),
    minting NUMERIC(18, 7),
    redemption NUMERIC(18, 7)
)
LANGUAGE 'plpgsql'
AS $$
BEGIN
	RETURN QUERY

    SELECT
        t.tx_date::date as tx_date,
        t.closed_at,
        t.ledger,
        t.operation_type,
        t.tx_type,
        t.source_account,
        t.dest_account,
        (CASE
            WHEN t.tx_type = 'Minting' THEN t.total_amount
            ELSE 0.0
        END)::decimal(18,7) as minting,
        (CASE
            WHEN t.tx_type = 'Redemption' THEN t.total_amount
            ELSE 0.0
        END)::decimal(18,7) as redemption
    FROM (
        -- 1. Minting
        SELECT
            qry.tx_date,
            qry.closed_at,
            qry.ledger,
            qry.operation_type,
            qry.source_account,
            qry.dest_account,
            qry.total_amount,
            'Minting'::varchar(20) as tx_type
        FROM kinesis_ledger_ops_rollup as qry
        WHERE qry.source_account = emission_account
        AND qry.dest_account <> root_account
        AND qry.dest_account <> inflation_account

        UNION ALL

        -- 2. Redemption
        SELECT
            qry.tx_date,
            qry.closed_at,
            qry.ledger,
            qry.operation_type,
            qry.source_account,
            qry.dest_account,
            qry.total_amount,
            'Redemption'::varchar(20) as tx_type
        FROM kinesis_ledger_ops_rollup as qry
        WHERE qry.source_account = hot_account
        AND (qry.dest_account = emission_account OR qry.dest_account = root_account)
        AND qry.dest_account <> inflation_account
    ) t;
END;
$$;
-- +migrate StatementEnd

-- +migrate Down
-- +migrate StatementBegin
DROP INDEX IF EXISTS idx_klor_source_account;
DROP INDEX IF EXISTS idx_klor_dest_account;
DROP INDEX IF EXISTS idx_klor_tx_date;
DROP TABLE IF EXISTS kinesis_ledger_ops_rollup;

-- Restore kinesis_coin_in_circulation_raw to original logic using view
CREATE OR REPLACE FUNCTION kinesis_coin_in_circulation_raw(
    IN root_account VARCHAR(56),
    IN emission_account VARCHAR(56),
	IN hot_account VARCHAR(56),
    IN inflation_account VARCHAR(56)
)
RETURNS TABLE(
    tx_date DATE,
    closed_at timestamp without time zone,
    ledger INT,
    operation_type VARCHAR(20),
    tx_type VARCHAR(20),
    source_account VARCHAR(56),
    dest_account VARCHAR(56),
    minting NUMERIC(18, 7),
    redemption NUMERIC(18, 7)
)
LANGUAGE 'plpgsql'
AS $$
BEGIN
	RETURN QUERY

    SELECT
        t.tx_date::date as tx_date,
        t.closed_at,
        t.ledger,
        t.operation_type,
        t.tx_type,
        t.source_account,
        t.dest_account,
        (CASE
            WHEN t.tx_type = 'Minting' THEN t.amount
            ELSE 0.0
        END)::decimal(18,7) as minting,
        (CASE
            WHEN t.tx_type = 'Redemption' THEN t.amount
            ELSE 0.0
        END)::decimal(18,7) as redemption
    FROM (
        SELECT
            qry.*,
        (CASE
            WHEN (
                qry.source_account = emission_account 	-- emission
                AND qry.dest_account <> root_account 	-- non-root
            ) THEN 'Minting' -- emission to non-root account
            WHEN (
                qry.source_account = hot_account -- Hot wallet
                AND (
                qry.dest_account = emission_account 	-- emission
                OR qry.dest_account = root_account -- root
                )
            ) THEN 'Redemption' -- hot wallet to emission/root
            ELSE 'N/A'
        END)::varchar(20) as tx_type
        FROM v_create_account_merge_and_payment_ops as qry
        WHERE qry.source_account <> inflation_account
        AND qry.dest_account <> inflation_account
    ) t WHERE t.tx_type <> 'N/A';
END;
$$;
-- +migrate StatementEnd
