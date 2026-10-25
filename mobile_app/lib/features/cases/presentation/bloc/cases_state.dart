import '../../../../core/error/failures.dart';
import '../../data/models/case_model.dart';

class CasesState {
  const CasesState({
    this.items = const [],
    this.loading = false,
    this.loadingMore = false,
    this.mine = true,
    this.status,
    this.cursor,
    this.failure,
  });
  final List<CaseModel> items;
  final bool loading, loadingMore, mine;
  final String? status, cursor;
  final Failure? failure;
}
