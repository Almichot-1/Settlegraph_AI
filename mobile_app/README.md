# SettleGraph Mobile

A Flutter Android/iOS reviewer app for SettleGraph. The app provides a case queue, source evidence, investigation citations, versioned human decisions, overview, organization switching, appearance settings, and session handling.

The interface uses bundled Inter typography, Lucide icons, warm neutral surfaces, and forest-green actions. The app includes light/dark themes and a lifecycle privacy cover. Financial values use exact string/BigInt formatting.

## Run without a backend

Requires Flutter **3.41.7** / Dart **3.11.5** or a compatible SDK and a connected emulator/device.

```bash
cd mobile_app
flutter pub get --enforce-lockfile
flutter run --dart-define=APP_ENV=fixture
```

Enter a name and choose **Explore sample workspace**. Sample mode is explicitly labeled and has no network/model dependency. In **Account → Sample walkthrough**, use **Show settlement arrival** to inspect an updated case, or **Reset sample story** to start again. Sample decisions are in memory and reset when the process restarts.

## Start the emulator reliably on macOS

From the repository root:

```bash
python3 mobile_app/scripts/start_android_emulator.py
```

Wait for **Ready: emulator-5554, Android API 35, arm64-v8a** before running Flutter. The helper defaults to your `Pixel_8` at `emulator-5554`, opens a visible window, cold-boots with software graphics, and waits for readable ADB properties. It never wipes emulator data. This avoids the graphics-context/snapshot failure that can make Flutter report `error: closed` and Android API `null`. If the emulator is already running, the helper checks its readiness instead of starting another instance.

### Pixel storage workaround

The existing Pixel_8 has limited free storage. A 32.7 MB Android-arm64 profile APK installed once, but later Flutter reinstall attempts still failed with insufficient storage. Free additional storage, or choose Wipe Data in Android Studio Device Manager to factory-reset that emulator, before relying on repeated installs. Wipe Data deletes that emulator's apps, accounts, files, and settings. No reset was performed by the agent. After resolving storage, manual testing against the current local API can use:

```bash
flutter run --profile -d emulator-5554 \
  --dart-define=APP_ENV=demo \
  --dart-define=API_BASE_URL=http://10.0.2.2:9080
```

Profile mode has no hot reload. Debug builds require more storage.

## Connect to the local product API

Start and seed the backend using the root README. Enter the locally distributed demo access code at sign-in; it is never bundled in app source, Dart defines, assets, or preferences. Live sessions use secure native storage. The backend currently mints the shared demo owner identity.

```bash
# Android emulator (default backend port)
flutter run --dart-define=APP_ENV=demo --dart-define=API_BASE_URL=http://10.0.2.2:8080

# iOS simulator
flutter run --dart-define=APP_ENV=demo --dart-define=API_BASE_URL=http://127.0.0.1:8080
```

During this implementation's local verification, port 8080 was occupied by an unrelated service. The API was started on **127.0.0.1:9080**, so use `http://10.0.2.2:9080` for that running Android demo. The repository Compose default remains 8080.

A physical Android device can use `adb reverse tcp:9080 tcp:9080`, then `API_BASE_URL=http://127.0.0.1:9080`. Device networking must be configured explicitly; the product API is bound to host loopback by default.

Development HTTP permissions are restricted to debug and profile configurations. Live release builds require an HTTPS base URL. The app calls only the public SettleGraph API; no simulator/private database credentials are present.

## Structure

```text
lib/
  app/             # bootstrap, dependency injection, router, theme, shell
  core/            # HTTP, sessions, storage, money, errors, common controls
  features/
    auth/          # data sources → repository → auth BLoC → sign-in
    cases/         # queue BLoC, detail Cubit, decision Cubit, evidence/history
    investigations/# result history, polling, citations
    overview/      # separate financial observations and source freshness
    account/       # organization, appearance, environment, sign-out
```

Models map snake_case JSON directly. Feature BLoCs/Cubits call repository interfaces under `data/repositories/`. The fixture and HTTP transports share the same feature repositories. Unsupported auth flows remain deferred because the API exposes no signup, password reset, Google sign-in, or token-refresh endpoints. Session expiry returns to sign-in.

Decisions send a quoted `If-Match` version and one idempotency key per action. Uncertain submissions keep their retry body/key in memory until confirmed. A 412 retains the reason, loads updated evidence, and requires explicit reviewer acknowledgement. Decisions are never queued offline or shown as successful before a server response. App-termination retry-envelope persistence remains future work.

## Validate

```bash
python3 scripts/sync_fixtures.py --check
dart format --output=none --set-exit-if-changed lib test integration_test
flutter analyze
TZ=Africa/Addis_Ababa flutter test --coverage
flutter build apk --debug
flutter build ios --simulator --no-codesign
flutter test integration_test/app_test.dart -d <device-id>
```

Golden baselines use the pinned SDK, bundled fonts, 390×844 logical pixels, and **Africa/Addis_Ababa** time. Update them only after inspecting intended visual changes. The opt-in live test is skipped in the default suite; it requires a running synthetic backend and an access code supplied through the host process environment.

Native live Android verification can be launched from the repository root without compiling credentials into the test app:

```bash
python3 mobile_app/scripts/run_live_android.py \
  --device emulator-5556 \
  --api-url http://127.0.0.1:9080 \
  --device-api-url http://10.0.2.2:9080
```

This script reads the ignored root `.env`, starts a nonce-scoped loopback harness, creates a delayed scenario through public product APIs, and supplies the short-lived session to the native test. The test restores that session through secure storage, reviews the assigned case, encounters a stale-version conflict after settlement, and resolves it. The host independently checks the resulting API case/history and writes a credential-free trace to ignored `verification/`.

The original app mark is in `assets/icons/app_mark.svg`. Android uses a native vector; `swift scripts/render_ios_icons.swift` regenerates the iOS icon catalog on macOS. CI lives in repository-root `.github/workflows/mobile_ci_validation.yml`. Store release automation is deferred.

## Verification and remaining work

See [implementation progress](../docs/implementation-progress.md), [risk log](../docs/risk-log.md), and [mobile plan](../docs/mobile-implementation-plan.md) for actual results and remaining gates. Native iOS acceptance currently requires a compatible Xcode platform/runtime; the first build failed because Xcode requested iOS 26.5 while only the iOS 26.4 runtime was installed. No app-store, AWS, or production identity setup has been performed.
