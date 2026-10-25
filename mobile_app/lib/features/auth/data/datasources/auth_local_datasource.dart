import 'dart:convert';
import '../../../../core/storage/secure_storage.dart';
import '../models/user_model.dart';

class AuthLocalDatasource {
  const AuthLocalDatasource(this.storage);
  final SessionStorage storage;
  Future<AuthSession?> restore() async {
    final value = await storage.read();
    if (value == null) return null;
    try {
      return AuthSession.fromJson(jsonDecode(value) as Map<String, dynamic>);
    } on FormatException {
      await storage.clear();
      return null;
    } on TypeError {
      await storage.clear();
      return null;
    }
  }

  Future<void> save(AuthSession session) =>
      storage.write(jsonEncode(session.toJson()));
  Future<void> clear() => storage.clear();
}
