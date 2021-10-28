#! /usr/bin/env bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
GOTOP="$( cd "$DIR/../../../../../../../.." && pwd )"
PACKAGES=$(find $GOTOP/src/github.com/stellar/go/services/horizon/internal/test/scenarios -iname '*.rb' -not -name '_common_accounts.rb')
# PACKAGES=$(find $GOTOP/src/github.com/stellar/go/services/horizon/internal/test/scenarios -iname 'kahuna.rb')

go install github.com/stellar/go/services/horizon

dropdb hayashi_scenarios --if-exists
createdb hayashi_scenarios

export STELLAR_CORE_DATABASE_URL="postgres://localhost/hayashi_scenarios?sslmode=disable"
export STELLAR_HORIZON_DATABASE_URL="postgres://localhost/horizon_scenarios?sslmode=disable"
export DATABASE_URL="postgres://localhost/horizon_scenarios?sslmode=disable"
export NETWORK_PASSPHRASE="Test SDF Network ; September 2015"
export STELLAR_CORE_URL="http://localhost:8080"
export SKIP_CURSOR_UPDATE="true"

# run all scenarios
for i in $PACKAGES; do
  CORE_SQL="${i%.rb}-core.sql"
  HORIZON_SQL="${i%.rb}-horizon.sql"
  bundle exec scc -r $i --dump-root-db > $CORE_SQL

  # load the core scenario
  psql $STELLAR_CORE_DATABASE_URL < $CORE_SQL

  # recreate horizon dbs
  dropdb horizon_scenarios --if-exists
  createdb horizon_scenarios

  # Load the horizon sql
  psql $STELLAR_HORIZON_DATABASE_URL < $HORIZON_SQL

  # Run updates against the correct tables
  psql -d horizon_scenarios -c "ALTER TABLE history_ledgers add base_percentage_fee INTEGER" || true
  psql -d horizon_scenarios -c "UPDATE history_ledgers set base_percentage_fee = 45" || true

  # Run updates against the correct tables
  psql -d horizon_scenarios -c "ALTER TABLE history_ledgers add max_fee BIGINT" || true
  psql -d horizon_scenarios -c "UPDATE history_ledgers set max_fee = 1000" || true

  # write horizon data to sql file
  pg_dump $DATABASE_URL \
    --clean --if-exists --no-owner --no-acl --inserts \
    | sed '/SET idle_in_transaction_session_timeout/d' \
    | sed '/SET row_security/d' \
    > $HORIZON_SQL
done


# commit new sql files to bindata
go generate github.com/stellar/go/services/horizon/internal/test/scenarios
# go test github.com/stellar/go/services/horizon/internal/ingest
