import '../../../../core/network/api_config.dart';

class InvestigationsRemoteDatasource {
  const InvestigationsRemoteDatasource(this.api);
  final ApiClient api;
  Future<Map<String, dynamic>> list(String id, {String? cursor}) => api.get(
    '/v1/cases/$id/investigations',
    query: {'limit': 100, 'cursor': ?cursor},
  );
  Future<Map<String, dynamic>> get(String caseId, String id) =>
      api.get('/v1/cases/$caseId/investigations/$id');
  Future<Map<String, dynamic>> create(String id, String key) => api.post(
    '/v1/cases/$id/investigations',
    {'mode': 'EVIDENCE_ANALYSIS'},
    headers: {'Idempotency-Key': key},
  );
}
