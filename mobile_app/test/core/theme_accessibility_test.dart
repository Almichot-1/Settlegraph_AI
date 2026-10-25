import 'package:flutter_test/flutter_test.dart';
import 'package:settlegraph_mobile/app/theme/app_colors.dart';

void main() {
  for (final entry in {
    'light': AppColors.light,
    'dark': AppColors.dark,
  }.entries) {
    test('${entry.key} semantic text pairs exceed 4.5:1', () {
      final c = entry.value;
      for (final pair in [
        (c.ink, c.canvas),
        (c.ink, c.card),
        (c.muted, c.canvas),
        (c.muted, c.card),
        (c.onBrand, c.brand),
        (c.attention, c.attentionBg),
        (c.negative, c.negativeBg),
        (c.positive, c.positiveBg),
      ]) {
        final l = pair.$1.computeLuminance(), r = pair.$2.computeLuminance();
        final contrast =
            (l > r ? l + .05 : r + .05) / (l > r ? r + .05 : l + .05);
        expect(
          contrast,
          greaterThanOrEqualTo(4.5),
          reason: '${pair.$1} on ${pair.$2}',
        );
      }
    });
  }
}
