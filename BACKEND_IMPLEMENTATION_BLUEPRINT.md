# SETTLEGRAPH AI
## Solo-backend implementation blueprint and API handoff

**Version:** 2.0 — contract freeze candidate  
**Prepared:** 24 October 2026  
**Audience:** Backend implementation agent, Go engineer, Next.js engineer, Flutter engineer  
**Status:** Proposed engineering specification. The endpoints and code described below are **not yet implemented**.  
**Primary deliverables:** This document, `contracts/settlegraph-public.openapi.yaml`, `contracts/banksim-internal.openapi.yaml`, `contracts/bank-event.schema.json`, `contracts/ai-investigation.schema.json`, SQL migration starters, fixtures and validation scripts.

> **One-sentence target:** A simulated customer pays a merchant through a simulated financial provider; SettleGraph independently observes that payment, matches it against an order and a settlement, identifies an injected delay or contradiction, creates an auditable case, and optionally uses Amazon Bedrock to explain the evidence without controlling payment state.

## 0. Read this before writing a line of backend code

### 0.1 The change in team composition

The three-backend-developer architecture has become a **solo-backend** architecture. Do not turn the simulator into a bank-core product. There are **three Go executables but one Go repository**: `cmd/api`, `cmd/worker`, `cmd/banksim`; they can share foundational Go packages and OpenAPI contracts but must not share BankSim financial data directly with SettleGraph business logic. Frontend and Flutter independently consume a **frozen public HTTP contract**.

### 0.2 MVP (must demonstrate)

- [ ] Demo owner signs in and selects a synthetic organization.
- [ ] BankSim exposes **two fictitious banks** (`BANK_A`, `BANK_B`) and optionally one wallet (`WALLET_C`). All institution names must be presented as simulated, never as real licensed institutions.
- [ ] An order of ETB 2,500.00 exists in SettleGraph's organization dataset.
- [ ] BankSim accepts a virtual transfer, debits the simulated payer, and later posts a settlement to the simulated merchant. Every journal posting is balanced.
- [ ] The BankSim provider connector publishes signed webhooks; SettleGraph deduplicates messages and stores immutable normalized observations.
- [ ] Two-stage reconciliation independently connects **order→payment** and **payment→settlement**; different statuses are not conflated.
- [ ] A delayed-settlement scenario produces one legitimate investigation case after the **scenario's configured grace period**.
- [ ] Bedrock or an explicitly labeled offline AI stub explains the available records using only stored evidence IDs; Go validates its output.
- [ ] Releasing the settlement changes the reconciliation outcome and preserves previous run history and investigation audit events.
- [ ] Next.js and Flutter can consume the same public API: web shows dashboard/transactions/cases; Flutter shows assigned cases and submits a human review decision.
- [ ] A deterministic replay proves duplicate notifications do not double-count funds.

### 0.3 Explicit exclusions (do not implement before demo works)

No real bank credentials; no real payments; no bank KYC; no real account aggregation; no payment disbursement; no fraud accusations; no ACH/SWIFT integration; no Kafka/Kubernetes/GraphQL; no unrestricted AI agent with write tools; no OCR; no multi-currency conversion; no automatic refunds; no full accounting package; no advanced split settlements. CSV importing is a secondary fallback, not the primary demo path. These exclusions are part of the plan, not missing specifications.

### 0.4 Non-negotiable correctness invariants

**FIN-01:** Money uses signed **int64 minor units** internally; HTTP encodes minor units as a **decimal string** to avoid JavaScript precision loss. ETB has two decimal places. Currency field remains explicit.

**FIN-02:** A BankSim *journal* is balanced: the sum of signed postings is exactly zero. All transfer acceptance and settlement state changes are atomic DB transactions. No simulated ledger balance can go negative for a payer account.

**FIN-03:** BankSim uses its **own PostgreSQL database**. SettleGraph has no SQL access to it; only provider-shaped read APIs and signed notifications.

**FIN-04:** Imported BankSim statements and signed callbacks are **observations**, not direct database ground truth. SettleGraph can show source-attested data in the simulator, but must not imply real-world bank confirmation.

**FIN-05:** Reconciliation is two-stage. Accepted payment != settled payment; case resolution != transfer settlement.

**FIN-06:** Never automatically match on equal amount or customer name alone. Ambiguous references produce `AMBIGUOUS_MATCH` with no confirmed allocation.

**FIN-07:** For a single reconciliation run, any eligible payment/settlement can be allocated **at most once** in the MVP. More complicated one-to-many allocations are deferred.

**FIN-08:** Queue delivery is at least once. Same webhook/event/message replayed ten times results in one financial observation and at most one logical output per reconciled ruleset snapshot.

**FIN-09:** Every tenant-owned query is filtered by organization membership; unrecognized IDs from other tenants return 404 and cannot be inferred through totals.

**FIN-10:** AI cannot modify payment facts, create matches, approve settlement, or close a case. Its output must be validated against the current case evidence snapshot; fabricated evidence IDs cause rejection.

**FIN-11:** Every human decision is version checked and audit-logged; manual closure does not change the financial record.

**FIN-12:** A demonstration's injected fault changes data availability and timing, **not** physical (simulated) ledger correctness unless a dedicated, labeled consistency test explicitly requests malformed external data.

### 0.5 Frozen conventions: everyone agrees before implementation

| Concern | Contract |
|---|---|
| REST base | `/v1` for product endpoints; `/health/live`, `/health/ready` for checks |
| Default content | JSON (`application/json`) |
| Identity | `Authorization: Bearer <JWT>`; demo-only session minting behind access code |
| Tenant | `X-Organization-Id: <uuid>` required on tenant-scoped product calls |
| Mutation retry | `Idempotency-Key: <8..128 chars>` required on specified POSTs |
| Optimistic update | `If-Match: "<integer>"`; missing => 428, stale => 412 |
| ID | UUID; DB `uuid`; timestamps | RFC3339 UTC, e.g. `2026-10-24T09:30:00Z` |
| Money | `{ "minor_units": "250000", "currency": "ETB" }` |
| Pagination | `limit` (1–100), opaque `cursor`; response includes `next_cursor` or null |
| Error | `{ "error": { "code", "message", "request_id", "details" } }` |
| Async | mutation => HTTP 202 with job/run resource; GET job/run until terminal state |
| Roles | `OWNER`, `ANALYST`, `REVIEWER`, `VIEWER` |
| Naming | snake_case JSON, camelCase/OpenAPI operation IDs, uppercase enum values |
| Demo truth | BankSim private evaluator only; must not be returned in SettleGraph UI/API |

**Breaking-change rule:** Once frontend/mobile begins development, do not silently change property names, enums, or response structures. Modify the YAML contract first, regenerate client types, update sample JSON/contract tests, and communicate migration.

## 1. Architecture, trust boundaries, and repository

### 1.1 System shape

```text
┌──────────────────────────── CLIENTS ────────────────────────────┐
│ Next.js Web (dashboard, cases, demo controls)    Flutter Review │
└──────────────────────────────┬──────────────────────────────────┘
                               │ JWT + X-Organization-Id
                        ┌──────▼──────────┐
                        │ SettleGraph API │──────────────┐
                        └──────┬──────────┘              │
                               │                    Amazon SQS
                       SettleGraph DB                 │
                               ▲                    Go worker
                               │                       │
                        observations ◄─────signed provider events
                               │                       │
                        reconciliation           Amazon Bedrock
                               ▲
                        provider adapter  ◄─── BankSim provider API
                                                      │
                            CONTROL ONLY ◄──────── BankSim service
                                                      │
                                            Separate BankSim DB
```

A scenario control request from the owner passes **API → BankSim admin adapter**; financial observations flow **BankSim provider interface → SettleGraph inbox/worker**. The API cannot use a BankSim internal ground-truth endpoint to cheat reconciliation.

### 1.2 Repository that a solo backend engineer should build

```text
settlegraph/
  go.mod
  cmd/api/main.go
  cmd/worker/main.go
  cmd/banksim/main.go
  internal/config/
  internal/httpapi/           # routes, middleware, validated DTOs
  internal/auth/              # demo JWT verifier, Cognito verifier interface
  internal/tenancy/           # membership/role enforcement
  internal/banksim/           # simulator ledger, scenarios, provider adapters
  internal/provider/          # normalized event types + sync interface
  internal/ingest/            # authenticated webhook inbox, dedup, validation
  internal/reconcile/         # PURE deterministic match engine
  internal/cases/             # state machine, decisions, events
  internal/investigation/     # AI input, Bedrock adapter, output evidence gate
  internal/reporting/         # dashboard projections
  internal/jobs/              # outbox dispatcher, SQS processing, retry leases
  internal/platform/          # DB, AWS clients, clock, logger, instrumentation
  db/settlegraph/             # migratable SQL schema
  db/banksim/                # separate migratable SQL schema
  api/                       # copy of frozen OpenAPI YAML
  contracts/                 # event and AI schemas
  fixtures/                  # synthetic scenarios, golden expectations
  tests/contract/ tests/integration/ tests/e2e/
  deployments/compose.yaml
  deployments/aws/           # IaC only after local MVP
  Makefile
  .env.example
```

Module rules: `reconcile` imports no AWS or HTTP packages; `banksim` cannot import `internal/cases` or product repositories; HTTP handlers never hold ledger rules; `investigation` has read-only evidence inputs; DB boundaries are enforced by separate DSNs and SQL users.

### 1.3 Sequence of the flagship demo

1. Owner calls `POST /v1/demo/scenarios/delayed_settlement/runs` with an idempotency key.
2. API delegates orchestration to BankSim admin endpoint; BankSim creates seeded accounts and a scenario run. It also generates the merchant's demo order in SettleGraph through a product-domain seed service **before** transfers begin.
3. API returns a scenario run ID and `step=0` to the web dashboard.
4. Owner calls `POST /v1/demo/scenario-runs/{id}/advance` with `{ "expected_step":0 }`. BankSim commits a balanced source debit and clearing credit and enqueues a signed `PAYMENT_ACCEPTED` webhook.
5. Product webhook validates signature, deduplicates event, persists raw payload as a new observation in one DB transaction, writes reconciliation-job outbox entry, and ACKs.
6. Worker runs reconciliation against the organization's scenario snapshot; order/payment can match but payment/settlement remains outstanding.
7. Owner advances the scenario virtual clock beyond the stated settlement due period. New reconciliation generates `SETTLEMENT_NOT_OBSERVED` case. No funds-loss claim is made.
8. Analyst triggers `POST /v1/cases/{id}/investigations`, and Bedrock generates a structured, evidence-linked hypothesis. Invalid output produces a visible `FAILED` AI job but leaves the case and financial state intact.
9. Owner advances the scenario to `settlement_released`; BankSim posts balanced clearing debit/merchant credit and emits `SETTLEMENT_POSTED`.
10. Worker normalizes, reconciles, links evidence, and marks the discrepancy as `EVIDENCE_UPDATED` (do not silently erase or automatically human-resolve the case).
11. Flutter reviewer opens case, checks both versions of the evidence, resolves case with a reason and `If-Match` version, while web audit view shows history.

## 2. Phase 0 — Contract, local environment, and scope freeze

**Objective:** Frontend and Flutter begin with stable mocks; backend agent has concrete gates.

**Implement**
- [ ] Commit both OpenAPI YAML files and both JSON schemas exactly as supplied; label version `2.0.0-draft` until first integration.
- [ ] Create root Go module; `cmd/api`, `cmd/worker`, `cmd/banksim` must compile even if initially skeletal.
- [ ] Add `.env.example` and fail-fast configuration checks, separate DB DSNs, safe demo defaults.
- [ ] Create `docker compose` with PostgreSQL product DB, PostgreSQL BankSim DB, API, worker, BankSim; local mode may substitute a PostgreSQL-backed job runner for AWS SQS behind an interface.
- [ ] Define roles, enums, money encoding, error envelope, case versioning, sample scenarios, timestamps, endpoint ownership.
- [ ] Set deterministic UUID/clock injection strategy for fixture tests; seed fixture has reproducible virtual time.
- [ ] Configure Make targets: `make lint`, `make test`, `make test-integration`, `make migrate`, `make seed`, `make up`, `make down`, `make contract-check`.
- [ ] Give generated TypeScript and Dart clients or typed mock service implementations to other developers; no silent breaking API changes.

**Tests and acceptance gates**
- [ ] YAML parses; each `$ref`, parameter, request/response component, error response, security scheme and operationId is valid and unique.
- [ ] JSON Schema validates supplied examples and rejects missing required event fields.
- [ ] `go test ./...` and `go vet ./...` pass; containers start from clean checkout.
- [ ] `GET /health/live` returns 200; `GET /health/ready` returns 503 until required dependencies are ready, then 200.
- [ ] All components use UTC in JSON and ETB integer minor units.
- [ ] Frontend and Flutter can render **mock** dashboard and case-detail JSON without backend availability.

**Done when:** Every process starts, contract is frozen, mocks parse, and another teammate can integrate without asking what field names mean. **Do not start the ledger before this gate.**

## 3. Phase 1 — Product foundation, identity, tenancy, and errors

**Objective:** Harden the API skeleton before writing transaction logic.

**Implement**
- [ ] `/health/live` and `/health/ready`; readiness checks PostgreSQL and required configuration, not Bedrock availability.
- [ ] Middleware: request ID; recover panics; structured logging (without token or full transaction descriptions); size/time limits; strict JSON decoder with unknown-field rejection on mutations; CORS allowlist; rate limiting for demo auth and expensive endpoints.
- [ ] Demo token endpoint `POST /v1/demo/sessions`, only when `APP_MODE=demo`, protected by `DEMO_ACCESS_CODE` and rate limit. Mint short-lived signed JWT with synthetic user subject; never ship demo signing key to production.
- [ ] JWT validation interface supports local demo verifier and future Cognito access-token verification (signature, issuer, `token_use`, `client_id`, expiry, key rotation).
- [ ] `GET /v1/me` returns user and organization membership(s).
- [ ] Authorization middleware verifies active membership for `X-Organization-Id`, and each route checks role; never trust a role from the client body/header.
- [ ] Product DB baseline tables: organizations, users, memberships, idempotency_requests, audit_events.
- [ ] Idempotency lookup scoped by `(org_id, actor_sub, method, route_template, key)` and request body hash. Persist original response for retries and return 409 on body mismatch.

**Tests**
- [ ] Missing bearer => 401; tampered/expired token => 401; valid token but unauthorized role => 403.
- [ ] Wrong organization ID, including another existing tenant, cannot access resources and does not leak identifiers or counts.
- [ ] Production mode rejects `/v1/demo/sessions` with 404 even if caller guesses the URL.
- [ ] Same idempotency key/body returns same status/body/resource ID. Same key/different body returns 409.
- [ ] Unknown JSON request properties return 400; malformed UUID => 400; oversized body => 413.
- [ ] Panic in handler returns 500 `INTERNAL_ERROR` with request ID and no stack trace to client.

**Done when:** Demo owner can sign in, `GET /v1/me` works, role and tenant isolation tests pass, and errors conform to OpenAPI.

## 4. Phase 2 — BankSim ledger, transfers, and scenario clock

**Objective:** A deterministic, financially coherent source of events separate from the product.

**Implement**
- [ ] Two institution identities (fictional Bank A and Bank B) with independent source IDs and account descriptions. Third fictitious wallet optional; do not duplicate code for each bank.
- [ ] `sim_accounts`: owner reference, institution, currency ETB, type `CUSTOMER | MERCHANT | CLEARING | EQUITY`, status.
- [ ] `sim_journals` + `sim_entries`: each posting includes signed minor units; each posted journal balances to exactly zero in one currency.
- [ ] Seed opening balances with matching equity counter-postings; payer's available balance is derived transactionally or maintained in a locked cache with exact journal correspondence.
- [ ] Transfer state machine `CREATED -> ACCEPTED -> SETTLEMENT_PENDING -> SETTLED` plus terminal `FAILED`. Invalid transitions rejected.
- [ ] On `ACCEPTED`, atomically debit payer and credit clearing liability/holding account. On `SETTLED`, debit clearing and credit merchant. Neither action is repeated if the same transfer request is retried.
- [ ] Scenario run has deterministic `seed`, `virtual_now`, `step`, merchant organization mapping and a private manifest of expected events. `advance` uses `expected_step`, so concurrent controllers cannot skip or replay a step accidentally.
- [ ] Four scripts: `normal_flow`, `delayed_settlement`, `duplicate_webhook`, `provider_outage`; **faults affect notification delivery/provider visibility, not journal correctness**.
- [ ] Use transactional outbox for provider events; event includes stable event ID and source sequence. Webhook delivery has separate attempt IDs, so the same event can be delivered twice legitimately.
- [ ] API to read provider transactions/settlements with cursor pagination, using machine credentials scoped to the relevant simulated institution.
- [ ] Private `ground-truth` endpoint accessible only to a **test-evaluator credential**, not product service credentials.

**Tests**
- [ ] Double-entry property: every posted journal sums to zero, including concurrent transfers and retries.
- [ ] Payer balance never becomes negative; insufficient funds => transfer FAILED with no debit.
- [ ] 100 concurrent requests with one `Idempotency-Key` result in one transfer and exactly one accepted debit.
- [ ] 100 distinct concurrent transfers cannot overspend one account (`SELECT FOR UPDATE` or equivalent lock strategy).
- [ ] A delayed webhook does not delay the BankSim ledger posting unless the scenario specifically delays **settlement**.
- [ ] `advance` with stale `expected_step` => 412; repeated identical idempotent advance returns original response.
- [ ] Two runs with same seed/scenario generate the same *logical* events and balances (IDs may differ; normalize IDs when comparing).
- [ ] Product DB user cannot connect to BankSim DB and BankSim's evaluator endpoint is forbidden to it.

**Done when:** A simulated 2,500 ETB transfer has balanced journal entries, visible provider events, scenario-state consistency, and isolated ground truth.

## 5. Phase 3 — Signed provider ingestion, durable inbox, and jobs

**Objective:** SettleGraph can safely consume a stream even with duplication, delay and process crashes.

**Implement**
- [ ] Contract: BankSim sends `POST /v1/integrations/banksim/events` with event ID, event payload JSON, delivery timestamp and HMAC-SHA256 signature over `timestamp + "." + event_id + "." + raw_body`. The receiver uses constant-time compare.
- [ ] Enforce timestamp freshness (e.g. +/- 300 seconds on **delivery time**, not event occurrence time), payload cap 256 KiB, allowlisted event types, known provider ID.
- [ ] Insert `(provider_id, provider_event_id)` into `inbox_events` UNIQUE; insert normalized immutable `observations` and `outbox_events` in the same PostgreSQL transaction. Only ACK after commit.
- [ ] On duplicate event, return 202 with `duplicate:true`, never insert another normalized observation. A new delivery attempt retains the event's stable identity.
- [ ] Outbox dispatcher safely publishes work to SQS; mark dispatch only after confirmed enqueue; if crash occurs before/after send, re-publish is safe.
- [ ] Worker processes via DB `jobs` lease with owner token, retry_count, next_retry_at and terminal status. SQS redelivery must not repeat a completed job.
- [ ] Backfill provider cursor: `GET /provider/v1/institutions/{id}/events` on startup or periodically; store cursor only after batch commit to recover missed callbacks.
- [ ] For webhook ordering, retain provider event sequence and effective occurrence time, recompute derived status rather than assuming arrival order.
- [ ] Recognize `PAYMENT_ACCEPTED`, `SETTLEMENT_POSTED`, `REFUND_POSTED`, `TRANSFER_FAILED`; the canonical model preserves original provider ref, source ID, payload hash, signed-source metadata, observed-at and occurred-at.
- [ ] Add observable metrics: ingest_ack_total, duplicate_event_total, invalid_signature_total, queue_lag_seconds, job_retry_total, dead_letter_total.

**Tests**
- [ ] Valid signature => 202. Wrong body/timestamp/HMAC/unknown provider => 401 or 400 as specified.
- [ ] Repeat the exact webhook ten times => one stored event and one logical observation.
- [ ] Crash after DB inbox commit but before HTTP response; retry preserves one observation.
- [ ] Crash after outbox send but before dispatcher ACK; SQS redelivery changes no financial totals.
- [ ] Receive `SETTLEMENT_POSTED` before `PAYMENT_ACCEPTED`; reconcile correctly once both exist.
- [ ] Provider outage causes a gap state and recovery by backfill; gaps do not become negative balances or a false claim of theft.
- [ ] Poison message goes to DLQ after configured attempts; monitoring warns and other events continue.

**Done when:** The ingestion and job pipeline is idempotent, crash-recoverable and deterministic under the four seeded faults.

## 6. Phase 4 — Pure reconciliation engine and financial proof

**Objective:** Reconcile independently of transport, AI, HTTP, and AWS; false-positive confirmed matches are unacceptable.

### 6.1 Implement the core as a pure Go domain package

- [ ] Model `Order`, `PaymentObservation`, `SettlementObservation`, `Match`, `Ambiguity`, `ProofCheck`, `RunSnapshot` and `ReconciliationResult` with `Money{MinorUnits int64, Currency string}`; no floats and no provider SDK imports.
- [ ] Enforce distinct `ORDER_PAYMENT` and `PAYMENT_SETTLEMENT` phases, each with independently counted results. The dashboard must not add their totals together.
- [ ] Derive stable canonical reference keys using documented provider adapter rules: trim, case-normalize and remove explicitly defined harmless separators **only for candidate generation**. Preserve original strings. Never strip arbitrary digits or invent customer identity matches.
- [ ] Build candidates with indexed lookups on tenant, provider, order reference, payment reference, amount, currency, timestamp range and scenario scope. Do not compare every record against every other record when indexable keys exist.
- [ ] Stage A: an order may match a payment only if the explicit normalized order reference agrees, amount and ETB currency agree, and the source record is an accepted payment. Stage B: a payment may match a settlement only if the explicit transfer/payment reference agrees and the amounts/currency agree. Both require uniqueness among eligible candidates.
- [ ] On multiple feasible candidates or conflicting references, produce `AMBIGUOUS_MATCH`; **do not auto-select the first, nearest or AI-best-scoring candidate**. The AI may offer hypotheses for manual follow-up but cannot generate a proof.
- [ ] Snapshot `snapshot_cutoff`, `rules_version`, involved observation IDs and source coverage. Sort inputs deterministically by `(occurred_at, source_sequence, id)` to make runs reproducible.
- [ ] Verify each proposed edge uses evidence IDs in the selected tenant/snapshot, valid stage transitions, exact integer amount and one-to-one uniqueness. Persist in one transaction with `reconciliation_runs`, `match_edges`, `case_evidence`, `investigation_cases`, and `outbox_events`.
- [ ] Create a proof witness containing checks, rule ID/version, source evidence IDs, hash of raw events, snapshot hash and amount. The proof establishes correspondence **among available records**, not actual custody of money.
- [ ] Re-run reconciliation on new evidence without overwriting previous runs. Create/append case timeline events through a stable discrepancy fingerprint. Do not create 100 duplicate cases for one enduring missing settlement.
- [ ] Open a missing-settlement case only when both `virtual_now > settlement_due_at` and source coverage is sufficient to justify an absence statement. When source unavailable, classify as `SOURCE_UNAVAILABLE` or `INCOMPLETE_SOURCE`, never confirmed loss.
- [ ] On late settlement, update case to `EVIDENCE_UPDATED` with a timeline event. Human decision `RESOLVE` closes the case. A different financial discrepancy can reopen through a new versioned event.

**Recommended reference algorithm (implement behavior, not necessarily this code shape):**

```text
snapshot := loadEligibleOrdersPaymentsSettlements(tenant, scenario, cutoff)
orderCandidates := indexBy(orderReference, currency, amount, acceptedPayments)
for order in deterministicOrder(snapshot.orders):
    candidates := filterByExactRule(order, orderCandidates)
    if len(candidates) == 1 and not allocated(candidates[0]):
        createOrderPaymentProof(order, candidates[0])
    else if len(candidates) > 0: emitAmbiguity(order, candidates)
for acceptedPayment in deterministicOrder(snapshot.acceptedPayments):
    matches := exactSettlementCandidates(acceptedPayment)
    if len(matches) == 1 and not allocated(matches[0]):
        createPaymentSettlementProof(acceptedPayment, matches[0])
    else if len(matches) > 0: emitAmbiguity(acceptedPayment, matches)
    else if passedGracePeriod(acceptedPayment) and sourceCoverageComplete():
        openOrUpdateUnobservedSettlementCase(acceptedPayment)
transactionallyPersist(run, proofs, caseChanges, auditAndOutbox)
```

### 6.2 Tests, including negative cases

- [ ] Normal order/payment/settlement matches both stages with exactly two proof edges and no cases.
- [ ] Two identical 2,500 ETB payments for different references are distinguishable. Two same-value records with *missing* references remain ambiguous.
- [ ] Two settlements referencing one payment must not produce two confirmed matches.
- [ ] Amount mismatch of **1 minor unit** produces a discrepancy, not rounding forgiveness.
- [ ] `PAYMENT_ACCEPTED` without bank settlement remains payment-observed, not settled.
- [ ] Settlement-first event followed by payment event produces correct result after re-run.
- [ ] Provider outage/incomplete cursor stops a “missing funds” conclusion and exposes incomplete coverage.
- [ ] Reconciliation of same input, cutoff and rules version produces equal normalized outputs independent of input ordering.
- [ ] Concurrent attempts to write the same run/matches cannot double-allocate; DB unique indexes reject it and handler reports a safe conflict.
- [ ] Historical run remains unchanged after late settlement; next run changes only derived view/case timeline.
- [ ] Cross-tenant payment ID referenced by an order is rejected before matching and during persistence.

**Done when:** `go test ./internal/reconcile/...` includes deterministic golden fixtures and property/fuzz tests; no known case matches solely by amount, customer name or model output.

## 7. Phase 5 — Public read model and frontend/mobile integration

**Objective:** Both client developers can build fully functioning UIs without knowing BankSim internals.

### 7.1 Implement contract-driven endpoints

- [ ] `GET /v1/providers` and `GET /v1/accounts`: provider health, source of evidence, last-observed times, optional observed balance. **Never label a computed/imported amount as a verified real-world bank balance.**
- [ ] `GET /v1/dashboard/summary`: distinct `orders_expected`, `payments_accepted`, `settlements_observed`, `unmatched_payments`, `open_cases`, provider coverage and `as_of`; monetary summaries are from one org and an explicit snapshot.
- [ ] `GET /v1/dashboard/series`: UTC `from/to`, granularity `HOUR|DAY`, deterministic bucket endpoints and non-overlapping time bounds. Limit range (e.g., 31 days hourly, 365 days daily) to avoid runaway queries.
- [ ] `GET /v1/orders`, `GET /v1/transactions`, `GET /v1/settlements`: list with opaque keyset cursor and detailed read endpoints. Filtering must occur before cursor pagination.
- [ ] `POST /v1/reconciliation/runs`: create async run with idempotency key; `GET` status and proof/matches, including `snapshot_cutoff` internally. Prevent duplicate run jobs for the same request hash while permitting explicit new runs after evidence changes.
- [ ] `GET /v1/cases`, case detail, evidence, event timeline and audit queries. Flutter uses `GET /v1/cases?assigned_to=<current-user-id>` for its review queue.
- [ ] Expose `ETag: "<case.version>"` on case detail and mutation responses; never use wall-clock timestamps for optimistic locking.
- [ ] Reuse shared DTO definitions generated or manually tested against `contracts/settlegraph-public.openapi.yaml`. UI components must consume those DTOs, not invent local equivalents.
- [ ] Add stable error codes and preserve `X-Request-Id` through async job correlation; return 404 for resource IDs belonging to another organization.

### 7.2 Dashboard semantics: this is a reconciliation view, not an online bank

| Metric | Exactly what it means | Dangerous interpretation to avoid |
|---|---|---|
| `orders_expected` | Sum of merchant order amounts in selected scope | Total received funds |
| `payments_accepted` | Amounts of unique accepted payment observations | Settled in merchant bank |
| `settlements_observed` | Unique settlement observations visible in selected snapshot | Current live bank balance |
| `unmatched_payments` | Unique accepted payments lacking an eligible settlement match | Confirmed stolen/lost money |
| `coverage.source_status` | Whether expected data sources have reported through watermark | Authentication of uploaded data |

**Tests and integration acceptance**

- [ ] Swagger/editor loads YAML, a TypeScript and Dart client can deserialize sample responses, no undocumented enum case occurs.
- [ ] All pagination cursors are tenant-bound and query-bound; altering cursor or filters returns `400 INVALID_CURSOR`.
- [ ] UTC boundary tests for daylight-saving-independent bucketing and half-open `[from,to)` range.
- [ ] E2E read flow: start scenario → advance → poll run → dashboard totals → open case → evidence timeline.
- [ ] Same tenant without proper role cannot assign/resolve even if the UI hides the button.
- [ ] API responses exclude full raw PII-bearing provider payloads; expose only public evidence display fields and SHA-256 hashes.
- [ ] Under a seeded 1,000-event workload, endpoints respond within the agreed performance budget in the team environment (target P95 under 1 s for GET reads; measure and publish actual hardware).

**Done when:** Next.js can implement its dashboard/case workbench and Flutter can implement a review queue using the HTTP contract only.

## 8. Phase 6 — Cases, human review, and Bedrock investigation

**Objective:** AI offers bounded, evidence-cited hypotheses; authenticated humans own case decisions.

### 8.1 Case state machine and authorization

- [ ] `OPEN -> EVIDENCE_UPDATED -> RESOLVED`; allow `RESOLVED -> OPEN` only through explicit `REOPEN` decision that creates a new versioned timeline event. A new settlement cannot silently close a case.
- [ ] Every case mutation uses `If-Match: "version"` and `Idempotency-Key`; stale version => 412, missing => 428, same mutation retry => persisted original response.
- [ ] `OWNER/ANALYST`: can assign. `OWNER/REVIEWER` (or assigned `ANALYST`, if role policy explicitly authorizes): can decide; `VIEWER`: read only. Server enforces this independent of UI.
- [ ] Use transaction: `UPDATE investigation_cases SET version=version+1 WHERE organization_id=? AND id=? AND version=?` + insert `case_events` + `audit_events` + outbox. Check affected rows == 1.
- [ ] Validate case resolution reason (5–2,000 chars), audit actor, timestamp, request ID and previous/new status. A manual resolution does **not** change match edges or bank source records.

### 8.2 AI investigation pipeline

- [ ] `POST /v1/cases/{caseId}/investigations` returns `202` investigation resource. Worker loads **only the selected org's** evidence IDs and builds a bounded evidence snapshot; never give model raw DB connection or money-writing tools.
- [ ] Send a structured, sanitized evidence packet with `case_id`, `evidence_snapshot_hash`, source refs, amounts, timestamps and coverage limitations; strip secrets and unnecessary personal data.
- [ ] Call Amazon Bedrock through a small interface (`Investigate(ctx, evidence) -> InvestigationResult`); select one supported model and budget maximum request size/output tokens. Offline stub must be **clearly labeled `OFFLINE_STUB`** and must never masquerade as Bedrock output.
- [ ] Ask for `summary`, `hypotheses[]`, `recommended_actions[]`, `evidence_ids[]`, `limitations[]` using `contracts/ai-investigation.schema.json` and the supported structured-output method of the selected model. The JSON schema is a format gate, not an evidence/truth guarantee.
- [ ] Deterministically reject results citing evidence IDs outside this case's snapshot, unsupported transaction references, disallowed status claims, or prohibited conclusions like “funds stolen” without verified proof. Limit explanation length and number of actions after generation in Go.
- [ ] Persist provider/model identifier, invocation ID (redacted), latency, token use if available, prompt template version, snapshot hash, validation outcome and explanation for audits.
- [ ] Re-run after evidence change only as a **new** investigation version; prior AI result remains linked to its historical evidence snapshot.
- [ ] On Bedrock timeouts, throttling or unavailable model, mark investigation FAILED with retryable reason and keep financial reconciliation fully functional. No automated case decisions.

### 8.3 AI correctness tests

- [ ] Valid two-evidence answer passes schema and membership checks.
- [ ] Fabricated evidence ID, other-tenant evidence ID and changed snapshot all cause `REJECTED`; the case stays untouched.
- [ ] Prompt-injection text embedded in a provider description (e.g. `ignore evidence rules`) is treated as untrusted data; never affects tool permissions, code paths or authorizations.
- [ ] AI claiming `SETTLED` when there is no settlement observation is rejected or rewritten by an independent deterministic status presenter.
- [ ] Offline stub returns a recognizably different provenance enum and deterministic fixture output.
- [ ] A Bedrock API failure consumes a bounded retry budget and records safe failure status; no retries in an infinite hot loop.
- [ ] Concurrent `If-Match` decisions: one succeeds, other returns 412 and audit history is ordered.

**Done when:** A reviewer can inspect an AI hypothesis, trace every cited observation, and close a case without altering financial facts.

## 9. Phase 7 — Fault injection, exact replay, and audit demonstration

**Objective:** Prove SettleGraph is correct under unreliable delivery, not just happy-path data.

### 9.1 Reproducible simulator scenarios

| Key | Synthetic source behavior | Required product outcome |
|---|---|---|
| `normal_flow` | One accepted 2,500 ETB payment and matching 2,500 ETB settlement | Two match edges; zero open cases |
| `delayed_settlement` | Payment accepted; settlement deliberately not posted until subsequent virtual step | Missing-settlement case only after grace time and complete source watermark; late settlement → `EVIDENCE_UPDATED` |
| `duplicate_webhook` | Same immutable event ID delivered twice with distinct delivery attempts | One observation; no duplicate money; duplicate metric increments |
| `provider_outage` | Simulated provider read API is temporarily unavailable and callbacks are held | `INCOMPLETE_SOURCE`, safe uncertainty; backfill on recovery, no fabricated settlement |

- [ ] A scenario run is a **new** immutable namespace; do not reset or delete earlier financial history to replay. Deterministic seed and virtual time remove flaky sleeps.
- [ ] Step advance accepts `expected_step` and idempotency key; simulate condition changes without mutating ledger history retroactively.
- [ ] Script each virtual step's actions (post journal, emit event, delay webhook, advance virtual clock, recover API, backfill) in `fixtures/scenario-catalog.json` so reviewers can predict the next action.
- [ ] Scenario controller is an admin-only capability, separate from merchant reconciliation. Normal tenants and mobile reviewer roles have no access to fault injection.
- [ ] Keep BankSim evaluation truth private. Test harness compares expected ledger transitions to product observations but product HTTP handlers cannot access ground truth.
- [ ] Inject process restarts during pending webhook, outbox publish, SQS delivery, worker transaction and AI retry; no double counting or lost committed jobs.
- [ ] Log a single correlation trace (`scenario_run_id`, `provider_event_id`, `job_id`, `reconciliation_run_id`, `case_id`) across components, excluding secrets.

**Acceptance demonstration**

- [ ] Judges can start a clean delayed scenario, advance one click at a time and see all states without waiting real minutes.
- [ ] The exact same scenario seed produces the same normalized outcomes when re-run.
- [ ] Duplicate webhook attempts are visible as *delivery events* but not duplicate *financial events*.
- [ ] Settlement recovery produces a second completed reconciliation run; prior case history and old proof remain readable.

**Done when:** An unedited screen recording of the entire flagship demo can be reproduced from a clean checkout using a documented seed.

## 10. Phase 8 — Hardening, delivery and optional AWS architecture

**Objective:** Deploy only after phase 0–7 end-to-end flow works locally. Keep an offline mode for hackathon failure recovery.

### 10.1 Security and privacy

- [ ] Configure credentials as environment variables or AWS Secrets Manager; no secrets in repository, images, source-controlled fixture files or frontend bundle.
- [ ] Use TLS for public endpoints, HMAC for provider webhooks, constant-time signature comparison, bounded timestamp skew and replay deduplication.
- [ ] Keep BankSim admin/evaluator interfaces off public routes; use separate credentials and security groups, not a shared master secret.
- [ ] Add resource-based tenant scoping on every query and mutation; test foreign IDs and foreign pagination cursors.
- [ ] Mask customer names, account numbers and full descriptions in request logs. Use synthetic data exclusively in demo.
- [ ] Restrict presigned S3 imports by content length/expiration/object key; verify actual object bytes/hash before parsing and never trust `Content-Type` alone.
- [ ] Sanitize CSV formula-like fields when exporting to Excel; bound record lengths, total file size and row count.
- [ ] Test invalid JWT, expired JWT, stale HMAC, body alteration, privilege escalation, SSRF-like URLs in descriptions and unsafe AI instructions in source text.

### 10.2 Cloud topology: intentionally economical

**Hackathon target:** A single small EC2 VM running three Go containers and two isolated PostgreSQL containers via Compose (private DB ports); Amazon Bedrock for AI; optional Amazon SQS for queued jobs and S3 for optional CSV uploads. This is a pragmatic demo deployment, **not** a recommended production hosting architecture. Use a dedicated test account/budget alarm, HTTPS ingress, AWS IAM instance role and minimum privileges. Verify cloud credits, model access and service availability before enabling paid services.

**Production evolution:** ECS/Fargate for independently scaled API and workers, managed PostgreSQL, encrypted S3, CloudWatch dashboards, IAM least privilege, real bank-approved integrations, data retention controls and formal security review.

**Important implementation sequence:** Worker processes durable PostgreSQL jobs before SQS integration; introduce a `Queue` abstraction and transactional outbox. Once SQS is enabled, consumers remain idempotent. SQS standard queues may deliver duplicates or out-of-order, so never rely on exactly-once transport semantics.

### 10.3 Deployment and operating checklist

- [ ] Separate `.env.example` and secure deployment config, with all keys documented but no actual values.
- [ ] Build reproducible Docker images with pinned Go major/minor version; health checks and readiness fail safely.
- [ ] `make up && make migrate && make seed && make smoke` works on a clean workstation.
- [ ] Deploy HTTPS endpoint; verify `/health/live` and `/health/ready`; validate allowed origins for Next.js and Flutter.
- [ ] Give frontend the **exact** base URL and access code distribution process through a secure channel; never put server signing key in public UI.
- [ ] Add CloudWatch or JSON logs with request/event/case IDs, job delay and error counts. Add budget alert and Bedrock request timeout.
- [ ] Run the four seeded scenarios on deployed environment; ensure operator can return to the offline local demo if cloud APIs fail.
- [ ] Record a 3–5 minute fallback video of the working synthetic demo before final presentation.

**Done when:** the backend deploy is accessible by approved clients, local mode remains runnable, and the core demo succeeds twice end-to-end after a clean restart.

## 11. Phase 9 — Optional, explicitly deferred items

Implement **only** if all gates above are green and the demo is reliable.

- [ ] CSV/S3 import fallback (`GET/POST /v1/imports`, `/v1/imports/{id}/complete`), safe parsing and per-row feedback.
- [ ] Extra fictional wallet provider as a third adapter.
- [ ] Notification feed for mobile (poll case timeline first; real push notifications later).
- [ ] AI-assisted ambiguous-reference suggestions kept behind human review (never deterministic match proof).
- [ ] Advanced allocation: installment payments, settlement batches, provider fees and refunds with explicit exact-sum reconciliation; these require new rules and tests, not heuristic fallbacks.
- [ ] True OAuth/Cognito production identity and automated real-provider integrations after authorized access and legal/security checks.

## 12. Backend implementation responsibility and dependency order

Since you are the **sole backend owner**, make work sequentially shippable. Frontend and Flutter engineers can still implement mocks independently from the same frozen contract.

```text
Phase 0 contracts/mocks (parallel UI development starts)
     │
Phase 1 security + tenancy
     │
Phase 2 BankSim ledger + one happy-path transfer
     │
Phase 3 webhook inbox + worker + outbox
     │
Phase 4 pure reconciliation + proof
     │
Phase 5 dashboard and case read endpoints
     │
Phase 6 review + Bedrock (offline stub first)
     │
Phase 7 fault replay and end-to-end tests
     │
Phase 8 deploy + smoke test
     └──────────── Phase 9 optional ONLY after passing all previous gates
```

### Change-control rule for coding agents

1. Read `README.md`, this blueprint, `contracts/settlegraph-public.openapi.yaml`, `contracts/banksim-internal.openapi.yaml`, the schemas and fixture catalog before editing code.
2. Implement **one phase at a time**. Do not mark a phase complete until its listed test gates pass; if a tool or external service is unavailable, record a blocker and continue only along the documented offline fallback.
3. Add Go unit tests while implementing each domain service. Keep invariant tests deterministic; no sleep-based scenario tests.
4. Run migration from empty DB, `go test ./...`, `go vet ./...`, contract validation and the phase-specific integration suite. Report actual commands and failures. Never claim tests were run when they were not.
5. Preserve contract semantics. If a breaking API change is necessary, update YAML + mock examples + tests + a concise changelog in one commit, and notify both client developers.
6. Protect domain boundaries: BankSim product API cannot read evaluator truth or private ledger tables, reconciliation must not call Bedrock, and AI may never write a case decision or money fact.
7. Do not add Kafka, Kubernetes, separate per-bank services, a new programming language or extra infrastructure until a measured requirement justifies them.

## 13. Shared API endpoint map

**Normative API source:** `contracts/settlegraph-public.openapi.yaml` (39 HTTP operations); `contracts/banksim-internal.openapi.yaml` (10 operations). The tables below give frontend and Flutter developers a quick navigation map; the YAML defines full JSON request, response, path/query/header parameters, status codes and security.

| Client / consumer | Endpoints | Purpose |
|---|---|---|
| Web + Flutter | `POST /v1/demo/sessions`; `GET /v1/me` | Demo login and memberships |
| Web | `GET /v1/providers`; `/v1/accounts`; `/v1/dashboard/summary`; `/v1/dashboard/series` | Provider coverage and dashboard |
| Web | `GET /v1/orders[/{id}]`; `/v1/transactions[/{id}]`; `/v1/settlements[/{id}]` | Financial event evidence |
| Web | `GET/POST /v1/reconciliation/runs`; `GET /v1/reconciliation/runs/{runId}/matches`; `/v1/reconciliation/proofs/{id}` | Runs, edges and proofs |
| Both | `GET /v1/cases`; `/v1/cases/{caseId}`; `/evidence`; `/events` | Case review and provenance |
| Both | `POST /v1/cases/{caseId}/assign`; `/decisions` | Human actions (role gated) |
| Both | `POST/GET /v1/cases/{caseId}/investigations`; `GET /v1/jobs/{id}` | Bounded Bedrock analysis |
| Web | `GET /v1/demo/scenarios`; `POST /v1/demo/scenarios/{scenarioKey}/runs`; `GET/POST /v1/demo/scenario-runs/{scenarioRunId}[/advance]` | Simulation control room |
| Provider webhook | `POST /v1/integrations/banksim/events` | Signed provider events (not user bearer auth) |
| Web fallback | `GET/POST /v1/imports`; `GET /v1/imports/{id}`; `POST /v1/imports/{id}/complete` | Deferred CSV fallback |
| Flutter | `GET /v1/cases?assigned_to={user_id}`; case evidence/events; `POST /v1/cases/{caseId}/decisions` | Focused mobile workflow |

### 13.1 Error, concurrency and polling examples

```http
POST /v1/cases/22222222-2222-4222-8222-222222222222/decisions
Authorization: Bearer <access-token>
X-Organization-Id: 11111111-1111-4111-8111-111111111111
Idempotency-Key: review-case-2222-v3-20261024
If-Match: "3"
Content-Type: application/json

{"decision":"RESOLVE","reason":"The later settlement observation is verified in the synthetic source history."}
```

```json
{
  "error": {
    "code": "VERSION_CONFLICT",
    "message": "This case has changed; reload the current version.",
    "request_id": "req-demo-019",
    "details": { "expected_version": 3, "current_version": 4 }
  }
}
```

Expected HTTP: **412** with the above error if `If-Match: "3"` is stale; **428** if missing. Case mutations return an updated case including a new `version` and `ETag`. Scenario advance uses `expected_step` in JSON plus idempotency key, returns **412** if another controller advanced first.

**Polling:** after starting a reconciliation run or investigation, wait for response `202`, poll resource GET at about 1–2 seconds during demo, and stop when `COMPLETED`, `FAILED` or `REJECTED` (if applicable). Do not assume eventual messages arrive immediately. When opening cases, refresh timeline after state change, not by assuming an LLM answer is the event of record.

### 13.2 Frontend and mobile rules

- Set `Authorization: Bearer <jwt>` and `X-Organization-Id` on every organization-scoped request. Login and health are exceptions; signed webhooks are machine-to-machine only.
- Map money from decimal **strings** into presentation using integer/decimal libraries, never JS `Number` arithmetic on raw minor units. Display `250000` as `2,500.00 ETB`.
- Use `as_of`, `last_observed_at` and provider `sync_status` to label freshness. Display `INCOMPLETE_SOURCE` visibly; don't infer missing funds from an outage.
- Use `status` enums exactly; handle unknown enum values defensively during development, but report contract drift.
- Use shared `Case.version` for mutations and refresh on 412; HTTP-only demo auth can use a Next.js BFF or in-memory token and Flutter secure storage. Do not ship demo access code hardcoded into production builds.
- A 404 may be either nonexistent or intentionally hidden cross-organization resource; show a generic not-found state.
- Only show `source_attestation=SIGNED_SIMULATED_PROVIDER` as synthetic-bank attestation, never as real financial institution confirmation.

## 14. Test matrix and evidence required to declare "done"

| Test ID | Requirement | Minimal reproducible proof |
|---|---|---|
| T-01 | Ledger balance | All POSTED journal sums = 0; no negative payer account |
| T-02 | Concurrent correctness | 100 same-key requests => 1 payment; 100 different-key attempts never overspend |
| T-03 | Auth/tenant | Org B payment/case ID inaccessible from Org A; all list counts isolated |
| T-04 | Webhook integrity | Invalid HMAC/time rejected; signed retries accepted once |
| T-05 | Replay idempotency | 10 duplicate notifications => 1 observation, 1 logical match |
| T-06 | Event reordering | Settlement first, payment later => correct recovered state |
| T-07 | Deterministic reconciliation | Same seed/cutoff/rules => same canonical match set |
| T-08 | Safe nonmatch | Equal amounts without unique refs => ambiguous, no auto-match |
| T-09 | Late settlement | Case opens only after grace/coverage; later `EVIDENCE_UPDATED` |
| T-10 | Source outage | Unknown state not reported as confirmed lost money; backfill recovers |
| T-11 | AI evidence gate | Fabricated/foreign ID rejected, no side effects on money/case decision |
| T-12 | Case race | Version-stale reviewer receives 412; one accepted audit event |
| T-13 | End-to-end scenario | Fresh demo login → scenario → 2 runs → case → AI → Flutter review |
| T-14 | Contract compatibility | All sample responses deserialize against both client models |
| T-15 | Deployment smoke | `/health/ready` good, TLS enforced, keys absent from logs |

**Pass thresholds for demo:** 0 incorrect automatically confirmed matches across the adversarial fixture set; 100% known injected fault types classified within seeded scenarios; duplicate replay creates no extra money; zero cross-org leaks in negative tests. Latency claims must come from measured runs, not the proposal. This fixture suite is a demo validation target, not proof of production-wide financial accuracy.

## 15. Final implementation sequence / release gates

- **Gate 0 — Contract ready:** YAML, fixtures, Go skeleton and mock client parse checks.
- **Gate 1 — API secure:** demo token, organization scoping, errors, health, migrations.
- **Gate 2 — BankSim correct:** posted journal balanced, payer cannot overspend, deterministic transfer events.
- **Gate 3 — Ingestion durable:** signed event, inbox uniqueness, crash replay, backfill.
- **Gate 4 — Finance correct:** two independent match stages, proof, exact sum, no ambiguous auto-match.
- **Gate 5 — Clients unblocked:** stable dashboard/orders/transactions/settlements/cases with pagination.
- **Gate 6 — Human+AI safe:** bounded Bedrock analysis, evidence ID validation, versioned decision, audit.
- **Gate 7 — Replay reliable:** all four seeded scenarios and recovery tests green.
- **Gate 8 — Deployable:** cloud HTTPS, secrets/budgets/telemetry, two full smoke runs.

**Stop rule:** If time runs out, retain gates 0–5 plus delayed-settlement case and a clearly labeled offline AI explanation. Never bypass ledger/reconciliation correctness to make a flashy demo.

## 16. Specification limitations and handoff notes

This package is a **build specification**, not an implemented or tested Go service. The provided SQL migrations are strong **starter schemas**, but code is still required for ledger account locking, multi-row atomicity, tenant authorization, composite case evidence validation, idempotency request replay and transaction-safe case transitions. `contracts/*.yaml` are normative proposed HTTP contracts; this pack validates their structure and its JSON example fixtures but does not independently establish a running OpenAPI server. If your agent changes any contract, modify the source YAML, snapshots, validation script and both clients in the same change.

### Cloud references to verify during deployment

- Amazon SQS standard queues: at-least-once delivery and possible out-of-order messages — https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/standard-queues.html
- Amazon Bedrock structured outputs and supported schema subset/model selection — https://docs.aws.amazon.com/bedrock/latest/userguide/structured-output.html
- Use live AWS account information for IAM permissions, Bedrock model availability/quotas and prices; do not assume a model is enabled in your chosen Region.

## 17. Solo-engineer release prioritization: P0/P1/P2

The public OpenAPI defines the **full intended client contract**, but implementing every operation in a 48-hour hackathon is not realistic for a single backend engineer. Each operation carries an `x-implementation-priority` extension; clients should build against fixtures for P1/P2 operations until implemented. No deployed endpoint may return successful placeholder data while pretending to be complete.

- **P0 = end-to-end core:** 24 routes covering demo login, live/readiness, providers and summary, transactions and settlements, run creation/status/matches, cases and evidence, decision and AI investigation, scenario creation/advance, and signed event ingestion. Complete these first.
- **P1 = richer product:** orders/detail, settlement detail, dashboard series, proof detail, audit listing, case assignment, deep investigation detail, account observations and other read screens. Implement only after the core experience works.
- **P2 = fallback:** CSV/S3 import flows, unnecessary for the primary BankSim live-demo path.

UI engineers must not infer backend implementation status from the YAML alone; ask for the current `docs/implementation-progress.md` gate report. Start with local fixture responses, then switch screens individually to real P0 endpoints as they become available. If a P1 screen is needed for presentation, implement a narrow read-only endpoint with contract tests rather than weakening reconciliation or ledger correctness.

**Realistic delivery approach:** Divide work into milestones, not promises tied to an unknown hackathon duration. If the event is only 24–48 hours, use the deterministic offline investigator first, run the BankSim and SettleGraph databases locally, and prioritize one rehearsed delayed-settlement scenario. If more time is available, enable Amazon Bedrock and optional SQS/S3. The cloud deployment can be demonstrated as an architecture and partial integration if the fully tested local system is more reliable.
