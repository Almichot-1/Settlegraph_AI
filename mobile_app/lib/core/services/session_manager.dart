import 'dart:async';
import 'package:flutter/foundation.dart';
import '../../features/auth/data/models/user_model.dart';

class SessionManager extends ChangeNotifier {
  AuthSession? _session;
  Timer? _expiry;
  int generation = 0;
  bool initialized = false;
  bool authenticated = false;
  VoidCallback? onExpired;
  AuthSession? get session => _session;
  Membership? get membership {
    final s = _session;
    if (s == null) return null;
    for (final m in s.user.memberships) {
      if (m.id == s.organizationId) return m;
    }
    return null;
  }

  void update(AuthSession? value, {bool validated = true}) {
    authenticated = value != null && validated;
    _expiry?.cancel();
    _session = value;
    generation++;
    initialized = true;
    if (value != null) {
      final remaining = value.expiresAt.difference(DateTime.now());
      _expiry = Timer(remaining.isNegative ? Duration.zero : remaining, expire);
    }
    notifyListeners();
  }

  void expire() {
    if (_session == null) return;
    update(null);
    onExpired?.call();
  }

  @override
  void dispose() {
    _expiry?.cancel();
    super.dispose();
  }
}
