import 'dart:convert';
import 'dart:async';
import 'package:uuid/uuid.dart';
import '../../../../core/storage/pending_decision_storage.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/models/paginated_response.dart';
import '../../../../core/services/session_manager.dart';
import '../datasources/cases_remote_datasource.dart';
import '../models/case_model.dart';
import '../models/case_event_model.dart';
import '../models/evidence_model.dart';
import 'cases_repository.dart';

class CasesRepositoryImpl implements CasesRepository {
  CasesRepositoryImpl(
    this.remote,
    this.sessions, {
    PendingDecisionStorage? storage,
  }) : storage = storage ?? MemoryPendingDecisionStorage() {
    sessions.addListener(_clear);
  }
  final CasesRemoteDatasource remote;
  final SessionManager sessions;
  final PendingDecisionStorage storage;
  final Map<String, int> _created = {};
  String? _loadedScope;
  Future<void>? _restoring;
  Future<void> _mutations = Future<void>.value();
  Future<T> _serialized<T>(Future<T> Function() action) {
    final done = Completer<T>();
    _mutations = _mutations.then((_) async {
      try {
        done.complete(await action());
      } catch (e, st) {
        done.completeError(e, st);
      }
    });
    return done.future;
  }

  Future<void> _restorePending() {
    if (_loadedScope != null && _loadedScope == _scope) {
      return Future<void>.value();
    }
    return _restoring ??= _loadPending().whenComplete(() => _restoring = null);
  }

  String? get _scope {
    final s = sessions.session;
    return s == null ? null : '${s.organizationId}:${s.user.id}';
  }

  Future<void> _loadPending() async {
    final scope = _scope;
    if (scope == null) return;
    if (_loadedScope != scope) {
      final raw = await storage.read(scope);
      if (_scope != scope) {
        throw const Failure('Workspace changed.', code: 'CANCELLED');
      }
      _pending.clear();
      _created.clear();
      if (raw != null) {
        try {
          if (raw.length > 524288) {
            throw const FormatException(
              'Pending review data exceeds its bound',
            );
          }
          final decoded = jsonDecode(raw) as Map<String, dynamic>;
          if (decoded.length > 100) {
            throw const FormatException('Too many pending reviews');
          }
          for (final entry in decoded.entries) {
            final v = Map<String, dynamic>.from(entry.value as Map);
            final created = v['created_at'] as int;
            final age = DateTime.now().millisecondsSinceEpoch - created;
            if (age < 0 || age > 3600000) continue;
            _pending[entry.key] = DecisionAttempt(
              caseId: entry.key,
              decision: v['decision'] as String,
              reason: v['reason'] as String,
              version: v['version'] as int,
              key: v['key'] as String,
            );
            _created[entry.key] = created;
          }
        } catch (_) {
          throw const Failure(
            'The pending review could not be restored. Please sign out and contact the demo operator.',
            code: 'STORAGE_UNAVAILABLE',
          );
        }
      }
      _loadedScope = scope;
    }
  }

  Future<void> _savePending(String scope) async {
    if (_pending.length > 100) {
      throw const Failure(
        'Confirm existing pending reviews before starting another.',
        code: 'STORAGE_UNAVAILABLE',
      );
    }
    final records = <String, dynamic>{};
    for (final entry in _pending.entries) {
      records[entry.key] = {
        ...entry.value.body,
        'version': entry.value.version,
        'key': entry.value.key,
        'created_at': _created[entry.key],
      };
    }
    await storage.write(scope, jsonEncode(records));
  }

  final Map<String, DecisionAttempt> _pending = {};
  void _clear() {
    if (_loadedScope != _scope) {
      _pending.clear();
      _created.clear();
      _loadedScope = null;
    }
  }

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
    await _restorePending();
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
  Future<CaseModel> decide(DecisionAttempt attempt) {
    final requestedScope = _scope;
    return _serialized(() {
      if (requestedScope == null || _scope != requestedScope) {
        throw const Failure('Workspace changed.', code: 'CANCELLED');
      }
      return _decide(attempt);
    });
  }

  Future<CaseModel> _decide(DecisionAttempt attempt) async {
    await _restorePending();
    final scope = _scope;
    if (scope == null) {
      throw const Failure('Please sign in again.', status: 401);
    }
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
    _created.putIfAbsent(
      attempt.caseId,
      () => DateTime.now().millisecondsSinceEpoch,
    );
    try {
      await _savePending(scope);
    } catch (_) {
      throw const Failure(
        'Secure review storage is unavailable. No decision was sent.',
        code: 'STORAGE_UNAVAILABLE',
      );
    }
    if (_scope != scope) {
      throw const Failure('Workspace changed.', code: 'CANCELLED');
    }
    try {
      final result = CaseModel.fromJson(
        await remote.decide(
          attempt.caseId,
          attempt.body,
          attempt.key,
          attempt.version,
        ),
      );
      if (_scope != scope) {
        throw const Failure('Workspace changed.', code: 'CANCELLED');
      }
      _pending.remove(attempt.caseId);
      _created.remove(attempt.caseId);
      try {
        await _savePending(scope);
      } catch (_) {
        _pending[attempt.caseId] = attempt;
        _created[attempt.caseId] = DateTime.now().millisecondsSinceEpoch;
        throw const Failure(
          'Decision confirmation could not be saved. Retry the same review.',
          uncertain: true,
          code: 'STORAGE_UNAVAILABLE',
        );
      }
      return result;
    } on Failure catch (e) {
      if (!e.uncertain && _scope == scope) {
        _pending.remove(attempt.caseId);
        _created.remove(attempt.caseId);
        await _savePending(scope);
      }
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
