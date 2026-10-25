# Local integrated demo

Use the repository root containing `compose.yaml`, `settlegraph-web/`, and `mobile_app/`.

```sh
make bootstrap
make local-up
```

This starts the Go API/worker, BankSim, separate PostgreSQL databases, and the web application in Docker. It migrates and seeds the databases, preserves existing records, generates ignored local credentials, and configures the web server to call the actual API. Open http://localhost:3000/login. API readiness is at http://localhost:8080/health/ready.

If a port is already occupied, select free ports explicitly and use the same settings on subsequent commands:

```sh
WEB_PORT=3001 API_PORT=9080 BANKSIM_PORT=9082 make local-up
```

`make local-status` shows services; `make local-logs` shows recent logs. `make local-down` stops the integrated stack and retains database volumes. Starting it again preserves scenarios and audit history. Do not delete volumes to restart a demo.

For editing web code with hot reload on the host, stop the Compose web service first, install locked dependencies with `make frontend-install`, and supply an ignored `settlegraph-web/.env.local` containing backend mode, the host API URL, and the local demo access code. Then run `make frontend-dev`. Credentials belong only in local environment files and must not be committed.

```sh
make test lint
make test-integration
make contract-check
make test-contract-runtime
make test-e2e
make frontend-check frontend-test frontend-build
```

`make test-integration` creates one disposable PostgreSQL container, runs each Go integration test on a freshly migrated/seeded database, and removes only that container. It fails on skipped integration tests. `make test-e2e` runs synthetic scenarios and service restart recovery against the selected Compose project; run it when no manual reviewer is using that demo.

Mobile setup and native acceptance are tracked in the integration progress report. The shared host Flutter installation may differ from the pinned mobile SDK; do not assume a successful web build verifies the native app.

## Android toolchain and review

The mobile project pins Flutter 3.41.7 (Dart 3.11.5), Java 21, Android platforms/build-tools 35 and 36, NDK 28.2, CMake 3.22.1 and an API 35 x86_64 emulator. On Linux x86_64 the project installer uses official archives, verified published checksums and an isolated `.tools/` directory. Downloads resume after interruption. It requires Docker for the pinned JDK image and explicit acceptance of Google's SDK licence:

```sh
python3 scripts/setup_mobile.py --accept-android-license
python3 scripts/mobile.py pub get --enforce-lockfile
python3 mobile_app/scripts/sync_fixtures.py --check
python3 scripts/mobile.py analyze
python3 scripts/mobile.py test --coverage
python3 scripts/mobile.py build apk --debug
python3 mobile_app/scripts/start_android_emulator.py --headless
python3 mobile_app/scripts/run_live_android.py --device emulator-5554
```

Pass the SDK licence flag only after reading and accepting the linked terms printed by the installer. The installation leaves shared Flutter, Java and Android settings unchanged. Exact official download URLs/checksums are recorded in `docs/mobile-toolchain-lock.json`. Other hosts may provide Flutter 3.41.7 through `FLUTTER_BIN`, Java 21 through `JAVA_HOME`, and a compatible Android SDK through `ANDROID_HOME`; the wrappers select project tools when they exist. Native iOS requires a compatible macOS/Xcode installation and is not verified on this Linux host.

The native harness authenticates separate owner/reviewer identities and supplies runtime test credentials through a random loopback endpoint. Android signs in through its form and restores its encrypted session on restart. Access codes are not compiled into the app. It checks the late-settlement 412 conflict, explicit acknowledgement, a lost successful reply, app process termination, encrypted pending-decision restoration and exact idempotent retry. It verifies one final reviewer decision through the product API. Test fault injection exists only in `integration_test/`.

For an actual browser/native handoff, start one emulator, then run:

```sh
make frontend-install
cd settlegraph-web
npx playwright install chromium
cd ..
make test-cross-client
```

This creates two consecutive fresh test deployments with random loopback ports, separate product/BankSim databases and `settlegraph-test-*` project names. A browser owner starts/assigns/investigates a case; Android reviews that exact case while the browser releases the late settlement. Only volumes belonging to those newly created test projects are removed. The normal local demo and its records remain intact. Screenshots, logs and credential-free case/version/actor traces are stored under ignored `verification/`; `verification/latest-acceptance.json` records the tested Git commit and whether the working tree had changes. A passing run is evidence only after the command actually completes.

The offline investigation provider is the local default. See [Bedrock configuration and its external acceptance gate](bedrock.md) for the optional adapter. Production identity, live AWS deployment and real financial providers remain separate work.

To build an app that connects to the normal host demo from an Android emulator (rather than the labelled fixture workspace), use:

```sh
python3 scripts/mobile.py build apk --debug --dart-define=APP_ENV=demo --dart-define=API_BASE_URL=http://10.0.2.2:8080
```

Enter the reviewer access code from the ignored root `.env` on the device's sign-in screen. A physical device needs a reachable API URL instead of `10.0.2.2`. No access code belongs in build defines. The integration workflow preserves this runnable demo APK separately before compiling its test-only harness, so uploaded app artifacts do not contain an expired local test endpoint.
