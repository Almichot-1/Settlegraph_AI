sealed class AuthEvent {
  const AuthEvent();
}

class AuthStarted extends AuthEvent {
  const AuthStarted();
}

class AuthSignIn extends AuthEvent {
  const AuthSignIn(this.name, this.code);
  final String name, code;
}

class AuthSignOut extends AuthEvent {
  const AuthSignOut({this.expired = false});
  final bool expired;
}

class AuthOrganizationChanged extends AuthEvent {
  const AuthOrganizationChanged(this.id);
  final String id;
}
