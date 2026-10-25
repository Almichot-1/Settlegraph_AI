import 'package:get_it/get_it.dart';
import '../../core/network/api_config.dart';
import '../../core/services/session_manager.dart';
import 'data/datasources/cases_remote_datasource.dart';
import 'data/repositories/cases_repository.dart';
import 'data/repositories/cases_repository_impl.dart';

void registerCases(GetIt di) {
  di.registerLazySingleton<CasesRepository>(
    () => CasesRepositoryImpl(
      CasesRemoteDatasource(di<ApiClient>()),
      di<SessionManager>(),
    ),
    dispose: (r) => (r as CasesRepositoryImpl).dispose(),
  );
}
