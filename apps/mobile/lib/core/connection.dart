import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:http/http.dart' as http;

import 'state.dart';

class ConnectionState {
  const ConnectionState({
    this.origin = '',
    this.user,
    this.couple,
    this.devices = const [],
    this.busy = false,
    this.error,
    this.invitation,
    this.cursor = 0,
  });
  final String origin;
  final Map<String, dynamic>? user, couple;
  final List<Map<String, dynamic>> devices;
  final bool busy;
  final String? error, invitation;
  final int cursor;
  bool get signedIn => user != null;
  ConnectionState copy({
    String? origin,
    Map<String, dynamic>? user,
    Map<String, dynamic>? couple,
    List<Map<String, dynamic>>? devices,
    bool? busy,
    String? error,
    String? invitation,
    int? cursor,
    bool clearCouple = false,
    bool clearUser = false,
  }) => ConnectionState(
    origin: origin ?? this.origin,
    user: clearUser ? null : user ?? this.user,
    couple: clearCouple ? null : couple ?? this.couple,
    devices: devices ?? this.devices,
    busy: busy ?? this.busy,
    error: error,
    invitation: invitation,
    cursor: cursor ?? this.cursor,
  );
}

class ApiException implements Exception {
  const ApiException(this.code);
  final String code;
}

abstract interface class SessionVault {
  Future<String?> read();
  Future<void> write(String value);
  Future<void> clear();
}

class PlatformVault implements SessionVault {
  final _secure = const FlutterSecureStorage();
  String? _webSession;
  @override
  Future<String?> read() async =>
      kIsWeb ? _webSession : await _secure.read(key: 'couplelink.session.v1');
  @override
  Future<void> write(String value) async {
    if (kIsWeb) {
      _webSession = value;
    } else {
      await _secure.write(key: 'couplelink.session.v1', value: value);
    }
  }

  @override
  Future<void> clear() async {
    if (kIsWeb) {
      _webSession = null;
    } else {
      await _secure.delete(key: 'couplelink.session.v1');
    }
  }
}

final vaultProvider = Provider<SessionVault>((ref) => PlatformVault());
final httpProvider = Provider<http.Client>((ref) {
  final client = http.Client();
  ref.onDispose(client.close);
  return client;
});
final connectionProvider =
    NotifierProvider<ConnectionController, ConnectionState>(
      ConnectionController.new,
    );

const configuredLocalHttpHosts = String.fromEnvironment(
  'COUPLELINK_LOCAL_HTTP_HOSTS',
);

bool isPrivateIPv4(String host) {
  final parts = host.split('.');
  if (parts.length != 4) return false;
  final bytes = <int>[];
  for (final part in parts) {
    if (!RegExp(r'^(0|[1-9][0-9]{0,2})$').hasMatch(part)) return false;
    final byte = int.parse(part);
    if (byte > 255) return false;
    bytes.add(byte);
  }
  return bytes[0] == 10 ||
      (bytes[0] == 172 && bytes[1] >= 16 && bytes[1] <= 31) ||
      (bytes[0] == 192 && bytes[1] == 168);
}

String normalizeOrigin(
  String value, {
  String localHttpHosts = configuredLocalHttpHosts,
}) {
  final uri = Uri.tryParse(value.trim());
  if (uri == null ||
      !uri.hasAuthority ||
      uri.host.isEmpty ||
      uri.userInfo.isNotEmpty ||
      uri.hasQuery ||
      uri.hasFragment ||
      (uri.path.isNotEmpty && uri.path != '/') ||
      (uri.scheme != 'https' &&
          !(uri.scheme == 'http' &&
              (['127.0.0.1', 'localhost', '10.0.2.2'].contains(uri.host) ||
                  (isPrivateIPv4(uri.host) &&
                      localHttpHosts
                          .split(',')
                          .map((host) => host.trim())
                          .contains(uri.host)))))) {
    throw const ApiException('https_required');
  }
  return uri.replace(path: '').toString();
}

String newDeviceID() {
  final random = Random.secure(), bytes = List<int>.generate(16, (_) => 0);
  for (var i = 0; i < bytes.length; i++) {
    bytes[i] = random.nextInt(256);
  }
  bytes[6] = (bytes[6] & 15) | 64;
  bytes[8] = (bytes[8] & 63) | 128;
  final hex = bytes.map((v) => v.toRadixString(16).padLeft(2, '0')).join();
  return '${hex.substring(0, 8)}-${hex.substring(8, 12)}-${hex.substring(12, 16)}-${hex.substring(16, 20)}-${hex.substring(20)}';
}

class ConnectionController extends Notifier<ConnectionState> {
  String? _token;
  int _generation = 0;
  @override
  ConnectionState build() {
    final origin =
        ref.read(preferencesProvider).getString('couplelink.server.v1') ?? '';
    if (origin.isNotEmpty) scheduleMicrotask(_restore);
    return ConnectionState(origin: origin);
  }

  Future<void> _restore() async {
    try {
      final raw = await ref.read(vaultProvider).read();
      if (raw == null || !ref.mounted) return;
      final data = jsonDecode(raw) as Map;
      if (data['origin'] != state.origin || data['token'] is! String) {
        await ref.read(vaultProvider).clear();
        return;
      }
      _token = data['token'] as String;
      await refresh();
    } catch (_) {
      if (ref.mounted) state = state.copy(error: 'session_restore_failed');
    }
  }

  Future<Map<String, dynamic>> request(
    String method,
    String path, {
    Map<String, dynamic>? body,
    bool auth = true,
  }) async {
    final client = ref.read(httpProvider);
    final req = http.Request(method, Uri.parse('${state.origin}$path'));
    // The selected origin must not redirect requests to a different destination.
    req.followRedirects = false;
    if (auth) {
      if (_token == null) throw const ApiException('unauthorized');
      req.headers['Authorization'] = 'Bearer $_token';
    }
    if (body != null) {
      req.headers['Content-Type'] = 'application/json';
      req.body = jsonEncode(body);
    }
    final response = await client
        .send(req)
        .timeout(const Duration(seconds: 10));
    var count = 0;
    final bytes = <int>[];
    await for (final chunk in response.stream.timeout(
      const Duration(seconds: 10),
    )) {
      count += chunk.length;
      if (count > 256 * 1024) throw const ApiException('invalid_response');
      bytes.addAll(chunk);
    }
    dynamic data;
    try {
      data = jsonDecode(utf8.decode(bytes));
    } on FormatException {
      throw const ApiException('invalid_response');
    }
    if (data is! Map<String, dynamic>) {
      throw const ApiException('invalid_response');
    }
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw ApiException(
        data['code'] is String ? data['code'] as String : 'server_error',
      );
    }
    return data;
  }

  Future<void> _perform(Future<void> Function() operation) async {
    if (state.busy) return;
    final generation = ++_generation;
    state = state.copy(busy: true);
    try {
      await operation();
      if (ref.mounted && generation == _generation) {
        state = state.copy(busy: false, invitation: state.invitation);
      }
    } catch (error) {
      if (!ref.mounted || generation != _generation) return;
      var code = error is ApiException
          ? error.code
          : error is TimeoutException
          ? 'network_timeout'
          : error is PlatformException || error is MissingPluginException
          ? 'secure_storage_failed'
          : 'network_failed';
      if (error is FormatException || error is TypeError) {
        code = 'invalid_response';
      }
      if (code == 'unauthorized') {
        _token = null;
        try {
          await ref.read(vaultProvider).clear();
        } on PlatformException {
          code = 'secure_storage_failed';
        } on MissingPluginException {
          code = 'secure_storage_failed';
        } finally {
          if (ref.mounted) {
            state = ConnectionState(origin: state.origin, error: code);
          }
        }
      } else {
        state = state.copy(
          busy: false,
          error: code,
          invitation: state.invitation,
        );
      }
    }
  }

  Future<void> signIn({
    required String origin,
    required String username,
    required String password,
    required bool register,
    String displayName = '',
  }) => _perform(() async {
    final url = normalizeOrigin(origin);
    if (url != state.origin) {
      _token = null;
      state = ConnectionState(origin: url, busy: true);
    }
    final health = await request('GET', '/healthz', auth: false);
    if (health['api_version'] != 1) {
      throw const ApiException('incompatible_server');
    }
    final prefs = ref.read(preferencesProvider);
    final accountScope = base64Url.encode(
      utf8.encode('$url|${username.trim().toLowerCase()}'),
    );
    final deviceKey = 'couplelink.device.$accountScope';
    var device = prefs.getString(deviceKey);
    if (device == null) {
      device = newDeviceID();
      if (!await prefs.setString(deviceKey, device)) {
        throw const ApiException('preferences_failed');
      }
    }
    final data = await request(
      'POST',
      register ? '/auth/register' : '/auth/login',
      auth: false,
      body: {
        'username': username,
        'password': password,
        'device_id': device,
        'device_label': kIsWeb ? 'Browser preview' : 'Android phone',
        'platform': kIsWeb ? 'web' : 'android',
        if (register) 'display_name': displayName,
      },
    );
    final token = data['token'];
    if (token is! String || token.length != 43) {
      throw const ApiException('invalid_response');
    }
    // Credentials never enter SharedPreferences; only the server origin and
    // random, account-scoped device IDs are saved there.
    await ref
        .read(vaultProvider)
        .write(jsonEncode({'origin': url, 'token': token}));
    if (!await prefs.setString('couplelink.server.v1', url)) {
      throw const ApiException('preferences_failed');
    }
    _token = token;
    state = state.copy(cursor: 0);
    await _snapshot();
  });
  Future<void> _snapshot() async {
    final account = await request('GET', '/account');
    final devices = await request('GET', '/devices');
    final user = account['user'], couple = account['couple'];
    if (user is! Map<String, dynamic> ||
        devices['devices'] is! List ||
        (couple != null && couple is! Map<String, dynamic>)) {
      throw const ApiException('invalid_response');
    }
    if (user['id'] is! String ||
        user['username'] is! String ||
        user['display_name'] is! String) {
      throw const ApiException('invalid_response');
    }
    if (couple != null &&
        (couple['partner'] is! Map<String, dynamic> ||
            couple['version'] is! int ||
            couple['days_together'] is! int ||
            couple['start_at'] is! String ||
            couple['timezone'] is! String ||
            (couple['partner'] as Map)['display_name'] is! String)) {
      throw const ApiException('invalid_response');
    }
    for (final device in devices['devices'] as List) {
      if (device is! Map<String, dynamic> ||
          device['id'] is! String ||
          device['label'] is! String ||
          device['platform'] is! String ||
          device['current'] is! bool) {
        throw const ApiException('invalid_response');
      }
    }
    state = state.copy(
      user: user,
      couple: couple as Map<String, dynamic>?,
      clearCouple: couple == null,
      devices: (devices['devices'] as List)
          .map((v) => Map<String, dynamic>.from(v as Map))
          .toList(),
      invitation: state.invitation,
    );
  }

  Future<void> refresh() => _perform(() async {
    final changes = await request(
      'GET',
      '/sync/changes?cursor=${state.cursor}',
    );
    final cursor = changes['next_cursor'];
    if (cursor is! int || cursor < state.cursor) {
      throw const ApiException('invalid_response');
    }
    await _snapshot();
    state = state.copy(cursor: cursor, invitation: state.invitation);
  });
  Future<void> createInvitation() => _perform(() async {
    final result = await request('POST', '/couples/invitations');
    if (result['token'] is! String) {
      throw const ApiException('invalid_response');
    }
    state = state.copy(invitation: result['token'] as String);
  });
  Future<void> join(String token, String startAt, String timezone) =>
      _perform(() async {
        await request(
          'POST',
          '/couples/join',
          body: {
            'token': token.trim(),
            'start_at': startAt,
            'timezone': timezone.trim(),
          },
        );
        await _snapshot();
      });
  Future<void> unbind() => _perform(() async {
    final couple = state.couple;
    if (couple == null) return;
    await request(
      'DELETE',
      '/couples/current',
      body: {'version': couple['version']},
    );
    await _snapshot();
  });
  Future<void> revoke(String id, bool current) => _perform(() async {
    await request('DELETE', '/devices/$id');
    if (current) {
      await _clear();
    } else {
      await _snapshot();
    }
  });
  Future<void> signOut() => _perform(() async {
    await request('POST', '/auth/logout');
    await _clear();
  });
  Future<void> _clear() async {
    _token = null;
    if (ref.mounted) state = ConnectionState(origin: state.origin);
    // Clear the visible identity even if the OS credential store is unavailable.
    // A persisted token has already been revoked by the server at this point.
    await ref.read(vaultProvider).clear();
  }
}
