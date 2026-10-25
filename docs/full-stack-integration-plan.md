# SettleGraph full-stack integration plan

**Date:** 25 October 2026, Africa/Addis_Ababa  
**Status:** Local integration implemented and verified, including two fresh browser/native rehearsals, on `integration/full-stack`. Current CI checks are linked from PR #1; external gates remain deferred.
**Working checkout:** `/home/ahmed/Projects/aws hackthon/SettleGraph_Local_Test`  
**Baseline:** `cfd91bb`, local integration of backend `5eeb6a5`, web `960b5bf`, and mobile `7723fc1`. Remote branch heads were rechecked during planning and still match these revisions.

## 1. Outcome and scope

Deliver a reproducible local system in which the Next.js owner workstation, Flutter reviewer app, Go API, worker, and two simulated banks complete the same auditable investigation workflow. Both clients must use the actual product API and agree on organization, user, case ID, evidence, version, and final decision.

The proposed approval covers the seven phases below: code fixes, additive migrations and contracts, isolated development tooling, automated tests, local service execution, commits, and a dedicated remote integration branch with a reviewable PR. Execute sequentially without asking for routine implementation choices. Resolve failures before claiming their gate passed; continue independent work if a device or external service blocks one check.

AWS deployment remains deferred as requested. The Bedrock adapter can be implemented and tested with a fake client, but a real invocation cannot be accepted without model access and AWS configuration. Linux cannot provide native iOS verification. Production Cognito identity, real bank integrations, optional CSV/S3 importing, and optional provider account-balance observations are separate extensions, not prerequisites for this local integration. Keep these limitations visible in the final report.

## 2. What the review actually established

The previous local setup pass ran Go tests/vet, web type checking and 18 tests, static contracts, live read-schema checks, all four backend scenarios, and delivery recovery. Web login, dashboard reads, and scenario create/advance through the Next.js proxy also passed. These are useful baselines; they do not prove a browser-to-native-mobile handoff. No runtime tests were rerun during this planning pass.

| Finding | Source evidence | Integration consequence |
|---|---|---|
| Web assignees are hardcoded and do not match the seeded owner or any seeded reviewer. | `settlegraph-web/app/(workspace)/cases/[caseId]/page.tsx`, `db/seed_settlegraph.sql` | A realistic assignment from web to a distinct mobile reviewer cannot work as presented. |
| Demo login always authenticates `demo-owner` and updates that shared user's display name. | `internal/httpapi/httpapi.go:140` | Entering different names does not create different identities; actor histories and role testing need separate demo principals. |
| The Go decision route permits ANALYST; Flutter and the handoff permit OWNER/REVIEWER. | `internal/httpapi/case_mutation.go:18`, mobile `user_model.dart`, blueprint section 8.1 | Clients and server disagree about who may make decisions. |
| Transaction `provider_id` and `scenario_run_id` filters are declared and used by clients but absent from the SQL filter. | `internal/httpapi/reads.go:145`, public OpenAPI | The web provider filter can show records outside the selected provider/scenario. |
| Case events and run matches ignore pagination; audit reads stop at 100 with no continuation. | `internal/httpapi/reporting.go:320`, `internal/httpapi/reads.go:306,406` | Larger histories cannot be consumed reliably using the frozen list contract. |
| Web case detail fetches case/evidence/history concurrently without checking the version on both sides and uses only the first pages. Its decision key survives an error but the body/version can still change. | web case-detail page | A reviewer can see a mixed snapshot or reuse a retry key with changed input. |
| Web manual reconciliation always requests `LATEST_OBSERVATIONS`, which the backend deliberately treats as unscoped observations. | web reconciliation page, `internal/httpapi/manual_reconcile.go` | A user expecting the selected BankSim scenario to be reconciled can instead get an empty unscoped run. |
| Investigation creation computes the offline result synchronously; the worker handles only RECONCILE jobs. | `internal/httpapi/investigation.go:134`, `internal/jobs/jobs.go` | The planned asynchronous investigation lifecycle and Bedrock failure/retry path are not integrated. |
| Web cookies last eight hours while demo tokens last one hour; the proxy omits ETag/Retry-After forwarding. | web session/proxy routes, `internal/httpapi/auth.go` | Session expiration and concurrency/error behavior need consistent handling. |
| The file named `bff-security.test.mjs` tests error mapping, not the BFF routes. CI currently covers mobile only. | web tests, `.github/workflows/mobile_ci_validation.yml` | Passing existing tests does not establish proxy controls or full-system compatibility. |
| This host has Flutter 3.27.4 / Dart 3.6.2; the mobile branch pins Flutter 3.41.7 and requires Dart 3.11.5. Android tools were not found on PATH. | local tool inspection, mobile pubspec and workflow | Native local acceptance needs an isolated compatible toolchain. `/dev/kvm` exists; emulator readiness still needs verification. |

## 3. Implementation phases and gates

### Phase 1 — Reproducible integrated workspace

**Work:** Create a dedicated implementation branch from the reviewed local merge; record upstream SHAs. Add a repeatable local launcher/preflight for API, worker, BankSim, both databases, and Next.js. Generate ignored credentials once and configure the web proxy for live backend mode. Use explicit ports/project names and preserve existing demo volumes. Make documented Make targets use the project Python environment. Scope Go commands to project packages so installed JavaScript dependencies are not traversed as Go source.

Provide start, status, stop, log, and demo instructions. Fresh test runs use a separately named Compose project and its own databases; the harness must refuse destructive reset of the user's normal demo project. Fix stale `starter/` paths and documentation claiming clients are absent.

**Gate:** A clean checkout starts and migrates/seeds with documented commands, health checks pass, a web session reaches the real API, and a normal restart preserves data. Missing dependencies or occupied ports produce actionable errors. Existing local setup remains recoverable.

**Main files:** `Makefile`, Compose/Docker configuration, environment examples, `validation/init_demo_env.py`, local tooling scripts, README/handoff.

### Phase 2 — Shared API behavior, identity, and assignment

**Work:** Build an operation matrix from the YAML and reconcile it with registered Go routes, TypeScript wrappers, and Dart consumers. Repair transaction filters and missing pagination before relying on them in client tests. Preserve opaque cursors bound to tenant, route, parent resource, and filters; use a total order consistent with each query.

Seed distinct OWNER, ANALYST, REVIEWER, and VIEWER demo principals. Keep the existing demo session request shape: server-configured, role-specific access codes select the permitted principal; client-supplied roles never grant permissions. Preserve existing owner-code compatibility. Let both clients authenticate the appropriate demo identity and stop treating display-name changes as identity changes.

Add a minimal tenant-scoped `GET /v1/members` directory for authorized assignment users (OWNER/ANALYST), exposing only ID, display name, and role for active members. Replace the web roster with this source. Make the default decision policy OWNER/REVIEWER, matching the existing handoff and Flutter; ANALYST may assign/investigate but cannot decide. Record this enforcement correction explicitly in the API change log.

Update the YAML first for the additive member endpoint, regenerate TypeScript types/manifests, update affected Dart models/fixtures and contract checks. Remove brittle assumptions that the operation count must always remain 39; validate the actual versioned contract and expected supported operations instead.

**Gate:** Assignment from web returns an actual seeded reviewer ID; that user's Flutter session has the same ID. OWNER/ANALYST can assign, OWNER/REVIEWER can decide, and VIEWER mutations fail server-side. Foreign tenants/members/cursors fail. Filtering happens before pagination, and multi-page histories contain no gaps/duplicates on a fixed dataset.

**Main files:** `contracts/`, `db/`, `internal/httpapi/`, `internal/config/`, web API/session/case pages, mobile auth/case layers, generators and fixtures.

### Phase 3 — Reliable web and mobile workflows

**Work:** Carry scenario context through the web simulation, payments, cases, and reconciliation screens. Resume a scenario by its ID after navigation; use `SCENARIO_RUN` with that ID when the user selects a demo run. Keep the separate unscoped reconciliation option clear. Check dashboard chart time ranges against the simulator's clock and source `as_of` so simulated activity is not hidden by an unrelated wall-clock range.

Use server filters and cursor controls for list screens. Fetch all required evidence/history pages or expose explicit continuation. Check the case version before and after reading its evidence, as the mobile repository already attempts, and refuse to present a changing snapshot as ready for review. Cancel obsolete requests after navigation, sign-out, or organization changes.

Give each logical mutation a stable retry envelope: organization, user, resource, action, exact body, expected version, and idempotency key. Preserve it after uncertain responses and retry it unchanged before allowing a new submission. Add secure, bounded mobile persistence for pending decisions across process restarts; choose browser storage appropriate to the synthetic environment and exclude credentials. A stale version requires refreshed evidence and explicit acknowledgement. Clean up or isolate pending data on sign-out/organization changes; never submit it automatically as an offline decision.

Align web cookie lifetime with token expiry, clear expired sessions, and implement role-aware UI from the authenticated membership. Tighten the BFF to declared method/path operations, size/time limits, and same-origin mutations; retain its private-endpoint exclusions. Forward ETag, Retry-After, and request IDs safely. Add rate limits for demo login and expensive operations, with testable configuration.

**Gate:** Browser tests drive actual pages against Go for filters, assignment, scope selection, pagination, expiration, and 412 conflicts. A lost response followed by a same-envelope retry creates exactly one action. Changing the body under the same key yields 409. An interrupted mobile decision survives restart and cannot leak across users or organizations. BFF route tests cover the real routes, including denied origins/paths and upstream failures.

**Main files:** web session/proxy/client and workspace pages, mobile session/storage/repositories/cubits, Go middleware, browser and mobile regression tests.

### Phase 4 — Durable investigations and Bedrock adapter

**Work:** Change investigation creation to atomically persist a QUEUED investigation, an immutable bounded evidence snapshot, and a durable job. Return the existing 202 resource shape promptly. Extend the worker for INVESTIGATE jobs with leases, bounded retries/backoff, and terminal failure states. Perform model calls outside database transactions and make completion idempotent after crashes/retries.

Create a small investigator interface with the existing clearly labeled offline implementation and a configurable Bedrock adapter. Hash the canonical snapshot content and evidence IDs, retain provenance/model metadata and validation outcomes, and preserve historical results when new evidence arrives. Strengthen validation beyond membership checks: schema/length/count limits, required citations, permitted references, and deterministic presentation of financial status. AI output cannot create matches, settle money, or make a human decision.

Default to offline mode. In explicit Bedrock mode, failures produce visible bounded failure status; do not silently label a fallback response as Bedrock. Tests use a fake Bedrock client for malformed output, fabricated/foreign IDs, throttling, timeout, and retry exhaustion. Real Bedrock availability remains a separately recorded external gate.

**Gate:** Both clients display QUEUED/RUNNING and terminal states correctly. Killing/restarting the worker does not duplicate investigations or timeline entries. Invalid output is rejected without changing financial facts or resolving cases. A later evidence snapshot produces a new investigation while the old one remains traceable. A failed investigation does not stop reconciliation.

**Main files:** `internal/investigation/`, `internal/jobs/`, investigation HTTP handlers, worker configuration, migrations, client polling/state handling.

### Phase 5 — Native Android readiness

**Work:** Install or use a project-selected Flutter 3.41.7 SDK and compatible Java/Android tooling without changing the shared Flutter 3.27.4 installation. Pin versions and document reproducible commands. Adapt emulator helpers to Linux and macOS SDK discovery and parameterize device IDs/base URLs. Verify available memory/KVM access before starting one emulator.

Run dependency locking, fixture sync, formatting, analysis, unit/widget/golden tests, and APK build. Run native secure-storage/session restoration and live API review against this merged backend. Use per-role sessions, not a shared owner token, for the reviewer path. Keep test secrets out of artifacts and compiled assets.

**Gate:** Android APK builds and a native emulator/device completes sign-in, assigned case review, evidence/AI display, stale-version handling, and a persisted human decision. Host-based Dart tests and native tests are reported separately. If emulator tooling/license/device access genuinely blocks native execution, preserve that explicit blocker and continue other phases; never label API-only tests as native acceptance.

**Main files:** mobile tooling scripts, auth/session/review tests, integration harness, SDK configuration and runbook.

### Phase 6 — One workflow across both clients

**Work:** Add a coordinated harness using a real browser for web and Flutter integration tests for Android, both connected to the same isolated Go/BankSim deployment. Preserve a credential-free trace with the shared scenario/case ID, distinct actor IDs, evidence snapshot, versions, timestamps, response codes, and final history.

The required walkthrough is:

1. Web OWNER starts a delayed-settlement scenario and observes accepted payment without presenting it as settled.
2. Advance virtual time past the grace period; wait for source coverage and reconciliation to produce the case.
3. Web assigns the case to the actual REVIEWER and requests an asynchronous investigation.
4. Android REVIEWER opens that exact assigned case and inspects its evidence and cited investigation.
5. Web releases the late settlement while mobile holds the earlier case version.
6. Mobile's stale decision receives 412; refresh and explicit acknowledgement permit a new decision.
7. Simulate a lost successful reply and retry the same decision envelope; one decision and one corresponding audit effect remain.
8. Web refresh shows the same final status/version/actor; previous runs, proofs, and investigations remain available. Human review has not changed financial source records.

Also retain normal flow, duplicate notification, and outage/recovery checks, plus reviewer/viewer permission and cross-tenant negative checks. Exercise histories beyond one page. Use bounded polling instead of assuming webhook delivery is immediate.

**Gate:** Two consecutive fresh-database rehearsals pass with the same logical outcome. Separate mock, live API, browser, and native evidence is recorded. A failure in one layer is attributed to the exact step/resource rather than hidden by mocked data.

### Phase 7 — CI, acceptance evidence, and delivery

**Work:** Add Go build/vet/unit and isolated PostgreSQL integration jobs, contract drift/fixture checks, web type/test/build and browser integration jobs, and mobile locked analysis/test/APK jobs. Run the coordinated client workflow in a suitable emulator runner; if native CI cannot run, require the explicit native local artifact before accepting that gate. Ensure backend changes trigger affected client integration checks. Limit test execution to project packages.

Update `AGENT_BACKLOG.json`, handoff, progress, risk log, operation support matrix, and a local testing runbook with actual commands and results. Reconcile SG-002 and SG-014 through SG-020 against evidence; distinguish automated contract verification from human client-owner acknowledgement. Report SG-022 local rehearsal separately from the still-deferred SG-021 cloud gate. Do not mark cloud or iOS acceptance complete from local Linux evidence.

Commit coherent checkpoints, fetch/check remote drift, and reconcile compatible teammate changes before final verification. Push a dedicated integration branch and open a PR containing the change summary, test results, artifact pointers, and remaining external gates. The plan does not include merging into remote master or cloud deployment.

**Gate:** Required checks are green for the delivered commit; the user can start the integrated demo from documented commands; the PR and final report clearly identify completed work versus blocked external verification.

## 4. Verification commands and test isolation

Existing commands to retain or make reliable from a clean checkout:

- Go unit/vet/build checks scoped to `./cmd/...` and `./internal/...`.
- PostgreSQL integration tests with dedicated test databases and explicit DSNs; a skipped integration suite is not a pass.
- `.venv/bin/python validation/validate_pack.py` and `make test-contract-runtime`.
- `make test-e2e` for four scenarios and delivery recovery.
- Web `npm ci`, `npm run check:contracts`, `npm run typecheck`, `npm test`, and `npm run build`.
- Flutter locked dependencies, `scripts/sync_fixtures.py --check`, format/analyze/test, APK build, and `integration_test/app_test.dart` on Android.

New targets proposed for implementation: `make local-up`, `make local-status`, `make local-down`, `make test-web-live`, and `make test-cross-client`. Their documented behavior must match the implementation. The last two require actual services; they must fail clearly instead of silently using fixtures.

Start regressions with the smallest failing behavior, then run the affected layer and required integration gate. Repeat ledger concurrency checks when ingestion, migrations, or financial behavior changes. Do not rerun unrelated suites indefinitely after a passing checkpoint.

## 5. Decisions included in approval

- Prioritize a verified local web-to-mobile workflow; preserve the current ledger and matching rules except for defects demonstrated by tests.
- Add the minimal member-directory API and server-owned demo identities; synchronize contracts and both clients together.
- Use OWNER/REVIEWER decision permissions as the common policy and document the ANALYST correction.
- Implement durable offline investigations plus locally tested Bedrock adapter code; keep live AWS calls disabled until the external configuration/authorization exists.
- Prepare an isolated Android toolchain; native iOS remains a macOS verification task.
- Keep CSV/S3 importing and optional observed account balances as separately tracked feature extensions. Missing balances remain null; product processes never read the private BankSim ledger to fill them.
- Preserve existing user data, use isolated resettable test databases, and make checkpoints recoverable with Git and the documented local restart procedure. Avoid destructive migration rollback; use backups or forward fixes if schema rollback would lose evidence.
- Continue through independent phases when an external prerequisite blocks one check, report the precise blocker, and require proof before claiming that gate complete.

Implementation began after the user's approval. Actual gate results are recorded in `docs/implementation-progress.md`; this plan describes intended acceptance, not proof that a gate has passed.
