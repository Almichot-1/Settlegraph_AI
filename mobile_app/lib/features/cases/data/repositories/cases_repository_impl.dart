import 'package:uuid/uuid.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/models/paginated_response.dart';
import '../../../../core/services/session_manager.dart';
import '../datasources/cases_remote_datasource.dart';
import '../models/case_model.dart';
import '../models/case_event_model.dart';
import '../models/evidence_model.dart';
import 'cases_repository.dart';

class CasesRepositoryImpl implements CasesRepository {
  CasesRepositoryImpl(this.remote, this.sessions) {
    sessions.addListener(_clear);
  }
  final CasesRemoteDatasource remote;
  final SessionManager sessions;
  final Map<String, DecisionAttempt> _pending = {};
  void _clear() => _pending.clear();
  void dispose() => sessions.removeListener(_clear);
  @override
  Future<PageResult<CaseModel>> list({
    String? cursor,
    String? assignedTo,
    String? status,
  }) async => PageResult.fromJson(
    await remote.list(cursor: cursor, assignedTo: assignedTo, status: status),
    CaseModel.fromJson,
  );
  Future<List<T>> _all<T>(
    String id,
    String resource,
    T Function(Map<String, dynamic>) decode,
  ) async {
    final items = <T>[];
    final seen = <String>{};
    String? cursor;
    do {
      final page = PageResult.fromJson(
        await remote.records(id, resource, cursor: cursor),
        decode,
      );
      items.addAll(page.items);
      cursor = page.nextCursor;
      if (cursor != null && !seen.add(cursor)) {
        throw const Failure(
          'The workspace returned a repeated page.',
          code: 'INVALID_RESPONSE',
        );
      }
    } while (cursor != null);
    return items;
  }

  @override
  Future<CaseBundle> detail(String id) async {
    // Read the version on both sides so evidence is never presented as a coherent
    // snapshot when a concurrent update happened between the separate endpoints.
    for (var attempt = 0; attempt < 3; attempt++) {
      final before = CaseModel.fromJson(await remote.get(id));
      final evidence = await _all(id, 'evidence', EvidenceModel.fromJson);
      final events = await _all(id, 'events', CaseEventModel.fromJson);
      final after = CaseModel.fromJson(await remote.get(id));
      if (before.version == after.version) {
        return CaseBundle(
          after,
          evidence,
          events..sort((a, b) => b.occurredAt.compareTo(a.occurredAt)),
        );
      }
    }
    throw const Failure(
      'This case is receiving new evidence. Please refresh.',
      code: 'CHANGING_SNAPSHOT',
    );
  }

  @override
  DecisionAttempt? pending(String id) => _pending[id];
  @override
  Future<CaseModel> decide(DecisionAttempt attempt) async {
    final old = _pending[attempt.caseId];
    if (old != null &&
        (old.key != attempt.key ||
            old.decision != attempt.decision ||
            old.reason != attempt.reason ||
            old.version != attempt.version)) {
      throw const Failure(
        'Retry the unconfirmed submission before starting another decision.',
        code: 'PENDING_DECISION',
      );
    }
    _pending[attempt.caseId] = attempt;
    try {
      final result = CaseModel.fromJson(
        await remote.decide(
          attempt.caseId,
          attempt.body,
          attempt.key,
          attempt.version,
        ),
      );
      _pending.remove(attempt.caseId);
      return result;
    } on Failure catch (e) {
      if (!e.uncertain) _pending.remove(attempt.caseId);
      rethrow;
    }
  }

  @override
  Future<CaseModel> assignToMe(CaseModel record) async {
    final user = sessions.session?.user.id;
    if (user == null || sessions.membership?.canAssign != true) {
      throw const Failure('You cannot assign cases.', status: 403);
    }
    return CaseModel.fromJson(
      await remote.assign(record.id, user, const Uuid().v4(), record.version),
    );
  }
}
