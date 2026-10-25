import 'dart:async';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:uuid/uuid.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/models/load_state.dart';
import '../../data/models/investigation_model.dart';
import '../../data/repositories/investigations_repository.dart';

class InvestigationsCubit extends Cubit<LoadState<List<InvestigationModel>>> {
  InvestigationsCubit(this.repository, this.caseId) : super(const LoadState());
  final InvestigationsRepository repository;
  final String caseId;
  Timer? _timer;
  int _polls = 0;
  bool _active = true;
  String? _createKey;
  Future<void> refresh() async {
    _timer?.cancel();
    if (state.loading || !_active) return;
    emit(LoadState(value: state.value, loading: true));
    try {
      final results = await repository.list(caseId);
      if (isClosed) return;
      emit(LoadState(value: results));
      if (_active && results.any((i) => i.pending) && _polls++ < 60) {
        _timer = Timer(Duration(seconds: _polls < 10 ? 2 : 5), refresh);
      }
    } catch (e) {
      if (!isClosed) {
        emit(LoadState(value: state.value, failure: Failure.from(e)));
      }
    }
  }

  Future<void> create({required bool permitted}) async {
    if (state.loading || !permitted) return;
    _timer?.cancel();
    _createKey ??= const Uuid().v4();
    emit(LoadState(value: state.value, loading: true));
    try {
      await repository.create(caseId, _createKey!);
      _createKey = null;
      if (!isClosed) {
        emit(LoadState(value: state.value));
        _polls = 0;
        await refresh();
      }
    } catch (e) {
      final f = Failure.from(e);
      if (!f.uncertain) _createKey = null;
      if (!isClosed) emit(LoadState(value: state.value, failure: f));
    }
  }

  void setActive(bool active) {
    _active = active;
    _timer?.cancel();
    if (active) {
      _polls = 0;
      unawaited(refresh());
    }
  }

  @override
  Future<void> close() {
    _timer?.cancel();
    return super.close();
  }
}
