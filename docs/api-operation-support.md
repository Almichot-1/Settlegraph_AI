# Public operation support

The normative contract declares 40 operations. This local implementation supports 38; optional import upload/parse creation and completion remain deferred. This table describes implementation scope, not a claim that every operation has a browser or native screen. Detailed executed checks are in [implementation progress](implementation-progress.md).

| Method | Path | Priority | Local runtime |
|---|---|---|---|
| GET | `/health/live` | P0 | Implemented |
| GET | `/health/ready` | P0 | Implemented |
| POST | `/v1/demo/sessions` | P0 | Implemented |
| GET | `/v1/me` | P0 | Implemented |
| GET | `/v1/members` | P0 | Implemented |
| GET | `/v1/providers` | P0 | Implemented |
| GET | `/v1/accounts` | P1 | Implemented |
| GET | `/v1/dashboard/summary` | P0 | Implemented |
| GET | `/v1/dashboard/series` | P1 | Implemented |
| GET | `/v1/orders` | P1 | Implemented |
| GET | `/v1/orders/{id}` | P1 | Implemented |
| GET | `/v1/transactions` | P0 | Implemented |
| GET | `/v1/transactions/{id}` | P0 | Implemented |
| GET | `/v1/settlements` | P0 | Implemented |
| GET | `/v1/settlements/{id}` | P1 | Implemented |
| GET | `/v1/reconciliation/runs` | P1 | Implemented |
| POST | `/v1/reconciliation/runs` | P0 | Implemented |
| GET | `/v1/reconciliation/runs/{runId}` | P0 | Implemented |
| GET | `/v1/reconciliation/runs/{runId}/matches` | P0 | Implemented |
| GET | `/v1/reconciliation/proofs/{id}` | P1 | Implemented |
| GET | `/v1/cases` | P0 | Implemented |
| GET | `/v1/cases/{caseId}` | P0 | Implemented |
| GET | `/v1/cases/{caseId}/evidence` | P0 | Implemented |
| GET | `/v1/cases/{caseId}/events` | P0 | Implemented |
| POST | `/v1/cases/{caseId}/assign` | P1 | Implemented |
| POST | `/v1/cases/{caseId}/decisions` | P0 | Implemented |
| POST | `/v1/cases/{caseId}/investigations` | P0 | Implemented |
| GET | `/v1/cases/{caseId}/investigations` | P0 | Implemented |
| GET | `/v1/cases/{caseId}/investigations/{investigationId}` | P1 | Implemented |
| GET | `/v1/jobs/{id}` | P1 | Implemented |
| GET | `/v1/audit-events` | P1 | Implemented |
| GET | `/v1/demo/scenarios` | P0 | Implemented |
| POST | `/v1/demo/scenarios/{scenarioKey}/runs` | P0 | Implemented |
| GET | `/v1/demo/scenario-runs/{scenarioRunId}` | P0 | Implemented |
| POST | `/v1/demo/scenario-runs/{scenarioRunId}/advance` | P0 | Implemented |
| POST | `/v1/integrations/banksim/events` | P0 | Implemented |
| GET | `/v1/imports` | P2 | Implemented |
| POST | `/v1/imports` | P2 | DEFERRED: optional S3/CSV upload and parser |
| GET | `/v1/imports/{id}` | P2 | Implemented |
| POST | `/v1/imports/{id}/complete` | P2 | DEFERRED: optional S3/CSV upload and parser |

`GET /v1/accounts` exposes known metadata; balance stays null until a provider observation supplies it. Import reads return stored product records, without claiming an implemented upload pipeline. Investigation jobs default to OFFLINE_STUB; the Bedrock adapter is fake-client tested and actual account invocation is NOT_RUN. Provider webhooks are server-to-server and intentionally unavailable through the browser BFF. Production API identity and AWS deployment remain separate gates.
