import '../../../../core/models/money.dart';

class ProviderModel {
  ProviderModel(Map<String, dynamic> j)
    : id = j['id'] as String,
      name = j['name'] as String,
      status = j['sync_status'] as String,
      lastObservedAt = j['last_observed_at'] == null
          ? null
          : DateTime.parse(j['last_observed_at'] as String);
  final String id, name, status;
  final DateTime? lastObservedAt;
}

class DashboardModel {
  DashboardModel.fromJson(Map<String, dynamic> j)
    : asOf = DateTime.parse(j['as_of'] as String),
      orders = Money.fromJson(j['orders_expected'] as Map<String, dynamic>),
      payments = Money.fromJson(j['payments_accepted'] as Map<String, dynamic>),
      settlements = Money.fromJson(
        j['settlements_observed'] as Map<String, dynamic>,
      ),
      unmatched = Money.fromJson(
        j['unmatched_payments'] as Map<String, dynamic>,
      ),
      openCases = j['open_cases'] as int,
      coverage = (j['coverage'] as Map)['source_status'] as String,
      providers = (j['providers'] as List)
          .map((e) => ProviderModel(Map<String, dynamic>.from(e as Map)))
          .toList();
  final DateTime asOf;
  final Money orders, payments, settlements, unmatched;
  final int openCases;
  final String coverage;
  final List<ProviderModel> providers;
}
