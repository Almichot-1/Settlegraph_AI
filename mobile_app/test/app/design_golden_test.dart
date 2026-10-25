import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import '../helpers/test_workspace.dart';

void main() {
  testWidgets('review desk and evidence detail keep their visual baseline', (
    tester,
  ) async {
    final icons = FontLoader('packages/lucide_icons_flutter/Lucide')
      ..addFont(
        rootBundle.load('packages/lucide_icons_flutter/assets/lucide.ttf'),
      );
    await tester.runAsync(icons.load);
    final d = await mountWorkspace(tester);
    await expectLater(
      find.byType(MaterialApp),
      matchesGoldenFile('goldens/login-light.png'),
    );
    await enterWorkspace(tester);
    await expectLater(
      find.byType(MaterialApp),
      matchesGoldenFile('goldens/cases-light.png'),
    );
    await openSampleCase(tester);
    await expectLater(
      find.byType(MaterialApp),
      matchesGoldenFile('goldens/case-detail-light.png'),
    );
    await d.appearance.setMode(ThemeMode.dark);
    await tester.pumpAndSettle();
    await expectLater(
      find.byType(MaterialApp),
      matchesGoldenFile('goldens/case-detail-dark.png'),
    );
  });
}
