import 'dart:async';
import 'dart:convert';
import 'dart:math';
import 'dart:ui';

import 'package:flutter_foreground_task/flutter_foreground_task.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

import 'chat_store.dart';
import 'connection.dart';
import 'notifications.dart';
import 'strings.dart';

const _backgroundConfigKey = 'couplelink.background.config.v1';
const _foregroundKey = 'couplelink.app.foreground.v1';

@pragma('vm:entry-point')
void startCoupleBackgroundTask() {
  DartPluginRegistrant.ensureInitialized();
  FlutterForegroundTask.setTaskHandler(CoupleBackgroundTask());
}

Future<void> setAppForeground(SharedPreferences prefs, bool value) async {
  if (!await prefs.setBool(_foregroundKey, value)) {
    throw const ApiException('preferences_failed');
  }
}

Future<void> updateBackgroundBaseline(
  SharedPreferences prefs,
  ChatStore store,
  ConnectionState connection,
) async {
  await prefs.reload();
  final raw = prefs.getString(_backgroundConfigKey);
  if (raw == null ||
      connection.user?['id'] is! String ||
      connection.couple?['id'] is! String) {
    return;
  }
  final config = jsonDecode(raw) as Map<String, dynamic>;
  if (config['origin'] != connection.origin ||
      config['user'] != connection.user!['id'] ||
      config['couple'] != connection.couple!['id']) {
    return;
  }
  final scope = ChatStore.scope(
    connection.origin,
    connection.user!['id'] as String,
    connection.couple!['id'] as String,
  );
  final cursor = await store.cursor(scope);
  final previous = config['cursor'];
  if (previous is! int || cursor > previous) {
    config['cursor'] = cursor;
    if (!await prefs.setString(_backgroundConfigKey, jsonEncode(config))) {
      throw const ApiException('preferences_failed');
    }
  }
}

Future<bool> startBackgroundNotifications(
  SharedPreferences prefs,
  ChatStore store,
  ConnectionState connection,
  AppStrings strings,
) async {
  if (!supportsAndroidNotifications ||
      connection.user?['id'] is! String ||
      connection.couple?['id'] is! String) {
    return false;
  }
  if (!await coupleNotifications.requestPermission()) return false;
  final scope = ChatStore.scope(
    connection.origin,
    connection.user!['id'] as String,
    connection.couple!['id'] as String,
  );
  final cursor = await store.cursor(scope);
  final cursorKey = 'couplelink.background.cursor.$scope';
  final backgroundCursor = max(cursor, prefs.getInt(cursorKey) ?? 0);
  if (!await prefs.setInt(cursorKey, backgroundCursor)) {
    throw const ApiException('preferences_failed');
  }
  final value = jsonEncode({
    'origin': connection.origin,
    'user': connection.user!['id'],
    'couple': connection.couple!['id'],
    'cursor': backgroundCursor,
  });
  if (!await prefs.setString(_backgroundConfigKey, value)) {
    throw const ApiException('preferences_failed');
  }
  await setAppForeground(prefs, true);
  FlutterForegroundTask.init(
    androidNotificationOptions: AndroidNotificationOptions(
      channelId: 'couplelink_background',
      channelName: strings.t('notification.backgroundChannel'),
      channelDescription: strings.t('notification.backgroundDescription'),
      onlyAlertOnce: true,
    ),
    iosNotificationOptions: const IOSNotificationOptions(
      showNotification: false,
      playSound: false,
    ),
    foregroundTaskOptions: ForegroundTaskOptions(
      eventAction: ForegroundTaskEventAction.repeat(5000),
      autoRunOnBoot: false,
      autoRunOnMyPackageReplaced: false,
      allowWakeLock: false,
      allowAutoRestart: false,
    ),
  );
  if (await FlutterForegroundTask.isRunningService) return true;
  final result = await FlutterForegroundTask.startService(
    serviceId: 951,
    serviceTypes: [ForegroundServiceTypes.dataSync],
    notificationTitle: 'CoupleLink',
    notificationText: strings.t('notification.backgroundActive'),
    callback: startCoupleBackgroundTask,
  );
  return result is ServiceRequestSuccess;
}

class CoupleBackgroundTask extends TaskHandler {
  bool _busy = false;
  bool _offline = false;
  SharedPreferences? _prefs;
  final _client = http.Client();
  StringBundle? _bundle;

  String _statusText(String key) {
    var locale = 'zh_CN';
    try {
      final options = jsonDecode(
        _prefs?.getString('couplelink.settings.v1') ?? '{}',
      );
      if (options is Map && options['locale'] == 'en') locale = 'en';
    } on FormatException {
      locale = 'zh_CN';
    }
    return _bundle?.locales[locale]?[key] ?? key;
  }

  @override
  Future<void> onStart(DateTime timestamp, TaskStarter starter) async {
    _prefs = await SharedPreferences.getInstance();
    _bundle = await StringBundle.load();
  }

  @override
  void onRepeatEvent(DateTime timestamp) {
    unawaited(_poll());
  }

  @override
  Future<void> onDestroy(DateTime timestamp, bool isTimeout) async {
    _client.close();
  }

  Future<Map<String, dynamic>> _request(
    String origin,
    String token,
    String method,
    String path, [
    Map<String, dynamic>? body,
  ]) async {
    final req = http.Request(method, Uri.parse('$origin$path'))
      ..followRedirects = false;
    req.headers['Authorization'] = 'Bearer $token';
    if (body != null) {
      req.headers['Content-Type'] = 'application/json';
      req.body = jsonEncode(body);
    }
    final response = await _client
        .send(req)
        .timeout(const Duration(seconds: 10));
    final bytes = <int>[];
    await for (final chunk in response.stream.timeout(
      const Duration(seconds: 10),
    )) {
      bytes.addAll(chunk);
      if (bytes.length > 2 * 1024 * 1024) {
        throw const ApiException('invalid_response');
      }
    }
    if (response.statusCode == 401) throw const ApiException('unauthorized');
    if (response.statusCode == 403) {
      throw const ApiException('relationship_changed');
    }
    if (response.statusCode != 200) throw const ApiException('network_failed');
    final data = jsonDecode(utf8.decode(bytes));
    if (data is! Map<String, dynamic>) {
      throw const ApiException('invalid_response');
    }
    return data;
  }

  Future<bool> _stillBackground(
    SharedPreferences prefs,
    String origin,
    String token,
    String user,
    String couple,
  ) async {
    await prefs.reload();
    if (prefs.getBool(_foregroundKey) != false ||
        prefs.getBool('couplelink.session.disabled.v1') == true) {
      return false;
    }
    final raw = await const FlutterSecureStorage().read(
      key: 'couplelink.session.v1',
    );
    if (raw == null) return false;
    final session = jsonDecode(raw);
    return session is Map &&
        session['origin'] == origin &&
        session['token'] == token &&
        (session['user'] as Map?)?['id'] == user &&
        (session['couple'] as Map?)?['id'] == couple;
  }

  Future<void> _poll() async {
    if (_busy || _prefs == null || _bundle == null) return;
    _busy = true;
    try {
      final prefs = _prefs!;
      await prefs.reload();
      if (prefs.getBool(_foregroundKey) != false) return;
      if (prefs.getBool('couplelink.session.disabled.v1') == true) {
        await FlutterForegroundTask.stopService();
        return;
      }
      final raw = prefs.getString(_backgroundConfigKey);
      final sessionRaw = await const FlutterSecureStorage().read(
        key: 'couplelink.session.v1',
      );
      if (raw == null || sessionRaw == null) {
        await FlutterForegroundTask.stopService();
        return;
      }
      final config = jsonDecode(raw) as Map<String, dynamic>,
          session = jsonDecode(sessionRaw) as Map<String, dynamic>;
      final origin = normalizeOrigin(config['origin'] as String),
          user = config['user'] as String,
          couple = config['couple'] as String;
      if (session['origin'] != origin ||
          session['token'] is! String ||
          (session['user'] as Map?)?['id'] != user ||
          (session['couple'] as Map?)?['id'] != couple) {
        await FlutterForegroundTask.stopService();
        return;
      }
      final token = session['token'] as String;
      final account = await _request(origin, token, 'GET', '/account');
      if ((account['user'] as Map?)?['id'] != user ||
          (account['couple'] as Map?)?['id'] != couple) {
        await FlutterForegroundTask.stopService();
        return;
      }
      if (_offline) {
        await FlutterForegroundTask.updateService(
          notificationText: _statusText('notification.backgroundActive'),
        );
        _offline = false;
      }
      final settings = AppStrings(
        _bundle!,
        (jsonDecode(prefs.getString('couplelink.settings.v1') ?? '{}')
                    as Map)['locale'] ==
                'en'
            ? 'en'
            : 'zh_CN',
        const {},
      );
      final options =
          jsonDecode(prefs.getString('couplelink.settings.v1') ?? '{}') as Map;
      final cursorKey =
          'couplelink.background.cursor.${ChatStore.scope(origin, user, couple)}';
      var cursor = max(prefs.getInt(cursorKey) ?? 0, config['cursor'] as int);
      final data = await _request(
        origin,
        token,
        'GET',
        '/messages?couple_id=$couple&after=$cursor',
      );
      if (data['messages'] is! List ||
          data['next_cursor'] is! int ||
          (data['next_cursor'] as int) < cursor) {
        throw const ApiException('invalid_response');
      }
      final received = (data['messages'] as List)
          .whereType<Map<String, dynamic>>()
          .where((m) => m['sender_id'] != user)
          .toList();
      if (!await _stillBackground(prefs, origin, token, user, couple)) return;
      if (received.isNotEmpty) {
        if (!await coupleNotifications.show(
          id: 952,
          title: settings.t('notification.messageTitle'),
          body: settings.t('notification.messageBody'),
        )) {
          return;
        }
      }
      cursor = data['next_cursor'] as int;
      if (!await prefs.setInt(cursorKey, cursor)) {
        throw const ApiException('preferences_failed');
      }
      if (!await coupleNotifications.enabled()) return;
      if (!await _stillBackground(prefs, origin, token, user, couple)) return;
      final watch = Stopwatch()..start();
      final claimed = await _request(
        origin,
        token,
        'POST',
        '/interactions/claim',
        {'couple_id': couple},
      );
      final item = claimed['interaction'];
      if (item is Map<String, dynamic> &&
          item['id'] is String &&
          item['action'] is String &&
          claimed['remaining_ms'] is int) {
        final action = item['action'] as String;
        const keys = {
          'poke': 'interaction.poke',
          'miss_you': 'interaction.miss',
          'hug': 'interaction.hug',
          'ring': 'interaction.ring',
          'vibrate': 'interaction.vibrate',
        };
        if (!keys.containsKey(action)) {
          throw const ApiException('invalid_response');
        }
        final sound =
            action == 'ring' && options['allowInteractionSound'] == true;
        final vibration =
            action != 'ring' && options['allowInteractionVibration'] != false;
        final fresh =
            watch.elapsedMilliseconds < (claimed['remaining_ms'] as int);
        var displayed = false;
        if (fresh &&
            await _stillBackground(prefs, origin, token, user, couple)) {
          displayed = await coupleNotifications.show(
            id: 953,
            title: settings.t('notification.interactionTitle'),
            body: settings.t(keys[action]!),
            sound: sound,
            vibrate: vibration,
            category: 'interactions',
          );
        }
        await _request(origin, token, 'POST', '/interactions/result', {
          'id': item['id'],
          'couple_id': couple,
          'result': displayed && (sound || vibration)
              ? 'requested'
              : 'suppressed',
        });
      }
    } catch (error) {
      if (error is ApiException &&
          (error.code == 'unauthorized' ||
              error.code == 'relationship_changed')) {
        await FlutterForegroundTask.stopService();
      } else {
        if (!_offline) {
          await FlutterForegroundTask.updateService(
            notificationText: _statusText('notification.backgroundRetry'),
          );
          _offline = true;
        }
      }
    } finally {
      _busy = false;
    }
  }
}
