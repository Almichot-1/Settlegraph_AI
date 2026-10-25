import '../models/user_model.dart';

abstract interface class AuthRepository {
  Future<AuthSession?> restore();
  Future<AuthSession> signIn(String name, String code);
  Future<void> selectOrganization(String id);
  Future<void> signOut();
}
