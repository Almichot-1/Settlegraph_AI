import 'package:get_it/get_it.dart';
import '../../core/network/api_config.dart';
import 'data/datasources/overview_remote_datasource.dart';
import 'data/repositories/overview_repository.dart';
import 'data/repositories/overview_repository_impl.dart';

void registerOverview(GetIt di) => di.registerLazySingleton<OverviewRepository>(
  () => OverviewRepositoryImpl(OverviewRemoteDatasource(di<ApiClient>())),
);
