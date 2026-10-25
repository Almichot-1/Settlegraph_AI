import 'package:get_it/get_it.dart';
import '../../core/network/api_config.dart';
import '../../core/services/session_manager.dart';
import '../../core/storage/secure_storage.dart';
import 'data/datasources/auth_local_datasource.dart';
import 'data/datasources/auth_remote_datasource.dart';
import 'data/repositories/auth_repository.dart';
import 'data/repositories/auth_repository_impl.dart';

void registerAuth(GetIt di) {
  di.registerLazySingleton<AuthRepository>(
    () => AuthRepositoryImpl(
      AuthRemoteDatasource(di<ApiClient>()),
      AuthLocalDatasource(di<SessionStorage>()),
      di<SessionManager>(),
    ),
  );
}
