import 'dart:convert';
import 'dart:io';

import 'package:couplelink/core/chat_controller.dart';
import 'package:couplelink/core/chat_store.dart';
import 'package:couplelink/core/connection.dart';
import 'package:couplelink/core/state.dart';
import 'package:drift/native.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

class MemoryVault implements SessionVault {
  String? value;
  @override
  Future<String?> read() async => value;
  @override
  Future<void> write(String value) async {
    this.value = value;
  }

  @override
  Future<void> clear() async {
    value = null;
  }
}

Future<void> until(bool Function() condition) async {
  for (var i = 0; i < 300; i++) {
    if (condition()) return;
    await Future<void>.delayed(const Duration(milliseconds: 10));
  }
  fail('Chat state did not settle');
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test('offline restart and a lost POST response retain one durable ID; unbind cancels retries', () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance(), vault = MemoryVault();
    var online = true, bound = true, dropResponse = false, delivered = false;
    var postCalls = 0;
    const token = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
    final couple = {
      'id': 'relationship-a',
      'version': 1,
      'days_together': 1,
      'start_at': '2026-10-26',
      'timezone': 'Asia/Hong_Kong',
      'partner': {'display_name': 'Bob'},
    };
    final remote = <Map<String, dynamic>>[];
    final client = MockClient((request) async {
      if (!online) throw http.ClientException('offline');
      Map<String, dynamic> body = {};
      if (request.body.isNotEmpty) {
        body = jsonDecode(request.body) as Map<String, dynamic>;
      }
      dynamic result;
      var status = 200;
      switch (request.url.path) {
        case '/healthz':
          result = {'api_version': 1};
        case '/auth/login':
          result = {'token': token};
        case '/account':
          result = {
            'user': {
              'id': 'alice',
              'username': 'alice',
              'display_name': 'Alice',
            },
            'couple': bound ? couple : null,
          };
        case '/devices':
          result = {'devices': []};
        case '/sync/changes':
          result = {'events': [], 'next_cursor': 0};
        case '/realtime/token':
          status = 503;
          result = {'code': 'realtime_unavailable'};
        case '/messages/receipts':
          result = {'ok': true};
        case '/messages':
          if (request.method == 'POST') {
            postCalls++;
            var found = remote
                .where((row) => row['id'] == body['id'])
                .firstOrNull;
            if (found == null) {
              found = {
                ...body,
                'sender_id': 'alice',
                'seq': remote.length + 1,
                'created_at': '2026-10-27T00:00:00Z',
                'delivered': false,
              };
              remote.add(found);
            }
            if (dropResponse) {
              dropResponse = false;
              throw http.ClientException('response lost after commit');
            }
            result = {'message': found};
          } else {
            final after = int.parse(
              request.url.queryParameters['after'] ?? '0',
            );
            result = {
              'messages': remote
                  .where((row) => (row['seq'] as int) > after)
                  .toList(),
              'head': remote.length,
              'next_cursor': remote.length,
              'peer_delivered_through': delivered ? remote.length : 0,
              'has_more': false,
            };
          }
        default:
          throw StateError('Unexpected fixture path');
      }
      return http.Response(jsonEncode(result), status);
    });
    final directory = await Directory.systemTemp.createTemp(
          'couplelink-controller-test-',
        ),
        file = File('${directory.path}/chat.sqlite');
    var store = ChatStore(ChatDatabase(NativeDatabase(file)));
    ProviderContainer makeContainer() => ProviderContainer(
      overrides: [
        preferencesProvider.overrideWithValue(prefs),
        vaultProvider.overrideWithValue(vault),
        httpProvider.overrideWithValue(client),
        chatStoreProvider.overrideWithValue(store),
      ],
    );
    var container = makeContainer();
    container.read(chatProvider);
    try {
      await container
          .read(connectionProvider.notifier)
          .signIn(
            origin: 'https://example.com',
            username: 'alice',
            password: '12345678',
            register: false,
          );
      await until(
        () =>
            container.read(chatProvider).ready &&
            !container.read(chatProvider).syncing,
      );
      online = false;
      expect(
        await container.read(chatProvider.notifier).send('survives restart'),
        isTrue,
      );
      await until(() => !container.read(chatProvider).syncing);
      final id = container.read(chatProvider).messages.single.id;
      await container
          .read(chatProvider.notifier)
          .changeDraft('persistent draft');
      container.dispose();
      await store.close();
      store = ChatStore(ChatDatabase(NativeDatabase(file)));
      container = makeContainer();
      container.read(chatProvider);
      await until(
        () =>
            container.read(chatProvider).ready &&
            !container.read(chatProvider).syncing,
      );
      expect(container.read(connectionProvider).signedIn, isTrue);
      expect(container.read(chatProvider).messages.single.id, id);
      expect(container.read(chatProvider).draft, 'persistent draft');
      expect(remote, isEmpty);
      online = true;
      dropResponse = true;
      await container.read(chatProvider.notifier).sync();
      expect(remote.length, 1);
      expect(container.read(chatProvider).messages.single.status, 'queued');
      await container.read(chatProvider.notifier).sync();
      expect(remote.length, 1);
      expect(postCalls, 2);
      expect(container.read(chatProvider).messages.single.id, id);
      expect(container.read(chatProvider).messages.single.status, 'sent');
      delivered = true;
      await container.read(chatProvider.notifier).sync();
      expect(container.read(chatProvider).messages.single.status, 'delivered');
      online = false;
      await container
          .read(chatProvider.notifier)
          .send('do not send after unbind');
      await until(() => !container.read(chatProvider).syncing);
      bound = false;
      online = true;
      await container.read(chatProvider.notifier).sync();
      await until(() => container.read(chatProvider).scope.isEmpty);
      expect(postCalls, 2);
      expect(container.read(chatProvider).messages, isEmpty);
      await until(() => !container.read(connectionProvider).busy);
      expect(
        await store.rows(
          ChatStore.scope('https://example.com', 'alice', 'relationship-a'),
        ),
        isEmpty,
      );
    } finally {
      container.dispose();
      await store.close();
      await directory.delete(recursive: true);
      client.close();
    }
  });
}
