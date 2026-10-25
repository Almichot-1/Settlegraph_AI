import 'package:flutter_bloc/flutter_bloc.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/services/session_manager.dart';
import '../../data/repositories/cases_repository.dart';
import '../../data/models/case_model.dart';
import 'cases_event.dart';
import 'cases_state.dart';

class CasesBloc extends Bloc<CasesEvent, CasesState> {
  CasesBloc(this.repository, this.sessions) : super(const CasesState()) {
    on<CasesRequested>((event, emit) => _load(emit));
    on<CasesFilterChanged>((event, emit) async {
      _request++;
      emit(CasesState(mine: event.mine, status: event.status));
      await _load(emit);
    });
    on<CasesNextPage>((event, emit) => _load(emit, more: true));
  }
  final CasesRepository repository;
  final SessionManager sessions;
  int _request = 0;
  Future<void> _load(Emitter<CasesState> emit, {bool more = false}) async {
    final previous = state;
    if (more &&
        (previous.cursor == null || previous.loading || previous.loadingMore)) {
      return;
    }
    final ticket = ++_request;
    emit(
      CasesState(
        items: previous.items,
        mine: previous.mine,
        status: previous.status,
        cursor: previous.cursor,
        loading: !more,
        loadingMore: more,
      ),
    );
    try {
      final page = await repository.list(
        cursor: more ? previous.cursor : null,
        assignedTo: previous.mine ? sessions.session?.user.id : null,
        status: previous.status,
      );
      if (emit.isDone || ticket != _request) return;
      final unique = {
        for (final item in [
          ...(more ? previous.items : <CaseModel>[]),
          ...page.items,
        ])
          item.id: item,
      };
      emit(
        CasesState(
          items: List.unmodifiable(unique.values),
          mine: previous.mine,
          status: previous.status,
          cursor: page.nextCursor,
        ),
      );
    } catch (e) {
      if (!emit.isDone && ticket == _request) {
        emit(
          CasesState(
            items: previous.items,
            mine: previous.mine,
            status: previous.status,
            cursor: previous.cursor,
            failure: Failure.from(e),
          ),
        );
      }
    }
  }
}
