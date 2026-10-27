import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

const appVersion = '1.0-r3';
const cardIds = [
  'days',
  'interactions',
  'distance',
  'pet',
  'meeting',
  'battery',
];

enum CoupleTheme { sakura, lavender, midnight }

final preferencesProvider = Provider<SharedPreferences>(
  (ref) => throw StateError('Preferences not initialized'),
);

class AppSettings {
  const AppSettings({
    this.theme = CoupleTheme.sakura,
    this.locale = 'zh_CN',
    this.order = cardIds,
    this.hidden = const {},
    this.overrides = const {},
    this.reduceMotion = false,
    this.persistFailed = false,
  });
  final CoupleTheme theme;
  final String locale;
  final List<String> order;
  final Set<String> hidden;
  final Map<String, String> overrides;
  final bool reduceMotion;
  final bool persistFailed;
  factory AppSettings.restore(String? raw) {
    if (raw == null) return const AppSettings();
    try {
      final data = jsonDecode(raw);
      if (data is! Map<String, dynamic>) return const AppSettings();
      final order = <String>[];
      if (data['order'] is List) {
        for (final id in data['order'] as List) {
          if (id is String && cardIds.contains(id) && !order.contains(id)) {
            order.add(id);
          }
        }
      }
      order.addAll(cardIds.where((id) => !order.contains(id)));
      final theme =
          CoupleTheme.values
              .where((v) => v.name == data['theme'])
              .firstOrNull ??
          CoupleTheme.sakura;
      final overrides = <String, String>{};
      if (data['overrides'] is Map) {
        for (final entry in (data['overrides'] as Map).entries) {
          if (entry.key is String &&
              entry.value is String &&
              (entry.value as String).length <= 100) {
            overrides[entry.key as String] = entry.value as String;
          }
        }
      }
      return AppSettings(
        theme: theme,
        locale: data['locale'] == 'en' ? 'en' : 'zh_CN',
        order: order,
        hidden: data['hidden'] is List
            ? (data['hidden'] as List)
                  .whereType<String>()
                  .where(cardIds.contains)
                  .toSet()
            : {},
        overrides: overrides,
        reduceMotion: data['reduceMotion'] == true,
      );
    } on FormatException {
      return const AppSettings();
    }
  }
  AppSettings copyWith({
    CoupleTheme? theme,
    String? locale,
    List<String>? order,
    Set<String>? hidden,
    Map<String, String>? overrides,
    bool? reduceMotion,
    bool? persistFailed,
  }) => AppSettings(
    theme: theme ?? this.theme,
    locale: locale ?? this.locale,
    order: order ?? this.order,
    hidden: hidden ?? this.hidden,
    overrides: overrides ?? this.overrides,
    reduceMotion: reduceMotion ?? this.reduceMotion,
    persistFailed: persistFailed ?? this.persistFailed,
  );
  Map<String, Object> toJson() => {
    'theme': theme.name,
    'locale': locale,
    'order': order,
    'hidden': hidden.toList(),
    'overrides': overrides,
    'reduceMotion': reduceMotion,
  };
}

final settingsProvider = NotifierProvider<SettingsController, AppSettings>(
  SettingsController.new,
);

class SettingsController extends Notifier<AppSettings> {
  Future<void> _pendingWrite = Future.value();
  @override
  AppSettings build() => AppSettings.restore(
    ref.read(preferencesProvider).getString('couplelink.settings.v1'),
  );
  void update(AppSettings next) {
    state = next.copyWith(persistFailed: false);
    final snapshot = jsonEncode(state.toJson());
    // Serialize writes so rapid changes cannot persist in the wrong order.
    _pendingWrite = _pendingWrite.then((_) async {
      try {
        final saved = await ref
            .read(preferencesProvider)
            .setString('couplelink.settings.v1', snapshot);
        if (!saved && ref.mounted) state = state.copyWith(persistFailed: true);
      } catch (_) {
        // Local preference failure is surfaced in the UI, never silently swallowed.
        if (ref.mounted) state = state.copyWith(persistFailed: true);
      }
    });
    unawaited(_pendingWrite);
  }

  void theme(CoupleTheme value) => update(state.copyWith(theme: value));
  void locale(String value) => update(state.copyWith(locale: value));
  void motion(bool value) => update(state.copyWith(reduceMotion: value));
  void toggleCard(String id) {
    final hidden = {...state.hidden};
    hidden.contains(id) ? hidden.remove(id) : hidden.add(id);
    update(state.copyWith(hidden: hidden));
  }

  void reorder(int oldIndex, int newIndex) {
    final order = [...state.order];
    final item = order.removeAt(oldIndex);
    order.insert(newIndex, item);
    update(state.copyWith(order: order));
  }

  void overrideText(String key, String value) {
    final overrides = {...state.overrides};
    value.trim().isEmpty
        ? overrides.remove(key)
        : overrides[key] = value.trim();
    update(state.copyWith(overrides: overrides));
  }

  void resetLayout() => update(state.copyWith(order: cardIds, hidden: {}));
}
