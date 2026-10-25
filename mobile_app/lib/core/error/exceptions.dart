class ContractException implements Exception {
  const ContractException(this.field);
  final String field;
  @override
  String toString() => 'Unexpected response field: $field';
}
