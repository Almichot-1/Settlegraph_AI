import 'dart:convert';
import 'package:flutter/services.dart';
import 'package:uuid/uuid.dart';
import '../error/failures.dart';
import '../network/api_config.dart';

/// Explicit sample-only transport. Never used as a fallback for HTTP failures.
class FixtureApiClient implements ApiClient {
  FixtureApiClient._(this._original);
  static Future<FixtureApiClient> load() async {
    final originals = <String, dynamic>{};
    for (final name in [
      'demo-session-response',
      'case-open',
      'case-evidence',
      'dashboard-summary',
      'ai-investigation-valid',
    ]) {
      originals[name] = jsonDecode(
        await rootBundle.loadString('assets/fixtures/$name.json'),
      );
    }
    return FixtureApiClient._(originals)..reset();
  }

  final Map<String, dynamic> _original;
  late Map<String, dynamic> _case, _summary, _user;
  late List<Map<String, dynamic>> _evidence, _events, _investigations;
  final Map<String, ({String body, Map<String, dynamic> result})> _replays = {};
  Map<String, dynamic> _clone(Map<String, dynamic> source) =>
      jsonDecode(jsonEncode(source)) as Map<String, dynamic>;
  String get caseId => _case['id'] as String;
  void reset() {
    _case = _clone(_original['case-open'] as Map<String, dynamic>);
    _summary = _clone(_original['dashboard-summary'] as Map<String, dynamic>);
    _user = _clone(
      (_original['demo-session-response'] as Map<String, dynamic>)['user']
          as Map<String, dynamic>,
    );
    _evidence = ((_original['case-evidence'] as Map)['items'] as List)
        .map((e) => _clone(e as Map<String, dynamic>))
        .toList();
    _events = [
      _event(
        'OPENED',
        'Settlement not yet observed after the configured grace period. Source coverage is complete.',
        3,
        '2026-10-24T11:00:00Z',
      ),
    ];
    _investigations = [
      {
        'id': '77777777-7777-4777-8777-777777777777',
        'case_id': caseId,
        'status': 'COMPLETED',
        'provider': 'OFFLINE_STUB',
        'created_at': '2026-10-24T11:01:00Z',
        'completed_at': '2026-10-24T11:01:01Z',
        'evidence_snapshot_hash': 'sample-before-settlement',
        'result': _clone(
          _original['ai-investigation-valid'] as Map<String, dynamic>,
        ),
      },
    ];
    _replays.clear();
  }

  Map<String, dynamic> _event(
    String type,
    String message,
    int version,
    String at,
  ) => {
    'id': const Uuid().v4(),
    'case_id': caseId,
    'type': type,
    'message': message,
    'version': version,
    'occurred_at': at,
    'actor': type == 'DECISION_RECORDED' || type == 'RESOLVED'
        ? 'Sample reviewer'
        : 'System',
  };
  void receiveSettlement() {
    if (_evidence.any((e) => e['record_type'] == 'SETTLEMENT')) return;
    const at = '2026-10-24T11:10:00Z';
    _evidence.add({
      'id': '88888888-8888-4888-8888-888888888888',
      'record_type': 'SETTLEMENT',
      'reference': 'SIM-TRF-1045',
      'amount': {'minor_units': '250000', 'currency': 'ETB'},
      'occurred_at': at,
      'provider_id': 'BANK_B',
      'raw_payload_sha256': null,
    });
    _case['version'] = (_case['version'] as int) + 1;
    _case['status'] = 'EVIDENCE_UPDATED';
    _case['evidence_count'] = _evidence.length;
    _case['updated_at'] = at;
    _events.add(
      _event(
        'EVIDENCE_ADDED',
        'A later synthetic settlement was observed. Review the new evidence before recording a decision.',
        _case['version'] as int,
        at,
      ),
    );
    _summary['settlements_observed'] = {
      'minor_units': '250000',
      'currency': 'ETB',
    };
    _summary['unmatched_payments'] = {'minor_units': '0', 'currency': 'ETB'};
    _summary['as_of'] = at;
  }

  Map<String, dynamic> _page(
    List<Map<String, dynamic>> records,
    Map<String, dynamic>? query,
  ) {
    final ordered = [...records]
      ..sort((a, b) => (a['id'] as String).compareTo(b['id'] as String));
    final cursor = query?['cursor'] as String?;
    final remaining = ordered
        .where(
          (e) => cursor == null || (e['id'] as String).compareTo(cursor) > 0,
        )
        .toList();
    final limit = query?['limit'] as int? ?? 30;
    final items = remaining.take(limit).map(_clone).toList();
    return {
      'items': items,
      'next_cursor': remaining.length > limit ? items.last['id'] : null,
    };
  }

  @override
  Future<Map<String, dynamic>> get(
    String path, {
    Map<String, dynamic>? query,
  }) async {
    if (path == '/v1/me') return _clone(_user);
    if (path == '/v1/dashboard/summary') return _clone(_summary);
    if (path == '/v1/cases') {
      return _page([
        if ((query?['status'] == null || query?['status'] == _case['status']) &&
            (query?['assigned_to'] == null ||
                query?['assigned_to'] == _case['assigned_to']))
          _case,
      ], query);
    }
    if (path == '/v1/cases/$caseId') return _clone(_case);
    if (path == '/v1/cases/$caseId/evidence') return _page(_evidence, query);
    if (path == '/v1/cases/$caseId/events') return _page(_events, query);
    if (path == '/v1/cases/$caseId/investigations') {
      return _page(_investigations, query);
    }
    for (final item in _investigations) {
      if (path == '/v1/cases/$caseId/investigations/${item['id']}') {
        return _clone(item);
      }
    }
    throw const Failure(
      'This record is unavailable.',
      status: 404,
      code: 'NOT_FOUND',
    );
  }

  @override
  Future<Map<String, dynamic>> post(
    String path,
    Map<String, dynamic> body, {
    Map<String, String>? headers,
  }) async {
    if (path == '/v1/demo/sessions') {
      _user['display_name'] = body['display_name'];
      return {
        ..._clone(_original['demo-session-response'] as Map<String, dynamic>),
        'user': _clone(_user),
        'expires_at': DateTime.now()
            .add(const Duration(hours: 1))
            .toUtc()
            .toIso8601String(),
      };
    }
    final key = headers?['Idempotency-Key'];
    if (key == null || key.length < 8) {
      throw const Failure('A retry key is required.', status: 400);
    }
    final replayKey = '$path:$key';
    final signature = jsonEncode(body);
    final replay = _replays[replayKey];
    if (replay != null) {
      if (replay.body != signature) {
        throw const Failure('This action key was already used.', status: 409);
      }
      return _clone(replay.result);
    }
    Map<String, dynamic> result;
    if (path == '/v1/cases/$caseId/decisions' ||
        path == '/v1/cases/$caseId/assign') {
      if (headers?['If-Match'] != '"${_case['version']}"') {
        throw const Failure('This case has changed.', status: 412);
      }
      final decision = body['decision'];
      if (decision != null) {
        if ((body['reason'] as String).trim().length < 5) {
          throw const Failure('Add a review reason.', status: 400);
        }
        if (!['RESOLVE', 'REOPEN', 'NEEDS_MORE_EVIDENCE'].contains(decision)) {
          throw const Failure('Invalid decision.', status: 400);
        }
        if (decision == 'RESOLVE' && _case['status'] == 'RESOLVED' ||
            decision == 'REOPEN' && _case['status'] != 'RESOLVED') {
          throw const Failure('This decision no longer applies.', status: 409);
        }
        _case['status'] = decision == 'RESOLVE' ? 'RESOLVED' : 'OPEN';
      } else {
        _case['assigned_to'] = body['assignee_id'];
      }
      _case['version'] = (_case['version'] as int) + 1;
      _case['updated_at'] = DateTime.now().toUtc().toIso8601String();
      _events.add(
        _event(
          decision == 'RESOLVE'
              ? 'RESOLVED'
              : decision == null
              ? 'ASSIGNED'
              : 'DECISION_RECORDED',
          decision == null
              ? 'Assigned to the sample reviewer.'
              : '$decision: ${body['reason']}',
          _case['version'] as int,
          _case['updated_at'] as String,
        ),
      );
      _summary['open_cases'] = _case['status'] == 'RESOLVED' ? 0 : 1;
      result = _clone(_case);
    } else if (path == '/v1/cases/$caseId/investigations') {
      result = {
        ..._clone(_investigations.first),
        'id': const Uuid().v4(),
        'created_at': DateTime.now().toUtc().toIso8601String(),
      };
      if (_evidence.any((e) => e['record_type'] == 'SETTLEMENT')) {
        result['evidence_snapshot_hash'] = 'sample-after-settlement';
        result['result'] = {
          'summary':
              'A later settlement observation is now available for this synthetic payment. A reviewer can inspect the source records and decide how to handle the case.',
          'hypotheses': [],
          'recommended_actions': [
            'Review the payment and settlement records together.',
          ],
          'evidence_ids': _evidence
              .where((e) => e['record_type'] != 'SOURCE_STATUS')
              .map((e) => e['id'])
              .toList(),
          'limitations': [
            'Sample data describes simulated records. Case decisions do not alter payment facts.',
          ],
        };
      }
      _investigations.add(result);
    } else {
      throw const Failure('This action is unavailable.', status: 404);
    }
    _replays[replayKey] = (body: signature, result: _clone(result));
    return _clone(result);
  }

  @override
  void close() {}
}
