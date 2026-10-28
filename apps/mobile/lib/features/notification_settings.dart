import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_foreground_task/flutter_foreground_task.dart';

import '../core/background_notifications.dart';
import '../core/chat_controller.dart';
import '../core/chat_store.dart';
import '../core/connection.dart';
import '../core/notifications.dart';
import '../core/state.dart';
import '../core/strings.dart';

class NotificationSettings extends ConsumerStatefulWidget {
  const NotificationSettings({super.key});
  @override
  ConsumerState<NotificationSettings> createState() =>
      _NotificationSettingsState();
}

class _NotificationSettingsState extends ConsumerState<NotificationSettings> {
  bool busy = false, running = false;
  String? result;
  @override
  void initState() {
    super.initState();
    _refresh();
  }

  Future<void> _refresh() async {
    if (!supportsAndroidNotifications) return;
    final value = await FlutterForegroundTask.isRunningService;
    if (mounted) setState(() => running = value);
  }

  Future<void> _run(String operation) async {
    setState(() {
      busy = true;
      result = null;
    });
    try {
      if (operation == 'permission') {
        result = await coupleNotifications.requestPermission()
            ? 'notification.permissionGranted'
            : 'notification.permissionDenied';
      } else if (operation == 'test') {
        final s = ref.read(stringsProvider);
        result =
            await coupleNotifications.show(
              id: 954,
              title: s.t('notification.testTitle'),
              body: s.t('notification.testBody'),
            )
            ? 'notification.testSent'
            : 'notification.permissionDenied';
      } else if (operation == 'settings') {
        result = await coupleNotifications.openSettings()
            ? null
            : 'notification.backgroundFailed';
      } else if (operation == 'start') {
        if (!ref.read(chatProvider).ready) {
          result = 'notification.pairFirst';
        } else {
          final started = await startBackgroundNotifications(
            ref.read(preferencesProvider),
            ref.read(chatStoreProvider),
            ref.read(connectionProvider),
            ref.read(stringsProvider),
          );
          result = started
              ? 'notification.backgroundStarted'
              : 'notification.backgroundFailed';
          running = started;
        }
      } else {
        final stopped = await FlutterForegroundTask.stopService();
        running = stopped is! ServiceRequestSuccess;
        result = running
            ? 'notification.backgroundFailed'
            : 'notification.backgroundStopped';
      }
    } catch (_) {
      result = 'notification.backgroundFailed';
    }
    if (mounted) setState(() => busy = false);
  }

  @override
  Widget build(BuildContext context) {
    final s = ref.watch(stringsProvider);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: 12),
        Text(
          s.t('notification.title'),
          style: Theme.of(context).textTheme.titleMedium,
        ),
        Text(
          s.t('notification.hint'),
          style: Theme.of(context).textTheme.bodySmall,
        ),
        Wrap(
          spacing: 8,
          runSpacing: 4,
          children: [
            OutlinedButton(
              onPressed: busy ? null : () => _run('permission'),
              child: Text(s.t('notification.permission')),
            ),
            OutlinedButton(
              onPressed: busy ? null : () => _run('test'),
              child: Text(s.t('notification.test')),
            ),
            OutlinedButton(
              onPressed: busy ? null : () => _run('settings'),
              child: Text(s.t('notification.settings')),
            ),
            OutlinedButton(
              onPressed: busy ? null : () => _run(running ? 'stop' : 'start'),
              child: Text(
                s.t(
                  running
                      ? 'notification.stopBackground'
                      : 'notification.startBackground',
                ),
              ),
            ),
          ],
        ),
        if (result != null)
          Semantics(liveRegion: true, child: Text(s.t(result!))),
        const SizedBox(height: 8),
      ],
    );
  }
}
