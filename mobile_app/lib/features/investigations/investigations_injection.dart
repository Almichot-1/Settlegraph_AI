import 'package:get_it/get_it.dart';
import '../../core/network/api_config.dart';
import 'data/datasources/investigations_remote_datasource.dart';
import 'data/repositories/investigations_repository.dart';
import 'data/repositories/investigations_repository_impl.dart';

void registerInvestigations(GetIt di) =>
    di.registerLazySingleton<InvestigationsRepository>(
      () => InvestigationsRepositoryImpl(
        InvestigationsRemoteDatasource(di<ApiClient>()),
      ),
    );
