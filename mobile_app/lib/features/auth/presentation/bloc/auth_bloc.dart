import 'package:flutter_bloc/flutter_bloc.dart';
import '../../../../core/error/failures.dart';
import '../../data/repositories/auth_repository.dart';
import 'auth_event.dart';
import 'auth_state.dart';

class AuthBloc extends Bloc<AuthEvent, AuthState> {
  AuthBloc(this.repository) : super(const AuthState(busy: true)) {
    on<AuthStarted>((event, emit) async {
      try {
        await repository.restore();
        emit(const AuthState());
      } catch (e) {
        emit(AuthState(message: Failure.from(e).message));
      }
    });
    on<AuthSignIn>((event, emit) async {
      if (state.busy) return;
      emit(const AuthState(busy: true));
      try {
        await repository.signIn(event.name, event.code);
        emit(const AuthState());
      } catch (e) {
        emit(AuthState(message: Failure.from(e).message));
      }
    });
    on<AuthSignOut>((event, emit) async {
      try {
        await repository.signOut();
      } finally {
        emit(
          AuthState(
            message: event.expired
                ? 'Your session ended. Please sign in again.'
                : null,
          ),
        );
      }
    });
    on<AuthOrganizationChanged>((event, emit) async {
      try {
        await repository.selectOrganization(event.id);
        emit(const AuthState());
      } catch (e) {
        emit(AuthState(message: Failure.from(e).message));
      }
    });
  }
  final AuthRepository repository;
}
