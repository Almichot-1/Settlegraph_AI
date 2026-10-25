# Visual baselines

The screenshot test uses exact pixel comparisons against `macos/` or `linux/`, selected by the host operating system. Flutter renders bundled fonts and icons differently across platforms, so each supported test host has its own reviewed reference images. Missing baselines fail the test; CI never updates them automatically.

Both sets use Flutter 3.41.7, the bundled Inter and Lucide fonts, the same fixture workspace, a 390 × 844 logical-pixel viewport, and `TZ=Africa/Addis_Ababa`.

- `macos/`: original development baselines, preserved byte for byte when introducing platform-specific paths.
- `linux/`: actual rendered images from the pinned Ubuntu 24.04 runner in [diagnostic run 40180925131](https://github.com/Almichot-1/Settlegraph_AI/actions/runs/40180925131), reviewed with the corresponding visual diffs before acceptance.

Run `flutter test test/app/design_golden_test.dart` to compare. To intentionally update references, run the same command with `--update-goldens` on the matching host and review every resulting image. A failing CI run uploads `flutter-golden-diffs` for inspection.
