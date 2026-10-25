import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:settlegraph_mobile/core/network/api_config.dart';
import 'package:settlegraph_mobile/core/testing/fixture_api_client.dart';
import '../helpers/test_workspace.dart';

void main() {
  testWidgets(
    'sample login, evidence and decision are a complete working flow',
    (tester) async {
      final d = await mountWorkspace(tester);
      await enterWorkspace(tester);
      await openSampleCase(tester);
      await tester.tap(find.text('Review case'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byType(TextFormField),
        'Reviewed the available synthetic records.',
      );
      await tester.ensureVisible(find.text('Record decision'));
      await tester.tap(find.text('Record decision'));
      await tester.pumpAndSettle();
      expect(find.text('Decision recorded'), findsOneWidget);
      expect(find.text('Resolved'), findsOneWidget);
      expect(tester.takeException(), isNull);
      expect(
        (await d.get<ApiClient>().get(
          '/v1/cases/44444444-4444-4444-8444-444444444444',
        ))['status'],
        'RESOLVED',
      );
    },
  );
  testWidgets(
    'a case version conflict preserves the typed reason and requires acknowledgement',
    (tester) async {
      final d = await mountWorkspace(tester);
      await enterWorkspace(tester);
      await openSampleCase(tester);
      await tester.tap(find.text('Review case'));
      await tester.pumpAndSettle();
      const reason = 'Reviewed the original evidence before the update.';
      await tester.enterText(find.byType(TextFormField), reason);
      (d.get<ApiClient>() as FixtureApiClient).receiveSettlement();
      await tester.ensureVisible(find.text('Record decision'));
      await tester.tap(find.text('Record decision'));
      await tester.pumpAndSettle();
      expect(find.text(reason), findsOneWidget);
      expect(find.textContaining('This case changed while'), findsOneWidget);
      expect(find.text('I have reviewed the updated evidence'), findsOneWidget);
      await tester.ensureVisible(
        find.text('I have reviewed the updated evidence'),
      );
      await tester.tap(find.text('I have reviewed the updated evidence'));
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.text('Record decision'));
      await tester.tap(find.text('Record decision'));
      await tester.pumpAndSettle();
      expect(find.text('Decision recorded'), findsOneWidget);
    },
  );
  testWidgets(
    'small phone and 200% text remain scrollable with usable review actions',
    (tester) async {
      await mountWorkspace(tester, size: const Size(320, 568), textScale: 2);
      await enterWorkspace(tester);
      await openSampleCase(tester);
      expect(tester.takeException(), isNull);
      await tester.ensureVisible(find.text('Review case'));
      await tester.tap(find.text('Review case'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byType(TextFormField),
        'Reviewed source evidence.',
      );
      await tester.ensureVisible(find.text('Record decision'));
      expect(tester.takeException(), isNull);
    },
  );
}
