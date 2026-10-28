import 'dart:convert';

import 'package:couplelink/core/connection.dart';
import 'package:couplelink/core/state.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

class TestVault implements SessionVault {
  String? value;
  bool failClear = false;
  @override
  Future<String?> read() async => value;
  @override
  Future<void> write(String next) async {
    value = next;
  }

  @override
  Future<void> clear() async {
    if (failClear) throw PlatformException(code: 'storage_unavailable');
    value = null;
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test('switching server clears the session without sending its token to the new address', () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance(), vault = TestVault();
    final requests = <Uri>[];
    final client = MockClient((req) async {
      requests.add(req.url);
      if (req.url.path == '/healthz') {
        return http.Response(jsonEncode({'api_version': 1}), 200);
      }
      if (req.url.path == '/auth/login') {
        return http.Response(
          jsonEncode({'token': 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'}),
          200,
        );
      }
      if (req.url.path == '/account') {
        return http.Response(
          jsonEncode({
            'user': {
              'id': 'alice',
              'username': 'alice',
              'display_name': 'Alice',
            },
            'couple': null,
          }),
          200,
        );
      }
      if (req.url.path == '/devices') {
        return http.Response(jsonEncode({'devices': []}), 200);
      }
      return http.Response(jsonEncode({'events': [], 'next_cursor': 0}), 200);
    });
    final container = ProviderContainer(
      overrides: [
        preferencesProvider.overrideWithValue(prefs),
        vaultProvider.overrideWithValue(vault),
        httpProvider.overrideWithValue(client),
      ],
    );
    addTearDown(container.dispose);
    final controller = container.read(connectionProvider.notifier);
    await controller.signIn(
      origin: 'https://original.example',
      username: 'alice',
      password: '12345678',
      register: false,
    );
    expect(container.read(connectionProvider).signedIn, isTrue);
    await controller.changeServer('https://backup.example');
    expect(container.read(connectionProvider).signedIn, isFalse);
    expect(container.read(connectionProvider).origin, 'https://backup.example');
    expect(vault.value, isNull);
    expect(requests.every((uri) => uri.host == 'original.example'), isTrue);
    client.close();
  });
  test(
    'server URL accepts configured LAN HTTP and rejects other plaintext hosts',
    () {
      for (final url in [
        'https://user:password@example.com',
        'https://example.com/api',
        'https://example.com?token=secret',
        'file:///tmp/a',
        '',
      ]) {
        expect(() => normalizeOrigin(url), throwsA(isA<ApiException>()));
      }
      expect(normalizeOrigin(' https://example.com/ '), 'https://example.com');
      expect(
        normalizeOrigin(
          'http://192.168.137.1:18080',
          localHttpHosts: '192.168.137.1',
        ),
        'http://192.168.137.1:18080',
      );
      expect(
        () => normalizeOrigin('http://example.com:8080'),
        throwsA(isA<ApiException>()),
      );
    },
  );
  test('connection test sends no credentials, checks server identity, and rejects redirects', () async {
    var answer = 200;
    var body = '{"status":"ok","api_version":1}';
    final client = MockClient((request) async {
      expect(request.method, 'GET');
      expect(request.url.toString(), 'http://192.168.137.1:18080/healthz');
      expect(request.headers.containsKey('Authorization'), isFalse);
      expect(request.followRedirects, isFalse);
      return http.Response(body, answer);
    });
    await testServerConnection(
      client,
      'http://192.168.137.1:18080',
      localHttpHosts: '192.168.137.1',
    );
    body = '{"status":"ok","api_version":2}';
    await expectLater(
      testServerConnection(
        client,
        'http://192.168.137.1:18080',
        localHttpHosts: '192.168.137.1',
      ),
      throwsA(isA<ApiException>()),
    );
    answer = 302;
    await expectLater(
      testServerConnection(
        client,
        'http://192.168.137.1:18080',
        localHttpHosts: '192.168.137.1',
      ),
      throwsA(isA<ApiException>()),
    );
    client.close();
  });
  test('bearer token stays in vault and revoked sessions clear authenticated state', () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance(), vault = TestVault();
    const token = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
    var revoked = false;
    var authenticatedCalls = 0;
    final client = MockClient((req) async {
      if (req.url.path == '/healthz') {
        expect(req.followRedirects, isFalse);
        return http.Response(jsonEncode({'api_version': 1}), 200);
      }
      if (req.url.path == '/auth/login') {
        final body = jsonDecode(req.body) as Map;
        expect(body['device_id'], matches(RegExp(r'^[0-9a-f-]{36}$')));
        return http.Response(jsonEncode({'token': token}), 200);
      }
      expect(req.headers['Authorization'], 'Bearer $token');
      authenticatedCalls++;
      if (revoked) {
        return http.Response(jsonEncode({'code': 'unauthorized'}), 401);
      }
      if (req.url.path == '/account') {
        return http.Response(
          jsonEncode({
            'user': {'id': 'a', 'username': 'alice', 'display_name': 'Alice'},
            'couple': null,
          }),
          200,
        );
      }
      if (req.url.path == '/devices') {
        return http.Response(jsonEncode({'devices': []}), 200);
      }
      return http.Response(jsonEncode({'events': [], 'next_cursor': 0}), 200);
    });
    final container = ProviderContainer(
      overrides: [
        preferencesProvider.overrideWithValue(prefs),
        vaultProvider.overrideWithValue(vault),
        httpProvider.overrideWithValue(client),
      ],
    );
    addTearDown(container.dispose);
    final controller = container.read(connectionProvider.notifier);
    await controller.signIn(
      origin: 'https://example.com',
      username: 'alice',
      password: 'a long password',
      register: false,
    );
    expect(container.read(connectionProvider).signedIn, isTrue);
    expect(vault.value, contains(token));
    for (final key in prefs.getKeys()) {
      expect(prefs.get(key).toString(), isNot(contains(token)));
    }
    revoked = true;
    await controller.refresh();
    expect(container.read(connectionProvider).signedIn, isFalse);
    expect(vault.value, isNull);
    expect(container.read(connectionProvider).error, 'unauthorized');
    expect(authenticatedCalls, greaterThan(1));
    revoked = false;
    await controller.signIn(
      origin: 'https://example.com',
      username: 'alice',
      password: 'a long password',
      register: false,
    );
    vault.failClear = true;
    revoked = true;
    await controller.refresh();
    expect(container.read(connectionProvider).signedIn, isFalse);
    expect(container.read(connectionProvider).busy, isFalse);
    expect(container.read(connectionProvider).error, 'secure_storage_failed');
  });
}
