String caseStatusLabel(String value) => switch (value) {
  'OPEN' => 'Open',
  'EVIDENCE_UPDATED' => 'New evidence',
  'RESOLVED' => 'Resolved',
  _ => 'Unsupported status',
};
String kindLabel(String value) => switch (value) {
  'SETTLEMENT_NOT_OBSERVED' => 'Settlement pending',
  'AMOUNT_MISMATCH' => 'Amount difference',
  'AMBIGUOUS_MATCH' => 'Reference needs review',
  'SOURCE_UNAVAILABLE' => 'Source unavailable',
  _ => 'Unknown case type',
};
