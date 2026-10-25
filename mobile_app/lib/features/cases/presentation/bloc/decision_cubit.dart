import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:uuid/uuid.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/utils/validators.dart';
import '../../data/models/case_model.dart';
import '../../data/repositories/cases_repository.dart';

class DecisionState {
  const DecisionState({
    required this.record,
    this.busy = false,
    this.failure,
    this.conflict,
    this.completed = false,
    this.uncertain = false,
  });
  final CaseModel record;
  final bool busy, completed, uncertain;
  final Failure? failure;
  final CaseBundle? conflict;
}

class DecisionCubit extends Cubit<DecisionState> {
  DecisionCubit(this.repository, CaseModel record)
    : super(
        DecisionState(
          record: record,
          uncertain: repository.pending(record.id) != null,
        ),
      );
  final CasesRepository repository;
  DecisionAttempt? get pending => repository.pending(state.record.id);
  Future<void> submit(
    String decision,
    String reason, {
    required bool canReview,
  }) async {
    if (state.busy || state.completed || state.conflict != null) return;
    if (!canReview || !state.record.supported) {
      emit(
        DecisionState(
          record: state.record,
          failure: const Failure('You cannot review this case.', status: 403),
        ),
      );
      return;
    }
    final existing = pending;
    final validation = validateReason(existing?.reason ?? reason);
    if (validation != null) {
      emit(DecisionState(record: state.record, failure: Failure(validation)));
      return;
    }
    final attempt =
        existing ??
        DecisionAttempt(
          caseId: state.record.id,
          decision: decision,
          reason: reason.trim(),
          version: state.record.version,
          key: const Uuid().v4(),
        );
    emit(
      DecisionState(
        record: state.record,
        busy: true,
        uncertain: existing != null,
      ),
    );
    try {
      final record = await repository.decide(attempt);
      if (!isClosed) emit(DecisionState(record: record, completed: true));
    } catch (error) {
      final failure = Failure.from(error);
      CaseBundle? updated;
      if (failure.isConflict) {
        try {
          updated = await repository.detail(state.record.id);
        } catch (_) {
          /* Keep the conflict until a refresh succeeds. */
        }
      }
      if (!isClosed) {
        emit(
          DecisionState(
            record: state.record,
            failure: failure,
            conflict: updated,
            uncertain: failure.uncertain,
          ),
        );
      }
    }
  }

  Future<void> refreshConflict() async {
    if (state.busy) return;
    emit(
      DecisionState(record: state.record, busy: true, failure: state.failure),
    );
    try {
      final bundle = await repository.detail(state.record.id);
      if (!isClosed) {
        emit(
          DecisionState(
            record: state.record,
            failure: const Failure('This case changed.', status: 412),
            conflict: bundle,
          ),
        );
      }
    } catch (e) {
      if (!isClosed) {
        emit(DecisionState(record: state.record, failure: Failure.from(e)));
      }
    }
  }

  void acknowledgeConflict() {
    final bundle = state.conflict;
    if (bundle != null) emit(DecisionState(record: bundle.caseRecord));
  }
}
