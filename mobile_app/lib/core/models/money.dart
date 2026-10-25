class Money {
  Money({required this.minorUnits, required this.currency}) {
    if (!RegExp(r'^-?\d+$').hasMatch(minorUnits) || currency != 'ETB') {
      throw const FormatException('Invalid ETB money');
    }
  }
  factory Money.fromJson(Map<String, dynamic> json) => Money(
    minorUnits: json['minor_units'] as String,
    currency: json['currency'] as String,
  );
  final String minorUnits;
  final String currency;
  Map<String, dynamic> toJson() => {
    'minor_units': minorUnits,
    'currency': currency,
  };
}
