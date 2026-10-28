import 'dart:io';

import 'package:couplelink/core/chat_store.dart';
import 'package:couplelink/core/connection.dart';
import 'package:couplelink/core/background_notifications.dart';
import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'dart:convert';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test(
    'background baseline advances to already synchronized messages',
    () async {
      SharedPreferences.setMockInitialValues({
        'couplelink.background.config.v1': jsonEncode({
          'origin': 'https://example.com',
          'user': 'alice',
          'couple': 'pair',
          'cursor': 0,
        }),
      });
      final prefs = await SharedPreferences.getInstance();
      final store = ChatStore(ChatDatabase(NativeDatabase.memory()));
      addTearDown(store.close);
      final scope = ChatStore.scope('https://example.com', 'alice', 'pair');
      await store.merge(
        scope,
        [
          ChatRow(
            id: newDeviceID(),
            coupleId: 'pair',
            senderId: 'bob',
            body: 'hi',
            createdAt: DateTime.utc(2026),
            seq: 1,
            status: 'sent',
          ),
        ],
        cursor: 1,
        user: 'alice',
      );
      await updateBackgroundBaseline(
        prefs,
        store,
        const ConnectionState(
          origin: 'https://example.com',
          user: {'id': 'alice'},
          couple: {'id': 'pair'},
        ),
      );
      expect(
        (jsonDecode(prefs.getString('couplelink.background.config.v1')!)
            as Map)['cursor'],
        1,
      );
    },
  );
  test('version one database upgrades without losing chat state', () async {
    final directory = await Directory.systemTemp.createTemp(
      'couplelink-migration-test-',
    );
    final file = File('${directory.path}/chat.sqlite');
    var store = ChatStore(ChatDatabase(NativeDatabase(file)));
    try {
      await store.saveDraft('scope', 'retained draft');
      await store.merge(
        'scope',
        [
          ChatRow(
            id: newDeviceID(),
            coupleId: 'pair',
            senderId: 'bob',
            body: 'old message',
            createdAt: DateTime.utc(2026),
            seq: 1,
            status: 'sent',
          ),
        ],
        cursor: 1,
        user: 'alice',
      );
      await store.db.customStatement(
        'ALTER TABLE chat_state DROP COLUMN last_read',
      );
      await store.db.customStatement('PRAGMA user_version = 1');
      await store.close();
      store = ChatStore(ChatDatabase(NativeDatabase(file)));
      expect(await store.draft('scope'), 'retained draft');
      expect(await store.hasUnread('scope', 'alice'), isFalse);
      await store.markRead('scope');
      expect(await store.hasUnread('scope', 'alice'), isFalse);
    } finally {
      await store.close();
      await directory.delete(recursive: true);
    }
  });
  test(
    'unread indicator only tracks partner messages and survives reopen',
    () async {
      final directory = await Directory.systemTemp.createTemp(
        'couplelink-unread-test-',
      );
      final file = File('${directory.path}/chat.sqlite');
      var store = ChatStore(ChatDatabase(NativeDatabase(file)));
      ChatRow row(int seq, String sender) => ChatRow(
        id: newDeviceID(),
        coupleId: 'pair',
        senderId: sender,
        body: 'message',
        createdAt: DateTime.utc(2026),
        seq: seq,
        status: 'sent',
      );
      try {
        await store.merge('scope', [row(1, 'bob')], cursor: 1, user: 'alice');
        expect(await store.hasUnread('scope', 'alice'), isTrue);
        await store.markRead('scope');
        expect(await store.hasUnread('scope', 'alice'), isFalse);
        await store.merge('scope', [row(2, 'alice')], cursor: 2, user: 'alice');
        expect(await store.hasUnread('scope', 'alice'), isFalse);
        await store.merge('scope', [row(3, 'bob')], cursor: 3, user: 'alice');
        await store.close();
        store = ChatStore(ChatDatabase(NativeDatabase(file)));
        expect(await store.hasUnread('scope', 'alice'), isTrue);
        await store.markRead('scope');
        expect(await store.hasUnread('scope', 'alice'), isFalse);
      } finally {
        await store.close();
        await directory.delete(recursive: true);
      }
    },
  );
  test('pending IDs, cursor and draft survive a real SQLite reopen; receipts are monotonic', () async {
    final directory = await Directory.systemTemp.createTemp(
          'couplelink-chat-test-',
        ),
        file = File('${directory.path}/chat.sqlite');
    final scope = ChatStore.scope(
      'https://example.com',
      'alice',
      'relationship-a',
    );
    var store = ChatStore(ChatDatabase(NativeDatabase(file)));
    final id = newDeviceID();
    await store.enqueue(
      scope,
      ChatRow(
        id: id,
        coupleId: 'relationship-a',
        senderId: 'alice',
        body: 'persistent offline message',
        createdAt: DateTime.utc(2026, 10, 27),
      ),
    );
    await store.saveDraft(scope, 'draft after restart');
    await store.close();
    store = ChatStore(ChatDatabase(NativeDatabase(file)));
    try {
      final rows = await store.rows(scope);
      expect(rows.single.id, id);
      expect(rows.single.status, 'queued');
      expect(await store.draft(scope), 'draft after restart');
      final accepted = ChatRow(
        id: id,
        coupleId: 'relationship-a',
        senderId: 'alice',
        body: rows.single.body,
        createdAt: DateTime.utc(2026, 10, 27),
        seq: 1,
        status: 'sent',
      );
      await store.merge(
        scope,
        [accepted],
        cursor: 1,
        user: 'alice',
        deliveredThrough: 1,
      );
      await store.merge(scope, [accepted], cursor: 0, user: 'alice');
      expect((await store.rows(scope)).single.status, 'delivered');
      expect(await store.cursor(scope), 1);
      expect(
        await store.rows(
          ChatStore.scope('https://example.com', 'bob', 'relationship-a'),
        ),
        isEmpty,
      );
      await store.purge(scope);
      expect(await store.rows(scope), isEmpty);
      expect(await store.draft(scope), '');
    } finally {
      await store.close();
      await directory.delete(recursive: true);
    }
  });
  test('queue limits report errors without deleting queued messages', () async {
    final store = ChatStore(
      ChatDatabase(NativeDatabase.memory()),
      maxPending: 2,
      maxPendingBytes: 8,
    );
    addTearDown(store.close);
    Future<void> add(String body) => store.enqueue(
      'scope',
      ChatRow(
        id: newDeviceID(),
        coupleId: 'couple',
        senderId: 'alice',
        body: body,
        createdAt: DateTime.now(),
      ),
    );
    await add('hello');
    await expectLater(
      add('long'),
      throwsA(isA<ApiException>().having((e) => e.code, 'code', 'queue_full')),
    );
    expect((await store.rows('scope')).single.body, 'hello');
    await add('ok');
    await expectLater(add('x'), throwsA(isA<ApiException>()));
    expect((await store.rows('scope')).length, 2);
  });
}
