import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/state.dart';
import '../core/connection.dart';
import '../core/strings.dart';
import '../core/theme.dart';
import '../ui/widgets.dart';

class HomePage extends ConsumerWidget {
  const HomePage({super.key, required this.onMap});
  final VoidCallback onMap;
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final s = ref.watch(stringsProvider),
        settings = ref.watch(settingsProvider);
    final visible = settings.order
        .where((id) => !settings.hidden.contains(id))
        .toList();
    return CustomScrollView(
      key: const PageStorageKey('home-scroll'),
      slivers: [
        SliverToBoxAdapter(
          child: PageHeading(
            title: 'app.space',
            subtitle: 'home.subtitle',
            trailing: IconButton(
              key: const ValueKey('edit-home'),
              tooltip: s.t('home.edit'),
              icon: const Icon(Icons.tune_rounded, size: 19),
              onPressed: () => showLayoutEditor(context),
            ),
          ),
        ),
        SliverPadding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          sliver: SliverList.list(
            children: [
              if (visible.isEmpty)
                CoupleCard(
                  child: Column(
                    children: [
                      Icon(
                        Icons.spa_outlined,
                        size: 48,
                        color: context.colors.accent,
                      ),
                      const SizedBox(height: 16),
                      Text(s.t('home.empty')),
                      const SizedBox(height: 8),
                      Text(s.t('home.emptyHint')),
                      TextButton(
                        onPressed: () => showLayoutEditor(context),
                        child: Text(s.t('home.edit')),
                      ),
                    ],
                  ),
                ),
              for (final id in visible)
                Padding(
                  padding: const EdgeInsets.only(bottom: 16),
                  child: GestureDetector(
                    key: ValueKey('home-card-$id'),
                    onLongPress: () => showLayoutEditor(context),
                    child: switch (id) {
                      'days' => const _DaysCard(),
                      'interactions' => const _InteractionsCard(),
                      'distance' => _DistanceCard(onTap: onMap),
                      'pet' => const _PetCard(),
                      'meeting' => const _MeetingCard(),
                      'battery' => const _BatteryCard(),
                      _ => const SizedBox.shrink(),
                    },
                  ),
                ),
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 6, 20, 8),
                child: Text(
                  s.t('home.today'),
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 12,
                    color: context.colors.muted,
                    letterSpacing: 1,
                  ),
                ),
              ),
              Text(
                s.t('app.demoNote'),
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: 10, color: context.colors.muted),
              ),
              const SizedBox(height: 24),
            ],
          ),
        ),
      ],
    );
  }
}

class _DaysCard extends ConsumerWidget {
  const _DaysCard();
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final s = ref.watch(stringsProvider), c = context.colors;
    final connection = ref.watch(connectionProvider),
        couple = ref.watch(connectionProvider).couple;
    final names = couple == null
        ? s.t('common.couple')
        : '${connection.user!['display_name']} & ${(couple['partner'] as Map)['display_name']}';
    return ClipRRect(
      borderRadius: BorderRadius.circular(26),
      child: DecoratedBox(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [c.heroStart, c.heroEnd],
          ),
        ),
        child: Stack(
          children: [
            Positioned(
              right: -32,
              top: -30,
              child: Icon(
                Icons.favorite_rounded,
                size: 180,
                color: c.paper.withValues(alpha: .2),
              ),
            ),
            Padding(
              padding: const EdgeInsets.all(22),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      const PersonAvatar(partner: false, size: 39),
                      Transform.translate(
                        offset: const Offset(-8, 0),
                        child: const PersonAvatar(partner: true, size: 39),
                      ),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              names,
                              style: TextStyle(
                                fontSize: 12,
                                fontWeight: FontWeight.w700,
                                color: c.ink,
                              ),
                            ),
                            Text(
                              s.t('home.connected'),
                              style: TextStyle(fontSize: 10, color: c.muted),
                            ),
                          ],
                        ),
                      ),
                      Icon(
                        Icons.auto_awesome_rounded,
                        size: 20,
                        color: c.accent,
                      ),
                    ],
                  ),
                  const SizedBox(height: 26),
                  Text(
                    s.t('home.daysLabel'),
                    style: TextStyle(
                      fontSize: 13,
                      color: c.ink.withValues(alpha: .7),
                    ),
                  ),
                  Wrap(
                    crossAxisAlignment: WrapCrossAlignment.end,
                    spacing: 10,
                    children: [
                      Text(
                        couple == null ? '365' : '${couple['days_together']}',
                        style: TextStyle(
                          fontSize: 76,
                          height: 1.2,
                          fontWeight: FontWeight.w300,
                          letterSpacing: -4,
                          color: c.ink,
                        ),
                      ),
                      Padding(
                        padding: const EdgeInsets.only(bottom: 12),
                        child: Text(
                          s.t('home.daysUnit'),
                          style: TextStyle(fontSize: 15, color: c.ink),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 3),
                  Text(
                    couple == null
                        ? s.t('home.daysFoot')
                        : s.t('account.realStart', {
                            'date': '${couple['start_at']}',
                          }),
                    style: TextStyle(
                      fontSize: 11,
                      color: c.ink.withValues(alpha: .65),
                    ),
                  ),
                  const SizedBox(height: 22),
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 9,
                    ),
                    decoration: BoxDecoration(
                      color: c.paper.withValues(alpha: .5),
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Row(
                      children: [
                        Icon(Icons.cake_outlined, size: 16, color: c.accent),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            couple == null
                                ? s.t('home.anniversary')
                                : s.t(
                                    connection.error == null
                                        ? 'account.bound'
                                        : 'account.cached',
                                  ),
                            style: TextStyle(fontSize: 11, color: c.ink),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _InteractionsCard extends ConsumerWidget {
  const _InteractionsCard();
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final s = ref.watch(stringsProvider), c = context.colors;
    return CoupleCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SectionLabel(
            s.t('home.interactions'),
            subtitle: s.t('home.interactionSubtitle'),
            trailing: Icon(
              Icons.favorite_border_rounded,
              size: 19,
              color: c.accent,
            ),
          ),
          const SizedBox(height: 18),
          LayoutBuilder(
            builder: (context, bounds) => Wrap(
              spacing: 8,
              runSpacing: 12,
              children: [
                for (final id in remoteActionIds)
                  SizedBox(
                    width: (bounds.maxWidth - 24) / 4,
                    child: Semantics(
                      label: s.t(actionKeys[id]!),
                      button: true,
                      child: InkWell(
                        key: ValueKey('gesture-$id'),
                        onTap: () => previewGesture(context, s, id),
                        borderRadius: BorderRadius.circular(18),
                        child: Padding(
                          padding: const EdgeInsets.symmetric(vertical: 5),
                          child: Column(
                            children: [
                              Container(
                                width: 52,
                                height: 52,
                                decoration: BoxDecoration(
                                  borderRadius: BorderRadius.circular(18),
                                  color:
                                      (id == 'miss_you'
                                              ? c.accent
                                              : c.secondary)
                                          .withValues(alpha: .1),
                                ),
                                child: Icon(
                                  actionIcons[id],
                                  color: id == 'miss_you'
                                      ? c.accent
                                      : c.secondary,
                                  size: 24,
                                ),
                              ),
                              const SizedBox(height: 9),
                              ExcludeSemantics(
                                child: Text(
                                  s.t(actionKeys[id]!),
                                  maxLines: 2,
                                  overflow: TextOverflow.ellipsis,
                                  textAlign: TextAlign.center,
                                  style: TextStyle(fontSize: 11, color: c.ink),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ),
          const SizedBox(height: 14),
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              key: const ValueKey('gesture-vibrate'),
              onPressed: () => previewGesture(context, s, 'vibrate'),
              icon: const Icon(Icons.vibration_rounded, size: 19),
              label: Text(s.t('feedback.vibrationTest')),
            ),
          ),
        ],
      ),
    );
  }
}

class _DistanceCard extends ConsumerWidget {
  const _DistanceCard({required this.onTap});
  final VoidCallback onTap;
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final s = ref.watch(stringsProvider), c = context.colors;
    return CoupleCard(
      onTap: onTap,
      child: Column(
        children: [
          SectionLabel(
            s.t('home.distance'),
            trailing: Icon(Icons.north_east_rounded, size: 18, color: c.muted),
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              const PersonAvatar(partner: false, size: 44),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  children: [
                    Text(
                      '8.6 km',
                      style: TextStyle(
                        fontSize: 27,
                        fontWeight: FontWeight.w600,
                        color: c.ink,
                      ),
                    ),
                    const SizedBox(height: 3),
                    Row(
                      children: [
                        Expanded(child: Divider(color: c.line)),
                        Padding(
                          padding: const EdgeInsets.symmetric(horizontal: 8),
                          child: Icon(
                            Icons.favorite_rounded,
                            size: 12,
                            color: c.accent,
                          ),
                        ),
                        Expanded(child: Divider(color: c.line)),
                      ],
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 12),
              const PersonAvatar(partner: true, size: 44),
            ],
          ),
          const SizedBox(height: 12),
          Text(
            s.t('home.updated'),
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
      ),
    );
  }
}

class _PetCard extends ConsumerWidget {
  const _PetCard();
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final s = ref.watch(stringsProvider), c = context.colors;
    return CoupleCard(
      onTap: () => showPetHome(context),
      child: Row(
        children: [
          const PetArt(size: 94),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  s.t('home.pet'),
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 4),
                Text(
                  s.t('home.petLevel'),
                  style: Theme.of(context).textTheme.bodySmall,
                ),
                const SizedBox(height: 14),
                Row(
                  children: [
                    Text(
                      s.t('home.petFood'),
                      style: TextStyle(fontSize: 10, color: c.muted),
                    ),
                    const Spacer(),
                    Text(
                      '80%',
                      style: TextStyle(fontSize: 10, color: c.accent),
                    ),
                  ],
                ),
                const SizedBox(height: 5),
                ClipRRect(
                  borderRadius: BorderRadius.circular(5),
                  child: LinearProgressIndicator(
                    value: .8,
                    backgroundColor: c.soft,
                    color: c.accent,
                    minHeight: 5,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 6),
          Icon(Icons.chevron_right_rounded, color: c.muted, size: 18),
        ],
      ),
    );
  }
}

class _MeetingCard extends ConsumerWidget {
  const _MeetingCard();
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final s = ref.watch(stringsProvider), c = context.colors;
    return CoupleCard(
      child: Row(
        children: [
          IconTile(
            icon: Icons.calendar_month_rounded,
            color: c.secondary,
            size: 48,
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  s.t('home.meeting'),
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 5),
                Text(
                  s.t('home.meetingNote'),
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ],
            ),
          ),
          const SizedBox(width: 10),
          Column(
            children: [
              Text(
                '12',
                style: TextStyle(
                  fontSize: 32,
                  fontWeight: FontWeight.w500,
                  color: c.secondary,
                ),
              ),
              Text(
                s.t('home.meetingUnit'),
                style: TextStyle(fontSize: 10, color: c.muted),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _BatteryCard extends ConsumerWidget {
  const _BatteryCard();
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final s = ref.watch(stringsProvider), c = context.colors;
    return CoupleCard(
      child: Row(
        children: [
          IconTile(
            icon: Icons.battery_charging_full_rounded,
            color: const Color(0xFF76A894),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  s.t('home.battery'),
                  style: const TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                Text(
                  s.t('home.batteryNote'),
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ],
            ),
          ),
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Text('74%', style: TextStyle(fontSize: 20, color: c.ink)),
              Text(
                s.t('home.charging'),
                style: TextStyle(fontSize: 10, color: c.muted),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

Future<void> showLayoutEditor(BuildContext context) =>
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      builder: (context) => Consumer(
        builder: (context, ref, _) {
          final s = ref.watch(stringsProvider),
              state = ref.watch(settingsProvider),
              controller = ref.read(settingsProvider.notifier);
          return SizedBox(
            height: MediaQuery.sizeOf(context).height * .78,
            child: Padding(
              padding: const EdgeInsets.fromLTRB(20, 0, 20, 16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    s.t('home.layoutTitle'),
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    s.t('home.layoutSubtitle'),
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                  const SizedBox(height: 12),
                  Expanded(
                    child: ReorderableListView.builder(
                      buildDefaultDragHandles: false,
                      itemCount: state.order.length,
                      onReorderItem: controller.reorder,
                      itemBuilder: (context, i) {
                        final id = state.order[i];
                        return ListTile(
                          key: ValueKey(id),
                          contentPadding: const EdgeInsets.symmetric(
                            vertical: 4,
                          ),
                          leading: ReorderableDragStartListener(
                            index: i,
                            child: Tooltip(
                              message: s.t('home.layoutSubtitle'),
                              child: const SizedBox(
                                width: 48,
                                height: 48,
                                child: Icon(Icons.drag_indicator_rounded),
                              ),
                            ),
                          ),
                          title: Text(s.t('card.$id')),
                          trailing: Switch(
                            value: !state.hidden.contains(id),
                            onChanged: (_) => controller.toggleCard(id),
                          ),
                        );
                      },
                    ),
                  ),
                  Text(
                    s.t('home.layoutHint'),
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      TextButton(
                        onPressed: controller.resetLayout,
                        child: Text(s.t('common.reset')),
                      ),
                      const Spacer(),
                      FilledButton(
                        onPressed: () => Navigator.pop(context),
                        child: Text(s.t('common.done')),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
void showPetHome(BuildContext context) {
  infoSheet(
    context,
    '',
    Consumer(
      builder: (context, ref, _) {
        final s = ref.watch(stringsProvider);
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              s.t('pet.title'),
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 12),
            Text(s.t('pet.subtitle')),
            const Center(child: PetArt(size: 200)),
            Text(
              s.t('pet.preview'),
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodySmall,
            ),
            const SizedBox(height: 20),
            Wrap(
              alignment: WrapAlignment.center,
              spacing: 12,
              runSpacing: 12,
              children: [
                FilledButton.icon(
                  onPressed: () => toast(context, s.t('pet.fed')),
                  icon: const Icon(Icons.rice_bowl_rounded),
                  label: Text(s.t('pet.feed')),
                ),
                OutlinedButton.icon(
                  onPressed: () => toast(context, s.t('pet.played')),
                  icon: const Icon(Icons.sports_baseball_outlined),
                  label: Text(s.t('pet.play')),
                ),
              ],
            ),
          ],
        );
      },
    ),
  );
}
