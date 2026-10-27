import 'package:flutter/material.dart';

import 'state.dart';

@immutable
class SpaceColors extends ThemeExtension<SpaceColors> {
  const SpaceColors({
    required this.background,
    required this.paper,
    required this.ink,
    required this.muted,
    required this.accent,
    required this.soft,
    required this.secondary,
    required this.heroStart,
    required this.heroEnd,
    required this.line,
  });
  final Color background,
      paper,
      ink,
      muted,
      accent,
      soft,
      secondary,
      heroStart,
      heroEnd,
      line;
  static SpaceColors forTheme(CoupleTheme theme) => switch (theme) {
    CoupleTheme.sakura => const SpaceColors(
      background: Color(0xFFFCF8F7),
      paper: Color(0xFFFFFFFF),
      ink: Color(0xFF3D303B),
      muted: Color(0xFF968791),
      accent: Color(0xFFC96288),
      soft: Color(0xFFF9E8EF),
      secondary: Color(0xFF9181C2),
      heroStart: Color(0xFFF6DFE6),
      heroEnd: Color(0xFFEAE4F6),
      line: Color(0xFFF0E7EB),
    ),
    CoupleTheme.lavender => const SpaceColors(
      background: Color(0xFFF7F5FC),
      paper: Color(0xFFFFFFFF),
      ink: Color(0xFF383047),
      muted: Color(0xFF8B829A),
      accent: Color(0xFF8770BF),
      soft: Color(0xFFEEE8FA),
      secondary: Color(0xFFC0719B),
      heroStart: Color(0xFFE9DFF8),
      heroEnd: Color(0xFFF7E5EF),
      line: Color(0xFFECE6F4),
    ),
    CoupleTheme.midnight => const SpaceColors(
      background: Color(0xFF080B15),
      paper: Color(0xFF151925),
      ink: Color(0xFFF9F1F7),
      muted: Color(0xFFA59BAF),
      accent: Color(0xFFE58BB6),
      soft: Color(0xFF322237),
      secondary: Color(0xFFA89AF5),
      heroStart: Color(0xFF39263E),
      heroEnd: Color(0xFF25233F),
      line: Color(0xFF2A2B3D),
    ),
  };
  @override
  SpaceColors copyWith({
    Color? background,
    Color? paper,
    Color? ink,
    Color? muted,
    Color? accent,
    Color? soft,
    Color? secondary,
    Color? heroStart,
    Color? heroEnd,
    Color? line,
  }) => SpaceColors(
    background: background ?? this.background,
    paper: paper ?? this.paper,
    ink: ink ?? this.ink,
    muted: muted ?? this.muted,
    accent: accent ?? this.accent,
    soft: soft ?? this.soft,
    secondary: secondary ?? this.secondary,
    heroStart: heroStart ?? this.heroStart,
    heroEnd: heroEnd ?? this.heroEnd,
    line: line ?? this.line,
  );
  @override
  SpaceColors lerp(covariant SpaceColors? other, double t) {
    if (other == null) return this;
    return SpaceColors(
      background: Color.lerp(background, other.background, t)!,
      paper: Color.lerp(paper, other.paper, t)!,
      ink: Color.lerp(ink, other.ink, t)!,
      muted: Color.lerp(muted, other.muted, t)!,
      accent: Color.lerp(accent, other.accent, t)!,
      soft: Color.lerp(soft, other.soft, t)!,
      secondary: Color.lerp(secondary, other.secondary, t)!,
      heroStart: Color.lerp(heroStart, other.heroStart, t)!,
      heroEnd: Color.lerp(heroEnd, other.heroEnd, t)!,
      line: Color.lerp(line, other.line, t)!,
    );
  }
}

extension SpaceContext on BuildContext {
  SpaceColors get colors => Theme.of(this).extension<SpaceColors>()!;
  bool get reducedMotion => MediaQuery.disableAnimationsOf(this);
}

ThemeData buildTheme(CoupleTheme selection) {
  final c = SpaceColors.forTheme(selection);
  final brightness = selection == CoupleTheme.midnight
      ? Brightness.dark
      : Brightness.light;
  return ThemeData(
    useMaterial3: true,
    brightness: brightness,
    scaffoldBackgroundColor: c.background,
    colorScheme: ColorScheme.fromSeed(
      seedColor: c.accent,
      brightness: brightness,
      primary: c.accent,
      surface: c.paper,
      onSurface: c.ink,
    ),
    fontFamily: 'NotoSansSC',
    fontFamilyFallback: const ['Microsoft YaHei', 'PingFang SC', 'sans-serif'],
    extensions: [c],
    textTheme: TextTheme(
      bodyMedium: TextStyle(fontSize: 14, height: 1.55, color: c.ink),
      bodySmall: TextStyle(fontSize: 12, height: 1.5, color: c.muted),
      titleLarge: TextStyle(
        fontSize: 24,
        fontWeight: FontWeight.w700,
        color: c.ink,
        height: 1.35,
      ),
      titleMedium: TextStyle(
        fontSize: 16,
        fontWeight: FontWeight.w600,
        color: c.ink,
      ),
    ),
    dividerColor: c.line,
    iconButtonTheme: IconButtonThemeData(
      style: IconButton.styleFrom(minimumSize: const Size(48, 48)),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        minimumSize: const Size(48, 48),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        minimumSize: const Size(48, 48),
        side: BorderSide(color: c.line),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: c.soft.withValues(alpha: .6),
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16),
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(16),
        borderSide: BorderSide.none,
      ),
    ),
    snackBarTheme: SnackBarThemeData(
      behavior: SnackBarBehavior.floating,
      backgroundColor: c.ink,
      contentTextStyle: TextStyle(color: c.background),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
    ),
    bottomSheetTheme: BottomSheetThemeData(
      backgroundColor: c.paper,
      showDragHandle: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28)),
      ),
    ),
  );
}
