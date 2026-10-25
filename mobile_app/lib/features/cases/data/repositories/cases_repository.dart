import '../../../../core/models/paginated_response.dart';
import '../models/case_model.dart';
import '../models/evidence_model.dart';
import '../models/case_event_model.dart';

class CaseBundle {
  const CaseBundle(this.caseRecord, this.evidence, this.events);
  final CaseModel caseRecord;
  final List<EvidenceModel> evidence;
  final List<CaseEventModel> events;
}

class DecisionAttempt {
  const DecisionAttempt({
    required this.caseId,
    required this.decision,
    required this.reason,
    required this.version,
    required this.key,
  });
  final String caseId, decision, reason, key;
  final int version;
  Map<String, dynamic> get body => {'decision': decision, 'reason': reason};
}

abstract interface class CasesRepository {
  Future<PageResult<CaseModel>> list({
    String? cursor,
    String? assignedTo,
    String? status,
  });
  Future<CaseBundle> detail(String id);
  DecisionAttempt? pending(String id);
  Future<CaseModel> decide(DecisionAttempt attempt);
  Future<CaseModel> assignToMe(CaseModel record);
}
