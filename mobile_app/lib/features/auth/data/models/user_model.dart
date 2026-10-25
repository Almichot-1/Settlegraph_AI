class Membership {
  const Membership({required this.id, required this.name, required this.role});
  factory Membership.fromJson(Map<String, dynamic> json) {
    final org = json['organization'] as Map<String, dynamic>;
    return Membership(
      id: org['id'] as String,
      name: org['name'] as String,
      role: json['role'] as String,
    );
  }
  final String id, name, role;
  bool get canReview => ['OWNER', 'REVIEWER'].contains(role);
  bool get canInvestigate => ['OWNER', 'ANALYST'].contains(role);
  bool get canAssign => canInvestigate;
  Map<String, dynamic> toJson() => {
    'organization': {'id': id, 'name': name, 'mode': 'DEMO'},
    'role': role,
  };
}

class UserModel {
  UserModel({
    required this.id,
    required this.displayName,
    required List<Membership> memberships,
  }) : memberships = List.unmodifiable(memberships);
  factory UserModel.fromJson(Map<String, dynamic> json) => UserModel(
    id: json['id'] as String,
    displayName: json['display_name'] as String,
    memberships: (json['memberships'] as List)
        .map((e) => Membership.fromJson(Map<String, dynamic>.from(e as Map)))
        .toList(),
  );
  final String id, displayName;
  final List<Membership> memberships;
  Map<String, dynamic> toJson() => {
    'id': id,
    'display_name': displayName,
    'memberships': memberships.map((m) => m.toJson()).toList(),
  };
}

class AuthSession {
  const AuthSession({
    required this.token,
    required this.expiresAt,
    required this.user,
    required this.organizationId,
  });
  factory AuthSession.fromJson(Map<String, dynamic> json) => AuthSession(
    token: json['access_token'] as String,
    expiresAt: DateTime.parse(json['expires_at'] as String),
    user: UserModel.fromJson(json['user'] as Map<String, dynamic>),
    organizationId: json['default_organization_id'] as String,
  );
  final String token, organizationId;
  final DateTime expiresAt;
  final UserModel user;
  AuthSession copyWith({UserModel? user, String? organizationId}) =>
      AuthSession(
        token: token,
        expiresAt: expiresAt,
        user: user ?? this.user,
        organizationId: organizationId ?? this.organizationId,
      );
  Map<String, dynamic> toJson() => {
    'access_token': token,
    'expires_at': expiresAt.toUtc().toIso8601String(),
    'user': user.toJson(),
    'default_organization_id': organizationId,
  };
}
