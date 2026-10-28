import 'package:flutter/foundation.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';

bool get supportsAndroidNotifications =>
    !kIsWeb && defaultTargetPlatform == TargetPlatform.android;
final notificationTap = ValueNotifier<String?>(null);

class CoupleNotifications {
  final FlutterLocalNotificationsPlugin _plugin =
      FlutterLocalNotificationsPlugin();
  bool _initialized = false;

  Future<void> initialize() async {
    if (!supportsAndroidNotifications || _initialized) return;
    await _plugin.initialize(
      settings: const InitializationSettings(
        android: AndroidInitializationSettings('ic_couplelink_notification'),
      ),
      onDidReceiveNotificationResponse: (response) {
        notificationTap.value = response.payload;
      },
    );
    final launch = await _plugin.getNotificationAppLaunchDetails();
    if (launch?.didNotificationLaunchApp == true) {
      notificationTap.value = launch?.notificationResponse?.payload;
    }
    _initialized = true;
  }

  Future<bool> requestPermission() async {
    if (!supportsAndroidNotifications) return false;
    await initialize();
    final android = _plugin
        .resolvePlatformSpecificImplementation<
          AndroidFlutterLocalNotificationsPlugin
        >();
    if (android == null) return false;
    return await android.requestNotificationsPermission() ??
        await android.areNotificationsEnabled() ??
        false;
  }

  Future<bool> enabled() async {
    if (!supportsAndroidNotifications) return false;
    await initialize();
    return await _plugin
            .resolvePlatformSpecificImplementation<
              AndroidFlutterLocalNotificationsPlugin
            >()
            ?.areNotificationsEnabled() ??
        false;
  }

  Future<bool> openSettings() async {
    if (!supportsAndroidNotifications) return false;
    await initialize();
    return await _plugin.openAppNotificationSettings() ?? false;
  }

  Future<bool> show({
    required int id,
    required String title,
    required String body,
    bool sound = true,
    bool vibrate = true,
    String category = 'messages',
  }) async {
    if (!await enabled()) return false;
    final variant =
        '${sound ? 'sound' : 'quiet'}_${vibrate ? 'vibrate' : 'still'}';
    await _plugin.show(
      id: id,
      title: title,
      body: body,
      notificationDetails: NotificationDetails(
        android: AndroidNotificationDetails(
          'couplelink_${category}_$variant',
          title,
          channelDescription: 'CoupleLink',
          importance: Importance.high,
          priority: Priority.high,
          playSound: sound,
          enableVibration: vibrate,
          visibility: NotificationVisibility.private,
        ),
      ),
      payload: category,
    );
    return true;
  }
}

final coupleNotifications = CoupleNotifications();
