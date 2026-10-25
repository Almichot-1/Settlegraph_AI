import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:settlegraph_mobile/app/app.dart';
import 'package:settlegraph_mobile/app/injection_container.dart';
import 'package:settlegraph_mobile/core/config/app_config.dart';
import 'package:settlegraph_mobile/core/testing/fixture_api_client.dart';

Future<AppDependencies> mountWorkspace(
  WidgetTester tester, {
  Size size = const Size(390, 844),
  double textScale = 1,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  tester.platformDispatcher.textScaleFactorTestValue = textScale;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);
  final fonts = FontLoader('Inter')
    ..addFont(rootBundle.load('assets/fonts/Inter.ttf'));
  await tester.runAsync(fonts.load);
  final fixture = (await tester.runAsync(FixtureApiClient.load))!;
  final d = await AppDependencies.create(
    config: const AppConfig(),
    api: fixture,
    test: true,
  );
  await tester.pumpWidget(SettleGraphApp(dependencies: d));
  await tester.pumpAndSettle();
  addTearDown(() async {
    await tester.pumpWidget(const SizedBox.shrink());
    await d.dispose();
  });
  return d;
}

Future<void> enterWorkspace(WidgetTester tester) async {
  await tester.enterText(find.byType(TextFormField).first, 'Sam Reviewer');
  await tester.ensureVisible(find.text('Explore sample workspace'));
  await tester.tap(find.text('Explore sample workspace'));
  await tester.pumpAndSettle();
  expect(find.text('Your review desk'), findsOneWidget);
}

Future<void> openSampleCase(WidgetTester tester) async {
  final title = find.text('Settlement not yet observed for SIM-TRF-1045');
  await tester.ensureVisible(title);
  await tester.tap(title);
  await tester.pumpAndSettle();
  expect(find.text('Follow the evidence'), findsOneWidget);
}
