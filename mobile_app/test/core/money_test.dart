import 'package:flutter_test/flutter_test.dart';
import 'package:settlegraph_mobile/core/models/money.dart';
import 'package:settlegraph_mobile/core/utils/money_formatter.dart';
import 'package:settlegraph_mobile/core/utils/validators.dart';

void main() {
  for (final entry in {
    '250000': '2,500.00 ETB',
    '1': '0.01 ETB',
    '0': '0.00 ETB',
    '-101': '−1.01 ETB',
    '9223372036854775807': '92,233,720,368,547,758.07 ETB',
  }.entries) {
    test(
      'ETB ${entry.key} stays exact',
      () => expect(
        formatMoney(Money(minorUnits: entry.key, currency: 'ETB')),
        entry.value,
      ),
    );
  }
  test('money JSON roundtrip preserves string', () {
    final value = {'minor_units': '250000', 'currency': 'ETB'};
    expect(Money.fromJson(value).toJson(), value);
    expect(
      () => Money(minorUnits: '2.5', currency: 'ETB'),
      throwsFormatException,
    );
    expect(
      () => Money(minorUnits: '100', currency: 'USD'),
      throwsFormatException,
    );
  });
  test('review reason requires meaningful input within contract length', () {
    expect(validateReason('     '), isNotNull);
    expect(validateReason('abcd'), isNotNull);
    expect(validateReason('Reviewed source evidence.'), isNull);
    expect(validateReason('x' * 2001), isNotNull);
  });
}
