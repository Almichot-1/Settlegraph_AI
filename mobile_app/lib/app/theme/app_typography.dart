import 'package:flutter/material.dart';

abstract final class AppTypography {
  static TextTheme get textTheme => const TextTheme(
    headlineLarge: TextStyle(
      fontSize: 34,
      height: 1.15,
      fontWeight: FontWeight.w600,
      letterSpacing: -1.2,
    ),
    headlineMedium: TextStyle(
      fontSize: 28,
      height: 1.2,
      fontWeight: FontWeight.w600,
      letterSpacing: -.8,
    ),
    titleLarge: TextStyle(
      fontSize: 21,
      height: 1.3,
      fontWeight: FontWeight.w600,
      letterSpacing: -.4,
    ),
    titleMedium: TextStyle(
      fontSize: 16,
      height: 1.4,
      fontWeight: FontWeight.w600,
      letterSpacing: -.2,
    ),
    bodyLarge: TextStyle(fontSize: 16, height: 1.55),
    bodyMedium: TextStyle(fontSize: 14, height: 1.5),
    bodySmall: TextStyle(fontSize: 12, height: 1.5),
    labelLarge: TextStyle(
      fontSize: 15,
      height: 1.35,
      fontWeight: FontWeight.w600,
    ),
    labelMedium: TextStyle(
      fontSize: 12,
      height: 1.4,
      fontWeight: FontWeight.w600,
    ),
  );
}
