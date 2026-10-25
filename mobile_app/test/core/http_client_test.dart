import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:settlegraph_mobile/core/error/failures.dart';
import 'package:settlegraph_mobile/core/network/authenticated_http_client.dart';
import 'package:settlegraph_mobile/core/services/session_manager.dart';
import 'package:settlegraph_mobile/features/auth/data/models/user_model.dart';

class Adapter implements HttpClientAdapter {
  final requests = <RequestOptions>[];
  Future<ResponseBody> Function(RequestOptions)? respond;
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? stream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    return respond?.call(options) ??
        ResponseBody.fromString(
          '{}',
          200,
          headers: {
            Headers.contentTypeHeader: [Headers.jsonContentType],
          },
        );
  }

  @override
  void close({bool force = false}) {}
}

ResponseBody jsonResponse(Map<String, dynamic> body, int code) =>
    ResponseBody.fromString(
      jsonEncode(body),
      code,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
AuthSession session({String org = 'org-a'}) => AuthSession(
  token: 'sample-token',
  expiresAt: DateTime.now().add(const Duration(hours: 1)),
  user: UserModel(
    id: 'user',
    displayName: 'Reviewer',
    memberships: [Membership(id: org, name: org, role: 'OWNER')],
  ),
  organizationId: org,
);
void main() {
  late SessionManager sessions;
  late Adapter adapter;
  late AuthenticatedHttpClient client;
  setUp(() {
    sessions = SessionManager()..update(session());
    adapter = Adapter();
    final dio = Dio(BaseOptions(baseUrl: 'https://test.invalid'))
      ..httpClientAdapter = adapter;
    client = AuthenticatedHttpClient(
      'https://test.invalid',
      sessions,
      dio: dio,
    );
  });
  tearDown(() {
    client.close();
    sessions.dispose();
  });
  test(
    'scoped requests carry session headers and exact mutation headers',
    () async {
      await client.post(
        '/v1/cases/case/decisions',
        {'decision': 'RESOLVE', 'reason': 'Reviewed.'},
        headers: {'Idempotency-Key': 'same-key-123', 'If-Match': '"3"'},
      );
      final r = adapter.requests.single;
      expect(r.headers['Authorization'], 'Bearer sample-token');
      expect(r.headers['X-Organization-Id'], 'org-a');
      expect(r.headers['If-Match'], '"3"');
      expect(r.headers['Idempotency-Key'], 'same-key-123');
      expect(r.headers.containsKey('X-Demo-Access-Code'), isFalse);
    },
  );
  test(
    'demo login carries only the supplied code, not bearer or org headers',
    () async {
      await client.post(
        '/v1/demo/sessions',
        {'display_name': 'Sam'},
        headers: {'X-Demo-Access-Code': 'entered-at-runtime'},
      );
      expect(adapter.requests.single.headers['Authorization'], isNull);
      expect(adapter.requests.single.headers['X-Organization-Id'], isNull);
    },
  );
  test(
    '401 expires the session once and preserves the structured request reference',
    () async {
      var expirations = 0;
      sessions.onExpired = () => expirations++;
      adapter.respond = (_) async => jsonResponse({
        'error': {
          'message': 'Expired',
          'code': 'UNAUTHORIZED',
          'request_id': 'request-1',
        },
      }, 401);
      await expectLater(
        client.get('/v1/cases'),
        throwsA(
          isA<Failure>()
              .having((f) => f.status, 'status', 401)
              .having((f) => f.requestId, 'request ID', 'request-1'),
        ),
      );
      expect(sessions.session, isNull);
      expect(expirations, 1);
      await expectLater(client.get('/v1/cases'), throwsA(isA<Failure>()));
      expect(expirations, 1);
    },
  );
  test(
    'a connection timeout on a POST is uncertain and never automatically retried',
    () async {
      adapter.respond = (options) async => throw DioException(
        requestOptions: options,
        type: DioExceptionType.connectionTimeout,
      );
      await expectLater(
        client.post('/v1/cases/a/decisions', {}),
        throwsA(isA<Failure>().having((f) => f.uncertain, 'uncertain', true)),
      );
      expect(adapter.requests.length, 1);
    },
  );
  test(
    'a throttled POST preserves an earlier unconfirmed submission',
    () async {
      adapter.respond = (_) async => jsonResponse({
        'error': {
          'code': 'RATE_LIMITED',
          'message': 'Wait before retrying.',
          'request_id': 'rate-1',
        },
      }, 429);
      await expectLater(
        client.post('/v1/cases/a/decisions', {}),
        throwsA(
          isA<Failure>()
              .having((f) => f.status, 'status', 429)
              .having((f) => f.uncertain, 'uncertain', true),
        ),
      );
      expect(adapter.requests.length, 1);
    },
  );
  test('an old organization reply cannot update the new workspace', () async {
    final reply = Completer<ResponseBody>();
    adapter.respond = (_) => reply.future;
    final request = client.get('/v1/cases');
    final expectation = expectLater(
      request,
      throwsA(isA<Failure>().having((f) => f.isCancelled, 'cancelled', true)),
    );
    await Future<void>.delayed(Duration.zero);
    sessions.update(session(org: 'org-b'));
    reply.complete(
      jsonResponse({
        'items': [
          {'id': 'old-org-record'},
        ],
      }, 200),
    );
    await expectation;
  });
}
