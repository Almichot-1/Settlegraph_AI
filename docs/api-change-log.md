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
