import 'dart:io';
import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:settlegraph_mobile/app/injection_container.dart';
import 'package:settlegraph_mobile/core/config/app_config.dart';
import 'package:settlegraph_mobile/core/network/api_config.dart';
import 'package:settlegraph_mobile/core/storage/secure_storage.dart';
import 'package:settlegraph_mobile/features/auth/data/repositories/auth_repository.dart';
import 'package:settlegraph_mobile/features/cases/data/repositories/cases_repository.dart';
import 'package:settlegraph_mobile/features/cases/presentation/bloc/decision_cubit.dart';
import 'package:settlegraph_mobile/features/investigations/data/repositories/investigations_repository.dart';
import 'package:uuid/uuid.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test(
    'real product API: delayed case, citations, late evidence, stale write and human decision',
    () async {
      HttpOverrides.global = null;
      final code = Platform.environment['DEMO_ACCESS_CODE'];
      expect(
        code,
        isNotNull,
        reason:
            'Read access code from local runtime environment, never a bundled define.',
      );
      final d = await AppDependencies.create(
        config: AppConfig(
          environment: 'demo',
          apiBaseUrl:
              Platform.environment['SETTLEGRAPH_API_URL'] ??
              'http://127.0.0.1:8080',
        ),
        test: true,
        storage: MemorySessionStorage(),
      );
      addTearDown(d.dispose);
      final s = await d.get<AuthRepository>().signIn(
        'Mobile API verification',
        code!,
      );
      final api = d.get<ApiClient>();
      final repository = d.get<CasesRepository>();
      final run = await api.post(
        '/v1/demo/scenarios/delayed_settlement/runs',
        {'seed': 102},
        headers: {'Idempotency-Key': const Uuid().v4()},
      );
      final runId = run['id'] as String;
      Future<void> advance(int step) async {
        await api.post(
          '/v1/demo/scenario-runs/$runId/advance',
          {'expected_step': step},
          headers: {'Idempotency-Key': const Uuid().v4()},
        );
      }

      Future<Map<String, dynamic>> waitCase(String status) async {
        for (var i = 0; i < 60; i++) {
          final page = await api.get(
            '/v1/cases',
            query: {'scenario_run_id': runId, 'status': status},
          );
          final records = page['items'] as List;
          if (records.isNotEmpty) {
            return Map<String, dynamic>.from(records.first as Map);
          }
          await Future<void>.delayed(const Duration(milliseconds: 300));
        }
        throw StateError('Timed out waiting for $status');
      }

      await advance(0);
      await advance(1);
      final opened = await waitCase('OPEN');
      final id = opened['id'] as String;
      var bundle = await repository.detail(id);
      expect(
        bundle.evidence.where((e) => e.recordType == 'PAYMENT'),
        isNotEmpty,
      );
      expect(
        bundle.evidence.where((e) => e.recordType == 'SETTLEMENT'),
        isEmpty,
      );
      await repository.assignToMe(bundle.caseRecord);
      expect(
        (await repository.list(
          assignedTo: s.user.id,
          status: 'OPEN',
        )).items.any((c) => c.id == id),
        isTrue,
      );
      var analysis = await d.get<InvestigationsRepository>().create(
        id,
        const Uuid().v4(),
      );
      for (var attempt = 0; attempt < 80 && analysis.pending; attempt++) {
        await Future<void>.delayed(const Duration(milliseconds: 250));
        analysis = await d.get<InvestigationsRepository>().get(id, analysis.id);
      }
      expect(analysis.completed, isTrue);
      expect(analysis.provider, 'OFFLINE_STUB');
      bundle = await repository.detail(id);
      final allowed = bundle.evidence.map((e) => e.id).toSet();
      expect(allowed.containsAll(analysis.result!.evidenceIds), isTrue);
      final cubit = DecisionCubit(repository, bundle.caseRecord);
      addTearDown(cubit.close);
      await advance(2);
      final updated = await waitCase('EVIDENCE_UPDATED');
      await cubit.submit(
        'RESOLVE',
        'Reviewed the late settlement observation.',
        canReview: true,
      );
      expect(cubit.state.failure!.status, 412);
      expect(cubit.state.conflict, isNotNull);
      cubit.acknowledgeConflict();
      await cubit.submit(
        'RESOLVE',
        'Reviewed the late settlement and the source evidence.',
        canReview: true,
      );
      expect(cubit.state.completed, isTrue);
      final finalBundle = await repository.detail(id);
      expect(finalBundle.caseRecord.status, 'RESOLVED');
      expect(
        finalBundle.evidence.where((e) => e.recordType == 'SETTLEMENT'),
        isNotEmpty,
      );
      expect(finalBundle.events.any((e) => e.type == 'RESOLVED'), isTrue);
      final trace = {
        'scenario_run_id': runId,
        'case_id': id,
        'user_id': s.user.id,
        'opened_version': opened['version'],
        'updated_version': updated['version'],
        'resolved_version': finalBundle.caseRecord.version,
        'analysis_id': analysis.id,
        'evidence_ids': finalBundle.evidence.map((e) => e.id).toList(),
        'stale_write_status': 412,
        'final_status': finalBundle.caseRecord.status,
      };
      await File(
        '/tmp/settlegraph-mobile-live-trace.json',
      ).writeAsString(const JsonEncoder.withIndent('  ').convert(trace));
    },
    skip: !const bool.fromEnvironment('RUN_LIVE'),
    timeout: const Timeout(Duration(minutes: 2)),
  );
}
