# Active risk log

The authoritative starting risk list is in `PROJECT_MANAGER_DELIVERY_PLAYBOOK.md`, section 8. Update this file with **observed** changes only. Do not premark mitigations complete.

| Risk ID | Current assessment | Owner | Evidence / next mitigation | State |
|---|---|---|---|---|
| R-01 Solo backend scope overload | HIGH | Project lead | Reassess at each gate; protect first E2E slice | OPEN |
| R-02 Bedrock account/model unavailable | MEDIUM | Backend | Verify access; preserve labeled local stub | OPEN |
| R-04 Ledger concurrency correctness | MEDIUM residual | Backend | Isolated PostgreSQL test passed: 100 same-key retries and 100 distinct attempts; repeat after ledger lifecycle changes | MITIGATED_LOCAL |
| R-06 OpenAPI breaking changes | HIGH | Project lead | Contract freeze + client acknowledgment | OPEN |
| R-09 Live demo network instability | MEDIUM | Project lead | Local compose recovery rehearsal | OPEN |
| R-10 Local trust authentication copied to deployment | HIGH | Backend | Compose DB ports are unpublished and DB networks private; replace trust with secret-managed authentication before any deployment | OPEN |
| R-11 Provider delivery failure leaves source incomplete | MEDIUM residual | Backend | Local fault injection passed: BankSim redelivery, deduplicated inbox, coverage/cursor repair and reconciliation after restart; retest under cloud queue configuration | MITIGATED_LOCAL |
| R-12 Unfinished public import routes | MEDIUM | Backend | Three optional P2 import routes are not registered; do not promise CSV/S3 fallback to clients yet | OPEN |
| R-13 Mobile case queue filters ignored | LOW residual | Backend / SG-015 | Fixed assignee/status/scenario filtering before pagination. `TestMobileCaseFiltersAndInvestigationPagination` passed on isolated PostgreSQL; real Flutter API test verified assigned queue | MITIGATED_LOCAL |
| R-14 Distinct mobile reviewer identity unavailable through demo login | MEDIUM | Backend + Mobile | `session()` always mints `demo-owner`; owner demo is possible, but separate reviewer/viewer tests need server-owned provisioning. Never distribute signing secrets or assume entering another name changes role | OPEN |
| R-15 Investigation history pagination ignored | LOW residual | Backend / SG-017 | Added keyset limit/cursor pagination; isolated PostgreSQL regression passed. Flutter consumes all pages and selects newest by timestamp/id | MITIGATED_LOCAL |

Create a new row immediately for any newly discovered blocker, severity, owner, and reproducible evidence.

## Mobile implementation observations — 25 October 2026

| Risk ID | Current assessment | Owner | Evidence / next mitigation | State |
|---|---|---|---|---|
| R-16 iOS platform/runtime mismatch | HIGH for iOS acceptance | Mobile / workstation | `flutter build ios --simulator --no-codesign` failed: Xcode requests iOS 26.5; installed runtime is 26.4. Align the runtime/platform and rerun build/device flow | OPEN |
| R-17 Unconfirmed decision retry across app termination | MEDIUM residual | Mobile | Body/key/version survive in-session navigation and retry; lost-reply test passed. Process termination clears the pending envelope; secure short-lived persistence remains future work | OPEN |
| R-18 Mobile/web final acceptance | MEDIUM | Client DRIs | Flutter tests and real API flow pass. Next.js application is outside this repository; no actual web/mobile joint smoke has run | OPEN |
| R-19 Mobile golden images vary across host platforms | LOW residual | Mobile / CI | Exact comparisons now use reviewed macOS and Linux references; original macOS bytes preserved. Ubuntu 24.04 and Flutter 3.41.7 pinned. GitHub run 40181307106 passed all 26 tests and Android build with zero annotations; SDK/runner changes require visual review | MITIGATED_CI |
