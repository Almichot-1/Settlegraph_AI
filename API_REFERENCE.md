# Complete HTTP endpoint inventory and implementation schedule

**Status:** Proposed contract; no endpoints are implemented by this document. The YAML files under `contracts/` are normative for request/response schemas, error payloads, security and parameter details.

**Priority key:** P0=first end-to-end demo; P1=rich product; P2=CSV import fallback. Web and Flutter can mock P1/P2 responses until implemented.

## Product API

### Health

**GET `/health/live`** — Process liveness  
Operation: `getLiveness` · Success: `200` · Priority: **P0**
Response JSON schema: `HealthStatus`

**GET `/health/ready`** — Required dependency readiness  
Operation: `getReadiness` · Success: `200` · Priority: **P0**
Response JSON schema: `HealthStatus`

### Identity

**POST `/v1/demo/sessions`** — Mint demo-only session token  
Operation: `createDemoSession` · Success: `201` · Priority: **P0**
Request JSON schema: `DemoSessionRequest`
Response JSON schema: `DemoSessionResponse`

**GET `/v1/me`** — Current user and organization memberships  
Operation: `getMe` · Success: `200` · Priority: **P0**
Response JSON schema: `UserProfile`

### Providers

**GET `/v1/providers`** — Provider sync and availability status  
Operation: `listProviders` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), limit (query), cursor (query)
Response JSON schema: `ProviderList`

**GET `/v1/accounts`** — Known business account views  
Operation: `listAccounts` · Success: `200` · Priority: **P1**
Parameters: X-Organization-Id* (header), limit (query), cursor (query)
Response JSON schema: `AccountPage`

### Dashboard

**GET `/v1/dashboard/summary`** — Payment and settlement summary from observations  
Operation: `getDashboardSummary` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header)
Response JSON schema: `DashboardSummary`

**GET `/v1/dashboard/series`** — Time bucket series  
Operation: `getDashboardSeries` · Success: `200` · Priority: **P1**
Parameters: X-Organization-Id* (header), granularity (query), from* (query), to* (query)
Response JSON schema: `DashboardSeries`

### Orders

**GET `/v1/orders`** — List known merchant orders  
Operation: `listOrders` · Success: `200` · Priority: **P1**
Parameters: X-Organization-Id* (header), limit (query), cursor (query)
Response JSON schema: `OrderPage`

**GET `/v1/orders/{id}`** — Get merchant order  
Operation: `getOrder` · Success: `200` · Priority: **P1**
Parameters: X-Organization-Id* (header), id* (path)
Response JSON schema: `Order`

### Transactions

**GET `/v1/transactions`** — List normalized accepted/failed/refunded payment observations  
Operation: `listTransactions` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), limit (query), cursor (query), provider_id (query), scenario_run_id (query)
Response JSON schema: `TransactionPage`

**GET `/v1/transactions/{id}`** — Get single payment observation  
Operation: `getTransaction` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), id* (path)
Response JSON schema: `Transaction`

### Settlements

**GET `/v1/settlements`** — List observed settlement records  
Operation: `listSettlements` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), limit (query), cursor (query)
Response JSON schema: `SettlementPage`

**GET `/v1/settlements/{id}`** — Get settlement observation  
Operation: `getSettlement` · Success: `200` · Priority: **P1**
Parameters: X-Organization-Id* (header), id* (path)
Response JSON schema: `Settlement`

### Reconciliation

**GET `/v1/reconciliation/runs`** — List reconciliation attempts  
Operation: `listReconciliationRuns` · Success: `200` · Priority: **P1**
Parameters: X-Organization-Id* (header), limit (query), cursor (query)
Response JSON schema: `RunPage`

**POST `/v1/reconciliation/runs`** — Queue a new deterministic reconciliation run  
Operation: `createReconciliationRun` · Success: `202` · Priority: **P0**
Parameters: X-Organization-Id* (header), Idempotency-Key* (header)
Request JSON schema: `CreateRunRequest`
Response JSON schema: `ReconciliationRun`

**GET `/v1/reconciliation/runs/{runId}`** — Get reconciliation status and counts  
Operation: `getReconciliationRun` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), runId* (path)
Response JSON schema: `ReconciliationRun`

**GET `/v1/reconciliation/runs/{runId}/matches`** — Get evidence-backed matched edges  
Operation: `listRunMatches` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), runId* (path), limit (query), cursor (query)
Response JSON schema: `MatchPage`

**GET `/v1/reconciliation/proofs/{id}`** — Get checkable match proof  
Operation: `getMatchProof` · Success: `200` · Priority: **P1**
Parameters: X-Organization-Id* (header), id* (path)
Response JSON schema: `MatchProof`

### Cases

**GET `/v1/cases`** — List cases (including mobile assigned queue)  
Operation: `listCases` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), limit (query), cursor (query), status (query), assigned_to (query), scenario_run_id (query)
Response JSON schema: `CasePage`

**GET `/v1/cases/{caseId}`** — Get case summary with version  
Operation: `getCase` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), caseId* (path)
Response JSON schema: `Case`

**GET `/v1/cases/{caseId}/evidence`** — Evidence available at latest case version  
Operation: `listCaseEvidence` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), caseId* (path), limit (query), cursor (query)
Response JSON schema: `EvidencePage`

**GET `/v1/cases/{caseId}/events`** — Immutable case timeline  
Operation: `listCaseEvents` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), caseId* (path), limit (query), cursor (query)
Response JSON schema: `CaseEventPage`

**POST `/v1/cases/{caseId}/assign`** — Assign reviewer (role OWNER or ANALYST)  
Operation: `assignCase` · Success: `200` · Priority: **P1**
Parameters: X-Organization-Id* (header), caseId* (path), Idempotency-Key* (header), If-Match* (header)
Request JSON schema: `AssignCaseRequest`
Response JSON schema: `Case`

**POST `/v1/cases/{caseId}/decisions`** — Record reviewer decision; financial truth unchanged  
Operation: `recordCaseDecision` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), caseId* (path), Idempotency-Key* (header), If-Match* (header)
Request JSON schema: `ReviewDecisionRequest`
Response JSON schema: `Case`

### Investigations

**POST `/v1/cases/{caseId}/investigations`** — Queue read-only AI analysis for current evidence snapshot  
Operation: `createInvestigation` · Success: `202` · Priority: **P0**
Parameters: X-Organization-Id* (header), caseId* (path), Idempotency-Key* (header)
Request JSON schema: `InvestigationRequest`
Response JSON schema: `Investigation`

**GET `/v1/cases/{caseId}/investigations`** — View AI investigation history  
Operation: `listInvestigations` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), caseId* (path), limit (query), cursor (query)
Response JSON schema: `InvestigationPage`

**GET `/v1/cases/{caseId}/investigations/{investigationId}`** — Retrieve evidence-gated AI result  
Operation: `getInvestigation` · Success: `200` · Priority: **P1**
Parameters: X-Organization-Id* (header), caseId* (path), investigationId* (path)
Response JSON schema: `Investigation`

**GET `/v1/jobs/{id}`** — Get async job status  
Operation: `getJob` · Success: `200` · Priority: **P1**
Parameters: X-Organization-Id* (header), id* (path)
Response JSON schema: `Job`

### Audit

**GET `/v1/audit-events`** — Read immutable audit trail  
Operation: `listAuditEvents` · Success: `200` · Priority: **P1**
Parameters: X-Organization-Id* (header), limit (query), cursor (query)
Response JSON schema: `AuditEventPage`

### Demo

**GET `/v1/demo/scenarios`** — List permitted simulator demonstrations  
Operation: `listDemoScenarios` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header)
Response JSON schema: `ScenarioList`

**POST `/v1/demo/scenarios/{scenarioKey}/runs`** — Create fresh isolated seeded simulation run  
Operation: `createScenarioRun` · Success: `201` · Priority: **P0**
Parameters: X-Organization-Id* (header), scenarioKey* (path), Idempotency-Key* (header)
Request JSON schema: `StartScenarioRequest`
Response JSON schema: `ScenarioRun`

**GET `/v1/demo/scenario-runs/{scenarioRunId}`** — Get scenario progress and virtual clock  
Operation: `getScenarioRun` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), scenarioRunId* (path)
Response JSON schema: `ScenarioRun`

**POST `/v1/demo/scenario-runs/{scenarioRunId}/advance`** — Advance a scenario by one deterministic step  
Operation: `advanceScenarioRun` · Success: `200` · Priority: **P0**
Parameters: X-Organization-Id* (header), scenarioRunId* (path), Idempotency-Key* (header)
Request JSON schema: `AdvanceScenarioRequest`
Response JSON schema: `ScenarioRun`

### Ingestion

**POST `/v1/integrations/banksim/events`** — Ingest signed BankSim webhook  
Operation: `acceptBankSimEvent` · Success: `202` · Priority: **P0**
Parameters: X-BankSim-Event-Id* (header), X-BankSim-Delivery-Timestamp* (header), X-BankSim-Signature* (header)
Request JSON schema: `BankEvent`
Response JSON schema: `WebhookAccepted`

### Imports

**GET `/v1/imports`** — [Fallback] List uploaded evidence  
Operation: `listImports` · Success: `200` · Priority: **P2**
Parameters: X-Organization-Id* (header), limit (query), cursor (query)
Response JSON schema: `ImportPage`

**POST `/v1/imports`** — [Fallback] Create S3 presigned import  
Operation: `createImport` · Success: `201` · Priority: **P2**
Parameters: X-Organization-Id* (header), Idempotency-Key* (header)
Request JSON schema: `CreateImportRequest`
Response JSON schema: `ImportUploadSession`

**GET `/v1/imports/{id}`** — [Fallback] Get ingestion progress  
Operation: `getImport` · Success: `200` · Priority: **P2**
Parameters: X-Organization-Id* (header), id* (path)
Response JSON schema: `Import`

**POST `/v1/imports/{id}/complete`** — [Fallback] Verify uploaded blob and queue parser  
Operation: `completeImport` · Success: `202` · Priority: **P2**
Parameters: X-Organization-Id* (header), id* (path), Idempotency-Key* (header)
Request JSON schema: `CompleteImportRequest`
Response JSON schema: `Import`

## BankSim internal simulator

### Simulator

**GET `/health/live`** — Simulator liveness  
Operation: `getBankSimHealth` · Success: `200` · Priority: **-**
Response JSON schema: `HealthStatus`

### Provider

**GET `/provider/v1/institutions`** — List institutions  
Operation: `listSimInstitutions` · Success: `200` · Priority: **-**
Response JSON schema: `SimInstitutionList`

**GET `/provider/v1/institutions/{institutionId}/events`** — Read source events for durable backfill  
Operation: `listBankEvents` · Success: `200` · Priority: **-**
Parameters: institutionId* (path), limit (query), cursor (query)
Response JSON schema: `EventPage`

**POST `/provider/v1/transfers`** — Create synthetic payment (admin demo only)  
Operation: `createSimTransfer` · Success: `201` · Priority: **-**
Parameters: Idempotency-Key* (header)
Request JSON schema: `SimTransferRequest`
Response JSON schema: `SimTransfer`

**GET `/provider/v1/transfers/{id}`** — Get one synthetic transfer  
Operation: `getSimTransfer` · Success: `200` · Priority: **-**
Parameters: id* (path)
Response JSON schema: `SimTransfer`

### ScenarioAdmin

**GET `/admin/v1/scenarios`** — List deterministic scenarios  
Operation: `listBankScenarios` · Success: `200` · Priority: **-**
Response JSON schema: `ScenarioList`

**POST `/admin/v1/scenarios/{scenarioKey}/runs`** — Start a new scenario  
Operation: `createBankScenario` · Success: `201` · Priority: **-**
Parameters: scenarioKey* (path), Idempotency-Key* (header)
Request JSON schema: `StartBankScenarioRequest`
Response JSON schema: `ScenarioRun`

**GET `/admin/v1/runs/{runId}`** — Scenario state  
Operation: `getBankScenario` · Success: `200` · Priority: **-**
Parameters: runId* (path)
Response JSON schema: `ScenarioRun`

**POST `/admin/v1/runs/{runId}/advance`** — Advance exactly one scenario step  
Operation: `advanceBankScenario` · Success: `200` · Priority: **-**
Parameters: runId* (path), Idempotency-Key* (header)
Request JSON schema: `AdvanceScenarioRequest`
Response JSON schema: `ScenarioRun`

### Evaluator

**GET `/evaluator/v1/runs/{runId}/truth`** — Test-only simulator ground truth  
Operation: `getSimulationTruth` · Success: `200` · Priority: **-**
Parameters: runId* (path)
Response JSON schema: `EvaluationTruth`

## Required shared headers

- `Authorization: Bearer <JWT>` on product routes except `/health/*`, demo session creation and signed webhook.
- `X-Organization-Id: <uuid>` on tenant-scoped product routes.
- `Idempotency-Key: <stable key>` on documented mutating operations.
- `If-Match: "<integer>"` on optimistic case mutations.
- Webhook only: `X-BankSim-Event-Id`, `X-BankSim-Delivery-Timestamp` and `X-BankSim-Signature`. HMAC requires exact raw-body bytes.

## Shared data and errors

- Money example: `{"minor_units":"250000","currency":"ETB"}` = ETB 2,500.00.
- Timestamp example: `2026-10-24T09:00:00Z`.
- Error: `{"error":{"code":"VERSION_CONFLICT","message":"...","request_id":"...","details":{}}}`.
- Tenant-scoped read on foreign ID: 404 to avoid enumeration.
- Webhook duplicates ACK with HTTP 202 and `{"accepted":true,"duplicate":true}`.
- No live-money or real-bank guarantees: all BankSim amounts are simulated.
