import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:settlegraph_mobile/app/app.dart';
import 'package:settlegraph_mobile/app/injection_container.dart';
import 'package:settlegraph_mobile/core/config/app_config.dart';
import 'package:settlegraph_mobile/core/network/api_config.dart';
import 'package:settlegraph_mobile/core/storage/secure_storage.dart';
import 'package:settlegraph_mobile/core/testing/fixture_api_client.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  testWidgets(
    'native sample review: source citation, late evidence conflict and decision',
    (tester) async {
      final d = await AppDependencies.create(config: const AppConfig());
      await tester.pumpWidget(SettleGraphApp(dependencies: d));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byType(TextFormField).first,
        'Mobile Reviewer',
      );
      await tester.ensureVisible(find.text('Explore sample workspace'));
      await tester.tap(find.text('Explore sample workspace'));
      await tester.pumpAndSettle();
      final title = find.text('Settlement not yet observed for SIM-TRF-1045');
      await tester.ensureVisible(title);
      await tester.tap(title);
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.text('Read the analysis'));
      await tester.tap(find.text('Read the analysis'));
      await tester.pumpAndSettle();
      expect(find.text('Demo analysis · offline'), findsOneWidget);
      final citation = find.text('Source 1').first;
      await tester.ensureVisible(citation);
      await tester.tap(citation);
      await tester.pumpAndSettle();
      expect(find.text('Evidence record'), findsOneWidget);
      await tester.pageBack();
      await tester.pumpAndSettle();
      await tester.pageBack();
      await tester.pumpAndSettle();
      await tester.tap(find.text('Review case'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byType(TextFormField),
        'Reviewed source records and the later settlement.',
      );
      (d.get<ApiClient>() as FixtureApiClient).receiveSettlement();
      await tester.ensureVisible(find.text('Record decision'));
      await tester.tap(find.text('Record decision'));
      await tester.pumpAndSettle();
      expect(find.textContaining('This case changed while'), findsOneWidget);
      await tester.ensureVisible(
        find.text('I have reviewed the updated evidence'),
      );
      await tester.tap(find.text('I have reviewed the updated evidence'));
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.text('Record decision'));
      await tester.tap(find.text('Record decision'));
      await tester.pumpAndSettle();
      expect(find.text('Decision recorded'), findsOneWidget);
      expect(find.text('Resolved'), findsOneWidget);
      await tester.pumpWidget(const SizedBox.shrink());
      await d.dispose();
    },
  );
  const harness = String.fromEnvironment('LIVE_HARNESS');
  if (harness.isNotEmpty) {
    testWidgets(
      'live native API review persists a human decision after late settlement',
      (tester) async {
        Future<Map<String, dynamic>> getHarness(String resource) async {
          final client = HttpClient();
          try {
            final request = await client.getUrl(
              Uri.parse('$harness/$resource'),
            );
            final response = await request.close();
            if (response.statusCode != 200) {
              throw StateError('Test harness unavailable');
            }
            return jsonDecode(await utf8.decoder.bind(response).join())
                as Map<String, dynamic>;
          } finally {
            client.close();
          }
        }

        final setup = await getHarness('session');
        const storage = SecureSessionStorage();
        await storage.write(jsonEncode(setup['session']));
        final d = await AppDependencies.create(
          config: const AppConfig(
            environment: 'demo',
            apiBaseUrl: String.fromEnvironment('API_BASE_URL'),
          ),
          storage: storage,
        );
        await tester.pumpWidget(SettleGraphApp(dependencies: d));
        await tester.pumpAndSettle();
        expect(find.text('Your review desk'), findsOneWidget);
        await tester.tap(find.text('Open').first);
        await tester.pumpAndSettle();
        final title = find.text((setup['case'] as Map)['title'] as String);
        await tester.ensureVisible(title);
        await tester.tap(title);
        await tester.pumpAndSettle();
        expect(find.text('Follow the evidence'), findsOneWidget);
        await tester.tap(find.text('Review case'));
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byType(TextFormField),
          'Reviewed the payment and later synthetic settlement evidence.',
        );
        await getHarness('settlement');
        final api = d.get<ApiClient>();
        for (var i = 0; i < 50; i++) {
          final current = await api.get(
            '/v1/cases/${(setup['case'] as Map)['id']}',
          );
          if (current['status'] == 'EVIDENCE_UPDATED') break;
          await Future<void>.delayed(const Duration(milliseconds: 300));
        }
        await tester.ensureVisible(find.text('Record decision'));
        await tester.tap(find.text('Record decision'));
        await tester.pumpAndSettle();
        expect(find.textContaining('This case changed while'), findsOneWidget);
        await tester.ensureVisible(
          find.text('I have reviewed the updated evidence'),
        );
        await tester.tap(find.text('I have reviewed the updated evidence'));
        await tester.pumpAndSettle();
        await tester.ensureVisible(find.text('Record decision'));
        await tester.tap(find.text('Record decision'));
        await tester.pumpAndSettle();
        expect(find.text('Decision recorded'), findsOneWidget);
        expect(find.text('Resolved'), findsOneWidget);
        await storage.clear();
        await tester.pumpWidget(const SizedBox.shrink());
        await d.dispose();
      },
    );
  }
}
