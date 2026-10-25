import '../models/investigation_model.dart';

abstract interface class InvestigationsRepository {
  Future<List<InvestigationModel>> list(String caseId);
  Future<InvestigationModel> get(String caseId, String id);
  Future<InvestigationModel> create(String caseId, String key);
}
