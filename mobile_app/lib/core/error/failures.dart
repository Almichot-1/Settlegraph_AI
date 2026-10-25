class Failure implements Exception {
  const Failure(
    this.message, {
    this.status,
    this.code = 'UNKNOWN',
    this.requestId,
    this.uncertain = false,
  });
  final String message;
  final int? status;
  final String code;
  final String? requestId;
  final bool uncertain;
  bool get isConflict => status == 412;
  bool get isCancelled => code == 'CANCELLED';
  @override
  String toString() => message;

  static Failure from(Object error) => error is Failure
      ? error
      : const Failure(
          'We could not read this response. Please try again.',
          code: 'INVALID_RESPONSE',
        );
}
