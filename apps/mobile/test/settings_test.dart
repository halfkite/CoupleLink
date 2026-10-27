import 'dart:convert';

import 'package:couplelink/core/state.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('corrupt and non-object preferences recover to a usable default', () {
    for (final raw in ['not json', 'null', '[]', '42', '"text"']) {
      final settings = AppSettings.restore(raw);
      expect(settings.theme, CoupleTheme.sakura);
      expect(settings.order, cardIds);
      expect(settings.hidden, isEmpty);
    }
  });
  test('old or malformed card preferences retain every supported card exactly once', () {
    final settings = AppSettings.restore(
      jsonEncode({
        'order': ['pet', 'pet', 'old_feature', null, 7, 'days'],
        'hidden': ['pet', 'missing', 1],
        'theme': 'unknown',
        'locale': 'unknown',
        'overrides': {
          'interaction.miss': 'Hi',
          'bad': 42,
          'too_long': 'x' * 101,
        },
      }),
    );
    expect(settings.order.take(2), ['pet', 'days']);
    expect(settings.order.toSet(), cardIds.toSet());
    expect(settings.order.length, cardIds.length);
    expect(settings.hidden, {'pet'});
    expect(settings.overrides, {'interaction.miss': 'Hi'});
    expect(settings.locale, 'zh_CN');
  });
}
