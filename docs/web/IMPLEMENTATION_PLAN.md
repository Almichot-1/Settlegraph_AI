# WEB ENGINEERING DELIVERY PLAN — NEXT.JS

**Owner:** Next.js frontend engineer  
**Backend owner:** Go/Codex autonomous builder  
**Deliverable:** Merchant workstation + controlled mock mode + integration-ready BFF  
**Contract:** `contracts/settlegraph-public.openapi.yaml` (`2.0.0-draft`)

## Working agreement

- Keep this project in its **own repository**. Never run the frontend agent inside the backend agent's working directory.
- Backend OpenAPI YAML is the immutable starting point. Propose modifications through a versioned issue/PR; do not silently fork.
- Every task below has a tested definition of done. Check it only after executing listed checks.
- Mark each task `TODO`, `DOING`, `BLOCKED`, or `ACCEPTED` with test evidence in `docs/WEB_PROGRESS_REPORT.md`.
- Start from existing `starter/settlegraph-web`, don't replace it without a demonstrable architectural reason.
- Once a phase passes, commit it to Git. Mock screenshots alone do not establish backend integration.

## Phase 0 — Bootstrap and contract freeze

### WEB-001: Restore reproducible build (MVP)
**Depends:** none.  
**Implement:** install dependencies, copy `.env.example` to `.env.local` (mock mode), confirm Next routes, inspect Node/Next versions, ensure app launches with no backend. Keep `.env.local` out of Git.  
**Test:** `npm install`, `npm run typecheck`, `npm test`, `npm run build`, `npm run dev`. Record failures accurately; fix only genuine starter issues.  
**Accept:** login and `/dashboard` render using fictional data; all four commands pass, or failures are documented and repaired. No backend is required.

### WEB-002: Generate contract awareness (MVP)
**Depends:** WEB-001.  
**Implement:** validate `contracts/settlegraph-public.openapi.yaml` against the backend copy (SHA-256); check `src/generated/types.ts` and wrapper operations. Keep `docs/API_OPERATION_MATRIX.csv` in repo.  
**Test:** `npm run check:contracts`; inspect operation IDs `getDashboardSummary`, `createInvestigation`, `recordCaseDecision` and `advanceScenarioRun`.  
**Accept:** no invented endpoints, no missing required enum conversions, app and mobile share the same contract SHA.

## Phase 1 — Shell, session, client transport

### WEB-003: Responsive layout and navigation (MVP)
**Depends:** WEB-001.  
**Implement:** desktop side nav, tablet and phone navigation, workspace header, source-mode badge, role-aware navigation; reusable `Loading`, `Empty`, `Failure`, `Badge` components; scrollable tables.  
**Test:** browser widths 375/768/1280, keyboard-only navigation, focus visibility, text zoom, error and empty states.  
**Accept:** every route reachable, no hidden actions on narrow layouts, synthetic data visibly labeled.

### WEB-004: Session and BFF security (MVP)
**Depends:** WEB-002.  
**Implement:** `POST/GET/DELETE /api/session` flow and secure BFF proxy; distinguish login from Bearer-authenticated calls; follow allowed operations only; enforce same-origin mutation policy; production `Secure` cookie policy and request-size / timeout limit. No `NEXT_PUBLIC_*` secrets.  
**Test:** login success/failure, 401 on missing cookie, 403 on cross-origin mutation, logout clears cookies, browser bundle lacks token/demo code, proxy denies `/v1/integrations/banksim/events` and internal bank URLs.  
**Accept:** role and tenant info come from session/backend and are not trusted solely from browser-selected values. No bank API traffic from browser.

### WEB-005: Unified typed request states (MVP)
**Depends:** WEB-002 and WEB-004.  
**Implement:** one typed request client, clear `ApiRequestError`, aborted stale requests, status-specific UI errors, request IDs, one idempotency key per logical mutation, support quoted `If-Match`.  
**Test:** 400/401/403/404/409/412/428/429/502/503 mocks; canceled request; retry with same key; error returned without JSON body.  
**Accept:** view never silently interprets server failure as no transactions; mutation result uncertainty is visible.

## Phase 2 — Merchant overview and financial observations

### WEB-006: Dashboard source-aware metrics (MVP)
**Depends:** WEB-003, WEB-005.  
**Implement:** four distinct metrics from `DashboardSummary`, provider freshness/coverage tiles, optional time series, open-case preview and last observed time.  
**Test:** `minor_units="250000"` → 2,500 ETB, massive and negative integer strings, incomplete source, null provider observation time, 1,000-record fixture responsiveness.  
**Accept:** user cannot confuse successful gateway payments with confirmed settlement; source status readable without color.

### WEB-007: Payment activity and details (MVP)
**Depends:** WEB-005.  
**Implement:** provider filter from `listProviders`, cursor-based `listTransactions`, detail using `getTransaction`, source attestation, timestamp and reference copy.  
**Test:** changing provider resets cursor; empty result; 30/50 row pages; delayed server; a payment marked accepted but not settled.  
**Accept:** no invented bank balance, no client-side float arithmetic or offset pagination.

## Phase 3 — Reconciliation and investigation cases

### WEB-008: Async reconciliation UI (MVP)
**Depends:** WEB-005, WEB-006; backend required for live gate.  
**Implement:** run button with stable logical-action key, 202 handling, bounded `getReconciliationRun` poll and terminal UI, partial/failure states, optional match proof details as P1.  
**Test:** queued→running→completed, queued→failed, timeout, 503 worker offline, repeated tap, duplicate request replay with same key.  
**Accept:** no false 'completed' state before API terminal result; refreshing is safe.

### WEB-009: Case inbox and evidence detail (MVP)
**Depends:** WEB-005.  
**Implement:** list/filter cases, exact case detail, evidence IDs, source reference labels, event timeline, case version display, assignee and severity.  
**Test:** `OPEN`, `EVIDENCE_UPDATED`, `RESOLVED`, missing case 404, tenant 404, pagination, evidence count mismatch, long reference overflow.  
**Accept:** reviewer can trace source records and identify what remains unverified.

### WEB-010: AI investigation panel (MVP)
**Depends:** WEB-009.  
**Implement:** `createInvestigation` returns 202; bounded poll; completed/rejected/failed states; evidence citation matching and limitation panel; Bedrock vs `OFFLINE_STUB` label.  
**Test:** invalid/fabricated evidence ID fixture is visibly unsupported, rejected output never appears verified, 429 timeout and 503 inference unavailable.  
**Accept:** no UI copy says AI alone confirmed bank receipt.

### WEB-011: Assignments and reasoned human decisions (MVP)
**Depends:** WEB-009.  
**Implement:** role-gated decision form with reason validation and `If-Match` from current case; fixed logical-action idempotency key; 412 conflict reload and user re-review; refresh events on 200. Assignment optional P1 if backend unavailable.  
**Test:** OWNER/REVIEWER/VIEWER, valid and invalid reasons, stale version 412, duplicate same key, 409 changed payload, 428 no version, 403 unauthorized.  
**Accept:** case cannot close on AI output or on settlement arrival automatically.

## Phase 4 — Demonstration and audit

### WEB-012: BankSim control room (MVP)
**Depends:** WEB-005 and backend demo API.  
**Implement:** scenario catalog, selected key, deterministic seed, create/advance run, step progress, run ID, refresh and links to resulting cases/transactions; label all records synthetic.  
**Test:** `normal_flow`, `delayed_settlement`, `duplicate_webhook`, `provider_outage`; out-of-order updates; step advance 200 before cases are ready.  
**Accept:** UI never calls private BankSim API, source outage is not characterized as missing money, duplicate event does not inflate totals.

### WEB-013: Audit trail and demo rehearsal (MVP)
**Depends:** WEB-009, WEB-011, WEB-012.  
**Implement:** `listAuditEvents` page; deterministic demonstration script: accepted → unmatched → AI hypotheses → settlement observed → reviewer closes → timeline.  
**Test:** refresh audit history, event ordering, long event list, unsupported evidence display, 401 mid-demo.  
**Accept:** same seeded demo can be repeated and explained by a different teammate without scripting hidden data mutations.

## Phase 5 — Live backend and release gate

### WEB-014: Backend integration (MVP)
**Depends:** WEB-001 through WEB-013; Go API implementation is external dependency.  
**Implement:** switch server to backend mode, verify login and organization, read paths, reconciliation 202, evidence, mutations, simulation and audit; remove mock-only assumptions.  
**Test:** integration smoke on actual Go backend; observe network traces to confirm only Next server calls Go; 401/403/409/412/429/502; verify Go and frontend schema hashes.  
**Accept:** full demo works with Go, not just mock adapter; team signs off contract revision.

### WEB-015: Quality and deployment (MVP)
**Depends:** WEB-014.  
**Implement:** release configuration, HTTPS, restrictive BFF route forwarding, sensible timeouts, browser error boundaries, runtime logging without tokens, accessibility and responsive QA.  
**Test:** `npm run typecheck`, `npm test`, `npm run build`, `npm run check:contracts`; manual desktop/phone browser demo; add Playwright if time available.  
**Accept:** zero exposed secrets, no broken routes, all live critical journeys pass, documented release build and rollback.

## Backend dependencies to report early

| Backend dependency | Web blocker if missing | Local mitigation |
|---|---|---|
| Demo login and `GET /v1/me` | Cannot test live auth | Mock `/api/session` only; do not fake success in backend mode |
| Dashboard/provides and payments | No live money feed | Fixture-driven mock with conspicuous simulated badge |
| Case detail/evidence/events | No live investigation | Seed case fixtures and typed renderers |
| Investigation async endpoints | No live Bedrock proof | Display `OFFLINE_STUB` rather than AI live claim |
| BankSim scenario endpoints | No live fault injection | Demo mock scenario; label as mock, not running bank network |
| Review mutation and versions | Cannot validate human authorization | Unit + mock conflict tests; hold live sign-off |

## No-compromise cross-team contract rules

- The YAML and its SHA-256 are normative across backend, Next.js, and Flutter.
- Every breaking change needs a proposal with changed operation, DTO field, error code and example response, plus frontend/mobile acknowledgment.
- Test actual request/response behavior, not only mock types. No promise of real-money visibility.
- Review report should distinguish `mock-tested`, `contract-checked`, `backend-integrated`, and `release-tested` clearly.

## Final proof bundle from frontend owner

Deliver: Git URL/commit, screenshots of seven main screens, one screen recording of delayed settlement, exact test command outputs, OpenAPI SHA, source mode/version, security checklist, unresolved blockers and a backend integration matrix. **Do not mark a task accepted without a reproducible test result.**
