import 'package:flutter_test/flutter_test.dart';
import 'package:settlegraph_mobile/app/injection_container.dart';
import 'package:settlegraph_mobile/core/config/app_config.dart';
import 'package:settlegraph_mobile/core/storage/secure_storage.dart';
import 'package:settlegraph_mobile/features/auth/data/repositories/auth_repository.dart';
import 'package:settlegraph_mobile/features/auth/data/models/user_model.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test(
    'session restores through me and sign out removes credentials',
    () async {
      final storage = MemorySessionStorage();
      final first = await AppDependencies.create(
        config: const AppConfig(),
        storage: storage,
        test: true,
      );
      await first.get<AuthRepository>().signIn('Sam', '');
      expect(first.sessions.authenticated, isTrue);
      await first.dispose();
      final restored = await AppDependencies.create(
        config: const AppConfig(),
        storage: storage,
        test: true,
      );
      final session = await restored.get<AuthRepository>().restore();
      expect(session, isNotNull);
      expect(restored.sessions.authenticated, isTrue);
      await restored.get<AuthRepository>().signOut();
      expect(await storage.read(), isNull);
      expect(restored.sessions.session, isNull);
      await restored.dispose();
    },
  );
  test(
    'corrupt stored data returns to sign in instead of exposing cached identity',
    () async {
      final storage = MemorySessionStorage()..value = 'not-json';
      final d = await AppDependencies.create(
        config: const AppConfig(),
        storage: storage,
        test: true,
      );
      expect(await d.get<AuthRepository>().restore(), isNull);
      expect(d.sessions.authenticated, isFalse);
      expect(d.sessions.initialized, isTrue);
      await d.dispose();
    },
  );
  test('unknown and viewer roles never enable mutations', () {
    for (final role in ['VIEWER', 'UNKNOWN']) {
      final m = Membership(id: 'org', name: 'Organization', role: role);
      expect(m.canReview, isFalse);
      expect(m.canAssign, isFalse);
      expect(m.canInvestigate, isFalse);
    }
    expect(
      const Membership(
        id: 'org',
        name: 'Organization',
        role: 'REVIEWER',
      ).canReview,
      isTrue,
    );
  });
}
