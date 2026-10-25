# SETTLEGRAPH AI — WEB APPLICATION ARCHITECTURE

**Dedicated handoff for the Next.js engineer**  
**Baseline:** Public API `2.0.0-draft` · Next.js 15.x App Router · React 19 · TypeScript · October 2026  
**Status:** Engineering design and existing starter; not a claim that the Go backend is deployed.

## 1. Mission, boundaries and the first demo

Build the **merchant investigation workstation** for SettleGraph AI. The web product turns independently sourced orders, payment observations and settlements into a usable dashboard, reconciliation flow, evidence-review queue and controlled BankSim demonstration. It **does not process or move real money**, authenticate bank balances, or allow AI-generated claims to become financial truth.

**First demonstrable workflow:** Demo Owner signs in → chooses `delayed_settlement` → advances BankSim through payment acceptance → sees missing settlement / incomplete evidence → opens an investigation case → requests Bedrock analysis → examines cited source evidence and limitations → releases the delayed settlement → observes an `EVIDENCE_UPDATED` case → enters a human decision → reads the audit history.

**Hard product invariants**

- Show `orders_expected`, `payments_accepted`, `settlements_observed` and `unmatched_payments` as four different measures. Never add these as though they were four independent balances.
- Show the **source completeness** and `last_observed_at` time. An unavailable provider is not proof of a failed or stolen payment.
- Label every BankSim source and offline mock as **SIMULATED**. Treat `SIGNED_SIMULATED_PROVIDER` as evidence from a simulator, not a real bank.
- Distinguish AI hypotheses from verified matching results; provide clickable evidence references and limitations.
- All bank/provider interaction is through the Go public API; the browser never calls BankSim private/admin APIs.
- Keep tenant enforcement, role authorization, financial invariants and case version checking on the backend even if web buttons are hidden by role.

## 2. Context architecture and trust zones

![Web system context](diagrams/01_system_context.svg)

**Zone A — Browser.** Renders pages, collects user intent, holds only non-sensitive view state, and sends same-origin requests to Next.js. No bearer token, bank signature, Bedrock credentials or demo access code belongs in client JavaScript or `NEXT_PUBLIC_*` variables.

**Zone B — Next.js web server (BFF).** Exposes `/api/session` for demo login/logout and `/api/sg/*` as a controlled same-origin API proxy. Owns HTTP-only session cookies, adds backend `Authorization` and `X-Organization-Id`, forwards request identifiers and normalizes backend errors. This is not where money is reconciled.

**Zone C — Go backend.** Owns authentication authorization, organizations, financial data, reconciliation, case versioning, idempotency and audit. Only the Go backend can decide if a deterministic matching proof exists.

**Zone D — BankSim + worker + Bedrock.** BankSim is an independent simulated financial source; the worker runs reconciliation/investigation; Bedrock proposes explanations, not accounting entries. These are never directly reachable from browser code.

The current package already has a `GET/POST` proxy and mock-mode switching. Before public deployment harden the proxy with a **route/operation allowlist, request-size limits, CSRF policy, secure cookie settings, and upstream timeouts**; these are required upgrades, not claims about the starter.

## 3. Web architecture: deliberate simplicity

![Component diagram](diagrams/02_web_components.svg)

**Pattern: App Router + typed client + server-side BFF + pluggable data adapter.** Keep the existing starter as the baseline, then incrementally improve it. Do not rewrite the app into microfrontends or create a second financial-rules engine in TypeScript.

| Layer | Location in starter | Ownership and rules |
|---|---|---|
| Route and page composition | `app/(workspace)/**/page.tsx`, `app/login` | URLs, transitions, loading/error components, accessibility |
| Presentation | `src/components/shell.tsx`, `src/components/ui.tsx` | Reusable visual components, no direct bank calls |
| Public operation wrappers | `src/lib/api.ts` | One function per allowed public operation, defined by OpenAPI |
| Fetch + errors | `src/lib/client.ts` | Same-origin `/api/sg/*`, typed error, idempotency and `If-Match` |
| Generated DTO types | `src/generated/types.ts` | Generated from authoritative YAML; do not edit by hand |
| BFF session | `app/api/session/route.ts` | Login, logout, HttpOnly cookies; demo code stays on server |
| BFF proxy | `app/api/sg/[...path]/route.ts` | Tenant header, forward allowed endpoints, no caching of money data |
| Offline test adapter | `src/server/mock-engine.ts` | Synthetic response fixtures and workflow state; not a bank ledger |
| Feature-specific view models (planned) | `src/features/*` or local hooks | Own view state, polling and form state, not matching rules |

**Dependency direction:** pages/components → typed operation wrappers → fetch client → BFF → Go API (or mock adapter). Generated DTOs are read-only representations of the wire contract. No page may call BankSim; no component may implement financial matching or change a case status locally without an API response.

## 4. Route map and screen ownership

| URL | User goal and component behavior | Main public API operations | Priority |
|---|---|---|---|
| `/login` | Request demo session, handle invalid code and expiry | `createDemoSession`, `getMe` through BFF | MVP |
| `/dashboard` | Four separate financial measures, provider freshness, case preview, source coverage | `getDashboardSummary`, `getDashboardSeries`, `listProviders`, `listCases` | MVP |
| `/transactions` | Filter by provider, cursor pagination, inspect source attestation/reference | `listTransactions`, `getTransaction`, `listProviders` | MVP |
| `/reconciliation` | Start run, show queued/running/complete/failed results | `createReconciliationRun`, `listReconciliationRuns`, `getReconciliationRun` | MVP |
| `/cases` | Investigation inbox, status/severity filters, case navigation | `listCases` | MVP |
| `/cases/[caseId]` | Evidence, case events, AI result, assignment and human decision | `getCase`, `listCaseEvidence`, `listCaseEvents`, `createInvestigation`, `listInvestigations`, `getInvestigation`, `assignCase`, `recordCaseDecision` | MVP |
| `/simulation` | Choose and advance BankSim fault scenario through public demo endpoints | `listDemoScenarios`, `createScenarioRun`, `getScenarioRun`, `advanceScenarioRun` | MVP |
| `/audit` | Read-only trace of user and workflow actions | `listAuditEvents` | MVP |
| Future import wizard | Optional CSV intake fallback, must not block BankSim demo | `listImports`, `createImport`, `getImport`, `completeImport` | P2 |

**Front-end deliverables**: a full responsive workstation for owners/reviewers, component library, mock-mode flows, client integration tests, deployment-ready Next server runtime and production security review. The mobile developer owns the companion review experience, not this workstation.

## 5. Financial data visualization rules

The public API's `Money` object is `{ "minor_units": "250000", "currency": "ETB" }`; that's **2,500.00 ETB**. Never parse money via JavaScript `Number` or use floating-point math for monetary totals. Use `BigInt` solely for formatting, and use totals returned by the backend instead of computing new business totals by summing displayed fields.

Use consistent semantics:

| UI element | Data and meaning | Required caveat |
|---|---|---|
| Orders expected | `DashboardSummary.orders_expected` | Orders are not received money |
| Gateway payments accepted | `payments_accepted` | Provider acceptance is not final bank settlement |
| Settlement observations | `settlements_observed` | Observed in imported/simulated sources |
| Unmatched amount | `unmatched_payments` | Unreconciled, **not proven lost** |
| Source health | `providers[].sync_status` | `HEALTHY`, `STALE`, `OUTAGE`, `NEVER_SYNCED` |
| Coverage | `coverage.source_status`, `coverage.is_complete` | Incomplete data invalidates strong absence claims |
| AI provenance | `Investigation.provider` | `OFFLINE_STUB` never marketed as live Bedrock |
| Payment source | `Transaction.source_attestation` | `SIGNED_SIMULATED_PROVIDER` / `IMPORTED_UNVERIFIED` |

Present timestamps in the user's timezone while displaying original UTC/ISO data in detail views if ambiguity matters. Display source staleness, not an invented "live" badge.

## 6. Authentication, session and role architecture

![Web BFF session](diagrams/03_login_bff.svg)

1. Browser submits a **display name only** to `POST /api/session`. The Next server appends the server-only `DEMO_ACCESS_CODE` to the backend's `POST /v1/demo/sessions`. The response from that operation contains the JWT and default organization.
2. Next stores JWT and organization ID in HTTP-only cookies (`Secure` in production, `SameSite` and finite max age); the browser receives only non-secret identity details.
3. The browser calls `/api/sg/v1/*`; the BFF attaches bearer token and `X-Organization-Id`. `GET /v1/me` verifies membership.
4. Browser and backend role guards: `OWNER` controls simulation and assignment, `REVIEWER` submits decisions, `VIEWER` reads permitted resources. Hiding a button is UI only; Go is authoritative.
5. Logout deletes all session cookies, client query cache, in-flight investigations and organization-specific view state.

**Hardening backlog before real deployment:** force HTTPS; explicitly restrict proxy paths and methods to approved public operations; validate `Origin`/Host on every mutation and use a CSRF strategy appropriate for your hosting; enforce request-size limits; avoid logging cookie or Authorization headers; securely handle token expiry; verify each organization switch server-side. Do not set CORS to allow arbitrary origins. Avoid long-lived bearer cookies for real financial customers without production-grade identity architecture.

## 7. Request lifecycle, caching and asynchronous jobs

![Asynchronous reconciliation](diagrams/04_reconciliation_workflow.svg)

- **Read requests:** `GET` with tenant headers, `cache: 'no-store'` for financial records; cancel stale browser fetches on route/filter changes; show loading, retry and clear failure messages. Return to the previous data only with an explicit "last refreshed" indicator.
- **Mutations:** generate one `Idempotency-Key` for each *logical user action*; reuse it on retries of that same action. Never generate a fresh key after a timeout merely because the first response was lost.
- **Reconciliation:** `POST /v1/reconciliation/runs` returns HTTP `202` with a run (`QUEUED` or `RUNNING`); bounded polling of `GET /v1/reconciliation/runs/{runId}` stops at `COMPLETED` or `FAILED`. Poll every ~1–2 seconds while visible, use capped attempts, and offer manual refresh afterward. Do not assume a new case appears immediately when a simulation step advances.
- **AI investigation:** `POST /v1/cases/{caseId}/investigations` returns `202`; display `QUEUED`, `RUNNING`, `COMPLETED`, `REJECTED`, `FAILED`; only show explanations after a successful result; always surface `limitations` and evidence IDs.
- **Cursor pagination:** use API-provided `next_cursor`, never infer offsets; reset cursors when provider, case filter or active organization changes.
- **Conflict handling:** `412` means `If-Match` case version is stale; reload the case and ask the reviewer to reconsider. `409` means idempotency conflict, not a request to blindly retry with a fresh key. `428` means missing required precondition. `429` requires backoff and a human-readable message.

Recommended UI state machine for every API view: `idle → loading → success | empty | error`, with a separate `refreshing` state that does not erase valid currently displayed data. Keep `error.request_id` accessible in diagnostic views without dumping sensitive payloads.

## 8. Case review — the most important interface

![Case review](diagrams/05_case_review.svg)

**Case detail layout:** top summary (kind/status/severity/version and source coverage); evidence panel (each `Evidence.id`, type, amount, provider, SHA metadata); event timeline (`CaseEvent`, exact recorded times); AI hypothesis panel (`InvestigationResult.summary`, hypotheses, evidence IDs, recommended actions, limitations); human decision section (role gated, version gated, reason required).

**Evidence interaction** must link each AI cited evidence ID to a real case evidence record. If an evidence ID cannot be resolved in the current case snapshot, flag it as an unsupported claim and do not render it as verified evidence. Distinguish `REJECTED` AI result from "case resolved".

**Human decision behavior:** POST a `ReviewDecisionRequest` containing enum `RESOLVE | REOPEN | NEEDS_MORE_EVIDENCE` plus 5–2000-character reason, with `If-Match: "<case.version>"` and `Idempotency-Key`; on `200`, re-fetch `getCase` and `listCaseEvents`; on `412`, show a conflict dialog that presents new evidence before resubmitting. Never optimistically mutate an approved financial status.

## 9. BankSim demo UI architecture

The control room calls only four public demo operations: `listDemoScenarios`, `createScenarioRun`, `getScenarioRun`, and `advanceScenarioRun`. Use `ScenarioRun.step`, `total_steps`, and status `PREPARING | READY | RUNNING | COMPLETED | FAILED` as source-of-truth. Scenarios are `normal_flow`, `delayed_settlement`, `duplicate_webhook`, and `provider_outage`.

**Important:** `/v1/demo/scenario-runs/{id}/advance` returning `200` confirms step acceptance, **not** that webhooks were ingested and reconciled. Poll the relevant payments/cases until they appear or clearly show "processing". Maintain run ID across navigation (URL query parameter or server session), avoiding stale multiple-run confusion.

**Demonstration 1:** delayed settlement raises `SETTLEMENT_NOT_OBSERVED` only after supported source/grace conditions. Later evidence changes case to `EVIDENCE_UPDATED`, human alone resolves it.  
**Demonstration 2:** duplicate webhook does not double count accepted money or create duplicate matching rows.  
**Demonstration 3:** provider outage produces `SOURCE_UNAVAILABLE` / incomplete coverage rather than "money stolen".

## 10. Mock-first development without integration drift

![Mock to live](diagrams/06_modes.svg)

Use `SETTLEGRAPH_DATA_MODE=mock` for local design. The existing starter serves synthetic, stateful responses from `src/server/mock-engine.ts`; it is not a substitute for the Go ledger and resets on server restart. Use `SETTLEGRAPH_DATA_MODE=backend` and server-only `SETTLEGRAPH_API_BASE_URL` when the Go API is available. Both paths must honor **the same OpenAPI DTOs, status codes and operation names**.

Do not change backend field names to simplify React components. If the backend agent proposes a breaking change, edit `contracts/settlegraph-public.openapi.yaml` first, add a migration note, regenerate TypeScript types, and inform Flutter. In the meantime isolate temporary mapping in the BFF rather than silently forking the API contract.

## 11. UX system and accessibility

- **Shell:** desktop sidebar with overview, payments, reconciliation, investigations, simulation and audit; mobile web collapses to accessible navigation. The Flutter app is a dedicated companion, not a requirement to ship feature parity.
- **Visual priority:** four money metrics in separate groups; provider freshness and source coverage; a prominently accessible investigation inbox; audit detail behind an explicit click.
- **Use honest states:** ``Simulated source``, ``Source data incomplete``, ``AI hypothesis``, ``Review required``, ``Case resolved by reviewer``. Never call unsupported claims verified.
- **Typography/formatting:** tabular numerals, ETB exact formatting, relative time with full timestamp on hover/focus, long IDs with copy actions, severity not indicated by color alone.
- **Accessibility:** headings in order, meaningful button names, visible keyboard focus, dialog focus trap, escape-to-close, form error association, aria-live for background completion, screen-reader status labels.
- **Responsive:** design for 1280 px desktop, 768 px tablet and 375 px mobile browser; tables can scroll horizontally but actions must remain reachable.
- **Performance:** avoid huge client-side filtering of entire pages; use opaque cursor pagination. For 1,000 synthetic observations, render only requested pages. Poll only visible active jobs.

## 12. Environment and delivery architecture

**Development:** `cp settlegraph-web/.env.example settlegraph-web/.env.local` and run `npm install; npm run typecheck; npm test; npm run dev`. Backend mode uses `SETTLEGRAPH_API_BASE_URL=http://localhost:8080`, `DEMO_ACCESS_CODE` set server-side and mock mode disabled. No browser needs direct CORS access to the Go API.

**Production target:** a Next.js Node server behind HTTPS/reverse proxy, Go API on a private or access-controlled endpoint, restrictive egress and origin policy, runtime-injected secrets, log scrubbing, server-side session controls, security headers and health checks. AWS could host this on App Runner/ECS/other managed container service, but select the simplest supported runtime and verify the backend endpoints first. The Next.js proxy should never expose arbitrary upstream URL or forward arbitrary HTTP headers.

## 13. Testing pyramid and failure matrix

| Test scope | Minimum verification | Example failure to prevent |
|---|---|---|
| Format helpers | `Money.minor_units` with BigInt, negative/zero/large amounts | 2,500 ETB accidentally shown as 250,000 ETB |
| API client | Method, body, allowed path, `Idempotency-Key`, `If-Match`, error parsing | Wrong header or swallowed 412 |
| BFF security | No JWT/client demo code exposure, session cookie flags, origin/path restrictions | Browser bypasses auth or calls private webhook |
| Contract fixtures | Validate against YAML, reject invalid enum or fabricated evidence ID | Frontend silently accepts incompatible backend JSON |
| Screen states | Loading, empty, error, stale source, success, 401/403/412/429/502 | Silent empty dashboard during outage |
| Scenario integration | Delay, duplicate, outage and recovery driven via public demo API | Wrong payment totals or case auto-resolved |
| Accessibility | Keyboard nav, contrast, accessible modal, forms | Cannot complete reviewer decision by keyboard |
| Build/CI | `npm run typecheck`, `npm test`, `npm run build` | Package compiles in mock but breaks after type changes |

Recommended additions for production-quality CI: Playwright browser tests (not currently installed), mocked backend contract tests, dependency audit, content-security-policy tests and deployment smoke tests. Do not claim these already exist.

## 14. Exact handoff gates with backend and Flutter

**Gate A — Mock ready:** same OpenAPI SHA-256 as backend; all routes load using fixtures; complete mock scenario with honest synthetic labels.  
**Gate B — Session ready:** Go `POST /v1/demo/sessions` + `GET /v1/me` live; BFF correctly sets server-only cookie; session invalidation and role tests pass.  
**Gate C — Payment and case reads:** summary, providers, transactions, cases, evidence and events live; IDs, money and cursor handling match.  
**Gate D — Mutations:** reconciliation `202`, investigation `202`, review `If-Match` and idempotency errors tested live.  
**Gate E — Demo:** BankSim scenario, delayed settlement recovery, no false AI claim, all review/audit paths work end-to-end.  
**Gate F — Release:** HTTPS, no secrets in bundles/logs, build and browser smoke tests pass, simulation disclosures unmistakable.

**Definition of done for frontend owner:** a new teammate can unzip the pack and open the mock app, explain every displayed money number, perform the main review flow without touching internal BankSim APIs, pass contracted error-state tests, switch to backend mode through server config, and produce a reproducible demo checklist.

## 15. Non-goals and design trade-offs

Not building a payments gateway, live bank dashboard, customer funds transfer, production identity provider, self-service real bank onboarding, automatic financial corrections, custom charting framework or frontend microservices. Defer CSV import UI, elaborate analytics, exports, notifications and real-time sockets until the core scenario passes. Prefer a simple, correct BFF and typed fetch wrappers over adding five new libraries before MVP completion.

## 16. Reference implementations

- Existing source: `starter/settlegraph-web/`; its current structure is a scaffold, not a completed production product.
- Canonical contract: `contracts/settlegraph-public.openapi.yaml` (39 operations, `2.0.0-draft`).
- Endpoints and priority: `docs/API_OPERATION_MATRIX.csv` and `docs/API_OPERATION_MATRIX.json`.
- Detailed tasks and proof gates: `IMPLEMENTATION_PLAN.md`.
- Next.js 15 data security: https://nextjs.org/docs/15/app/guides/data-security

**Change control:** the backend lead owns the normative OpenAPI revision. The web owner may propose a change, but must coordinate impact with Flutter; never silently adjust a production JSON shape in only one client.
