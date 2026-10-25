import 'package:dio/dio.dart';
import '../error/failures.dart';
import '../services/session_manager.dart';
import 'api_config.dart';

class AuthenticatedHttpClient implements ApiClient {
  AuthenticatedHttpClient(String baseUrl, this.sessions, {Dio? dio})
    : _dio =
          dio ??
          Dio(
            BaseOptions(
              baseUrl: baseUrl,
              connectTimeout: const Duration(seconds: 12),
              receiveTimeout: const Duration(seconds: 20),
              sendTimeout: const Duration(seconds: 12),
            ),
          ) {
    sessions.addListener(_sessionChanged);
  }
  final Dio _dio;
  final SessionManager sessions;
  CancelToken _scope = CancelToken();
  void _sessionChanged() {
    _scope.cancel();
    _scope = CancelToken();
  }

  @override
  Future<Map<String, dynamic>> get(
    String path, {
    Map<String, dynamic>? query,
  }) => _request(path, 'GET', query: query);
  @override
  Future<Map<String, dynamic>> post(
    String path,
    Map<String, dynamic> body, {
    Map<String, String>? headers,
  }) => _request(path, 'POST', body: body, headers: headers);
  Future<Map<String, dynamic>> _request(
    String path,
    String method, {
    Map<String, dynamic>? query,
    Map<String, dynamic>? body,
    Map<String, String>? headers,
  }) async {
    final login = path == '/v1/demo/sessions';
    final generation = sessions.generation;
    final session = sessions.session;
    if (!login && session == null) {
      throw const Failure('Please sign in again.', status: 401);
    }
    try {
      final response = await _dio.request<dynamic>(
        path,
        data: body,
        queryParameters: query,
        cancelToken: login ? null : _scope,
        options: Options(
          method: method,
          headers: {
            if (!login && session != null)
              'Authorization': 'Bearer ${session.token}',
            if (!login && path != '/v1/me' && session != null)
              'X-Organization-Id': session.organizationId,
            ...?headers,
          },
        ),
      );
      if (!login && generation != sessions.generation) {
        throw const Failure('Workspace changed.', code: 'CANCELLED');
      }
      return Map<String, dynamic>.from(response.data as Map);
    } on DioException catch (e) {
      if (!login && generation != sessions.generation ||
          CancelToken.isCancel(e)) {
        throw const Failure('Request cancelled.', code: 'CANCELLED');
      }
      final status = e.response?.statusCode;
      if (status == 401 && !login) sessions.expire();
      final data = e.response?.data;
      final error = data is Map ? data['error'] : null;
      if (error is Map) {
        throw Failure(
          error['message'] as String? ?? 'Request failed.',
          status: status,
          code: error['code'] as String? ?? 'HTTP_ERROR',
          requestId: error['request_id'] as String?,
          uncertain:
              method == 'POST' &&
              status != null &&
              (status >= 500 || status == 429),
        );
      }
      throw Failure(
        status == null
            ? 'Cannot reach the workspace. Check your connection and try again.'
            : 'The workspace could not complete this request.',
        status: status,
        code: status == null ? 'NETWORK' : 'HTTP_ERROR',
        uncertain:
            method == 'POST' &&
            (status == null || status >= 500 || status == 429),
      );
    }
  }

  @override
  void close() {
    sessions.removeListener(_sessionChanged);
    _scope.cancel();
    _dio.close();
  }
}
