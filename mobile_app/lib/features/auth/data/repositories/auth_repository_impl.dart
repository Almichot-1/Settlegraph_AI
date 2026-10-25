import '../../../../core/error/failures.dart';
import '../../../../core/services/session_manager.dart';
import '../datasources/auth_local_datasource.dart';
import '../datasources/auth_remote_datasource.dart';
import '../models/user_model.dart';
import 'auth_repository.dart';

class AuthRepositoryImpl implements AuthRepository {
  const AuthRepositoryImpl(this.remote, this.local, this.sessions);
  final AuthRemoteDatasource remote;
  final AuthLocalDatasource local;
  final SessionManager sessions;
  @override
  Future<AuthSession?> restore() async {
    AuthSession? saved;
    try {
      saved = await local.restore();
    } catch (_) {
      sessions.update(null);
      throw const Failure(
        'Secure session storage is unavailable. Please try signing in again.',
        code: 'STORAGE_UNAVAILABLE',
      );
    }
    if (saved == null || !saved.expiresAt.isAfter(DateTime.now())) {
      await signOut();
      return null;
    }
    final restored = saved;
    sessions.update(restored, validated: false);
    try {
      final user = await remote.me();
      if (user.memberships.isEmpty) {
        throw const Failure(
          'You have no active workspace membership.',
          status: 403,
        );
      }
      final org = user.memberships.any((m) => m.id == restored.organizationId)
          ? restored.organizationId
          : user.memberships.first.id;
      final fresh = restored.copyWith(user: user, organizationId: org);
      await local.save(fresh);
      sessions.update(fresh);
      return fresh;
    } catch (_) {
      sessions.update(null);
      rethrow;
    }
  }

  @override
  Future<AuthSession> signIn(String name, String code) async {
    var session = await remote.signIn(name, code);
    sessions.update(session, validated: false);
    try {
      final user = await remote.me();
      if (user.memberships.isEmpty) {
        throw const Failure(
          'You have no active workspace membership.',
          status: 403,
        );
      }
      final org = user.memberships.any((m) => m.id == session.organizationId)
          ? session.organizationId
          : user.memberships.first.id;
      session = session.copyWith(user: user, organizationId: org);
      await local.save(session);
      sessions.update(session);
      return session;
    } catch (_) {
      sessions.update(null);
      rethrow;
    }
  }

  @override
  Future<void> selectOrganization(String id) async {
    final s = sessions.session;
    if (s == null || !s.user.memberships.any((m) => m.id == id)) {
      throw const Failure('Workspace unavailable.');
    }
    // Clear in-flight requests and the previous organization immediately.
    final selected = s.copyWith(organizationId: id);
    sessions.update(selected);
    await local.save(selected);
  }

  @override
  Future<void> signOut() async {
    sessions.update(null);
    await local.clear();
  }
}
