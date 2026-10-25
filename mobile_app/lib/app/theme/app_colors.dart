import 'package:flutter/material.dart';

class AppColors extends ThemeExtension<AppColors> {
  const AppColors({
    required this.canvas,
    required this.card,
    required this.ink,
    required this.muted,
    required this.line,
    required this.brand,
    required this.onBrand,
    required this.soft,
    required this.positive,
    required this.positiveBg,
    required this.attention,
    required this.attentionBg,
    required this.negative,
    required this.negativeBg,
  });
  final Color canvas,
      card,
      ink,
      muted,
      line,
      brand,
      onBrand,
      soft,
      positive,
      positiveBg,
      attention,
      attentionBg,
      negative,
      negativeBg;
  static const light = AppColors(
    canvas: Color(0xFFF6F7F3),
    card: Color(0xFFFFFFFF),
    ink: Color(0xFF1B2723),
    muted: Color(0xFF626E68),
    line: Color(0xFFDDE3DD),
    brand: Color(0xFF183F36),
    onBrand: Colors.white,
    soft: Color(0xFFEAF0E9),
    positive: Color(0xFF25604F),
    positiveBg: Color(0xFFE9F2EC),
    attention: Color(0xFF936100),
    attentionBg: Color(0xFFFFF4DB),
    negative: Color(0xFFB54736),
    negativeBg: Color(0xFFFFF0EB),
  );
  static const dark = AppColors(
    canvas: Color(0xFF111B17),
    card: Color(0xFF1A2721),
    ink: Color(0xFFF0F3EC),
    muted: Color(0xFFACB9AF),
    line: Color(0xFF35483C),
    brand: Color(0xFFC2DAC8),
    onBrand: Color(0xFF183F36),
    soft: Color(0xFF263C30),
    positive: Color(0xFFB5D7C4),
    positiveBg: Color(0xFF213D2F),
    attention: Color(0xFFF0CE85),
    attentionBg: Color(0xFF3C321F),
    negative: Color(0xFFFFB9A9),
    negativeBg: Color(0xFF432A25),
  );
  static AppColors of(BuildContext context) =>
      Theme.of(context).extension<AppColors>()!;
  @override
  AppColors copyWith() => this;
  @override
  AppColors lerp(covariant AppColors? other, double t) =>
      other == null || t < .5 ? this : other;
}
