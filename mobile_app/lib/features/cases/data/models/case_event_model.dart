class CaseEventModel {
  const CaseEventModel({
    required this.id,
    required this.type,
    required this.message,
    required this.actor,
    required this.version,
    required this.occurredAt,
  });
  factory CaseEventModel.fromJson(Map<String, dynamic> j) => CaseEventModel(
    id: j['id'] as String,
    type: j['type'] as String,
    message: j['message'] as String,
    actor: j['actor'] as String,
    version: j['version'] as int,
    occurredAt: DateTime.parse(j['occurred_at'] as String),
  );
  final String id, type, message, actor;
  final int version;
  final DateTime occurredAt;
}
