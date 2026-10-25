import '../../../../core/error/failures.dart';
import '../../../../core/models/paginated_response.dart';
import '../datasources/investigations_remote_datasource.dart';
import '../models/investigation_model.dart';
import 'investigations_repository.dart';

class InvestigationsRepositoryImpl implements InvestigationsRepository {
  const InvestigationsRepositoryImpl(this.remote);
  final InvestigationsRemoteDatasource remote;
  @override
  Future<List<InvestigationModel>> list(String caseId) async {
    final result = <InvestigationModel>[];
    final seen = <String>{};
    String? cursor;
    do {
      final page = PageResult.fromJson(
        await remote.list(caseId, cursor: cursor),
        InvestigationModel.fromJson,
      );
      result.addAll(page.items);
      cursor = page.nextCursor;
      if (cursor != null && !seen.add(cursor)) {
        throw const Failure('Repeated investigation page.');
      }
    } while (cursor != null);
    result.sort((a, b) {
      final c = b.createdAt.compareTo(a.createdAt);
      return c == 0 ? b.id.compareTo(a.id) : c;
    });
    return result;
  }

  @override
  Future<InvestigationModel> get(String caseId, String id) async =>
      InvestigationModel.fromJson(await remote.get(caseId, id));
  @override
  Future<InvestigationModel> create(String caseId, String key) async =>
      InvestigationModel.fromJson(await remote.create(caseId, key));
}
