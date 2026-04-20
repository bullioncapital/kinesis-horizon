# Agents Notes — kinesis-horizon

This file captures institutional knowledge for AI agents working on this fork.
Update it after every non-trivial fix so future agents can work faster.

---

## Repo Overview

This is a **fork of `stellar/go`** that modifies the XDR transaction encoding.
The primary change is that `Transaction.Fee` (and `TransactionV0.Fee`) was
widened from a 4-byte `Uint32` to an 8-byte `Uint64`.  That single structural
change cascades into:

- Different base64 XDR blobs for any transaction that encodes fee bytes.
- Different SHA-256 transaction hashes (since the hash covers the full XDR).
- Test SQL fixtures and hardcoded hash strings in test files that must be kept
  in sync with whatever transactions are used in those fixtures.

---

## Key Files

| Path | Purpose |
|------|---------|
| `services/horizon/internal/test/scenarios/*.sql` | SQL "scenario" fixtures loaded by integration-style tests |
| `services/horizon/internal/test/scenarios/bindata.go` | Go-embed of the SQL files; **must be regenerated** after any SQL edit |
| `services/horizon/internal/actions_transaction_test.go` | High-level transaction HTTP tests — hashes already updated |
| `services/horizon/internal/actions_effects_test.go` | Effects tests — contains hardcoded tx hashes |
| `services/horizon/internal/actions_operation_test.go` | Operation HTTP tests — contains hardcoded tx hashes |
| `services/horizon/internal/actions_payment_test.go` | Payment HTTP tests — contains hardcoded tx hashes |
| `services/horizon/internal/actions/operation_test.go` | Unit-level operation handler tests |
| `services/horizon/internal/db2/history/operation_test.go` | DB-layer operation tests |
| `services/horizon/internal/db2/history/transaction_test.go` | DB-layer transaction tests |
| `services/horizon/internal/txsub/results_test.go` | Tx-submission result lookup tests |

---

## Current Fixture → Hash Mapping

These are the **forked** (post-XDR-change) hashes used in the SQL fixtures and
all test files as of the last full passing run.  Update this table whenever
fixtures are regenerated.

### Scenario: `base`

| Role | Transaction Hash |
|------|-----------------|
| TX1 (only tx, ledger 2) | `ff5cba32e8918327f1d563f57cd54dc5f5906f33ce53aeb119df06a16f797387` |

### Scenario: `failed_transactions`

| Role | Transaction Hash |
|------|-----------------|
| Failed TX (ledger 5, `successful=false`) | `e34941080e33bf0ce90c7fac31ec13a0f7e9e5489204e766c3def374164aa3fa` |
| Successful TX (ledger 4, `successful=true`) | `263b8b93313084b938891187081f8c6d2a756f7f2cb1fc5aaa2f7737517e7f4c` |

### Pre-fork hashes (for reference / grep)

These hashes appear in git history and older build logs.  They must NOT appear
in any currently-active test file or fixture.

| Role | Old Hash |
|------|----------|
| base TX1 | `2374e99349b9ef7dba9a5db3339b78fda8f34777b1af33ba468ad5c0df946d4d` |
| failed_transactions — failed TX | `aa168f12124b7c196c0adaee7c73a64d37f99428cacb59a91ff389626845e7cf` |
| failed_transactions — successful TX | `56e3216045d579bea40f2d35a09406de3a894ecb5be70dbda5ec9c0427a0d5a1` |

---

## Upstream Merge Workflow

When pulling upstream `stellar/go` commits that touch XDR or transaction encoding:

### 1. Merge and build

```sh
git merge upstream/master   # or rebase
go build ./...              # confirm it compiles
```

### 2. Identify fixture breakage

Run the Horizon test suite and collect failures:

```sh
go test -race -cover ./services/horizon/... 2>&1 | tee /tmp/horizon_test.txt
grep "^--- FAIL\|^FAIL" /tmp/horizon_test.txt
```

Common failure signatures caused by hash drift:
- `expected: 200 / actual: 404` on routes like `/transactions/{hash}/effects`
- `sql: no rows in result set` in DB-layer tests
- `expected: "<old-hash>" / actual: "<new-hash>"` in assertion output
- Panics from nil/empty slice access when a tx lookup returns nothing

### 3. Compute new XDR + hashes

Write a small Go helper (e.g. `/tmp/gen_xdr.go`) that constructs the same
transactions using the fork's XDR structs and prints:
- `xdr.MarshalBase64(envelope)` — the new `envelope_xdr` column value
- `network.HashTransactionV0` or `network.HashTransaction` — the new hash

Use the existing SQL fixture as the template for transaction parameters
(source account, sequence number, operations, fee, etc.).

### 4. Update SQL fixtures

In `services/horizon/internal/test/scenarios/`:
- Replace old `transaction_hash` values in `INSERT INTO history_transactions`.
- Replace old `envelope_xdr` base64 blobs.
- Search for the old hash everywhere in the file (it appears in multiple tables:
  `history_transactions`, `history_transactions_filtered_tmp`, indices, etc.).

Quick sanity check — no old hash should remain:
```sh
grep -c "<old-hash>" services/horizon/internal/test/scenarios/*.sql
```

### 5. Regenerate bindata.go

```sh
go generate ./services/horizon/internal/test/scenarios/
```

Confirm `bindata.go` timestamp changed.  If the `go:generate` directive is not
present, look for `go-bindata` or equivalent invocation in `Makefile`.

### 6. Update test files

Search for hardcoded old hashes across all `*_test.go` files and replace with
new ones.  The safest approach is targeted `sed -i` per hash:

```sh
OLD="<old-hash>"
NEW="<new-hash>"
grep -rl "$OLD" services/horizon/ --include="*_test.go" \
  | xargs sed -i "s/$OLD/$NEW/g"
```

Repeat for each changed hash.  After substitution, verify nothing is left:

```sh
grep -rn "$OLD" services/horizon/ --include="*_test.go"
```

**Important distinctions:**
- Files that use hashes only as **format-validation examples** (e.g.
  `validators_test.go`, `helpers_test.go`, `resourceadapter/*_test.go`,
  `txsub/system_test.go`) do **not** need updating — they never do DB lookups.
- Files that use hashes to **look up rows in a test DB** do need updating.
  These are the ones that produce `sql: no rows` or `404` failures.

### 7. Verify

```sh
go test -race -cover ./services/horizon/... 2>&1 | grep "^FAIL\|^--- FAIL"
# Should be empty
```

---

## Pitfalls Encountered

1. **Panic hides later failures.** If an early test panics (e.g. index-out-of-
   range on an empty slice returned by a hash-lookup), the test binary crashes
   and tests that follow in the same package never run.  Always check both
   `--- FAIL` lines *and* `panic:` lines in test output.

2. **bindata.go must be regenerated.** Editing the `.sql` files is not enough —
   tests embed fixture data via `bindata.go`.  Forgetting this step causes tests
   to silently use the old SQL content.

3. **Multiple tables reference the same hash.** In the scenario SQL files the
   transaction hash appears in `history_transactions`, in
   `history_transactions_filtered_tmp`, and is referenced by foreign-key-like
   columns elsewhere.  A simple global replace handles all of them.

4. **Uppercase hash variants in tests.** Some tests assert that uppercase hex
   hashes return HTTP 400.  Those strings are format-only (no DB lookup) and
   need not match the real hash — leave them unless you want cosmetic
   consistency.

5. **`TestExtraChecks*` tests do an in-place SQL UPDATE then re-query.** They
   update `successful` on a specific `transaction_hash` to simulate data
   corruption.  If the hash in the UPDATE no longer exists in the fixture the
   UPDATE silently affects 0 rows, the corruption condition is never triggered,
   and the test fails expecting an error but getting nil — then panics on the
   nil dereference.

---

## Test Run Baseline (last full passing run)

Command: `go test -race -cover ./services/horizon/...`
Result: **all packages pass, exit code 0**

Relevant coverage numbers:
- `internal` — 46.5 %
- `internal/actions` — 53.7 %
- `internal/db2/history` — 65.3 %
- `internal/txsub` — 76.7 %
- `internal/ingest` — 75.2 %
- `internal/resourceadapter` — 77.8 %
