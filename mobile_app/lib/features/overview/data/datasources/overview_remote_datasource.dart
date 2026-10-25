import '../../../../core/network/api_config.dart';

class OverviewRemoteDatasource {
  const OverviewRemoteDatasource(this.api);
  final ApiClient api;
  Future<Map<String, dynamic>> summary() => api.get('/v1/dashboard/summary');
}
