# SettleGraph AI — implementation progress

**As of:** 2026-10-25 (Africa/Addis_Ababa)
**Scope:** Local synthetic backend plus Flutter mobile client; separate product and BankSim databases
**Revision:** See Git history for the reviewed backend commit
**Remote:** `https://github.com/Almichot-1/Settlegraph_AI.git` (empty at review time)

## What runs

Three Go services start under Docker Compose: public API, PostgreSQL-backed worker, and BankSim. Both databases migrate and seed independently. The demo API can create and advance normal, delayed settlement, duplicate webhook, and provider outage scenarios. BankSim uses a balanced journal and signed provider events. SettleGraph stores deduplicated inbox events and observations, runs two-stage reconciliation with source watermarks, persists immutable proofs, exposes dashboard/account/transaction/case reads, and records versioned human decisions. Account views contain known synthetic metadata without unobserved balances. AI investigations currently use the labeled offline evidence-linked stub.

## Verification executed

| Area | Evidence | Result |
|---|---|---|
| Contracts | `python validation/validate_pack.py` | Passed: 39 public and 10 internal operations, fixtures, schemas, backlog graph |
| Go | `go test ./...`, `go vet ./...`, `go build ./cmd/...` | Passed |
| Database | Fresh PostgreSQL databases: migrations twice, seeds twice | Passed; opening journal sums to zero |
| BankSim concurrency/lifecycle | `TestConcurrentLedgerEffects` with isolated PostgreSQL, 100 same-key retries and 100 distinct attempts | Passed; no overspend, balanced journals, valid CREATED → ACCEPTED → SETTLEMENT_PENDING → SETTLED and CREATED → FAILED history, invalid transition rejected |
| Reconciliation scopes | `TestConcurrentRunDeduplication` and `TestLatestObservationsUsesUnscopedEvidence` with isolated PostgreSQL | Passed; one immutable run with two unique proof edges under concurrency; latest scope matches unscoped order/payment/settlement data |
| Demo identity and tenancy | `TestDemoIdentityRoleAndTenantBoundary` with isolated PostgreSQL | Passed; 401 invalid token, 403 viewer mutation, 404 foreign organization, 400 malformed JSON |
| Local runtime | `make up`, `make smoke`, `make test-e2e` | Passed for all four synthetic scenarios, late evidence, human review, duplicate delivery, outage recovery, invalid signatures |
| Delivery recovery | `python tests/e2e_delivery_recovery.py` | Passed twice; pending event redelivered after BankSim restart, product inbox deduplicated, source cursor/coverage and reconciliation recovered |
| Live JSON contracts | `.venv/bin/python tests/contract_runtime.py` | Passed for demo login, 13 list/read surfaces, selected detail/evidence/match/proof routes against public OpenAPI schemas |

The detailed ticket states are in `AGENT_BACKLOG.json`. `TESTED` means its stated backend acceptance has direct evidence; it does not mean client or cloud acceptance.

## Open work and blockers

- SG-002 and SG-020 need the Next.js and Flutter clients to render the frozen fixtures and acknowledge the endpoint table; those client projects are not present in this repository.
- Production identity and distributed rate limits remain cloud hardening work; production API startup fails closed.
- Source coverage deliberately refuses to claim completeness when a provider event is absent. Local automatic delivery retry and coverage repair passed; cloud queue integration and broader fault testing remain with SG-021.
- SG-014 account metadata is seeded and listed without private balances; no provider account-balance observations are available yet.
- SG-017 Bedrock adapter is absent; the offline investigator is operational and evidence bounded.
- Optional import creation/upload/parse routes are absent; list/detail reads are backed by the product database.
- SG-021 cloud deploy is not configured: no AWS credentials, IAM, TLS, budget alarm or cloud smoke. SG-022 final acceptance and two fresh-seed rehearsals remain.

No real bank integration or live funds are involved. Local Compose PostgreSQL trust authentication is limited to unpublished database networks and must be replaced before cloud deployment.

## Recovery and next checks

`make down` stops services without deleting volumes. `make up && make migrate && make seed && make smoke` restarts the local demo. `make test-e2e` verifies scenario behavior after restart. Do not run `docker compose down -v` when retaining evidence is required.

## Mobile planning — 25 October 2026 (Africa/Addis_Ababa)

User requested a Flutter design/implementation plan and a separate branch. Created `feature/settlegraph-mobile`, `mobile_app/README.md`, and [the mobile plan](mobile-implementation-plan.md). Mobile application implementation and builds are NOT_RUN; no backend gate or SG ticket was newly accepted.

Actual checks: Flutter 3.41.7 / Dart 3.11.5 detected; `flutter doctor -v` reports unaccepted Android licenses and no attached Android/iOS target. Created the ignored `.venv` and installed `validation/requirements.txt`; `.venv/bin/python validation/validate_pack.py` PASS (39 public / 10 internal operations, schemas, fixtures, backlog graph). Go tests/vet, migrations, live API tests, and mobile tests were NOT_RUN in this planning pass.

Static implementation review found that `cases()` in `internal/httpapi/reporting.go` does not apply documented `assigned_to`, `status`, or `scenario_run_id` filters. `listInvestigations()` also ignores its pagination parameters. These need backend regression tests/fixes before related live mobile acceptance. Demo login mints the shared `demo-owner` identity; separate reviewer/viewer acceptance needs backend-owned identity provisioning. See R-13 through R-15 in the risk log.

Next mobile milestone: M01 runnable Flutter foundation and fixture model tests, followed by M02 reference case-detail design. Existing SG-002/SG-020 cross-client acceptance remains outstanding.

## Mobile implementation — 25 October 2026

Implemented the Flutter Android/iOS project in `mobile_app/` on `feature/settlegraph-mobile`, following the feature data/repository/presentation structure. The client includes auth/session restoration, organization membership, cases and filters, evidence/history, investigation citations and polling, versioned decisions, overview, account settings, dark/light themes, bundled Inter, consistent Lucide icons, and original vector launcher branding. Default fixture mode is labeled; live mode uses the actual product API and native secure session storage. No cloud or store deployment occurred.

The design uses warm neutral surfaces and forest-green actions, with an evidence trail that distinguishes observed records from deterministic match proof. Screens include loading, empty, error, source-unavailable, expired-session, and 412 conflict states. Decisions preserve body/key/version after an uncertain submission and require explicit re-review after conflicting evidence.

Backend prerequisites corrected without contract-shape changes: case assignee/status/scenario filters are applied before pagination, and investigation list now honors keyset limit/cursor. Public runtime test scripts accept an optional `SETTLEGRAPH_API_URL`; defaults remain unchanged. API change log records these as nonbreaking runtime corrections.

### Verification executed in this implementation pass

| Area | Actual command/evidence | Result |
|---|---|---|
| Flutter static checks | `dart format ...`; `flutter analyze` | PASS, no analysis issues |
| Flutter suite | `flutter test --coverage` | PASS: 26 tests; 1 opt-in live test skipped in the default suite |
| Design baselines | `flutter test test/app/design_golden_test.dart --update-goldens`; subsequent normal suite | PASS: login, queue, light and dark case detail; visually inspected; semantic contrast tests pass for both themes |
| Host live Flutter API | `flutter test test/live/product_flow_test.dart --dart-define=RUN_LIVE=true` with runtime environment code and API URL | PASS: delayed case, assignment, evidence citations, late settlement, 412, human resolution, preserved history |
| Native Android | `python3 mobile_app/scripts/run_live_android.py --device emulator-5556 --api-url http://127.0.0.1:9080 --device-api-url http://10.0.2.2:9080` | PASS: fixture UI flow and live native review; Android secure session storage restored; stale decision handled; final RESOLVED case verified independently through API/history |
| Android build | `flutter build apk --debug` | PASS; debug APK produced |
| Native iOS | `flutter build ios --simulator --no-codesign` | BLOCKED: Xcode requests iOS 26.5, installed simulator runtime is 26.4 |
| Go regression | `go test ./...` and `go vet ./...` via `golang:1.27.0-alpine` container | PASS; host Go is absent |
| PostgreSQL API integration | `go test -tags integration ./internal/httpapi -v`, isolated `mobile_filters_test2` database | PASS: existing identity/role/tenant checks plus new filters, pagination, cursor and tenant negative tests |
| Migrations and seeds | Both Compose databases migrated and seeded from empty state | PASS |
| Public runtime schema | `SETTLEGRAPH_API_URL=http://127.0.0.1:9080 .venv/bin/python tests/contract_runtime.py` | PASS |
| Scenario suite | Same API override with `tests/e2e_demo.py`; `tests/e2e_delivery_recovery.py` | PASS: four scenarios, replay, prior-run preservation, review, deduplication, delivery/coverage recovery |
| Frozen pack | `.venv/bin/python validation/validate_pack.py`; `python3 mobile_app/scripts/sync_fixtures.py --check` | PASS |

Port 8080 was occupied by an unrelated local service. Verification used an ephemeral Compose port override for API 9080; private internal service traffic remains unchanged. The existing Pixel_8 emulator had insufficient storage, so a new `SettleGraph_Review` AVD was created without deleting its data. The new emulator uses port 5556. Native verification trace is in ignored `mobile_app/verification/native-live-trace.json`; it contains resource IDs/status/version only, no credentials.

Root CI workflow subsequently passed on GitHub; see the CI verification entry below. iOS device behavior, real multi-role native sessions, TalkBack/VoiceOver manual review, profile-mode performance measurement, process-termination persistence for uncertain decisions, and Next.js/Flutter joint smoke remain outstanding. Current tab pages reload on navigation; preserving tab stacks is a later usability refinement. Local debug release is runnable; production/mobile-store release is not claimed.

Next gate: align iOS platform/runtime, run native iOS flow, and perform actual Next.js/Flutter SG-020 acceptance with the web developer. SG statuses were not promoted to TESTED or ACCEPTED in this mobile pass; direct evidence and current blockers were appended to the existing backlog.

### Android emulator recovery

After the visible host-GPU emulator shut down with graphics context/window surface errors, Flutter reported `error: closed` and API null. Restarted ADB, cold-booted `SettleGraph_Review` with `-gpu swiftshader -feature -Vulkan -no-snapshot-load`, and verified API 35 / arm64-v8a properties. Added `mobile_app/scripts/start_android_emulator.py` to open a visible emulator and wait for valid Android properties before Flutter runs. The readiness helper executed successfully without wiping emulator data.

### Preferred Pixel emulator

User requested their existing Pixel_8. Started it visibly on emulator-5554 with software graphics; API 35/arm64-v8a readiness passed. Changed the startup helper default to Pixel_8/5554. Built the Android-arm64 debug app with APP_ENV=demo and API_BASE_URL=http://10.0.2.2:9080. Installation on this existing AVD failed with INSTALL_FAILED_INSUFFICIENT_STORAGE. Android-managed cache trimming freed only a small amount; roughly 393 MiB remained. No wipe, app-data deletion, or userdata-image resize was performed. Further Pixel installation requires freeing storage or explicitly authorizing a reset; the backend remains healthy.

### Pixel profile build — storage workaround verified

Continued on the user's existing Pixel_8/emulator-5554. Android-arm64 `flutter build apk --profile` with APP_ENV=demo and API_BASE_URL=http://10.0.2.2:9080 produced a 32.7 MB app-profile.apk. `adb install -r` succeeded and the app was launched; Android reported SettleGraph as the top resumed activity. The visible dark-theme demo sign-in screen was inspected. Profile-only manifest now permits local development HTTP, matching debug; release transport policy remains unchanged. No userdata resize, wipe, or existing-app removal was needed. Pixel startup helper defaults remain Pixel_8/5554.

### Pre-commit review

Later Pixel profile reinstalls also hit insufficient storage. Flutter automatically removed the SettleGraph installation during one retry, and the subsequent install failed; the earlier one-time profile launch is not a reliable storage fix. The existing Pixel requires additional free space or a user-selected factory reset. No AVD wipe or existing third-party app removal was performed. An unfinished profile-minification experiment was removed before committing. The README records the actual remaining storage limitation.

### Mobile CI visual baseline repair

The first GitHub run (40180107206) passed 25 tests and failed the screenshot test: all four macOS references differed on Ubuntu at text/icon edges. Diagnostic run 40180925131 retained actual rendered images and diffs. Reviewed those images, preserved all original macOS PNGs byte for byte under `goldens/macos/`, and added Linux references under `goldens/linux/`. The test selects its host references and retains exact pixel comparisons; no thresholds, skips, or automatic reference updates were introduced.

Pinned CI to Ubuntu 24.04, upgraded checkout/setup-java/upload-artifact to Node 24 actions, and retained screenshot failure diagnostics plus coverage artifacts. Local `flutter analyze`, formatting, fixture sync, and `TZ=Africa/Addis_Ababa flutter test --coverage` passed (26 tests, one optional live test skipped).

[GitHub run 40181307106](https://github.com/Almichot-1/Settlegraph_AI/actions/runs/40181307106), testing commit `a05937c81a00cf75619479f7c6b4c600586108b8` on Ubuntu 24.04 with Flutter 3.41.7, completed successfully: locked dependencies, fixture sync, formatting, analysis, 26 tests, Android debug APK build and artifact uploads all passed. The run has zero annotations. Coverage and the debug APK are attached to the run. Backend code and contracts were unchanged in this repair; backend tests were not rerun. Existing iOS and joint web/mobile acceptance limitations remain open.

## Autonomous integration — 25 October 2026

User approved `docs/full-stack-integration-plan.md`; work runs on `integration/full-stack` from merge cfd91bb. Phase 1 added the Docker web service, integrated launcher/status/stop/log commands, configurable host ports, project-scoped Go commands, and isolated PostgreSQL test runner. `WEB_PORT=3001 make local-up` PASS, all six services healthy. Existing host web remains on port 3000 while integration is developed.

Phase 2 backend/contract changes: distinct seeded demo principals and per-role codes, tenant member directory, common decision permission, transaction filters, event/match/audit pagination, and bounded UTC dashboard series. YAML/fixture/generated clients updated together. `.venv/bin/python validation/validate_pack.py` and web contract checks PASS (40 operations). `make test-integration` PASS including new `TestSharedIdentityFiltersAndHistory` (110-row histories, filters, role identity, 412 and retry), tenant negatives, ledger concurrency, and reconciliation persistence. Each test uses a separate database with migrations and seeds applied twice. Client UI adoption and browser/native acceptance are still IN_PROGRESS; no final cross-client or AWS acceptance claimed.

### Durable analysis and browser integration checkpoint

Implemented asynchronous investigation jobs with immutable bounded evidence snapshots, content hashes and provenance; leases and bounded retries; isolated analysis/reconciliation worker loops; idempotent terminal publication; and an explicit optional Bedrock Converse adapter. Offline mode remains the default. Added migration 006 and fake-provider/real-PostgreSQL failure, rejection, replay and lease-recovery tests. Added bounded per-process authentication/mutation rate limits and documented 429 responses. Dashboard summary now reads one repeatable snapshot.

Executed on the integration working tree: `make test lint test-integration` PASS (fresh per-test databases, migrations/seeds applied twice); `make test-contract-runtime test-e2e` PASS against rebuilt API/worker (all four scenarios, historical runs, late evidence, restart deduplication and provider-delivery recovery); `make frontend-check frontend-test frontend-build` PASS (18 unit tests); `npm --prefix settlegraph-web run test:live` PASS (two real Chromium tests, actual role-controlled BFF routes, owner assignment/investigation, late-evidence 412 and lost-success-reply retry after browser reload). Static pack and generated-contract checks PASS, 40 operations/57 public schemas. One Docker registry metadata request failed transiently on IPv6; rerunning the build succeeded without changing host network settings.

Web work includes the real member directory, per-role sessions, scenario-scoped screens, pagination, version-bracketed case snapshots, immutable persisted retry envelopes, role-aware navigation, expiry-aligned cookies and bounded same-origin BFF requests. Native pending decisions now use encrypted API/environment + organization/user storage with a one-hour expiry and bounded entries. A coordinated browser/native harness, process-stop retry check, pinned project tool installer and full-stack CI workflow are implemented but their native/CI execution is still pending at this checkpoint. Android SDK licence acceptance was explicitly authorized by the user. Downloads are resumable and checksum verified; the shared host Flutter installation is unchanged.

No live AWS/Bedrock invocation, cloud deployment, native iOS execution or human client-owner contract sign-off is claimed.

### Android tooling and Flutter checks

Project-local Flutter 3.41.7 / Dart 3.11.5, Java 21, Android SDK 36/build-tools 36, NDK 28.2 and the API 35 x86_64 image are installed from checksum-verified official artifacts. Android emulator `SettleGraph_Review` is booted and reports readiness. Official SDK manager recognizes the installed packages; package metadata retains its licence definitions. The Gradle wrapper remains version 8.14, now uses the smaller binary distribution with its published SHA-256, and can be prefetched into an isolated project cache. Shared SDK installations are unchanged.

`flutter pub get --enforce-lockfile` PASS; fixture sync PASS (including the new member directory fixture); Dart formatting PASS (88 files unchanged); `flutter analyze` PASS with no issues; `flutter test --coverage` PASS: 27 tests, one explicitly opt-in live test skipped. The new repository restart test verifies the same pending decision envelope and one resolution after an uncertain successful response. Exact Linux visual baselines passed without updates. Native APK and actual browser/native process-restart acceptance remain pending; these host checks do not establish device acceptance.

Expanded live browser checks also PASS for provider/scenario filtering, explicit SCENARIO_RUN reconciliation, expired-token cookie removal and sign-in hydration. Public runtime support is listed in `docs/api-operation-support.md`: 38 implemented operations and two explicitly deferred optional import mutations out of the contract's 40 operations.
