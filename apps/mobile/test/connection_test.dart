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
  test('server URL rejects plaintext remote origins, embedded credentials and paths', () {
    for (final url in [
      'http://192.168.1.2:8080',
      'https://user:password@example.com',
      'https://example.com/api',
      'https://example.com?token=secret',
      'file:///tmp/a',
    ]) {
      expect(() => normalizeOrigin(url), throwsA(isA<ApiException>()));
    }
    expect(normalizeOrigin(' https://example.com/ '), 'https://example.com');
    expect(normalizeOrigin('http://127.0.0.1:18080'), 'http://127.0.0.1:18080');
    const allowed = '10.74.205.253,192.168.1.2,172.16.0.1,172.31.255.254';
    for (final host in allowed.split(',')) {
      expect(
        normalizeOrigin('http://$host:18080', localHttpHosts: allowed),
        'http://$host:18080',
      );
    }
    for (final host in [
      '8.8.8.8',
      '172.15.255.255',
      '172.32.0.1',
      '192.169.1.2',
      '10.1.2.999',
      '010.1.2.3',
      '192.168.1.2.example.com',
    ]) {
      expect(
        () => normalizeOrigin('http://$host:18080', localHttpHosts: host),
        throwsA(isA<ApiException>()),
      );
    }
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
