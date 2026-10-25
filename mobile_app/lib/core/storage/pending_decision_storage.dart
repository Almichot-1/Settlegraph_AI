import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Envelopes contain synthetic review data, never access tokens or access codes.
/// Each key is scoped to API environment, organization and authenticated user.
abstract interface class PendingDecisionStorage {
  Future<String?> read(String scope);
  Future<void> write(String scope, String value);
}

class SecurePendingDecisionStorage implements PendingDecisionStorage {
  const SecurePendingDecisionStorage({required this.apiBaseUrl});
  final String apiBaseUrl;
  static const _storage = FlutterSecureStorage();
  String _key(String scope) =>
      'settlegraph.pending.v1.${Uri.encodeComponent(apiBaseUrl)}.$scope';
  @override
  Future<String?> read(String scope) => _storage.read(key: _key(scope));
  @override
  Future<void> write(String scope, String value) =>
      _storage.write(key: _key(scope), value: value);
}

class MemoryPendingDecisionStorage implements PendingDecisionStorage {
  final Map<String, String> values = {};
  @override
  Future<String?> read(String scope) async => values[scope];
  @override
  Future<void> write(String scope, String value) async {
    values[scope] = value;
  }
}
