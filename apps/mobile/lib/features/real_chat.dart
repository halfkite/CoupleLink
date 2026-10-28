import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/chat_controller.dart';
import '../core/chat_store.dart';
import '../core/connection.dart';
import '../core/strings.dart';
import '../core/theme.dart';
import '../ui/widgets.dart';

class RealChatPage extends ConsumerStatefulWidget {
  const RealChatPage({super.key});
  @override
  ConsumerState<RealChatPage> createState() => _RealChatPageState();
}

class _RealChatPageState extends ConsumerState<RealChatPage> {
  late final input = TextEditingController(text: ref.read(chatProvider).draft);
  final scroll = ScrollController();
  @override
  void dispose() {
    input.dispose();
    scroll.dispose();
    super.dispose();
  }

  void bottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted && scroll.hasClients) {
        scroll.jumpTo(scroll.position.maxScrollExtent);
      }
    });
  }

  Future<void> send() async {
    final accepted = await ref.read(chatProvider.notifier).send(input.text);
    if (!mounted || !accepted) return;
    input.clear();
    bottom();
  }

  @override
  Widget build(BuildContext context) {
    final s = ref.watch(stringsProvider),
        c = context.colors,
        chat = ref.watch(chatProvider),
        connection = ref.watch(connectionProvider),
        controller = ref.read(chatProvider.notifier);
    ref.listen(chatProvider, (previous, next) {
      if (previous?.ready != true && next.ready) {
        input.text = next.draft;
        bottom();
      }
      final last = previous?.messages.lastOrNull?.id;
      if (!next.history &&
          next.messages.lastOrNull?.id != last &&
          (!scroll.hasClients || scroll.position.extentAfter < 120)) {
        bottom();
      }
    });
    final partner =
        (connection.couple?['partner'] as Map?)?['display_name'] ??
        s.t('common.partner');
    final status = chat.online
        ? (chat.realtime ? 'chat.live.online' : 'chat.live.polling')
        : 'chat.live.offline';
    final error = chat.error == null ? null : s.t('chat.error.${chat.error}');
    final errorText = error != null && error.startsWith('chat.error.')
        ? s.t('account.error.${chat.error}')
        : error;
    return Column(
      children: [
        Expanded(
          child: ListView(
            controller: scroll,
            padding: const EdgeInsets.fromLTRB(16, 18, 16, 20),
            children: [
              const PageHeading(
                showPreviewBadge: false,
                title: 'chat.title',
                subtitle: 'chat.live.subtitle',
                padding: EdgeInsets.only(bottom: 18),
              ),
              CoupleCard(
                padding: const EdgeInsets.all(16),
                child: Row(
                  children: [
                    const PersonAvatar(partner: true, size: 42),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            '$partner',
                            style: Theme.of(context).textTheme.titleMedium,
                          ),
                          Text(
                            s.t(status),
                            style: Theme.of(context).textTheme.bodySmall,
                          ),
                        ],
                      ),
                    ),
                    IconButton(
                      key: const ValueKey('chat-refresh'),
                      tooltip: s.t('account.refresh'),
                      onPressed: chat.ready ? controller.sync : null,
                      icon: const Icon(Icons.sync_rounded),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 12),
              Text(
                s.t('chat.live.notice'),
                style: Theme.of(context).textTheme.bodySmall,
              ),
              if (chat.queued > 0)
                Padding(
                  padding: const EdgeInsets.symmetric(vertical: 8),
                  child: Text(
                    s.t('chat.live.queued', {'count': chat.queued.toString()}),
                  ),
                ),
              if (errorText != null)
                Padding(
                  padding: const EdgeInsets.symmetric(vertical: 12),
                  child: Semantics(
                    liveRegion: true,
                    child: Text(
                      errorText,
                      style: TextStyle(
                        color: c.accent,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                ),
              if (!chat.ready) const LinearProgressIndicator(),
              if (chat.ready && chat.messages.isEmpty)
                Padding(
                  padding: const EdgeInsets.symmetric(vertical: 32),
                  child: Text(
                    s.t('chat.live.empty'),
                    textAlign: TextAlign.center,
                  ),
                ),
              if (chat.messages.any((row) => row.seq != null))
                Wrap(
                  spacing: 8,
                  children: [
                    TextButton.icon(
                      onPressed: controller.older,
                      icon: const Icon(Icons.history_rounded),
                      label: Text(s.t('chat.live.older')),
                    ),
                    if (chat.history)
                      TextButton(
                        onPressed: () {
                          controller.latest();
                          bottom();
                        },
                        child: Text(s.t('chat.live.latest')),
                      ),
                  ],
                ),
              for (final row in chat.messages)
                _bubble(row, row.senderId == connection.user?['id']),
            ],
          ),
        ),
        Container(
          padding: const EdgeInsets.fromLTRB(12, 10, 12, 12),
          decoration: BoxDecoration(
            color: c.paper,
            border: Border(top: BorderSide(color: c.line)),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Expanded(
                child: TextField(
                  key: const ValueKey('real-message-input'),
                  controller: input,
                  enabled: chat.ready,
                  minLines: 1,
                  maxLines: 4,
                  maxLength: 4000,
                  textCapitalization: TextCapitalization.sentences,
                  onChanged: controller.changeDraft,
                  decoration: InputDecoration(
                    hintText: s.t('chat.live.input'),
                    counterText: '',
                  ),
                ),
              ),
              const SizedBox(width: 8),
              IconButton.filled(
                key: const ValueKey('real-message-send'),
                tooltip: s.t('chat.live.send'),
                onPressed: chat.ready ? send : null,
                icon: const Icon(Icons.arrow_upward_rounded),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _bubble(ChatRow row, bool mine) {
    final s = ref.read(stringsProvider),
        c = context.colors,
        controller = ref.read(chatProvider.notifier);
    final time = MaterialLocalizations.of(context)
        .formatTimeOfDay(TimeOfDay.fromDateTime(row.createdAt.toLocal()));
    return Align(
      alignment: mine ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        key: ValueKey('chat-message-${row.id}'),
        constraints: BoxConstraints(
          maxWidth: MediaQuery.sizeOf(context).width * .82,
        ),
        margin: const EdgeInsets.only(top: 12, bottom: 6),
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: mine ? c.soft : c.paper,
          borderRadius: BorderRadius.circular(20),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SelectableText(
              row.body,
              semanticsLabel: row.body,
              style: TextStyle(fontSize: 15, height: 1.5, color: c.ink),
            ),
            const SizedBox(height: 7),
            Text(
              mine
                  ? '$time · ${s.t(row.status == 'queued' ? 'chat.live.pending' : 'chat.live.${row.status}')}'
                  : time,
              style: Theme.of(context).textTheme.bodySmall,
            ),
            if (mine && row.seq == null)
              Wrap(
                spacing: 8,
                children: [
                  if (row.status == 'failed')
                    TextButton(
                      onPressed: () => controller.retry(row.id),
                      child: Text(s.t('chat.live.retry')),
                    ),
                  TextButton(
                    onPressed: () => controller.deletePending(row.id),
                    child: Text(s.t('chat.live.cancelPending')),
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }
}
