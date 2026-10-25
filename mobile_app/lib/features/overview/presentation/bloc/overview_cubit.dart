import 'package:flutter_bloc/flutter_bloc.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/models/load_state.dart';
import '../../data/models/dashboard_model.dart';
import '../../data/repositories/overview_repository.dart';

class OverviewCubit extends Cubit<LoadState<DashboardModel>> {
  OverviewCubit(this.repository) : super(const LoadState());
  final OverviewRepository repository;
  Future<void> refresh() async {
    if (state.loading) return;
    emit(LoadState(value: state.value, loading: true));
    try {
      final value = await repository.summary();
      if (!isClosed) emit(LoadState(value: value));
    } catch (e) {
      if (!isClosed) {
        emit(LoadState(value: state.value, failure: Failure.from(e)));
      }
    }
  }
}
