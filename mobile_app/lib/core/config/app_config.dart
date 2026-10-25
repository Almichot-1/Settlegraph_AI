import 'package:flutter/foundation.dart';

class AppConfig {
  const AppConfig({
    this.environment = 'fixture',
    this.apiBaseUrl = 'http://127.0.0.1:8080',
  });

  factory AppConfig.fromEnvironment() {
    const environment = String.fromEnvironment(
      'APP_ENV',
      defaultValue: 'fixture',
    );
    const configuredUrl = String.fromEnvironment('API_BASE_URL');
    if (!['fixture', 'demo'].contains(environment)) {
      throw StateError('APP_ENV must be fixture or demo.');
    }
    final url = configuredUrl.isNotEmpty
        ? configuredUrl
        : defaultTargetPlatform == TargetPlatform.android
        ? 'http://10.0.2.2:8080'
        : 'http://127.0.0.1:8080';
    final uri = Uri.parse(url);
    if (!uri.hasAuthority || !['http', 'https'].contains(uri.scheme)) {
      throw StateError('API_BASE_URL must be an HTTP(S) origin.');
    }
    if (kReleaseMode && environment == 'demo' && uri.scheme != 'https') {
      throw StateError('Release demo connections require HTTPS.');
    }
    return AppConfig(environment: environment, apiBaseUrl: url);
  }

  final String environment;
  final String apiBaseUrl;
  bool get isFixture => environment == 'fixture';
  String get environmentLabel =>
      isFixture ? 'Sample workspace' : 'Simulated environment';
}
