import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'connection.dart';
import 'device_feedback.dart';
import 'interactions.dart';
import 'state.dart';
import 'notifications.dart';
import 'strings.dart';

class ReceivedInteraction {
  const ReceivedInteraction(this.id, this.action, this.outcome);
  final String id, action;
  final FeedbackOutcome outcome;
}

class InteractionState {
  const InteractionState({
    this.incoming,
    this.sent = const [],
    this.error,
    this.sending = false,
  });
  final ReceivedInteraction? incoming;
  final List<Map<String, dynamic>> sent;
  final String? error;
  final bool sending;
  InteractionState copy({
    ReceivedInteraction? incoming,
    bool clearIncoming = false,
    List<Map<String, dynamic>>? sent,
    String? error,
    bool? sending,
  }) => InteractionState(
    incoming: clearIncoming ? null : incoming ?? this.incoming,
    sent: sent ?? this.sent,
    error: error,
    sending: sending ?? this.sending,
  );
}

final interactionProvider =
    NotifierProvider<InteractionController, InteractionState>(
      InteractionController.new,
    );

class InteractionController extends Notifier<InteractionState>
    with WidgetsBindingObserver {
  Timer? _timer;
  bool _foreground = true, _busy = false;
  int _epoch = 0;
  String _scope = '';

  @override
  InteractionState build() {
    WidgetsBinding.instance.addObserver(this);
    ref.onDispose(() {
      _epoch++;
      _timer?.cancel();
      WidgetsBinding.instance.removeObserver(this);
    });
    ref.listen(
      connectionProvider.select(
        (c) => (c.origin, c.user?['id'], c.couple?['id']),
      ),
      (_, next) {
        final scope = next.$2 != null && next.$3 != null
            ? '${next.$1}|${next.$2}|${next.$3}'
            : '';
        if (scope != _scope) {
          _scope = scope;
          _epoch++;
          state = const InteractionState();
          _arm();
        }
      },
    );
    scheduleMicrotask(() {
      if (!ref.mounted) return;
      final c = ref.read(connectionProvider);
      if (c.user != null && c.couple != null) {
        _scope = '${c.origin}|${c.user!['id']}|${c.couple!['id']}';
        _epoch++;
        _arm();
      }
    });
    return const InteractionState();
  }

  bool _valid(int epoch) =>
      ref.mounted && _foreground && _scope.isNotEmpty && epoch == _epoch;
  void _arm() {
    _timer?.cancel();
    if (_foreground && _scope.isNotEmpty) {
      _timer = Timer.periodic(
        const Duration(seconds: 2),
        (_) => unawaited(sync()),
      );
      unawaited(sync());
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    _foreground = state == AppLifecycleState.resumed;
    if (!_foreground) {
      _timer?.cancel();
      unawaited(ref.read(deviceFeedbackProvider).stop());
    } else {
      _arm();
    }
  }

  Future<void> send(String action) async {
    if (!remoteActionIds.contains(action) || state.sending) return;
    final c = ref.read(connectionProvider), couple = c.couple?['id'];
    if (couple is! String || !c.signedIn) {
      state = state.copy(error: 'not_bound');
      return;
    }
    final epoch = _epoch;
    state = state.copy(sending: true);
    try {
      await ref
          .read(connectionProvider.notifier)
          .request(
            'POST',
            '/interactions',
            body: {'id': newDeviceID(), 'couple_id': couple, 'action': action},
          );
      if (_valid(epoch)) {
        state = state.copy(sending: false);
        unawaited(sync());
      }
    } catch (e) {
      if (_valid(epoch)) {
        state = state.copy(
          sending: false,
          error: e is ApiException ? e.code : 'network_failed',
        );
      }
    }
  }

  Future<void> sync() async {
    if (_busy || !_foreground || _scope.isEmpty) return;
    _busy = true;
    final epoch = _epoch, couple = ref.read(connectionProvider).couple?['id'];
    if (couple is! String) {
      _busy = false;
      return;
    }
    try {
      final api = ref.read(connectionProvider.notifier);
      final response = await api.request(
        'GET',
        '/interactions?couple_id=$couple',
      );
      if (!_valid(epoch)) return;
      if (response['interactions'] is! List) {
        throw const ApiException('invalid_response');
      }
      final sent = (response['interactions'] as List)
          .whereType<Map<String, dynamic>>()
          .toList();
      state = state.copy(sent: sent, error: state.error);
      if (state.incoming != null) return;
      final watch = Stopwatch()..start();
      final data = await api.request(
        'POST',
        '/interactions/claim',
        body: {'couple_id': couple},
      );
      if (!_valid(epoch)) return;
      final item = data['interaction'];
      if (item == null) return;
      if (item is! Map<String, dynamic> ||
          item['id'] is! String ||
          item['action'] is! String ||
          !remoteActionIds.contains(item['action']) ||
          data['remaining_ms'] is! int) {
        throw const ApiException('invalid_response');
      }
      final action = item['action'] as String, id = item['id'] as String;
      final settings = ref.read(settingsProvider);
      final allowed = action == 'ring'
          ? settings.allowInteractionSound
          : settings.allowInteractionVibration;
      final fresh = watch.elapsedMilliseconds < (data['remaining_ms'] as int);
      final outcome = allowed && fresh
          ? await ref
                .read(deviceFeedbackProvider)
                .preview(sound: action == 'ring')
          : FeedbackOutcome.suppressed;
      if (!_valid(epoch)) return;
      state = state.copy(
        incoming: ReceivedInteraction(id, action, outcome),
        error: null,
      );
      try {
        if (supportsAndroidNotifications) {
          final strings = ref.read(stringsProvider);
          await coupleNotifications.show(
            id: 953,
            title: strings.t('notification.interactionTitle'),
            body: strings.t(actionKeys[action]!),
            sound: false,
            vibrate: false,
            category: 'interactions',
          );
        }
      } catch (error, stack) {
        FlutterError.reportError(
          FlutterErrorDetails(
            exception: error,
            stack: stack,
            library: 'CoupleLink notifications',
          ),
        );
      }
      try {
        await api.request(
          'POST',
          '/interactions/result',
          body: {'id': id, 'couple_id': couple, 'result': outcome.name},
        );
      } catch (e) {
        if (_valid(epoch)) {
          state = state.copy(
            error: e is ApiException ? e.code : 'network_failed',
          );
        }
      }
    } catch (e) {
      if (_valid(epoch)) {
        state = state.copy(
          error: e is ApiException ? e.code : 'network_failed',
        );
      }
    } finally {
      _busy = false;
    }
  }

  void dismiss() {
    state = state.copy(clearIncoming: true, error: state.error);
    unawaited(sync());
  }
}
