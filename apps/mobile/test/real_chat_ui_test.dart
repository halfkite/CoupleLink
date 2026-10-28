import 'package:couplelink/app.dart';
import 'package:couplelink/core/chat_controller.dart';
import 'package:couplelink/core/chat_store.dart';
import 'package:couplelink/core/connection.dart';
import 'package:couplelink/core/state.dart';
import 'package:couplelink/core/strings.dart';
import 'package:couplelink/features/real_chat.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

class FixtureConnection extends ConnectionController {
  @override
  ConnectionState build() => const ConnectionState(
    origin: 'https://example.com',
    user: {'id': 'alice', 'username': 'alice', 'display_name': 'Alice'},
    couple: {
      'id': 'couple',
      'version': 1,
      'days_together': 1,
      'start_at': '2026-10-26',
      'timezone': 'Asia/Hong_Kong',
      'partner': {'display_name': 'A long partner display name that wraps'},
    },
  );
}

class FixtureChat extends ChatController {
  @override
  ChatState build() => ChatState(
    scope: 'fixture',
    ready: true,
    messages: [
      ChatRow(
        id: 'm',
        coupleId: 'couple',
        senderId: 'alice',
        body: 'A long message on a narrow screen. ' * 25,
        createdAt: DateTime.utc(2026, 10, 27),
        status: 'queued',
      ),
    ],
  );
}

class UnreadFixtureChat extends ChatController {
  @override
  ChatState build() =>
      const ChatState(scope: 'fixture', ready: true, unread: true);
  @override
  void setChatVisible(bool visible) {
    if (visible) state = state.copy(unread: false);
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  late StringBundle bundle;
  setUpAll(() async {
    bundle = await StringBundle.load();
  });
  testWidgets('message navigation shows unread dot until chat is opened', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          preferencesProvider.overrideWithValue(prefs),
          bundleProvider.overrideWithValue(bundle),
          connectionProvider.overrideWith(FixtureConnection.new),
          chatProvider.overrideWith(UnreadFixtureChat.new),
        ],
        child: const CoupleLinkApp(),
      ),
    );
    await tester.pumpAndSettle();
    final badge = find.descendant(
      of: find.byKey(const ValueKey('nav-messages')),
      matching: find.byType(Badge),
    );
    expect(tester.widget<Badge>(badge).isLabelVisible, isTrue);
    await tester.tap(find.byKey(const ValueKey('nav-messages')));
    await tester.pumpAndSettle();
    expect(tester.widget<Badge>(badge).isLabelVisible, isFalse);
  });
  for (final locale in ['zh_CN', 'en']) {
    testWidgets('real chat fits 320dp at 200% text in $locale', (tester) async {
      final semantics = tester.ensureSemantics();
      tester.view.physicalSize = const Size(320, 700);
      tester.view.devicePixelRatio = 1;
      tester.platformDispatcher.textScaleFactorTestValue = 2;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
        tester.platformDispatcher.clearTextScaleFactorTestValue();
      });
      SharedPreferences.setMockInitialValues({
        'couplelink.settings.v1': '{"locale":"$locale"}',
      });
      final prefs = await SharedPreferences.getInstance();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            preferencesProvider.overrideWithValue(prefs),
            bundleProvider.overrideWithValue(bundle),
            connectionProvider.overrideWith(FixtureConnection.new),
            chatProvider.overrideWith(FixtureChat.new),
          ],
          child: const CoupleLinkApp(),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('nav-messages')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('real-message-input')), findsOneWidget);
      await tester.scrollUntilVisible(
        find.byKey(const ValueKey('chat-message-m')),
        200,
        scrollable: find
            .descendant(
              of: find.byType(RealChatPage),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      expect(
        find.bySemanticsLabel(RegExp('A long message on a narrow screen')),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
      semantics.dispose();
    });
  }
}
