class CaseModel {
  const CaseModel({
    required this.id,
    required this.kind,
    required this.status,
    required this.severity,
    required this.title,
    required this.summary,
    required this.version,
    required this.createdAt,
    required this.updatedAt,
    required this.evidenceCount,
    this.assignedTo,
    this.scenarioRunId,
  });
  factory CaseModel.fromJson(Map<String, dynamic> j) => CaseModel(
    id: j['id'] as String,
    kind: j['kind'] as String,
    status: j['status'] as String,
    severity: j['severity'] as String,
    title: j['title'] as String,
    summary: j['summary'] as String,
    version: j['version'] as int,
    createdAt: DateTime.parse(j['created_at'] as String),
    updatedAt: DateTime.parse(j['updated_at'] as String),
    evidenceCount: j['evidence_count'] as int,
    assignedTo: j['assigned_to'] as String?,
    scenarioRunId: j['scenario_run_id'] as String?,
  );
  final String id, kind, status, severity, title, summary;
  final int version, evidenceCount;
  final DateTime createdAt, updatedAt;
  final String? assignedTo, scenarioRunId;
  bool get supported =>
      ['OPEN', 'EVIDENCE_UPDATED', 'RESOLVED'].contains(status) &&
      [
        'SETTLEMENT_NOT_OBSERVED',
        'AMOUNT_MISMATCH',
        'AMBIGUOUS_MATCH',
        'SOURCE_UNAVAILABLE',
      ].contains(kind);
  bool get resolved => status == 'RESOLVED';
}
