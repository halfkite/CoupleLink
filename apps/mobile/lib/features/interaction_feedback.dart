import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/device_feedback.dart';
import '../core/interactions.dart';
import '../core/strings.dart';
import '../core/theme.dart';

Future<void> previewGesture(
  BuildContext context,
  AppStrings strings,
  String action,
) => showDialog<void>(
  context: context,
  barrierColor: Colors.black.withValues(alpha: .72),
  builder: (_) => InteractionFeedback(action: action),
);

class InteractionFeedback extends ConsumerStatefulWidget {
  const InteractionFeedback({
    super.key,
    required this.action,
    this.received = false,
    this.receivedOutcome,
  });
  final String action;
  final bool received;
  final FeedbackOutcome? receivedOutcome;
  @override
  ConsumerState<InteractionFeedback> createState() =>
      _InteractionFeedbackState();
}

class _InteractionFeedbackState extends ConsumerState<InteractionFeedback>
    with SingleTickerProviderStateMixin {
  late final animation = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1100),
  );
  late final DeviceFeedback device;
  FeedbackOutcome? outcome;
  bool pending = false;
  int generation = 0;
  @override
  void initState() {
    super.initState();
    device = ref.read(deviceFeedbackProvider);
    outcome = widget.receivedOutcome;
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (context.reducedMotion) {
      animation.value = 1;
    } else {
      animation.forward();
    }
  }

  Future<void> run(bool sound) async {
    final request = ++generation;
    setState(() {
      pending = true;
      outcome = null;
    });
    final result = await device.preview(sound: sound);
    if (!mounted || request != generation) return;
    setState(() {
      outcome = result;
      pending = false;
    });
  }

  @override
  void dispose() {
    generation++;
    unawaited(device.stop());
    animation.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final s = ref.watch(stringsProvider), c = context.colors;
    final sound = widget.action == 'ring';
    final local =
        sound || widget.action == 'vibrate' || widget.action == 'poke';
    final status = pending
        ? 'feedback.pending'
        : outcome == null
        ? 'feedback.ready'
        : widget.received
        ? 'interaction.result.${outcome!.name}'
        : 'feedback.${outcome!.name}';
    return Dialog(
      key: const ValueKey('interaction-feedback-dialog'),
      insetPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 24),
      clipBehavior: Clip.antiAlias,
      child: ConstrainedBox(
        constraints: BoxConstraints(
          maxWidth: 460,
          maxHeight: MediaQuery.sizeOf(context).height * .86,
        ),
        child: SingleChildScrollView(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        s.t(
                          widget.received
                              ? 'interaction.receivedBadge'
                              : 'feedback.localBadge',
                        ),
                        style: TextStyle(
                          color: c.accent,
                          fontSize: 13,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                    IconButton(
                      key: const ValueKey('close-feedback'),
                      tooltip: s.t('common.close'),
                      onPressed: () => Navigator.pop(context),
                      icon: const Icon(Icons.close_rounded),
                    ),
                  ],
                ),
                AnimatedBuilder(
                  animation: animation,
                  builder: (context, _) => SizedBox(
                    height: 168,
                    child: Center(
                      child: Stack(
                        alignment: Alignment.center,
                        children: [
                          for (var i = 0; i < 3; i++)
                            Container(
                              width: 105 + i * 25.0,
                              height: 105 + i * 25.0,
                              decoration: BoxDecoration(
                                shape: BoxShape.circle,
                                border: Border.all(
                                  color: c.accent.withValues(
                                    alpha: .1 + i * .035,
                                  ),
                                  width: 2,
                                ),
                              ),
                            ),
                          Transform.scale(
                            scale: context.reducedMotion
                                ? 1
                                : 1 +
                                      .06 *
                                          math.sin(
                                            animation.value * math.pi * 3,
                                          ),
                            child: Icon(
                              actionIcons[widget.action],
                              size: 78,
                              color: c.accent,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
                Text(
                  s.t(actionKeys[widget.action]!),
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 28,
                    fontWeight: FontWeight.w700,
                    color: c.ink,
                  ),
                ),
                const SizedBox(height: 12),
                Semantics(
                  liveRegion: true,
                  child: Container(
                    key: const ValueKey('feedback-status'),
                    padding: const EdgeInsets.all(16),
                    decoration: BoxDecoration(
                      color: c.soft,
                      borderRadius: BorderRadius.circular(18),
                    ),
                    child: Text(
                      s.t(status),
                      textAlign: TextAlign.center,
                      style: TextStyle(
                        fontSize: 15,
                        height: 1.6,
                        color: c.ink,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                ),
                const SizedBox(height: 14),
                Text(
                  s.t(
                    widget.received
                        ? 'interaction.receivedHint'
                        : 'interaction.previewHint',
                  ),
                  textAlign: TextAlign.center,
                  style: TextStyle(color: c.muted, fontSize: 13, height: 1.6),
                ),
                if (local && !widget.received) ...[
                  const SizedBox(height: 20),
                  FilledButton.icon(
                    key: const ValueKey('run-local-feedback'),
                    onPressed: pending ? null : () => run(sound),
                    icon: Icon(
                      sound ? Icons.volume_up_rounded : Icons.vibration_rounded,
                    ),
                    label: Text(
                      s.t(
                        sound ? 'feedback.soundTest' : 'feedback.vibrationTest',
                      ),
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    s.t(
                      sound ? 'feedback.soundHint' : 'feedback.vibrationHint',
                    ),
                    textAlign: TextAlign.center,
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                ],
                const SizedBox(height: 16),
                OutlinedButton(
                  onPressed: () => Navigator.pop(context),
                  child: Text(s.t('common.done')),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
