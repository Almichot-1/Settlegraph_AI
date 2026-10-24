# SettleGraph AI — implementation progress

**As of:** 2026-10-24 UTC
**Scope:** Local synthetic backend, with separate product and BankSim databases
**Revision:** See Git history for the reviewed backend commit
**Remote:** `https://github.com/Almichot-1/Settlegraph_AI.git` (empty at review time)

## What runs

Three Go services start under Docker Compose: public API, PostgreSQL-backed worker, and BankSim. Both databases migrate and seed independently. The demo API can create and advance normal, delayed settlement, duplicate webhook, and provider outage scenarios. BankSim uses a balanced journal and signed provider events. SettleGraph stores deduplicated inbox events and observations, runs two-stage reconciliation with source watermarks, persists immutable proofs, exposes dashboard/account/transaction/case reads, and records versioned human decisions. Account views contain known synthetic metadata without unobserved balances. AI investigations currently use the labeled offline evidence-linked stub.

## Verification executed

| Area | Evidence | Result |
|---|---|---|
| Contracts | `python validation/validate_pack.py` | Passed: 39 public and 10 internal operations, fixtures, schemas, backlog graph |
| Go | `go test ./...`, `go vet ./...`, `go build ./cmd/...` | Passed |
| Database | Fresh PostgreSQL databases: migrations twice, seeds twice | Passed; opening journal sums to zero |
| BankSim concurrency/lifecycle | `TestConcurrentLedgerEffects` with isolated PostgreSQL, 100 same-key retries and 100 distinct attempts | Passed; no overspend, balanced journals, valid CREATED → ACCEPTED → SETTLEMENT_PENDING → SETTLED and CREATED → FAILED history, invalid transition rejected |
| Reconciliation concurrency | `TestConcurrentRunDeduplication` with isolated PostgreSQL, 20 simultaneous same-key runs | Passed; one immutable run with two unique proof edges |
| Demo identity and tenancy | `TestDemoIdentityRoleAndTenantBoundary` with isolated PostgreSQL | Passed; 401 invalid token, 403 viewer mutation, 404 foreign organization, 400 malformed JSON |
| Local runtime | `make up`, `make smoke`, `make test-e2e` | Passed for all four synthetic scenarios, late evidence, human review, duplicate delivery, outage recovery, invalid signatures |
| Delivery recovery | `python tests/e2e_delivery_recovery.py` | Passed twice; pending event redelivered after BankSim restart, product inbox deduplicated, source cursor/coverage and reconciliation recovered |
| Live JSON contracts | `.venv/bin/python tests/contract_runtime.py` | Passed for demo login, 13 list/read surfaces, selected detail/evidence/match/proof routes against public OpenAPI schemas |

The detailed ticket states are in `AGENT_BACKLOG.json`. `TESTED` means its stated backend acceptance has direct evidence; it does not mean client or cloud acceptance.

## Open work and blockers

- SG-002 and SG-020 need the Next.js and Flutter clients to render the frozen fixtures and acknowledge the endpoint table; those client projects are not present in this repository.
- Production identity and distributed rate limits remain cloud hardening work; production API startup fails closed.
- Source coverage deliberately refuses to claim completeness when a provider event is absent. Local automatic delivery retry and coverage repair passed; cloud queue integration and broader fault testing remain with SG-021.
- SG-014 account metadata is seeded and listed without private balances; no provider account-balance observations are available yet.
- SG-017 Bedrock adapter is absent; the offline investigator is operational and evidence bounded.
- Optional import creation/upload/parse routes are absent; `GET /v1/imports` returns an empty page.
- SG-021 cloud deploy is not configured: no AWS credentials, IAM, TLS, budget alarm or cloud smoke. SG-022 final acceptance and two fresh-seed rehearsals remain.

No real bank integration or live funds are involved. Local Compose PostgreSQL trust authentication is limited to unpublished database networks and must be replaced before cloud deployment.

## Recovery and next checks

`make down` stops services without deleting volumes. `make up && make migrate && make seed && make smoke` restarts the local demo. `make test-e2e` verifies scenario behavior after restart. Do not run `docker compose down -v` when retaining evidence is required.
