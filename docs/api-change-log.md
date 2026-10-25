# HTTP contract change log

Contract state: **2.0.0-draft; partially implemented.** Normative public file: `contracts/settlegraph-public.openapi.yaml`. Client acknowledgment is still pending.

| Date (UTC) | Change ID | operationId | Compatibility | Client impact | Reviewed by |
|---|---|---|---|---|---|
| 2026-10-24 | BASELINE | All 39 public operations | Initial definition | Next.js and Flutter build mocks from YAML | Awaiting team acknowledgment |
| 2026-10-24 | INT-001 | BankSim `listProviderEvents` | Backward compatible optional `scenario_run_id` query filter | No public client impact; prevents coverage from mixing scenario runs | Backend local test; client acknowledgment not required for internal route |
| 2026-10-24 | API-001 | `createReconciliationRun` | Behavioral correction within existing schema: `LATEST_OBSERVATIONS` uses unscoped evidence and returns a null `scenario_run_id` | Clients can display the scope without attributing it to a demo scenario | Backend integration and end-to-end tests; client acknowledgment pending |

## New change request template

```text
Change ID: API-___
Timestamp UTC:
OperationId / route / method:
Old schema and behavior:
New schema and behavior:
Backward compatible? Why:
Fixture(s) changed:
Next.js acknowledged (name/time):
Flutter acknowledged (name/time):
Runtime contract tests executed:
Approval and effective version:
```

No silent renaming of enum members, money encoding, version headers, error shape or required security headers. YAML must be changed first.

## 2026-10-25 — Mobile integration runtime corrections (nonbreaking)

No OpenAPI or fixture shape changed. `listCases` now applies documented `assigned_to`, `status`, and `scenario_run_id` before keyset pagination; invalid filters return the existing 400 error envelope. `listInvestigations` now honors existing `limit`/`cursor` parameters with ID ordering. No order was promised in the contract; clients sort investigation history by `created_at` and ID for presentation. PostgreSQL-backed filter/pagination and tenant/cursor negative tests passed. Runtime test scripts now accept optional `SETTLEGRAPH_API_URL`, preserving the default localhost:8080.

## 2026-10-25 — integrated client contract

- Additive: `GET /v1/members` (`listMembers`, OWNER/ANALYST) returns active tenant member IDs, display names and roles with opaque pagination. Public contract now has 40 operations; regenerate both client fixtures/types after changes.
- Demo login retains its JSON shape. Optional server-configured reviewer/analyst/viewer access codes select distinct seeded principals. Display-name input does not rename shared identities or grant a role. Existing owner code remains supported.
- Permission correction: decisions require OWNER/REVIEWER, matching the handoff and Flutter. ANALYST retains assignment/investigation permissions.
- Runtime corrections: transaction provider/scenario filters are applied before pagination; case-event/match/audit continuation is implemented; dashboard series enforces its required from/to range and half-open UTC buckets. Callers that previously omitted required dates must supply them.

### Local integration reliability

Nonbreaking response documentation: every rate-limited public POST now declares 429 with `Retry-After`; demo authentication declares its existing 401 response. Provider webhook delivery is excluded from the user mutation limiter. Defaults are 30 demo logins and 120 mutations per minute per API process; configure limits through the API runtime environment.

Investigation creation retains the 202 contract but now returns QUEUED promptly. The durable worker publishes COMPLETED/REJECTED/FAILED through polling; `result` is omitted until available. Canonical evidence snapshots/provenance are immutable. OFFLINE_STUB remains the default, and explicit Bedrock failure is visible rather than silently converted to an offline result. No financial observation or matching contract changes.
