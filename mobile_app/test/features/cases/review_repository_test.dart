import 'package:flutter_test/flutter_test.dart';
import 'package:settlegraph_mobile/app/injection_container.dart';
import 'package:settlegraph_mobile/core/config/app_config.dart';
import 'package:settlegraph_mobile/core/error/failures.dart';
import 'package:settlegraph_mobile/core/network/api_config.dart';
import 'package:settlegraph_mobile/core/testing/fixture_api_client.dart';
import 'package:settlegraph_mobile/features/auth/data/repositories/auth_repository.dart';
import 'package:settlegraph_mobile/features/cases/data/repositories/cases_repository.dart';
import 'package:settlegraph_mobile/features/cases/data/repositories/cases_repository_impl.dart';
import 'package:settlegraph_mobile/features/cases/data/datasources/cases_remote_datasource.dart';
import 'package:settlegraph_mobile/features/cases/presentation/bloc/decision_cubit.dart';
import 'package:settlegraph_mobile/features/overview/data/repositories/overview_repository.dart';

class UncertainApi implements ApiClient {
  UncertainApi(this.delegate);
  final ApiClient delegate;
  final List<({Map<String, dynamic> body, Map<String, String> headers})>
  attempts = [];
  bool loseFirstReply = true;
  @override
  Future<Map<String, dynamic>> get(
    String path, {
    Map<String, dynamic>? query,
  }) => delegate.get(path, query: query);
  @override
  Future<Map<String, dynamic>> post(
    String path,
    Map<String, dynamic> body, {
    Map<String, String>? headers,
  }) async {
    attempts.add((body: {...body}, headers: {...?headers}));
    final response = await delegate.post(path, body, headers: headers);
    if (loseFirstReply) {
      loseFirstReply = false;
      throw const Failure('Reply lost', uncertain: true, code: 'NETWORK');
    }
    return response;
  }

  @override
  void close() {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  late AppDependencies d;
  late CasesRepository repo;
  const id = '44444444-4444-4444-8444-444444444444';
  setUp(() async {
    d = await AppDependencies.create(config: const AppConfig(), test: true);
    await d.get<AuthRepository>().signIn('Reviewer', '');
    repo = d.get<CasesRepository>();
  });
  tearDown(() => d.dispose());
  test(
    'canonical case and nullable evidence deserialize; filters are real',
    () async {
      final page = await repo.list(
        assignedTo: d.sessions.session!.user.id,
        status: 'OPEN',
      );
      expect(page.items.single.id, id);
      expect((await repo.list(status: 'RESOLVED')).items, isEmpty);
      expect((await repo.list(assignedTo: 'other-user')).items, isEmpty);
      final detail = await repo.detail(id);
      expect(detail.caseRecord.version, 3);
      expect(
        detail.evidence
            .singleWhere((e) => e.recordType == 'SOURCE_STATUS')
            .amount,
        isNull,
      );
      expect(detail.evidence.any((e) => e.recordType == 'SETTLEMENT'), isFalse);
      final summary = await d.get<OverviewRepository>().summary();
      expect(summary.payments.minorUnits, '250000');
      expect(summary.settlements.minorUnits, '0');
    },
  );
  test(
    'late source record changes case version and preserves prior events',
    () async {
      final before = await repo.detail(id);
      (d.get<ApiClient>() as FixtureApiClient).receiveSettlement();
      final after = await repo.detail(id);
      expect(after.caseRecord.status, 'EVIDENCE_UPDATED');
      expect(after.caseRecord.version, before.caseRecord.version + 1);
      expect(
        after.events.map((e) => e.id),
        containsAll(before.events.map((e) => e.id)),
      );
      expect(
        after.evidence.where((e) => e.recordType == 'SETTLEMENT').length,
        1,
      );
    },
  );
  test(
    'a lost reply retries the same body, quoted version and key exactly once',
    () async {
      final wire = UncertainApi(d.get<ApiClient>());
      final retryRepo = CasesRepositoryImpl(
        CasesRemoteDatasource(wire),
        d.sessions,
      );
      addTearDown(retryRepo.dispose);
      final record = (await repo.detail(id)).caseRecord;
      final cubit = DecisionCubit(retryRepo, record);
      addTearDown(cubit.close);
      await cubit.submit(
        'RESOLVE',
        'Reviewed the synthetic records.',
        canReview: true,
      );
      expect(cubit.state.uncertain, isTrue);
      expect((await repo.detail(id)).caseRecord.status, 'RESOLVED');
      await cubit.submit(
        'REOPEN',
        'This changed input must not be sent.',
        canReview: true,
      );
      expect(cubit.state.completed, isTrue);
      expect(wire.attempts.length, 2);
      expect(wire.attempts[0].headers, wire.attempts[1].headers);
      expect(wire.attempts[0].headers['If-Match'], '"3"');
      expect(wire.attempts[0].body, wire.attempts[1].body);
      expect(
        (await repo.detail(
          id,
        )).events.where((e) => e.type == 'RESOLVED').length,
        1,
      );
    },
  );
  test(
    '412 keeps the case undecided until the reviewer acknowledges fresh evidence',
    () async {
      final initial = (await repo.detail(id)).caseRecord;
      final cubit = DecisionCubit(repo, initial);
      addTearDown(cubit.close);
      (d.get<ApiClient>() as FixtureApiClient).receiveSettlement();
      await cubit.submit('RESOLVE', 'Reviewed the records.', canReview: true);
      expect(cubit.state.failure!.status, 412);
      expect(cubit.state.conflict!.caseRecord.version, 4);
      await cubit.submit('RESOLVE', 'Reviewed the records.', canReview: true);
      expect((await repo.detail(id)).caseRecord.status, 'EVIDENCE_UPDATED');
      cubit.acknowledgeConflict();
      await cubit.submit(
        'RESOLVE',
        'Reviewed the new settlement evidence.',
        canReview: true,
      );
      expect(cubit.state.completed, isTrue);
      expect(cubit.state.record.version, 5);
    },
  );
  test('viewer cannot submit; unknown case status disables review', () async {
    final record = (await repo.detail(id)).caseRecord;
    final cubit = DecisionCubit(repo, record);
    addTearDown(cubit.close);
    await cubit.submit('RESOLVE', 'Reviewed the records.', canReview: false);
    expect(cubit.state.failure!.status, 403);
    expect((await repo.detail(id)).caseRecord.version, 3);
  });
}
