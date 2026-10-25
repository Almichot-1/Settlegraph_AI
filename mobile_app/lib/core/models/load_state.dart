import '../error/failures.dart';

class LoadState<T> {
  const LoadState({this.value, this.loading = false, this.failure});
  final T? value;
  final bool loading;
  final Failure? failure;
}
