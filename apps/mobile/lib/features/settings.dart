import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/state.dart';
import '../core/strings.dart';
import '../core/theme.dart';
import '../ui/widgets.dart';
import 'home.dart';
import 'account.dart';
import '../core/connection.dart';

class SettingsPage extends ConsumerWidget {
  const SettingsPage({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final s = ref.watch(stringsProvider), c = context.colors;
    final state = ref.watch(settingsProvider),
        controller = ref.read(settingsProvider.notifier);
    final connection = ref.watch(connectionProvider);
    return ListView(
      key: const PageStorageKey('settings-scroll'),
      padding: EdgeInsets.zero,
      children: [
        const PageHeading(title: 'me.title', subtitle: 'me.subtitle'),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Semantics(
                container: true,
                button: true,
                child: CoupleCard(
                  onTap: () => openAccount(context),
                  child: Row(
                    children: [
                      const PersonAvatar(partner: false, size: 54),
                      Transform.translate(
                        offset: const Offset(-10, 0),
                        child: const PersonAvatar(partner: true, size: 54),
                      ),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              connection.signedIn
                                  ? '${connection.user!['display_name']}'
                                  : s.t('common.couple'),
                              style: Theme.of(context).textTheme.titleMedium,
                            ),
                            const SizedBox(height: 4),
                            Text(
                              s.t(
                                connection.signedIn
                                    ? 'account.manage'
                                    : 'account.entry',
                              ),
                              style: Theme.of(context).textTheme.bodySmall,
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 28),
              SectionLabel(s.t('me.appearance')),
              const SizedBox(height: 14),
              CoupleCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      s.t('me.theme'),
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                    const SizedBox(height: 16),
                    Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        for (var i = 0; i < CoupleTheme.values.length; i++)
                          Expanded(
                            child: Padding(
                              padding: EdgeInsets.only(right: i == 2 ? 0 : 10),
                              child: Semantics(
                                button: true,
                                selected: state.theme == CoupleTheme.values[i],
                                child: InkWell(
                                  key: ValueKey(
                                    'theme-${CoupleTheme.values[i].name}',
                                  ),
                                  borderRadius: BorderRadius.circular(16),
                                  onTap: () =>
                                      controller.theme(CoupleTheme.values[i]),
                                  child: Column(
                                    children: [
                                      Container(
                                        height: 72,
                                        decoration: BoxDecoration(
                                          borderRadius: BorderRadius.circular(
                                            16,
                                          ),
                                          border: Border.all(
                                            color:
                                                state.theme ==
                                                    CoupleTheme.values[i]
                                                ? c.accent
                                                : c.line,
                                            width: 2,
                                          ),
                                          gradient: LinearGradient(
                                            begin: Alignment.topLeft,
                                            end: Alignment.bottomRight,
                                            colors: [
                                              SpaceColors.forTheme(
                                                CoupleTheme.values[i],
                                              ).heroStart,
                                              SpaceColors.forTheme(
                                                CoupleTheme.values[i],
                                              ).heroEnd,
                                            ],
                                          ),
                                        ),
                                        child: Center(
                                          child: Icon(
                                            state.theme == CoupleTheme.values[i]
                                                ? Icons.check_circle_rounded
                                                : Icons.favorite_border_rounded,
                                            color: SpaceColors.forTheme(
                                              CoupleTheme.values[i],
                                            ).accent,
                                            size: 24,
                                          ),
                                        ),
                                      ),
                                      const SizedBox(height: 9),
                                      Text(
                                        s.t(
                                          'theme.${CoupleTheme.values[i].name}',
                                        ),
                                        textAlign: TextAlign.center,
                                        style: TextStyle(
                                          fontSize: 11,
                                          color:
                                              state.theme ==
                                                  CoupleTheme.values[i]
                                              ? c.accent
                                              : c.muted,
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
                    const SizedBox(height: 14),
                    Divider(color: c.line),
                    SettingTile(
                      icon: Icons.dashboard_customize_outlined,
                      title: s.t('home.edit'),
                      onTap: () => showLayoutEditor(context),
                    ),
                    SettingTile(
                      icon: Icons.edit_note_rounded,
                      title: s.t('me.text'),
                      subtitle: s.t('interaction.miss'),
                      onTap: () => editWords(context, ref),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 28),
              SectionLabel(s.t('me.settings')),
              const SizedBox(height: 14),
              CoupleCard(
                padding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 6,
                ),
                child: Column(
                  children: [
                    SettingTile(
                      icon: Icons.language_rounded,
                      title: s.t('me.language'),
                      subtitle: s.t(state.locale == 'en' ? 'me.en' : 'me.zh'),
                      onTap: () => infoSheet(
                        context,
                        s.t('me.language'),
                        Consumer(
                          builder: (context, ref, _) {
                            final current = ref.watch(settingsProvider);
                            return Column(
                              children: [
                                for (final locale in ['zh_CN', 'en'])
                                  ListTile(
                                    title: Text(
                                      s.t(locale == 'en' ? 'me.en' : 'me.zh'),
                                    ),
                                    trailing: current.locale == locale
                                        ? Icon(
                                            Icons.check_rounded,
                                            color: context.colors.accent,
                                          )
                                        : null,
                                    onTap: () {
                                      ref
                                          .read(settingsProvider.notifier)
                                          .locale(locale);
                                      Navigator.pop(context);
                                    },
                                  ),
                              ],
                            );
                          },
                        ),
                      ),
                    ),
                    SwitchListTile(
                      contentPadding: const EdgeInsets.symmetric(
                        horizontal: 4,
                        vertical: 6,
                      ),
                      secondary: const IconTile(
                        icon: Icons.motion_photos_off_outlined,
                      ),
                      title: Text(
                        s.t('me.motion'),
                        style: const TextStyle(fontSize: 14),
                      ),
                      subtitle: Text(
                        s.t('me.motionHint'),
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                      value: state.reduceMotion,
                      onChanged: controller.motion,
                    ),
                    Divider(color: c.line),
                    SettingTile(
                      icon: Icons.shield_outlined,
                      title: s.t('me.privacy'),
                      subtitle: s.t('me.privacyHint'),
                      onTap: () => privacy(context, s),
                    ),
                    SettingTile(
                      icon: Icons.watch_outlined,
                      title: s.t('me.band'),
                      subtitle: s.t('me.bandHint'),
                      onTap: () => band(context, s),
                    ),
                    SettingTile(
                      icon: Icons.sensors_rounded,
                      title: s.t('me.diagnostics'),
                      subtitle: s.t('me.diagnosticsHint'),
                      onTap: () => diagnostics(context, s),
                    ),
                    SettingTile(
                      icon: Icons.dns_outlined,
                      title: s.t('me.server'),
                      subtitle: connection.origin.isEmpty
                          ? s.t('account.entry')
                          : connection.origin,
                      onTap: () => openAccount(context),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 28),
              Center(
                child: Icon(
                  Icons.favorite_rounded,
                  color: c.accent.withValues(alpha: .35),
                  size: 22,
                ),
              ),
              const SizedBox(height: 10),
              Center(
                child: Text(
                  s.t('me.about'),
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ),
              const SizedBox(height: 6),
              Center(
                child: Text(
                  s.t('app.version', {'version': appVersion}),
                  style: TextStyle(fontSize: 10, color: c.muted),
                ),
              ),
              const SizedBox(height: 28),
            ],
          ),
        ),
      ],
    );
  }
}

Future<void> editWords(BuildContext context, WidgetRef ref) async {
  final s = ref.read(stringsProvider);
  final input = TextEditingController(
    text:
        ref.read(settingsProvider).overrides['interaction.miss'] ??
        s.t('interaction.miss'),
  );
  await infoSheet(
    context,
    s.t('interaction.edit'),
    Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          s.t('interaction.editorHint'),
          style: Theme.of(context).textTheme.bodySmall,
        ),
        const SizedBox(height: 16),
        TextField(
          key: const ValueKey('custom-label'),
          controller: input,
          maxLength: 30,
          decoration: InputDecoration(hintText: s.t('interaction.labelHint')),
        ),
        const SizedBox(height: 16),
        FilledButton(
          onPressed: () {
            ref
                .read(settingsProvider.notifier)
                .overrideText('interaction.miss', input.text);
            Navigator.pop(context);
          },
          child: Text(s.t('common.save')),
        ),
      ],
    ),
  );
  input.dispose();
}

void privacy(BuildContext context, AppStrings s) => infoSheet(
  context,
  s.t('me.privacyTitle'),
  Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Text(s.t('me.noSharing')),
      const SizedBox(height: 16),
      for (final pair in [
        ('me.privacyLocation', Icons.location_off_outlined),
        ('me.privacyBattery', Icons.battery_unknown_rounded),
        ('me.privacyHealth', Icons.monitor_heart_outlined),
      ])
        SettingTile(
          icon: pair.$2,
          title: s.t(pair.$1),
          subtitle: s.t('me.notEnabled'),
          trailing: const Icon(Icons.lock_outline_rounded, size: 18),
        ),
    ],
  ),
);
void band(BuildContext context, AppStrings s) => infoSheet(
  context,
  s.t('me.bandTitle'),
  Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      const SizedBox(height: 8),
      Center(
        child: Container(
          width: 84,
          height: 170,
          decoration: BoxDecoration(
            color: const Color(0xFF080B15),
            borderRadius: BorderRadius.circular(42),
            border: Border.all(color: const Color(0xFF3D3448), width: 5),
          ),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Icon(
                Icons.favorite_rounded,
                color: Color(0xFFE58BB6),
                size: 22,
              ),
              const SizedBox(height: 8),
              const Text(
                '365',
                style: TextStyle(
                  color: Colors.white,
                  fontSize: 25,
                  fontWeight: FontWeight.w300,
                ),
              ),
              Text(
                s.t('common.demo'),
                style: const TextStyle(color: Color(0xFFA89AF5), fontSize: 9),
              ),
            ],
          ),
        ),
      ),
      const SizedBox(height: 22),
      Text(s.t('me.bandBody')),
      const SizedBox(height: 18),
      Text(s.t('me.bandPages'), style: Theme.of(context).textTheme.bodySmall),
    ],
  ),
);
void diagnostics(BuildContext context, AppStrings s) => infoSheet(
  context,
  s.t('me.diagnostics'),
  Column(
    children: [
      for (final id in ['Flutter', 'Server', 'Push', 'Bridge', 'Location'])
        Padding(
          padding: const EdgeInsets.symmetric(vertical: 12),
          child: Row(
            children: [
              Expanded(child: Text(s.t('me.diag$id'))),
              Text(
                s.t(id == 'Flutter' ? 'me.diagReady' : 'me.diagPending'),
                style: TextStyle(fontSize: 12, color: context.colors.muted),
              ),
            ],
          ),
        ),
      const SizedBox(height: 16),
      Text(s.t('me.diagFooter'), style: Theme.of(context).textTheme.bodySmall),
      const SizedBox(height: 12),
      Text(
        s.t('app.version', {'version': appVersion}),
        style: Theme.of(context).textTheme.bodySmall,
      ),
    ],
  ),
);
