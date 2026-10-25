import '../models/dashboard_model.dart';

abstract interface class OverviewRepository {
  Future<DashboardModel> summary();
}
