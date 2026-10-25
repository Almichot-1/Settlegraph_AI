import '../datasources/overview_remote_datasource.dart';
import '../models/dashboard_model.dart';
import 'overview_repository.dart';

class OverviewRepositoryImpl implements OverviewRepository {
  const OverviewRepositoryImpl(this.remote);
  final OverviewRemoteDatasource remote;
  @override
  Future<DashboardModel> summary() async =>
      DashboardModel.fromJson(await remote.summary());
}
