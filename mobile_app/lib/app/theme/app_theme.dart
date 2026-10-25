import 'package:flutter/material.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';
import 'app_colors.dart';
import 'app_typography.dart';

abstract final class AppTheme {
  static ThemeData get light => _build(AppColors.light, Brightness.light);
  static ThemeData get dark => _build(AppColors.dark, Brightness.dark);
  static ThemeData _build(AppColors c, Brightness brightness) {
    final scheme =
        ColorScheme.fromSeed(
          seedColor: c.brand,
          brightness: brightness,
        ).copyWith(
          primary: c.brand,
          onPrimary: c.onBrand,
          surface: c.canvas,
          onSurface: c.ink,
          onSurfaceVariant: c.muted,
          outline: c.muted,
          outlineVariant: c.line,
          error: c.negative,
          onError: c.card,
        );
    return ThemeData(
      useMaterial3: true,
      fontFamily: 'Inter',
      colorScheme: scheme,
      scaffoldBackgroundColor: c.canvas,
      extensions: [c],
      actionIconTheme: ActionIconThemeData(
        backButtonIconBuilder: (_) =>
            const Icon(LucideIcons.arrowLeft, size: 22),
      ),
      textTheme: AppTypography.textTheme.apply(
        bodyColor: c.ink,
        displayColor: c.ink,
      ),
      appBarTheme: AppBarTheme(
        backgroundColor: c.canvas,
        foregroundColor: c.ink,
        elevation: 0,
        scrolledUnderElevation: 0,
        centerTitle: false,
      ),
      dividerTheme: DividerThemeData(color: c.line, thickness: 1, space: 1),
      navigationBarTheme: NavigationBarThemeData(
        backgroundColor: c.card,
        indicatorColor: c.soft,
        height: 72,
        elevation: 0,
        labelTextStyle: WidgetStateProperty.resolveWith(
          (s) => TextStyle(
            fontFamily: 'Inter',
            fontSize: 12,
            color: s.contains(WidgetState.selected) ? c.ink : c.muted,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          minimumSize: const Size(48, 52),
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
          ),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          minimumSize: const Size(48, 52),
          side: BorderSide(color: c.muted),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
          ),
        ),
      ),
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(minimumSize: const Size(48, 48)),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: c.card,
        contentPadding: const EdgeInsets.all(16),
        border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: c.muted),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: c.brand, width: 2),
        ),
      ),
      chipTheme: ChipThemeData(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
        side: BorderSide(color: c.line),
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 9),
      ),
      bottomSheetTheme: BottomSheetThemeData(
        backgroundColor: c.canvas,
        showDragHandle: true,
      ),
      snackBarTheme: SnackBarThemeData(
        behavior: SnackBarBehavior.floating,
        backgroundColor: c.ink,
        contentTextStyle: TextStyle(color: c.canvas, fontFamily: 'Inter'),
      ),
    );
  }
}
