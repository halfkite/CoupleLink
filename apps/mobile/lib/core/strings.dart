import 'dart:convert';

import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'state.dart';

class StringBundle {
  const StringBundle(this.locales);
  final Map<String, Map<String, String>> locales;
  static Future<StringBundle> load() async {
    final result = <String, Map<String, String>>{};
    for (final locale in ['zh_CN', 'en']) {
      final raw = await rootBundle.loadString('assets/locales/$locale.json');
      result[locale] = Map<String, String>.from(jsonDecode(raw) as Map);
    }
    return StringBundle(result);
  }
}

final bundleProvider = Provider<StringBundle>(
  (ref) => throw StateError('Strings not initialized'),
);
final stringsProvider = Provider<AppStrings>((ref) {
  final state = ref.watch(settingsProvider);
  return AppStrings(ref.watch(bundleProvider), state.locale, state.overrides);
});

class AppStrings {
  const AppStrings(this.bundle, this.locale, this.overrides);
  final StringBundle bundle;
  final String locale;
  final Map<String, String> overrides;
  String t(String key, [Map<String, String> args = const {}]) {
    var value =
        overrides[key] ??
        bundle.locales[locale]?[key] ??
        bundle.locales['zh_CN']?[key] ??
        key;
    for (final entry in args.entries) {
      value = value.replaceAll('{${entry.key}}', entry.value);
    }
    return value;
  }
}
