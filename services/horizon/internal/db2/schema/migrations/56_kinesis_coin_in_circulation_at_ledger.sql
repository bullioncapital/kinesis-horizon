-- +migrate Up
-- +migrate StatementBegin
DROP FUNCTION IF EXISTS kinesis_coin_in_circulation_raw(character varying,character varying,character varying,character varying);
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


CREATE OR REPLACE FUNCTION kinesis_coin_in_circulation_at_ledger(
    IN root_account VARCHAR(56),
    IN emission_account VARCHAR(56),
	IN hot_account VARCHAR(56),
    IN inflation_account VARCHAR(56),
    IN ledger_id INT
)
RETURNS TABLE(
    last_ledger_timestamp timestamp without time zone,
    last_ledger INT,                     -- last ledger of the day where mint/redemption happened
    circulation  NUMERIC(18, 7),
    mint NUMERIC(18, 7),
    redemption NUMERIC(18, 7)
)
LANGUAGE 'plpgsql'
AS $$
BEGIN
	RETURN QUERY

    SELECT
        MAX(cc.closed_at) as last_ledger_timestamp,
        MAX(cc.ledger) as last_ledger,
        SUM(cc.minting - cc.redemption) as total_coins,
        SUM(cc.minting) as minted,
        SUM(cc.redemption) as redemption
    FROM kinesis_coin_in_circulation_raw(
        root_account,
        emission_account,
        hot_account,
        inflation_account
    ) as cc
    WHERE cc.ledger <= ledger_id;

END;
$$
-- +migrate StatementEnd

-- +migrate Down
DROP FUNCTION IF EXISTS kinesis_coin_in_circulation_at_ledger(character varying,character varying,character varying,character varying,int);
