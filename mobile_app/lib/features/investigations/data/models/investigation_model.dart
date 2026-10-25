class Hypothesis {
  Hypothesis(Map<String, dynamic> j)
    : explanation = j['explanation'] as String,
      evidenceIds = List<String>.from(j['evidence_ids'] as List);
  final String explanation;
  final List<String> evidenceIds;
}

class InvestigationModel {
  InvestigationModel.fromJson(Map<String, dynamic> j)
    : id = j['id'] as String,
      caseId = j['case_id'] as String,
      status = j['status'] as String,
      provider = j['provider'] as String,
      snapshotHash = j['evidence_snapshot_hash'] as String,
      createdAt = DateTime.parse(j['created_at'] as String),
      completedAt = j['completed_at'] == null
          ? null
          : DateTime.parse(j['completed_at'] as String),
      result = j['result'] == null
          ? null
          : InvestigationResult(j['result'] as Map<String, dynamic>);
  final String id, caseId, status, provider, snapshotHash;
  final DateTime createdAt;
  final DateTime? completedAt;
  final InvestigationResult? result;
  bool get pending => ['QUEUED', 'RUNNING'].contains(status);
  bool get completed => status == 'COMPLETED';
  bool get supported =>
      [
        'QUEUED',
        'RUNNING',
        'COMPLETED',
        'FAILED',
        'REJECTED',
      ].contains(status) &&
      ['OFFLINE_STUB', 'BEDROCK'].contains(provider);
  String get sourceLabel => provider == 'OFFLINE_STUB'
      ? 'Demo analysis · offline'
      : provider == 'BEDROCK'
      ? 'Generated analysis'
      : 'Unknown analysis source';
}

class InvestigationResult {
  InvestigationResult(Map<String, dynamic> j)
    : summary = j['summary'] as String,
      hypotheses = (j['hypotheses'] as List)
          .map((e) => Hypothesis(Map<String, dynamic>.from(e as Map)))
          .toList(),
      evidenceIds = List<String>.from(j['evidence_ids'] as List),
      actions = List<String>.from(j['recommended_actions'] as List),
      limitations = List<String>.from(j['limitations'] as List);
  final String summary;
  final List<Hypothesis> hypotheses;
  final List<String> evidenceIds, actions, limitations;
}
