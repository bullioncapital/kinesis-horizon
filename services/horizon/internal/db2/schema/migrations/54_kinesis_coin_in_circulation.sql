-- +migrate Up
-- +migrate StatementBegin
CREATE OR REPLACE VIEW v_create_account_merge_and_payment_ops
AS
SELECT
    tx.transaction_hash,
    tx.created_at::date as tx_date,
    tx.created_at,
    tx.fee_charged/10000000.0 fee_paid,
    case 
        WHEN ops.type = 0 THEN 'create_account' 
        WHEN ops.type = 1 THEN 'payment'
        WHEN ops.type = 8 THEN 'merge'
        ELSE 'N/A'
    end as operation_type,
    case 
        WHEN ops.type = 0 THEN (ops.details::jsonb)->>'funder'
        WHEN ops.type = 1 THEN (ops.details::jsonb)->>'from'
        WHEN ops.type = 8 THEN (ops.details::jsonb)->>'account'
        ELSE 'N/A'
    end as source_account,
    case 
        WHEN ops.type = 0 THEN (ops.details::jsonb)->>'account'
        WHEN ops.type = 1 THEN (ops.details::jsonb)->>'to'
        WHEN ops.type = 8 THEN (ops.details::jsonb)->>'into'
        ELSE 'N/A'
    end as dest_account,
    (case 
        WHEN ops.type = 0 THEN (ef.details::jsonb)->>'starting_balance'
        ELSE (ef.details::jsonb)->>'amount'
    end)::decimal(18,7) as amount
FROM history_transactions tx
  INNER JOIN history_operations ops ON tx.id = ops.transaction_id
  INNER JOIN history_effects ef ON ops.id = ef.history_operation_id AND ef.type IN (
    0, -- account_created
    2  -- account_credited
  )
WHERE ops.type IN (
    0, -- CREATE_ACCOUNT6
    1, -- PAYMENT
    8  -- ACCOUNT_MERGE
);

DROP FUNCTION IF EXISTS kinesis_coin_in_circulation(character varying,character varying,character varying,character varying);
CREATE OR REPLACE FUNCTION kinesis_coin_in_circulation(
    IN root_account VARCHAR(56),
    IN emission_account VARCHAR(56),
	IN hot_account VARCHAR(56),
    IN inflation_account VARCHAR(56)
)
RETURNS TABLE(
    tx_date DATE,
    total_coins  NUMERIC(18, 7),
    minted NUMERIC(18, 7),
    redemption NUMERIC(18, 7)
)
LANGUAGE 'plpgsql'
AS $$
BEGIN
	RETURN QUERY
    with data as (
        SELECT
            cc.tx_date,
            SUM(cc.minting - cc.redemption) as total_coins,
            SUM(cc.minting) as minted,
            SUM(cc.redemption) as redemption
        FROM (
            -- raw data
            SELECT
                t.transaction_hash,
                t.tx_date,
                t.created_at,
                t.operation_type,
                t.tx_type,
                t.source_account,
                t.dest_account,
                (CASE
                    WHEN t.source_account IN (
						emission_account 	-- emission
                        ,root_account 	-- root
                    ) THEN 0.0 -- exclude fee
                    ELSE t.fee_paid
                END) fee_paid,
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
                END) as tx_type
                FROM v_create_account_merge_and_payment_ops as qry
                WHERE qry.source_account <> inflation_account
                AND qry.dest_account <> inflation_account
            ) t WHERE t.tx_type <> 'N/A'
            ORDER BY t.created_at ASC
        ) as cc
        GROUP BY cc.tx_date 
        ORDER BY cc.tx_date ASC
    )

    SELECT 
        tx_date,
        sum(total_coins) over (order by tx_date asc rows between unbounded preceding and current row) circulation,
        sum(minted) over (order by tx_date asc rows between unbounded preceding and current row) minted,
        sum(redemption) over (order by tx_date asc rows between unbounded preceding and current row) redemption
    FROM data ORDER BY tx_date ASC;
END;
$$
-- +migrate StatementEnd

-- +migrate Down
DROP FUNCTION IF EXISTS kinesis_coin_in_circulation(character varying,character varying,character varying,character varying);
DROP VIEW IF EXISTS v_create_account_merge_and_payment_ops;
