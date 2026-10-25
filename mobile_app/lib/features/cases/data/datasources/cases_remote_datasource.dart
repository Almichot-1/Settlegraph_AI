import '../../../../core/network/api_config.dart';

class CasesRemoteDatasource {
  const CasesRemoteDatasource(this.api);
  final ApiClient api;
  Future<Map<String, dynamic>> list({
    String? cursor,
    String? assignedTo,
    String? status,
  }) => api.get(
    '/v1/cases',
    query: {
      'limit': 30,
      'cursor': ?cursor,
      'assigned_to': ?assignedTo,
      'status': ?status,
    },
  );
  Future<Map<String, dynamic>> get(String id) => api.get('/v1/cases/$id');
  Future<Map<String, dynamic>> records(
    String id,
    String resource, {
    String? cursor,
  }) => api.get(
    '/v1/cases/$id/$resource',
    query: {'limit': 100, 'cursor': ?cursor},
  );
  Future<Map<String, dynamic>> decide(
    String id,
    Map<String, dynamic> body,
    String key,
    int version,
  ) => api.post(
    '/v1/cases/$id/decisions',
    body,
    headers: {'Idempotency-Key': key, 'If-Match': '"$version"'},
  );
  Future<Map<String, dynamic>> assign(
    String id,
    String assignee,
    String key,
    int version,
  ) => api.post(
    '/v1/cases/$id/assign',
    {'assignee_id': assignee},
    headers: {'Idempotency-Key': key, 'If-Match': '"$version"'},
  );
}
