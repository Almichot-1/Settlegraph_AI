class PageResult<T> {
  const PageResult(this.items, this.nextCursor);
  factory PageResult.fromJson(
    Map<String, dynamic> json,
    T Function(Map<String, dynamic>) decode,
  ) => PageResult(
    List.unmodifiable(
      (json['items'] as List).map(
        (e) => decode(Map<String, dynamic>.from(e as Map)),
      ),
    ),
    json['next_cursor'] as String?,
  );
  final List<T> items;
  final String? nextCursor;
}
