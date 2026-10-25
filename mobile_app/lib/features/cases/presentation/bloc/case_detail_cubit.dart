import 'package:flutter_bloc/flutter_bloc.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/models/load_state.dart';
import '../../data/repositories/cases_repository.dart';

class CaseDetailCubit extends Cubit<LoadState<CaseBundle>> {
  CaseDetailCubit(this.repository, this.id) : super(const LoadState());
  final CasesRepository repository;
  final String id;
  int _request = 0;
  Future<void> refresh() async {
    final ticket = ++_request;
    emit(LoadState(value: state.value, loading: true));
    try {
      final value = await repository.detail(id);
      if (!isClosed && ticket == _request) emit(LoadState(value: value));
    } catch (e) {
      if (!isClosed && ticket == _request) {
        emit(LoadState(value: state.value, failure: Failure.from(e)));
      }
    }
  }

  Future<void> assign() async {
    final record = state.value?.caseRecord;
    if (record == null || state.loading) return;
    emit(LoadState(value: state.value, loading: true));
    try {
      await repository.assignToMe(record);
      await refresh();
    } catch (e) {
      if (!isClosed) {
        emit(LoadState(value: state.value, failure: Failure.from(e)));
      }
    }
  }
}
