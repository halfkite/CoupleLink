import 'dart:convert';

import 'package:couplelink/app.dart';
import 'package:couplelink/core/state.dart';
import 'package:couplelink/core/strings.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

late StringBundle testBundle;

Future<void> start(
  WidgetTester tester, {
  CoupleTheme theme = CoupleTheme.sakura,
  String locale = 'zh_CN',
  double scale = 1,
  Size size = const Size(390, 844),
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  tester.platformDispatcher.textScaleFactorTestValue = scale;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
    tester.platformDispatcher.clearTextScaleFactorTestValue();
  });
  SharedPreferences.setMockInitialValues({
    'couplelink.settings.v1': jsonEncode(
      AppSettings(theme: theme, locale: locale).toJson(),
    ),
  });
  final prefs = await SharedPreferences.getInstance();
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        preferencesProvider.overrideWithValue(prefs),
        bundleProvider.overrideWithValue(testBundle),
      ],
      child: const CoupleLinkApp(),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  // Asset IO runs outside individual fake-clock test zones. Cached futures from
  // a previous widget test must not be awaited by the next fake-clock zone.
  setUpAll(() async {
    testBundle = await StringBundle.load();
  });
  for (final theme in CoupleTheme.values) {
    for (final config in [(1.0, 'zh_CN'), (2.0, 'zh_CN'), (2.0, 'en')]) {
      testWidgets(
        'five pages fit 320dp, ${theme.name}, ${config.$2}, ${config.$1}x text',
        (tester) async {
          await start(
            tester,
            theme: theme,
            locale: config.$2,
            scale: config.$1,
            size: const Size(320, 700),
          );
          expect(tester.takeException(), isNull);
          for (final page in ['map', 'messages', 'time', 'me', 'home']) {
            await tester.tap(find.byKey(ValueKey('nav-$page')));
            await tester.pumpAndSettle();
            expect(
              tester.takeException(),
              isNull,
              reason: '$page should not overflow',
            );
          }
        },
      );
    }
  }
  testWidgets('local demo message and draft survive switching primary pages', (
    tester,
  ) async {
    await start(tester);
    await tester.tap(find.byKey(const ValueKey('nav-messages')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const ValueKey('message-input')),
      'Our local demo',
    );
    await tester.tap(find.byKey(const ValueKey('send-message')));
    await tester.pumpAndSettle();
    expect(find.text('Our local demo'), findsOneWidget);
    await tester.enterText(
      find.byKey(const ValueKey('message-input')),
      'Unsent draft',
    );
    await tester.tap(find.byKey(const ValueKey('nav-home')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('nav-messages')));
    await tester.pumpAndSettle();
    expect(find.text('Our local demo'), findsOneWidget);
    expect(find.text('Unsent draft'), findsOneWidget);
  });
  testWidgets('gesture preview clearly indicates no real delivery', (
    tester,
  ) async {
    await start(tester);
    await tester.ensureVisible(find.byKey(const ValueKey('gesture-miss_you')));
    await tester.tap(find.byKey(const ValueKey('gesture-miss_you')));
    await tester.pumpAndSettle();
    expect(find.text('仅在本机预览；未发送给对方'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
  testWidgets('rapid preference updates restore the latest theme and label', (
    tester,
  ) async {
    await start(tester);
    final context = tester.element(find.byType(SpaceShell));
    final container = ProviderScope.containerOf(context);
    final controller = container.read(settingsProvider.notifier);
    controller.theme(CoupleTheme.midnight);
    controller.overrideText('interaction.miss', 'Hello love');
    controller.theme(CoupleTheme.lavender);
    controller.reorder(0, 5);
    await tester.pumpAndSettle();
    final raw = container
        .read(preferencesProvider)
        .getString('couplelink.settings.v1');
    final restored = AppSettings.restore(raw);
    expect(restored.theme, CoupleTheme.lavender);
    expect(restored.overrides['interaction.miss'], 'Hello love');
    expect(restored.order.last, 'days');
    expect(tester.takeException(), isNull);
  });
}
