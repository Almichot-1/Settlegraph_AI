import '../models/money.dart';

String formatMoney(Money money, {bool includeCurrency = true}) {
  final n = BigInt.parse(money.minorUnits);
  final absolute = n.abs();
  final whole = (absolute ~/ BigInt.from(100)).toString().replaceAllMapped(
    RegExp(r'\B(?=(\d{3})+(?!\d))'),
    (_) => ',',
  );
  final fraction = (absolute % BigInt.from(100)).toString().padLeft(2, '0');
  return '${n.isNegative ? '−' : ''}$whole.$fraction${includeCurrency ? ' ${money.currency}' : ''}';
}
