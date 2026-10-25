import 'package:get_it/get_it.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../core/config/app_config.dart';
import '../core/network/api_config.dart';
import '../core/network/authenticated_http_client.dart';
import '../core/services/session_manager.dart';
import '../core/storage/local_storage.dart';
import '../core/storage/secure_storage.dart';
import '../core/testing/fixture_api_client.dart';
import '../features/auth/auth_injection.dart';
import '../features/auth/data/repositories/auth_repository.dart';
import '../features/auth/presentation/bloc/auth_bloc.dart';
import '../features/auth/presentation/bloc/auth_event.dart';
import '../features/cases/cases_injection.dart';
import '../features/investigations/investigations_injection.dart';
import '../features/overview/overview_injection.dart';

class AppDependencies {
  AppDependencies._(this.config, this.di, this.appearance, this.version);
  final AppConfig config;
  final GetIt di;
  final AppearanceController appearance;
  final String version;
  T get<T extends Object>() => di<T>();
  SessionManager get sessions => get<SessionManager>();
  AuthBloc get auth => get<AuthBloc>();
  static Future<AppDependencies> create({
    AppConfig? config,
    SessionStorage? storage,
    ApiClient? api,
    bool test = false,
  }) async {
    final c = config ?? AppConfig.fromEnvironment();
    final di = GetIt.asNewInstance();
    final sessions = SessionManager();
    di.registerSingleton(sessions);
    di.registerSingleton<SessionStorage>(
      storage ??
          (c.isFixture ? MemorySessionStorage() : const SecureSessionStorage()),
    );
    di.registerSingleton<ApiClient>(
      api ??
          (c.isFixture
              ? await FixtureApiClient.load()
              : AuthenticatedHttpClient(c.apiBaseUrl, sessions)),
    );
    registerAuth(di);
    registerCases(di);
    registerInvestigations(di);
    registerOverview(di);
    final auth = AuthBloc(di<AuthRepository>());
    di.registerSingleton(auth);
    sessions.onExpired = () => auth.add(const AuthSignOut(expired: true));
    SharedPreferences? prefs;
    String version = '1.0.0';
    if (!test) {
      prefs = await SharedPreferences.getInstance();
      final info = await PackageInfo.fromPlatform();
      version = '${info.version} (${info.buildNumber})';
    }
    return AppDependencies._(c, di, AppearanceController(prefs), version);
  }

  bool _disposed = false;
  Future<void> dispose() async {
    if (_disposed) return;
    _disposed = true;
    final manager = sessions;
    manager.onExpired = null;
    manager.dispose();
    get<ApiClient>().close();
    await auth.close();
    await di.reset();
    appearance.dispose();
  }
}
