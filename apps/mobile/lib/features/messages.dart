import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/strings.dart';
import '../core/theme.dart';
import '../ui/widgets.dart';

class MessagesPage extends ConsumerStatefulWidget {
  const MessagesPage({super.key});
  @override
  ConsumerState<MessagesPage> createState() => _MessagesPageState();
}

class _MessagesPageState extends ConsumerState<MessagesPage> {
  final input = TextEditingController(), scroll = ScrollController();
  final localMessages = <String>[];
  @override
  void dispose() {
    input.dispose();
    scroll.dispose();
    super.dispose();
  }

  void add() {
    final value = input.text.trim();
    if (value.isEmpty) return;
    setState(() => localMessages.add(value));
    input.clear();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || !scroll.hasClients) return;
      if (context.reducedMotion) {
        scroll.jumpTo(scroll.position.maxScrollExtent);
      } else {
        scroll.animateTo(
          scroll.position.maxScrollExtent,
          duration: const Duration(milliseconds: 220),
          curve: Curves.easeOut,
        );
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final s = ref.watch(stringsProvider), c = context.colors;
    return Column(
      children: [
        Expanded(
          child: ListView(
            controller: scroll,
            padding: const EdgeInsets.fromLTRB(20, 22, 20, 16),
            children: [
              const PageHeading(
                title: 'chat.title',
                subtitle: 'chat.subtitle',
                padding: EdgeInsets.only(bottom: 24),
              ),
              CoupleCard(
                padding: const EdgeInsets.all(14),
                child: Row(
                  children: [
                    const PersonAvatar(partner: true, size: 42),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            s.t('common.partner'),
                            style: const TextStyle(fontWeight: FontWeight.w600),
                          ),
                          Text(
                            s.t('chat.status'),
                            style: Theme.of(context).textTheme.bodySmall,
                          ),
                        ],
                      ),
                    ),
                    Icon(
                      Icons.favorite_border_rounded,
                      color: c.accent,
                      size: 22,
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 22),
              Text(
                s.t('chat.today'),
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.bodySmall,
              ),
              const SizedBox(height: 20),
              _bubble(s.t('chat.sample1'), false),
              _bubble(s.t('chat.sample2'), true),
              _bubble(s.t('chat.sample3'), false),
              Align(
                alignment: Alignment.centerLeft,
                child: Container(
                  width: 210,
                  margin: const EdgeInsets.only(bottom: 16),
                  padding: const EdgeInsets.all(8),
                  decoration: BoxDecoration(
                    color: c.paper,
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Container(
                        height: 112,
                        decoration: BoxDecoration(
                          borderRadius: BorderRadius.circular(14),
                          gradient: const LinearGradient(
                            begin: Alignment.topCenter,
                            end: Alignment.bottomCenter,
                            colors: [
                              Color(0xFFE8B8C6),
                              Color(0xFFF7D8B5),
                              Color(0xFF9E9DBA),
                            ],
                          ),
                        ),
                        child: Center(
                          child: Icon(
                            Icons.wb_sunny_rounded,
                            size: 50,
                            color: Colors.white.withValues(alpha: .8),
                          ),
                        ),
                      ),
                      Padding(
                        padding: const EdgeInsets.all(7),
                        child: Text(
                          s.t('chat.photo'),
                          style: const TextStyle(fontSize: 11),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              Align(
                alignment: Alignment.centerRight,
                child: OutlinedButton.icon(
                  onPressed: () => toast(context, s.t('chat.voiceLabel')),
                  icon: const Icon(Icons.graphic_eq_rounded),
                  label: Text(s.t('chat.voice')),
                ),
              ),
              for (final message in localMessages)
                _bubble(message, true, local: true),
            ],
          ),
        ),
        Container(
          padding: EdgeInsets.fromLTRB(12, 10, 12, 12),
          decoration: BoxDecoration(
            color: c.paper,
            border: Border(top: BorderSide(color: c.line)),
          ),
          child: Column(
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  IconButton(
                    tooltip: s.t('chat.attachment'),
                    onPressed: () => infoSheet(
                      context,
                      s.t('chat.attachment'),
                      Text(s.t('chat.noPermission')),
                    ),
                    icon: const Icon(Icons.add_circle_outline_rounded),
                  ),
                  Expanded(
                    child: TextField(
                      key: const ValueKey('message-input'),
                      controller: input,
                      minLines: 1,
                      maxLines: 4,
                      maxLength: 2000,
                      textInputAction: TextInputAction.newline,
                      decoration: InputDecoration(
                        hintText: s.t('chat.hint'),
                        hintMaxLines: 1,
                        counterText: '',
                        hintStyle: TextStyle(fontSize: 12, color: c.muted),
                        contentPadding: const EdgeInsets.symmetric(
                          horizontal: 14,
                          vertical: 13,
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: 7),
                  IconButton.filled(
                    key: const ValueKey('send-message'),
                    tooltip: s.t('chat.send'),
                    onPressed: add,
                    icon: const Icon(Icons.arrow_upward_rounded, size: 22),
                  ),
                ],
              ),
              const SizedBox(height: 6),
              Text(
                s.t('chat.local'),
                style: TextStyle(fontSize: 10, color: c.muted),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _bubble(String text, bool mine, {bool local = false}) {
    final c = context.colors;
    return Align(
      alignment: mine ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        constraints: const BoxConstraints(maxWidth: 280),
        margin: const EdgeInsets.only(bottom: 14),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 13),
        decoration: BoxDecoration(
          color: mine ? c.soft : c.paper,
          borderRadius: BorderRadius.only(
            topLeft: const Radius.circular(18),
            topRight: const Radius.circular(18),
            bottomLeft: Radius.circular(mine ? 18 : 5),
            bottomRight: Radius.circular(mine ? 5 : 18),
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(text),
            if (local) ...[
              const SizedBox(height: 4),
              Text(
                ref.watch(stringsProvider).t('chat.local'),
                style: TextStyle(fontSize: 9, color: c.muted),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
