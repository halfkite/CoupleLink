import 'dart:convert';

import 'package:drift/drift.dart';
import 'package:drift_flutter/drift_flutter.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'connection.dart';

class ChatRow {
  const ChatRow({
    required this.id,
    required this.coupleId,
    required this.senderId,
    required this.body,
    required this.createdAt,
    this.seq,
    this.status = 'queued',
    this.error,
  });
  final String id, coupleId, senderId, body, status;
  final String? error;
  final int? seq;
  final DateTime createdAt;
  factory ChatRow.server(dynamic value, String coupleId) {
    if (value is! Map<String, dynamic> ||
        value['id'] is! String ||
        value['couple_id'] != coupleId ||
        value['sender_id'] is! String ||
        value['body'] is! String ||
        value['seq'] is! int ||
        value['seq'] < 1 ||
        value['delivered'] is! bool ||
        value['created_at'] is! String) {
      throw const ApiException('invalid_response');
    }
    final date = DateTime.tryParse(value['created_at'] as String);
    if (date == null || (value['body'] as String).runes.length > 4000) {
      throw const ApiException('invalid_response');
    }
    return ChatRow(
      id: value['id'] as String,
      coupleId: coupleId,
      senderId: value['sender_id'] as String,
      body: value['body'] as String,
      seq: value['seq'] as int,
      createdAt: date,
      status: value['delivered'] == true ? 'delivered' : 'sent',
    );
  }
  factory ChatRow.cached(QueryRow row) => ChatRow(
    id: row.read<String>('id'),
    coupleId: row.read<String>('couple_id'),
    senderId: row.read<String>('sender_id'),
    body: row.read<String>('body'),
    seq: row.readNullable<int>('seq'),
    createdAt: DateTime.fromMillisecondsSinceEpoch(
      row.read<int>('created_at'),
      isUtc: true,
    ),
    status: row.read<String>('status'),
    error: row.readNullable<String>('error'),
  );
}

class ChatDatabase extends GeneratedDatabase {
  ChatDatabase(super.executor);
  ChatDatabase.defaults()
    : super(
        driftDatabase(
          name: 'couplelink_chat_v1',
          native: const DriftNativeOptions(shareAcrossIsolates: true),
          web: DriftWebOptions(
            sqlite3Wasm: Uri.parse('sqlite3.wasm'),
            driftWorker: Uri.parse('drift_worker.js'),
          ),
        ),
      );
  @override
  int get schemaVersion => 2;
  @override
  Iterable<TableInfo<Table, dynamic>> get allTables => const [];
  @override
  List<DatabaseSchemaEntity> get allSchemaEntities => const [];
  @override
  MigrationStrategy get migration => MigrationStrategy(
    onCreate: (m) async {
      await customStatement(
        'CREATE TABLE chat_messages(scope TEXT NOT NULL,id TEXT NOT NULL,couple_id TEXT NOT NULL,sender_id TEXT NOT NULL,body TEXT NOT NULL,seq INTEGER,status TEXT NOT NULL,created_at INTEGER NOT NULL,error TEXT,PRIMARY KEY(scope,id))',
      );
      await customStatement(
        'CREATE INDEX chat_scope_seq ON chat_messages(scope,seq)',
      );
      await customStatement(
        'CREATE TABLE chat_state(scope TEXT PRIMARY KEY,cursor INTEGER NOT NULL DEFAULT 0,draft TEXT NOT NULL DEFAULT "",last_read INTEGER NOT NULL DEFAULT 0)',
      );
    },
    onUpgrade: (m, from, to) async {
      if (from < 2) {
        await customStatement(
          'ALTER TABLE chat_state ADD COLUMN last_read INTEGER NOT NULL DEFAULT 0',
        );
        // Existing history predates unread tracking; don't mark it all new.
        await customStatement('UPDATE chat_state SET last_read=cursor');
      }
    },
  );
}

class ChatStore {
  ChatStore(
    this.db, {
    this.maxPending = 2000,
    this.maxPendingBytes = 16 * 1024 * 1024,
    this.maxCached = 2000,
  }) {
    if (maxPending < 1 || maxPendingBytes < 1 || maxCached < 100) {
      throw ArgumentError('Invalid chat storage limits');
    }
  }
  final ChatDatabase db;
  final int maxPending, maxPendingBytes, maxCached;
  static String scope(String origin, String user, String couple) =>
      base64Url.encode(utf8.encode('$origin|$user|$couple'));
  Future<List<ChatRow>> rows(String scope) async =>
      (await db
              .customSelect(
                'SELECT * FROM chat_messages WHERE scope=? ORDER BY seq IS NULL,seq,created_at,id',
                variables: [Variable(scope)],
              )
              .get())
          .map(ChatRow.cached)
          .toList();
  Future<int> cursor(String scope) async =>
      (await db
              .customSelect(
                'SELECT cursor FROM chat_state WHERE scope=?',
                variables: [Variable(scope)],
              )
              .getSingleOrNull())
          ?.read<int>('cursor') ??
      0;
  Future<String> draft(String scope) async =>
      (await db
              .customSelect(
                'SELECT draft FROM chat_state WHERE scope=?',
                variables: [Variable(scope)],
              )
              .getSingleOrNull())
          ?.read<String>('draft') ??
      '';
  Future<bool> hasUnread(String scope, String user) async =>
      (await db
              .customSelect(
                'SELECT EXISTS(SELECT 1 FROM chat_messages WHERE scope=? AND sender_id<>? AND seq IS NOT NULL AND seq>COALESCE((SELECT last_read FROM chat_state WHERE scope=?),0)) AS unread',
                variables: [Variable(scope), Variable(user), Variable(scope)],
              )
              .getSingle())
          .read<int>('unread') !=
      0;
  Future<void> markRead(String scope) => db.customStatement(
    'INSERT INTO chat_state(scope,last_read) VALUES(?,0) ON CONFLICT(scope) DO UPDATE SET last_read=max(chat_state.last_read,chat_state.cursor)',
    [scope],
  );
  Future<void> saveDraft(String scope, String draft) => db.customStatement(
    'INSERT INTO chat_state(scope,draft) VALUES(?,?) ON CONFLICT(scope) DO UPDATE SET draft=excluded.draft',
    [scope, draft],
  );
  Future<void> enqueue(String scope, ChatRow row) => db.transaction(() async {
    final size = await db
        .customSelect(
          "SELECT count(*) AS amount,COALESCE(sum(length(CAST(body AS BLOB))),0) AS bytes FROM chat_messages WHERE scope=? AND seq IS NULL",
          variables: [Variable(scope)],
        )
        .getSingle();
    if (size.read<int>('amount') >= maxPending ||
        size.read<int>('bytes') + utf8.encode(row.body).length >
            maxPendingBytes) {
      throw const ApiException('queue_full');
    }
    await db.customStatement(
      'INSERT INTO chat_messages(scope,id,couple_id,sender_id,body,status,created_at) VALUES(?,?,?,?,?,?,?)',
      [
        scope,
        row.id,
        row.coupleId,
        row.senderId,
        row.body,
        'queued',
        row.createdAt.millisecondsSinceEpoch,
      ],
    );
    await saveDraft(scope, '');
  });
  Future<void> merge(
    String scope,
    List<ChatRow> messages, {
    int? cursor,
    int deliveredThrough = 0,
    required String user,
  }) => db.transaction(() async {
    for (final row in messages) {
      await db.customStatement(
        "INSERT INTO chat_messages(scope,id,couple_id,sender_id,body,seq,status,created_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(scope,id) DO UPDATE SET seq=excluded.seq,status=CASE WHEN chat_messages.status='delivered' THEN 'delivered' ELSE excluded.status END,created_at=excluded.created_at,error=NULL",
        [
          scope,
          row.id,
          row.coupleId,
          row.senderId,
          row.body,
          row.seq,
          row.status,
          row.createdAt.millisecondsSinceEpoch,
        ],
      );
    }
    if (deliveredThrough > 0) {
      await db.customStatement(
        "UPDATE chat_messages SET status='delivered' WHERE scope=? AND sender_id=? AND seq<=?",
        [scope, user, deliveredThrough],
      );
    }
    if (cursor != null) {
      await db.customStatement(
        'INSERT INTO chat_state(scope,cursor) VALUES(?,?) ON CONFLICT(scope) DO UPDATE SET cursor=max(chat_state.cursor,excluded.cursor)',
        [scope, cursor],
      );
    }
    await db.customStatement(
      'DELETE FROM chat_messages WHERE scope=? AND seq IS NOT NULL AND id NOT IN (SELECT id FROM chat_messages WHERE scope=? AND seq IS NOT NULL ORDER BY seq DESC LIMIT ?)',
      [scope, scope, maxCached],
    );
  });
  Future<void> fail(String scope, String id, String error) =>
      db.customStatement(
        "UPDATE chat_messages SET status='failed',error=? WHERE scope=? AND id=? AND seq IS NULL",
        [error, scope, id],
      );
  Future<void> retry(String scope, String id) => db.customStatement(
    "UPDATE chat_messages SET status='queued',error=NULL WHERE scope=? AND id=? AND seq IS NULL",
    [scope, id],
  );
  Future<void> removePending(String scope, String id) => db.customStatement(
    'DELETE FROM chat_messages WHERE scope=? AND id=? AND seq IS NULL',
    [scope, id],
  );
  Future<void> purge(String scope) => db.transaction(() async {
    await db.customStatement('DELETE FROM chat_messages WHERE scope=?', [
      scope,
    ]);
    await db.customStatement('DELETE FROM chat_state WHERE scope=?', [scope]);
  });
  Future<void> close() => db.close();
}

final chatStoreProvider = Provider<ChatStore>((ref) {
  final store = ChatStore(ChatDatabase.defaults());
  ref.onDispose(store.close);
  return store;
});
