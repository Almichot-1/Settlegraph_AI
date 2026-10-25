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
| R-12 Unfinished public import routes | MEDIUM | Backend | Optional P2 import creation/completion and upload/parse flow are absent; list/detail reads exist. Do not promise CSV/S3 fallback to clients yet | OPEN |
| R-13 Mobile case queue filters ignored | LOW residual | Backend / SG-015 | Fixed assignee/status/scenario filtering before pagination. `TestMobileCaseFiltersAndInvestigationPagination` passed on isolated PostgreSQL; real Flutter API test verified assigned queue | MITIGATED_LOCAL |
| R-14 Distinct mobile reviewer identity unavailable through demo login | MEDIUM | Backend + Mobile | `session()` always mints `demo-owner`; owner demo is possible, but separate reviewer/viewer tests need server-owned provisioning. Never distribute signing secrets or assume entering another name changes role | OPEN |
| R-15 Investigation history pagination ignored | LOW residual | Backend / SG-017 | Added keyset limit/cursor pagination; isolated PostgreSQL regression passed. Flutter consumes all pages and selects newest by timestamp/id | MITIGATED_LOCAL |

Create a new row immediately for any newly discovered blocker, severity, owner, and reproducible evidence.

## Mobile implementation observations — 25 October 2026

| Risk ID | Current assessment | Owner | Evidence / next mitigation | State |
|---|---|---|---|---|
| R-16 iOS platform/runtime mismatch | HIGH for iOS acceptance | Mobile / workstation | `flutter build ios --simulator --no-codesign` failed: Xcode requests iOS 26.5; installed runtime is 26.4. Align the runtime/platform and rerun build/device flow | OPEN |
| R-17 Unconfirmed decision retry across app termination | MEDIUM residual | Mobile | Body/key/version survive in-session navigation and retry; lost-reply test passed. Process termination clears the pending envelope; secure short-lived persistence remains future work | OPEN |
| R-18 Mobile/web final acceptance | MEDIUM | Client DRIs | Local integration commit cfd91bb now contains both clients. Web proxy login/read/scenario calls passed; an actual browser-to-native-mobile joint workflow remains unverified | OPEN |
| R-19 Mobile golden images vary across host platforms | LOW residual | Mobile / CI | Exact comparisons now use reviewed macOS and Linux references; original macOS bytes preserved. Ubuntu 24.04 and Flutter 3.41.7 pinned. GitHub run 40181307106 passed all 26 tests and Android build with zero annotations; SDK/runner changes require visual review | MITIGATED_CI |

## Integration planning review — 25 October 2026

These are source-review findings, not newly executed failure tests. Proposed repairs and acceptance gates are in [the integration plan](full-stack-integration-plan.md); implementation awaits user approval.

| Risk ID | Current assessment | Owner | Evidence / next mitigation | State |
|---|---|---|---|---|
| R-20 Web assignee IDs do not exist in demo seed | HIGH for client handoff | Backend + Web | Case page hardcodes four people; seed contains only owner 2222… . Add real tenant member directory and separate role identities | OPEN |
| R-21 List filters/pagination incomplete | HIGH for read correctness | Backend + Clients | Transaction SQL ignores provider/scenario filters; case events and matches ignore pagination; audit stops at 100. Implement declared parameters and test multiple pages | OPEN |
| R-22 Web review snapshot/retry consistency | HIGH for review correctness | Web | Concurrent case/evidence reads lack a version bracket; decision key can be reused after input/version changes. Add coherent snapshot loading and immutable retry envelopes | OPEN |
| R-23 Investigation worker lifecycle absent | MEDIUM for offline demo; HIGH for Bedrock integration | Backend | API generates COMPLETED offline result synchronously; worker selects only RECONCILE jobs. Add durable INVESTIGATE jobs and independently validated model adapter | OPEN |
| R-24 Cross-stack automated coverage absent | MEDIUM | All components | CI covers mobile; BFF security-named test covers client error mapping only. Add actual route/browser and coordinated native integration checks | OPEN |
| R-25 Decision roles disagree | HIGH for permissions | Backend + Clients | Go allows ANALYST decisions; Flutter/handoff use OWNER/REVIEWER. Enforce the documented common policy and negative tests | OPEN |
| R-26 Local mobile SDK incompatible | HIGH for local native acceptance | Mobile / workstation | Linux host Flutter 3.27.4/Dart 3.6.2 is below the mobile SDK requirement; use an isolated pinned toolchain and verify Android runtime | OPEN |

## Integration implementation checkpoint — 25 October 2026

This entry supersedes the planning assessments above for the tested local scope. User approval is recorded in the integration plan.

| Risk | Observed mitigation | Current state |
|---|---|---|
| R-14 / R-20 | Distinct seeded role identities and actual member directory; PostgreSQL role/assignment tests and live browser assignment pass | MITIGATED_LOCAL; native handoff pending |
| R-21 | Provider/scenario filters before pagination and scoped multi-page cursors; 110-row integration history checks pass | MITIGATED_LOCAL |
| R-22 | Version-bracketed web case reads and immutable session-storage retry; actual lost successful reply + reload + one decision pass | MITIGATED_LOCAL |
| R-23 | Durable analysis jobs, immutable snapshot/provenance, bounded retries and separate reconciliation loop; real-DB failure/replay/lease tests pass | MITIGATED_LOCAL; actual Bedrock account invocation remains R-02 |
| R-25 | OWNER/REVIEWER decision policy enforced; ANALYST/VIEWER negative tests pass | MITIGATED_LOCAL |
| R-17 / R-18 / R-24 / R-26 | Secure bounded mobile retry storage, coordinated browser/native tests, isolated SDK setup and full CI are implemented; native/CI execution pending | IN_PROGRESS |

AWS/real-bank credentials and deployment remain absent by design for this local task. Linux does not establish native iOS acceptance. SG-002 written client-owner acknowledgement remains a human acceptance item; automated contract tests do not impersonate a DRI.
