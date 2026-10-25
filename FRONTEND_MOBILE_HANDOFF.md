# Frontend + Flutter API handoff

**Contract status:** Version `2.0.0-draft`. The local synthetic backend and Flutter client are implemented; see `docs/implementation-progress.md` for actual test evidence and remaining client/cloud gates. All mock examples are synthetic. `contracts/settlegraph-public.openapi.yaml` is the **normative schema**.

## Runtime conventions

| Topic | Required behavior |
|---|---|
| Development URL | `http://localhost:8080` (configurable in client environment) |
| Production URL | HTTPS URL provided after deployment, never assume public simulator address |
| Authentication | Login: `POST /v1/demo/sessions` with server-distributed `X-Demo-Access-Code`, then Bearer JWT |
| Organization | `X-Organization-Id` required for every tenant route; use membership returned from `GET /v1/me` |
| User role | Preserve `OWNER`, `ANALYST`, `REVIEWER`, `VIEWER`. Owner/analyst can assign and request analysis; mobile decisions are offered to owner/reviewer; viewer is read only. Server authorization remains authoritative. |
| Amounts | `{"minor_units":"250000","currency":"ETB"}` means 2,500.00 ETB |
| Case versions | `Case.version` → `If-Match: "3"` header on assign/decision; on 412 refresh case |
| Duplicate POST protection | `Idempotency-Key` unique per user action and reused on network retries |
| Asynchronous jobs | POST returns 202 resource → poll GET every ~1–2s until terminal result |
| Lists | Keyset pagination: `limit`, opaque `cursor`, response `items` + `next_cursor` |
| Provider freshness | Show `last_observed_at` and `coverage.source_status`; don't display "live bank" without authorization |
| Errors | HTTP error + `{ "error": {"code", "message", "request_id", "details"} }` |

## Web application screen ↔ operationId mapping

| Page | Calls | MVP |
|---|---|---|
| Login | `createDemoSession`, `getMe` | Yes |
| Overview dashboard | `getDashboardSummary`, `getDashboardSeries`, `listProviders` | Yes |
| Payment activity | `listTransactions`, `getTransaction`, `listOrders`, `listSettlements` | Yes |
| Recon runs | `createReconciliationRun`, `listReconciliationRuns`, `getReconciliationRun`, `listRunMatches`, `getMatchProof` | Yes |
| Cases inbox | `listCases`, `getCase`, `listCaseEvidence`, `listCaseEvents` | Yes |
| AI workbench | `createInvestigation`, `listInvestigations`, `getInvestigation` | Yes |
| Case assignment | `assignCase`, `recordCaseDecision` | Yes |
| BankSim control room | `listDemoScenarios`, `createScenarioRun`, `getScenarioRun`, `advanceScenarioRun` | Yes |
| Audit log | `listAuditEvents` | Yes |
| Upload fallback | `listImports`, `createImport`, `getImport`, `completeImport` | Only if time |

## Flutter screen ↔ operationId mapping

| Page | Calls | MVP |
|---|---|---|
| Sign-in and org | `createDemoSession`, `getMe` | Yes |
| My cases | `listCases?assigned_to=<me.id>` | Yes |
| Case detail | `getCase`, `listCaseEvidence`, `listCaseEvents` | Yes |
| AI review | `listInvestigations`, `getInvestigation` | Yes |
| Decision | `recordCaseDecision` with `If-Match` and idempotency key | Yes |
| Summary | `getDashboardSummary` | Optional |

## End-to-end demo request flow

```text
1. POST /v1/demo/sessions                         [X-Demo-Access-Code]
2. GET  /v1/me                                    [JWT]
3. GET  /v1/demo/scenarios                       [JWT + org, owner]
4. POST /v1/demo/scenarios/delayed_settlement/runs [JWT + org + idem, {seed:102}]
5. POST /v1/demo/scenario-runs/{id}/advance       [JWT + org + idem, {expected_step:0}]
6. GET  /v1/transactions                          [verify accepted payment]
7. POST /v1/demo/scenario-runs/{id}/advance       [expected_step:1]
8. GET  /v1/cases                                 [expect SETTLEMENT_NOT_OBSERVED after grace]
9. POST /v1/cases/{caseId}/investigations         [idem, {mode:"EVIDENCE_ANALYSIS"}]
10.GET  /v1/cases/{caseId}/investigations         [poll AI result]
11.POST /v1/demo/scenario-runs/{id}/advance       [expected_step:2]
12.GET  /v1/cases/{caseId}                        [EVIDENCE_UPDATED]
13.POST /v1/cases/{caseId}/decisions              [If-Match + idem, RESOLVE]
14.GET  /v1/cases/{caseId}/events                 [history retained]
```

**Note:** scenario advance and webhook delivery are asynchronous. Step 8 must poll the case list until the new reconciliation worker finishes; the UI must not assume that `advance` immediately means a case is present.

## Representative login

```http
POST /v1/demo/sessions HTTP/1.1
X-Demo-Access-Code: <distributed-demo-code>
Content-Type: application/json

{"display_name":"Demo Owner"}
```

Response: see `fixtures/demo-session-response.json`.

## Representative dashboard response

See `fixtures/dashboard-summary.json` for an exact object matching schema `DashboardSummary`. It keeps `orders_expected`, `payments_accepted`, and `settlements_observed` **separate**. Do not add them to calculate "money in bank".

## Review request from Flutter

```http
POST /v1/cases/44444444-4444-4444-8444-444444444444/decisions
Authorization: Bearer <JWT>
X-Organization-Id: 11111111-1111-4111-8111-111111111111
Idempotency-Key: mobile-resolve-case-4444-v3
If-Match: "3"
Content-Type: application/json

{"decision":"RESOLVE","reason":"The later synthetic bank settlement has now been verified in the source observations."}
```

Possible responses: `200 Case`, `401` missing auth, `403` role denied, `404` inaccessible case, `409` idempotency hash mismatch, `412` stale version, `428` missing `If-Match`.

## TypeScript client implementation pattern

```ts
export type Money = { minor_units: string; currency: 'ETB' };
export type CaseStatus = 'OPEN' | 'EVIDENCE_UPDATED' | 'RESOLVED';

export async function apiGet<T>(path: string, token: string, orgId: string): Promise<T> {
  const r = await fetch(`${process.env.NEXT_PUBLIC_API_BASE_URL}${path}`, {
    headers: { Authorization: `Bearer ${token}`, 'X-Organization-Id': orgId },
    cache: 'no-store',
  });
  const payload = await r.json();
  if (!r.ok) throw new Error(payload.error?.message ?? `HTTP ${r.status}`);
  return payload as T;
}

// For displaying money only: BigInt never converts via float.
export function formatEtb(v: Money): string {
  const n = BigInt(v.minor_units);
  const sign = n < 0n ? '-' : '';
  const a = n < 0n ? -n : n;
  return `${sign}${(a / 100n).toLocaleString('en-US')}.${(a % 100n).toString().padStart(2, '0')} ETB`;
}
```

These are **usage sketches**, not generated clients. For Next.js, prefer a secure backend-for-frontend session or an in-memory demo token; do not embed real signing keys or demo access code in public environment variables.

## Flutter client implementation pattern

```dart
class Money {
  final String minorUnits;
  final String currency;
  Money.fromJson(Map<String, dynamic> j)
      : minorUnits = j['minor_units'] as String,
        currency = j['currency'] as String;
}

String formatEtb(Money m) {
  final n = BigInt.parse(m.minorUnits);
  final sign = n.isNegative ? '-' : '';
  final a = n.abs();
  final whole = (a ~/ BigInt.from(100)).toString();
  final frac = (a % BigInt.from(100)).toString().padLeft(2, '0');
  return '$sign$whole.$frac ETB';
}
```

Use secure storage for app access token, do not log it. For Flutter case decisions, send `If-Match: '"${case.version}"'`, reuse idempotency key on network retries and refresh after 412.

## Contract change management

**No silent field changes.** Any change affecting `operationId`, URL, role, required header, JSON property, enum, status code, or money format is a contract change. Modify the YAML first, update example JSON and tests, send a concise migration note to both client engineers. If field addition is backward compatible, still announce its meaning.

## Implemented Flutter workspace

See [`mobile_app/README.md`](mobile_app/README.md). Default `APP_ENV=fixture` is a labeled sample workspace; `APP_ENV=demo` connects to the configurable product API with secure native session storage. Case filters and investigation pagination now follow the documented contract. Android native sample and live delayed-settlement review passed on 25 October 2026; iOS build/device verification is blocked by the local Xcode platform/runtime mismatch. Next.js joint smoke remains outstanding.


## Integrated local workflow

The integration checkout now includes both clients and the backend. Start with `make bootstrap && make local-up`; see [the local runbook](docs/local-testing.md). `GET /v1/members` returns the tenant's real assignment directory for OWNER/ANALYST. Seeded reviewer, analyst and viewer sessions use distinct server-configured access codes; display-name input never changes a role. OWNER/REVIEWER can decide; OWNER/ANALYST can assign or request analysis. Investigations now return QUEUED asynchronously and must be polled to a terminal status. Web cookies expire with the token. Both clients preserve an uncertain decision's exact body, key and version before allowing a replacement.

Browser tests now verify actual BFF/API assignment, investigations, stale evidence and lost-reply retry. Native and joint test results for this integration are reported in the progress log; older standalone mobile evidence above is historical and does not prove this combined revision. Human client-owner contract acknowledgement remains separate from automated tests.

The current integrated revision has now passed two consecutive fresh browser-owner/Android-reviewer rehearsals, including real form authentication, late-settlement 412, process termination and secure retry restoration, one decision/audit and unchanged source facts. The automated joint gate is TESTED; see [the verification record](docs/integration-verification.json) and PR #1 checks. This supersedes the earlier standalone-mobile joint-smoke limitation above. Written client-owner acknowledgement and cloud/iOS acceptance remain separate.
