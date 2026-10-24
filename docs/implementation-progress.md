# SettleGraph AI — implementation progress

**As of:** 2026-10-24 UTC
**Scope:** Local synthetic backend, with separate product and BankSim databases
**Revision:** See Git history for the reviewed backend commit
**Remote:** `https://github.com/Almichot-1/Settlegraph_AI.git` (empty at review time)

## What runs

Three Go services start under Docker Compose: public API, PostgreSQL-backed worker, and BankSim. Both databases migrate and seed independently. The demo API can create and advance normal, delayed settlement, duplicate webhook, and provider outage scenarios. BankSim uses a balanced journal and signed provider events. SettleGraph stores deduplicated inbox events and observations, runs two-stage reconciliation with source watermarks, persists immutable proofs, exposes dashboard/transaction/case reads, and records versioned human decisions. AI investigations currently use the labeled offline evidence-linked stub.

## Verification executed

| Area | Evidence | Result |
|---|---|---|
| Contracts | `python validation/validate_pack.py` | Passed: 39 public and 10 internal operations, fixtures, schemas, backlog graph |
| Go | `go test ./...`, `go vet ./...`, `go build ./cmd/...` | Passed |
| Database | Fresh PostgreSQL databases: migrations twice, seeds twice | Passed; opening journal sums to zero |
| BankSim concurrency/lifecycle | `TestConcurrentLedgerEffects` with isolated PostgreSQL, 100 same-key retries and 100 distinct attempts | Passed; no overspend, balanced journals, valid CREATED → ACCEPTED → SETTLEMENT_PENDING → SETTLED and CREATED → FAILED history, invalid transition rejected |
| Local runtime | `make up`, `make smoke`, `make test-e2e` | Passed for all four synthetic scenarios, late evidence, human review, duplicate delivery, outage recovery, invalid signatures |

The detailed ticket states are in `AGENT_BACKLOG.json`. `TESTED` means its stated backend acceptance has direct evidence; it does not mean client or cloud acceptance.

## Open work and blockers

- SG-002 and SG-020 need the Next.js and Flutter clients to render the frozen fixtures and acknowledge the endpoint table; those client projects are not present in this repository.
- SG-004 production identity, rate limits and broader role tests remain; production API startup fails closed.
- SG-010 needs automatic backfill/retry across failed webhook delivery; ten-delivery deduplication and worker restart replay passed. Source coverage deliberately refuses to claim completeness when an event is absent.
- SG-013 still needs concurrent run deduplication testing. SG-014 account observations are not implemented; `GET /v1/accounts` returns an empty page.
- SG-017 Bedrock adapter is absent; the offline investigator is operational and evidence bounded.
- Optional import creation/upload/parse routes are absent; `GET /v1/imports` returns an empty page.
- SG-021 cloud deploy is not configured: no AWS credentials, IAM, TLS, budget alarm or cloud smoke. SG-022 final acceptance and two fresh-seed rehearsals remain.

No real bank integration or live funds are involved. Local Compose PostgreSQL trust authentication is limited to unpublished database networks and must be replaced before cloud deployment.

## Recovery and next checks

`make down` stops services without deleting volumes. `make up && make migrate && make seed && make smoke` restarts the local demo. `make test-e2e` verifies scenario behavior after restart. Do not run `docker compose down -v` when retaining evidence is required.
