import 'dart:convert';

import 'package:couplelink/app.dart';
import 'package:couplelink/core/state.dart';
import 'package:couplelink/core/theme.dart';
import 'package:couplelink/core/strings.dart';
import 'package:couplelink/core/device_feedback.dart';
import 'package:couplelink/core/connection.dart';
import 'package:couplelink/features/interaction_feedback.dart';
import 'package:couplelink/features/account.dart';
import 'package:couplelink/features/server_addresses.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

late StringBundle testBundle;

class FakeFeedback implements DeviceFeedback {
  int stopped = 0;
  @override
  Future<FeedbackOutcome> preview({required bool sound}) async =>
      FeedbackOutcome.suppressed;
  @override
  Future<void> stop() async {
    stopped++;
  }
}

Future<void> start(
  WidgetTester tester, {
  CoupleTheme theme = CoupleTheme.sakura,
  String locale = 'zh_CN',
  double scale = 1,
  Size size = const Size(390, 844),
  DeviceFeedback? feedback,
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
        if (feedback != null)
          deviceFeedbackProvider.overrideWithValue(feedback),
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
  testWidgets('multiple server addresses can be added and removed', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          preferencesProvider.overrideWithValue(prefs),
          bundleProvider.overrideWithValue(testBundle),
        ],
        child: MaterialApp(
          theme: buildTheme(CoupleTheme.sakura),
          home: const ServerAddressesPage(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    for (final address in ['https://one.example', 'https://two.example']) {
      await tester.enterText(find.byType(TextField), address);
      await tester.tap(find.text('添加备用地址'));
      await tester.pumpAndSettle();
    }
    expect(prefs.getStringList('couplelink.server.addresses.v1'), [
      'https://one.example',
      'https://two.example',
    ]);
    await tester.tap(find.text('移除').first);
    await tester.pumpAndSettle();
    expect(prefs.getStringList('couplelink.server.addresses.v1'), [
      'https://two.example',
    ]);
  });
  testWidgets(
    'connection button tests server without requiring account or password',
    (tester) async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      var requests = 0;
      final client = MockClient((request) async {
        requests++;
        expect(request.url.path, '/healthz');
        expect(request.headers.containsKey('Authorization'), isFalse);
        return http.Response('{"status":"ok","api_version":1}', 200);
      });
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            preferencesProvider.overrideWithValue(prefs),
            bundleProvider.overrideWithValue(testBundle),
            httpProvider.overrideWithValue(client),
          ],
          child: MaterialApp(
            theme: buildTheme(CoupleTheme.sakura),
            home: const AccountPage(),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(const ValueKey('server-url')),
        'https://example.com',
      );
      await tester.ensureVisible(
        find.byKey(const ValueKey('test-server-connection')),
      );
      await tester.tap(find.byKey(const ValueKey('test-server-connection')));
      await tester.pumpAndSettle();
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 100)),
      );
      await tester.pumpAndSettle();
      expect(requests, 1);
      for (
        var i = 0;
        i < 20 &&
            find.byKey(const ValueKey('server-test-result')).evaluate().isEmpty;
        i++
      ) {
        await tester.pump(const Duration(milliseconds: 50));
      }
      final result = find.byKey(const ValueKey('server-test-result'));
      expect(result, findsOneWidget);
      expect(tester.widget<Text>(result).data, '连接成功，已找到 CoupleLink 服务器。');
      expect(find.text('密码至少 8 位'), findsNothing);
    },
  );
  testWidgets(
    'ring feedback is centered and suppressed feedback stays visible until closed',
    (tester) async {
      final fake = FakeFeedback();
      await start(tester, feedback: fake);
      await tester.ensureVisible(find.byKey(const ValueKey('gesture-ring')));
      await tester.tap(find.byKey(const ValueKey('gesture-ring')));
      await tester.pumpAndSettle();
      final dialog = find.byKey(const ValueKey('interaction-feedback-dialog'));
      expect(find.byType(InteractionFeedback), findsOneWidget);
      expect((tester.getCenter(dialog).dy - 422).abs(), lessThan(2));
      await tester.ensureVisible(
        find.byKey(const ValueKey('run-local-feedback')),
      );
      await tester.tap(find.byKey(const ValueKey('run-local-feedback')));
      await tester.pumpAndSettle();
      expect(find.text('系统静音、勿扰或触觉设置已阻止反馈'), findsOneWidget);
      expect(find.text('仅在本机预览；未发送给对方'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('close-feedback')));
      await tester.pumpAndSettle();
      expect(fake.stopped, 1);
      expect(tester.takeException(), isNull);
    },
  );
  testWidgets(
    'account and registration forms fit large text on a narrow screen',
    (tester) async {
      await start(tester, scale: 2, size: const Size(320, 700));
      final context = tester.element(find.byType(SpaceShell));
      openAccount(context);
      await tester.pumpAndSettle();
      expect(find.byType(AccountPage), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.ensureVisible(find.widgetWithText(ChoiceChip, '注册'));
      await tester.tap(find.widgetWithText(ChoiceChip, '注册'));
      await tester.pumpAndSettle();
      expect(find.widgetWithText(TextFormField, '服务器设置码'), findsNothing);
      final passwordField = tester.widget<TextFormField>(
        find.widgetWithText(TextFormField, '密码'),
      );
      expect(passwordField.validator!('1234567'), '密码至少 8 位');
      expect(passwordField.validator!('12345678'), isNull);
      expect(passwordField.validator!('abcdefgh'), isNull);
      expect(tester.takeException(), isNull);
    },
  );
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
