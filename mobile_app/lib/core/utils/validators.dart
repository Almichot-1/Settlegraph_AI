String? validateReason(String? input) {
  final value = input?.trim() ?? '';
  if (value.length < 5) return 'Add a reason with at least 5 characters.';
  if (value.length > 2000) return 'Keep your reason under 2,000 characters.';
  return null;
}
