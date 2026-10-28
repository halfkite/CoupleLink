import 'dart:async';

import 'package:centrifuge/centrifuge.dart' as realtime;
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'connection.dart';
import 'chat_store.dart';
import 'remote_interactions.dart';
import 'notifications.dart';
import 'strings.dart';

class ChatState {
  const ChatState({
    this.scope = '',
    this.messages = const [],
    this.draft = '',
    this.ready = false,
    this.syncing = false,
    this.online = false,
    this.realtime = false,
    this.history = false,
    this.error,
    this.unread = false,
  });
  final String scope, draft;
  final List<ChatRow> messages;
  final bool ready, syncing, online, realtime, history, unread;
  final String? error;
  int get queued => messages.where((row) => row.status == 'queued').length;
  ChatState copy({
    List<ChatRow>? messages,
    String? draft,
    bool? ready,
    bool? syncing,
    bool? online,
    bool? realtime,
    bool? history,
    String? error,
    bool? unread,
  }) => ChatState(
    scope: scope,
    messages: messages ?? this.messages,
    draft: draft ?? this.draft,
    ready: ready ?? this.ready,
    syncing: syncing ?? this.syncing,
    online: online ?? this.online,
    realtime: realtime ?? this.realtime,
    history: history ?? this.history,
    error: error,
    unread: unread ?? this.unread,
  );
}

final chatProvider = NotifierProvider<ChatController, ChatState>(
  ChatController.new,
);

class ChatController extends Notifier<ChatState> with WidgetsBindingObserver {
  ChatStore? _store;
  Timer? _timer;
  realtime.Client? _socket;
  final _listeners = <StreamSubscription<dynamic>>[];
  int _epoch = 0;
  bool _busy = false, _again = false, _foreground = true, _chatVisible = false;
  DateTime _realtimeAttempt = DateTime.fromMillisecondsSinceEpoch(0);
  String _scope = '', _user = '', _couple = '';
  static const fallbackInterval = Duration(seconds: 2);
  @override
  ChatState build() {
    WidgetsBinding.instance.addObserver(this);
    ref.onDispose(() {
      _epoch++;
      _timer?.cancel();
      WidgetsBinding.instance.removeObserver(this);
      unawaited(_closeSocket());
    });
    ref.listen(
      connectionProvider.select(
        (c) => (c.origin, c.user?['id'] as String?, c.couple?['id'] as String?),
      ),
      (before, after) {
        final scope = after.$2 != null && after.$3 != null
            ? ChatStore.scope(after.$1, after.$2!, after.$3!)
            : '';
        if (scope != _scope) {
          unawaited(_switch(scope, after.$2 ?? '', after.$3 ?? ''));
        }
      },
    );
    scheduleMicrotask(() {
      if (!ref.mounted) return;
      final c = ref.read(connectionProvider),
          user = c.user?['id'] as String?,
          couple = c.couple?['id'] as String?;
      if (user != null && couple != null) {
        unawaited(
          _switch(ChatStore.scope(c.origin, user, couple), user, couple),
        );
      }
    });
    return const ChatState();
  }

  bool _valid(int epoch) => ref.mounted && epoch == _epoch && _scope.isNotEmpty;
  Future<void> _closeSocket() async {
    final socket = _socket;
    _socket = null;
    final listeners = List<StreamSubscription<dynamic>>.of(_listeners);
    _listeners.clear();
    for (final listener in listeners) {
      await listener.cancel();
    }
    if (socket != null) await socket.close();
  }

  Future<void> _switch(String scope, String user, String couple) async {
    final previous = _scope, previousUser = _user;
    final epoch = ++_epoch;
    _timer?.cancel();
    _scope = scope;
    _user = user;
    _couple = couple;
    _realtimeAttempt = DateTime.fromMillisecondsSinceEpoch(0);
    state = ChatState(scope: scope);
    try {
      await _closeSocket();
      // An authenticated relationship change cancels old drafts and pending
      // sends. Losing authentication only hides them until the same account
      // and relationship are verified again; network errors never delete them.
      if (previous.isNotEmpty &&
          user.isNotEmpty &&
          previousUser == user &&
          previous != scope &&
          _store != null) {
        await _store!.purge(previous);
      }
      if (scope.isEmpty || !_valid(epoch)) return;
      _store ??= ref.read(chatStoreProvider);
      final rows = await _store!.rows(scope),
          draft = await _store!.draft(scope);
      if (!_valid(epoch)) return;
      final unread = await _store!.hasUnread(scope, user);
      if (!_valid(epoch)) return;
      state = state.copy(
        messages: rows,
        draft: draft,
        ready: true,
        unread: unread,
      );
      if (_chatVisible && _foreground) unawaited(markRead());
      _arm();
      unawaited(sync());
    } catch (_) {
      if (_valid(epoch)) state = state.copy(error: 'chat_storage_failed');
    }
  }

  void _arm() {
    _timer?.cancel();
    if (_foreground && _scope.isNotEmpty) {
      _timer = Timer.periodic(fallbackInterval, (_) => unawaited(sync()));
    }
  }

  void setChatVisible(bool visible) {
    _chatVisible = visible;
    if (visible && _foreground) unawaited(markRead());
  }

  Future<void> markRead() async {
    final epoch = _epoch, scope = _scope, user = _user;
    if (!_valid(epoch) || _store == null) return;
    try {
      await _store!.markRead(scope);
      if (!_valid(epoch)) return;
      final unread = await _store!.hasUnread(scope, user);
      if (_valid(epoch)) state = state.copy(unread: unread, error: state.error);
    } catch (_) {
      if (_valid(epoch)) state = state.copy(error: 'chat_storage_failed');
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    _foreground = state == AppLifecycleState.resumed;
    if (!_foreground) {
      _timer?.cancel();
      if (_socket != null) unawaited(_socket!.disconnect());
    } else {
      _arm();
      unawaited(sync());
      if (_chatVisible) unawaited(markRead());
    }
  }

  Future<void> changeDraft(String value) async {
    final scope = _scope, epoch = _epoch;
    if (scope.isEmpty || _store == null) return;
    state = state.copy(draft: value, error: state.error);
    try {
      await _store!.saveDraft(scope, value);
    } catch (_) {
      if (_valid(epoch)) state = state.copy(error: 'chat_storage_failed');
    }
  }

  Future<bool> send(String text) async {
    final epoch = _epoch, scope = _scope, user = _user, couple = _couple;
    if (!_valid(epoch) || !state.ready || _store == null) return false;
    if (text.trim().isEmpty || text.runes.length > 4000) {
      state = state.copy(error: 'invalid_message');
      return false;
    }
    try {
      await _store!.enqueue(
        scope,
        ChatRow(
          id: newDeviceID(),
          coupleId: couple,
          senderId: user,
          body: text,
          createdAt: DateTime.now().toUtc(),
        ),
      );
      if (!_valid(epoch)) return false;
      final rows = await _store!.rows(scope);
      if (!_valid(epoch)) return false;
      state = state.copy(messages: rows, draft: '', history: false);
      unawaited(sync());
      return true;
    } catch (error) {
      if (_valid(epoch)) {
        state = state.copy(
          error: error is ApiException ? error.code : 'chat_storage_failed',
        );
      }
      return false;
    }
  }

  Future<void> retry(String id) async {
    final epoch = _epoch, scope = _scope;
    if (!_valid(epoch) || _store == null) return;
    try {
      await _store!.retry(scope, id);
      if (!_valid(epoch)) return;
      final rows = await _store!.rows(scope);
      if (!_valid(epoch)) return;
      state = state.copy(messages: rows);
      unawaited(sync());
    } catch (_) {
      if (_valid(epoch)) state = state.copy(error: 'chat_storage_failed');
    }
  }

  Future<void> deletePending(String id) async {
    final epoch = _epoch, scope = _scope;
    if (!_valid(epoch) || _store == null) return;
    try {
      await _store!.removePending(scope, id);
      if (!_valid(epoch)) return;
      final rows = await _store!.rows(scope);
      if (!_valid(epoch)) return;
      state = state.copy(messages: rows);
    } catch (_) {
      if (_valid(epoch)) state = state.copy(error: 'chat_storage_failed');
    }
  }

  Future<void> _connectRealtime(int epoch) async {
    if (_socket != null) {
      if (_socket!.state == realtime.State.disconnected) {
        await _socket!.connect();
      }
      return;
    }
    if (DateTime.now().difference(_realtimeAttempt) <
        const Duration(seconds: 10)) {
      return;
    }
    _realtimeAttempt = DateTime.now();
    try {
      final connection = ref.read(connectionProvider.notifier);
      final credentials = await connection.request('POST', '/realtime/token');
      if (!_valid(epoch)) return;
      if (credentials['token'] is! String ||
          credentials['path'] != '/connection/websocket') {
        throw const ApiException('invalid_response');
      }
      final origin = Uri.parse(ref.read(connectionProvider).origin);
      final endpoint = origin.replace(
        scheme: origin.scheme == 'https' ? 'wss' : 'ws',
        path: '/connection/websocket',
      );
      final socket = realtime.createClient(
        endpoint.toString(),
        realtime.ClientConfig(
          token: credentials['token'] as String,
          name: 'couplelink',
          getToken: (_) async {
            if (!_valid(epoch)) throw realtime.UnauthorizedException();
            final value = await connection.request('POST', '/realtime/token');
            if (value['token'] is! String) {
              throw const ApiException('invalid_response');
            }
            return value['token'] as String;
          },
        ),
      );
      _socket = socket;
      _listeners.addAll([
        socket.connected.listen((_) {
          if (_valid(epoch)) {
            state = state.copy(realtime: true, error: state.error);
            unawaited(sync());
          }
        }),
        socket.disconnected.listen((_) {
          if (_valid(epoch)) {
            state = state.copy(realtime: false, error: state.error);
          }
        }),
        socket.error.listen((_) {
          if (_valid(epoch)) {
            state = state.copy(realtime: false, error: state.error);
          }
        }),
        socket.publication.listen((_) {
          if (_valid(epoch)) {
            unawaited(sync());
            unawaited(ref.read(interactionProvider.notifier).sync());
          }
        }),
      ]);
      await socket.connect();
    } catch (error) {
      if (_valid(epoch)) {
        state = state.copy(realtime: false, error: state.error);
        if (error is ApiException && error.code == 'unauthorized') {
          await ref.read(connectionProvider.notifier).refresh(quiet: true);
        }
      }
    }
  }

  Future<void> sync() async {
    if (!_foreground || _scope.isEmpty || _store == null || !state.ready) {
      return;
    }
    if (_busy) {
      _again = true;
      return;
    }
    _busy = true;
    final epoch = _epoch, scope = _scope, user = _user, couple = _couple;
    if (_valid(epoch)) state = state.copy(syncing: true, error: state.error);
    try {
      final connection = ref.read(connectionProvider.notifier);
      await connection.refresh(quiet: true);
      if (!_valid(epoch)) return;
      final session = ref.read(connectionProvider);
      if (session.error != null) throw ApiException(session.error!);
      // Durable IDs are allocated before any network request. Lost responses
      // retry the same record; no request generates a replacement ID.
      final pending = (await _store!.rows(scope))
          .where((row) => row.status == 'queued')
          .toList();
      for (final row in pending) {
        if (!_valid(epoch) || !_foreground) return;
        try {
          final response = await connection.request(
            'POST',
            '/messages',
            body: {'id': row.id, 'couple_id': couple, 'body': row.body},
            responseLimit: 2 * 1024 * 1024,
          );
          if (!_valid(epoch)) return;
          final message = ChatRow.server(response['message'], couple);
          await _store!.merge(scope, [message], user: user);
        } on ApiException catch (error) {
          if (['invalid_message', 'message_id_conflict'].contains(error.code)) {
            await _store!.fail(scope, row.id, error.code);
            continue;
          }
          rethrow;
        }
      }
      var cursor = await _store!.cursor(scope);
      var receivedNewMessage = false;
      for (var page = 0; page < 100; page++) {
        if (!_valid(epoch) || !_foreground) return;
        final response = await connection.request(
          'GET',
          '/messages?couple_id=$couple&after=$cursor',
          responseLimit: 2 * 1024 * 1024,
        );
        if (!_valid(epoch)) return;
        final rows = _parsePage(response, couple);
        if (rows.any((row) => row.senderId != user)) receivedNewMessage = true;
        final next = response['next_cursor'];
        if (next is! int ||
            next < cursor ||
            (rows.isNotEmpty && rows.first.seq != cursor + 1) ||
            response['peer_delivered_through'] is! int) {
          throw const ApiException('invalid_response');
        }
        await _store!.merge(
          scope,
          rows,
          cursor: next,
          deliveredThrough: response['peer_delivered_through'] as int,
          user: user,
        );
        cursor = next;
        if (response['has_more'] != true) break;
      }
      if (!_valid(epoch)) return;
      // Acknowledge only data already committed to the local database.
      if (cursor > 0) {
        await connection.request(
          'POST',
          '/messages/receipts',
          body: {'couple_id': couple, 'through': cursor},
        );
      }
      if (!_valid(epoch)) return;
      final rows = await _store!.rows(scope);
      if (!_valid(epoch)) return;
      if (_chatVisible && _foreground) await _store!.markRead(scope);
      final unread = await _store!.hasUnread(scope, user);
      if (!_valid(epoch)) return;
      state = state.copy(
        messages: state.history ? state.messages : rows,
        online: true,
        unread: unread,
      );
      if (receivedNewMessage && !_chatVisible && _foreground) {
        try {
          final strings = ref.read(stringsProvider);
          await coupleNotifications.show(
            id: 952,
            title: strings.t('notification.messageTitle'),
            body: strings.t('notification.messageBody'),
          );
        } catch (error, stack) {
          FlutterError.reportError(
            FlutterErrorDetails(
              exception: error,
              stack: stack,
              library: 'CoupleLink notifications',
            ),
          );
        }
      }
      await _connectRealtime(epoch);
    } catch (error) {
      if (!_valid(epoch)) return;
      final code = error is ApiException
          ? error.code
          : error is TimeoutException
          ? 'network_timeout'
          : 'network_failed';
      if (code == 'unauthorized' || code == 'relationship_changed') {
        await ref.read(connectionProvider.notifier).refresh(quiet: true);
      }
      if (_valid(epoch)) state = state.copy(online: false, error: code);
    } finally {
      _busy = false;
      if (_valid(epoch)) state = state.copy(syncing: false, error: state.error);
      if (_again) {
        _again = false;
        unawaited(sync());
      }
    }
  }

  List<ChatRow> _parsePage(Map<String, dynamic> response, String couple) {
    if (response['messages'] is! List ||
        response['head'] is! int ||
        response['has_more'] is! bool) {
      throw const ApiException('invalid_response');
    }
    final rows = (response['messages'] as List)
        .map((row) => ChatRow.server(row, couple))
        .toList();
    for (var i = 1; i < rows.length; i++) {
      if (rows[i].seq != rows[i - 1].seq! + 1) {
        throw const ApiException('invalid_response');
      }
    }
    return rows;
  }

  Future<void> older() async {
    final epoch = _epoch, couple = _couple;
    if (!_valid(epoch) || !state.ready) return;
    final persisted = state.messages.where((row) => row.seq != null).toList();
    if (persisted.isEmpty) return;
    try {
      final response = await ref
          .read(connectionProvider.notifier)
          .request(
            'GET',
            '/messages?couple_id=$couple&before=${persisted.first.seq}',
            responseLimit: 2 * 1024 * 1024,
          );
      if (!_valid(epoch)) return;
      final older = _parsePage(response, couple);
      state = state.copy(
        messages: [...older, ...state.messages].take(2000).toList(),
        history: true,
        error: older.isEmpty ? 'no_older_messages' : null,
      );
    } catch (error) {
      if (_valid(epoch)) {
        state = state.copy(
          error: error is ApiException ? error.code : 'network_failed',
        );
      }
    }
  }

  Future<void> latest() async {
    final epoch = _epoch, scope = _scope;
    if (!_valid(epoch) || _store == null) return;
    try {
      final rows = await _store!.rows(scope);
      if (_valid(epoch)) state = state.copy(messages: rows, history: false);
      unawaited(sync());
    } catch (_) {
      if (_valid(epoch)) state = state.copy(error: 'chat_storage_failed');
    }
  }
}
