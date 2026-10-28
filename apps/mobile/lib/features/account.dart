import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/connection.dart';
import '../core/strings.dart';
import '../core/theme.dart';
import '../ui/widgets.dart';

void openAccount(BuildContext context) =>
    Navigator.of(context)
        .push(MaterialPageRoute<void>(builder: (_) => const AccountPage()));

class AccountPage extends ConsumerStatefulWidget {
  const AccountPage({super.key});
  @override
  ConsumerState<AccountPage> createState() => _AccountPageState();
}

class _AccountPageState extends ConsumerState<AccountPage> {
  final form = GlobalKey<FormState>();
  final origin = TextEditingController(),
      username = TextEditingController(),
      password = TextEditingController(),
      displayName = TextEditingController(),
      invitation = TextEditingController(),
      timezone = TextEditingController(text: 'Asia/Hong_Kong');
  bool register = false, showPassword = false, testingConnection = false;
  String? testResult;
  int testGeneration = 0;
  DateTime start = DateTime.now();
  @override
  void initState() {
    super.initState();
    origin.text = ref.read(connectionProvider).origin;
  }

  @override
  void dispose() {
    for (final c in [
      origin,
      username,
      password,
      displayName,
      invitation,
      timezone,
    ]) {
      c.dispose();
    }
    super.dispose();
  }

  Future<void> authenticate() async {
    if (!(form.currentState?.validate() ?? false)) return;
    await ref
        .read(connectionProvider.notifier)
        .signIn(
          origin: origin.text,
          username: username.text,
          password: password.text,
          register: register,
          displayName: displayName.text,
        );
    if (mounted && ref.read(connectionProvider).signedIn) {
      password.clear();
    }
  }

  Future<void> testConnection() async {
    final generation = ++testGeneration;
    final address = origin.text;
    setState(() {
      testingConnection = true;
      testResult = null;
    });
    String result;
    try {
      await testServerConnection(ref.read(httpProvider), address);
      result = 'success';
    } on ApiException catch (error) {
      result = error.code;
    } on TimeoutException {
      result = 'timeout';
    } catch (_) {
      result = 'unreachable';
    }
    if (!mounted || generation != testGeneration) return;
    setState(() {
      testingConnection = false;
      testResult = result;
    });
  }

  Future<bool> confirm(String title, String body) async {
    final s = ref.read(stringsProvider);
    return await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: Text(s.t(title)),
            content: Text(s.t(body)),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(context, false),
                child: Text(s.t('common.cancel')),
              ),
              FilledButton(
                onPressed: () => Navigator.pop(context, true),
                child: Text(s.t('account.confirm')),
              ),
            ],
          ),
        ) ??
        false;
  }

  @override
  Widget build(BuildContext context) {
    final s = ref.watch(stringsProvider),
        c = context.colors,
        state = ref.watch(connectionProvider),
        controller = ref.read(connectionProvider.notifier);
    String? requiredValue(String? value) =>
        value == null || value.trim().isEmpty ? s.t('account.required') : null;
    final errorKey = 'account.error.${state.error}';
    final errorText =
        s.t(errorKey, {'hosts': configuredLocalHttpHosts}) == errorKey
        ? s.t('account.error.generic', {'code': state.error ?? ''})
        : s.t(errorKey, {'hosts': configuredLocalHttpHosts});
    return Scaffold(
      appBar: AppBar(
        title: Text(s.t('account.title')),
        backgroundColor: c.background,
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 28),
          children: [
            if (state.busy) const LinearProgressIndicator(),
            if (state.error != null)
              Padding(
                padding: const EdgeInsets.only(bottom: 16),
                child: Semantics(
                  liveRegion: true,
                  child: Container(
                    padding: const EdgeInsets.all(16),
                    decoration: BoxDecoration(
                      color: c.soft,
                      borderRadius: BorderRadius.circular(18),
                    ),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Icon(Icons.info_outline_rounded, color: c.accent),
                        const SizedBox(width: 10),
                        Expanded(child: Text(errorText)),
                      ],
                    ),
                  ),
                ),
              ),
            if (!state.signedIn)
              CoupleCard(
                child: Form(
                  key: form,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Text(
                        s.t('account.connectTitle'),
                        style: Theme.of(context).textTheme.titleLarge,
                      ),
                      const SizedBox(height: 8),
                      Text(
                        s.t('account.connectHint'),
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                      const SizedBox(height: 20),
                      TextFormField(
                        key: const ValueKey('server-url'),
                        controller: origin,
                        autocorrect: false,
                        keyboardType: TextInputType.url,
                        onChanged: (_) {
                          testGeneration++;
                          if (testingConnection || testResult != null) {
                            setState(() {
                              testingConnection = false;
                              testResult = null;
                            });
                          }
                        },
                        decoration: InputDecoration(
                          labelText: s.t('account.server'),
                          hintText: 'https://couplelink.example.com',
                        ),
                        validator: (v) {
                          try {
                            normalizeOrigin(v ?? '');
                            return null;
                          } on ApiException catch (error) {
                            return s.t('account.error.${error.code}', {
                              'hosts': configuredLocalHttpHosts,
                            });
                          }
                        },
                      ),
                      const SizedBox(height: 8),
                      Text(
                        s.t('account.serverHint', {
                          'hosts': configuredLocalHttpHosts,
                        }),
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                      const SizedBox(height: 8),
                      OutlinedButton.icon(
                        key: const ValueKey('test-server-connection'),
                        onPressed: testingConnection || state.busy
                            ? null
                            : testConnection,
                        icon: const Icon(Icons.wifi_tethering_rounded),
                        label: Text(
                          s.t(
                            testingConnection
                                ? 'account.probe.testing'
                                : 'account.probe.button',
                          ),
                        ),
                      ),
                      if (testResult != null)
                        Semantics(
                          liveRegion: true,
                          child: Text(
                            s.t('account.probe.$testResult', {
                              'hosts': configuredLocalHttpHosts,
                            }),
                            key: const ValueKey('server-test-result'),
                          ),
                        ),
                      const SizedBox(height: 16),
                      Wrap(
                        spacing: 10,
                        runSpacing: 8,
                        children: [
                          ChoiceChip(
                            label: Text(s.t('account.login')),
                            selected: !register,
                            onSelected: state.busy
                                ? null
                                : (_) => setState(() => register = false),
                          ),
                          ChoiceChip(
                            label: Text(s.t('account.register')),
                            selected: register,
                            onSelected: state.busy
                                ? null
                                : (_) => setState(() => register = true),
                          ),
                        ],
                      ),
                      const SizedBox(height: 16),
                      TextFormField(
                        controller: username,
                        autocorrect: false,
                        enableSuggestions: false,
                        maxLength: 32,
                        decoration: InputDecoration(
                          labelText: s.t('account.username'),
                          counterText: '',
                        ),
                        validator: (v) =>
                            RegExp(r'^[a-z0-9_-]{3,32}$')
                                .hasMatch((v ?? '').trim().toLowerCase())
                            ? null
                            : s.t('account.usernameHint'),
                      ),
                      const SizedBox(height: 16),
                      TextFormField(
                        controller: password,
                        obscureText: !showPassword,
                        autocorrect: false,
                        enableSuggestions: false,
                        maxLength: maximumPasswordLength,
                        decoration: InputDecoration(
                          labelText: s.t('account.password'),
                          counterText: '',
                          suffixIcon: IconButton(
                            tooltip: s.t('account.showPassword'),
                            onPressed: () =>
                                setState(() => showPassword = !showPassword),
                            icon: Icon(
                              showPassword
                                  ? Icons.visibility_off_outlined
                                  : Icons.visibility_outlined,
                            ),
                          ),
                        ),
                        validator: (v) =>
                            (v?.runes.length ?? 0) >= minimumPasswordLength
                            ? null
                            : s.t('account.passwordHint', {
                                'min': minimumPasswordLength.toString(),
                              }),
                      ),
                      if (register) ...[
                        const SizedBox(height: 16),
                        TextFormField(
                          controller: displayName,
                          maxLength: 40,
                          decoration: InputDecoration(
                            labelText: s.t('account.displayName'),
                            counterText: '',
                          ),
                          validator: requiredValue,
                        ),
                      ],
                      const SizedBox(height: 22),
                      FilledButton(
                        onPressed: state.busy ? null : authenticate,
                        child: Text(
                          s.t(register ? 'account.register' : 'account.login'),
                        ),
                      ),
                      const SizedBox(height: 12),
                      Text(
                        s.t('account.vaultHint'),
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ],
                  ),
                ),
              ),
            if (state.signedIn) ...[
              CoupleCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    SectionLabel(
                      '${state.user!['display_name']}',
                      subtitle: state.origin,
                      trailing: Icon(
                        Icons.verified_user_outlined,
                        color: c.accent,
                      ),
                    ),
                    const SizedBox(height: 16),
                    Text(
                      s.t(
                        state.error == null
                            ? 'account.connected'
                            : 'account.cached',
                      ),
                    ),
                    const SizedBox(height: 16),
                    Wrap(
                      spacing: 10,
                      runSpacing: 10,
                      children: [
                        FilledButton.icon(
                          onPressed: state.busy ? null : controller.refresh,
                          icon: const Icon(Icons.sync_rounded),
                          label: Text(s.t('account.refresh')),
                        ),
                        OutlinedButton(
                          onPressed: state.busy ? null : controller.signOut,
                          child: Text(s.t('account.logout')),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 16),
              if (state.couple == null)
                CoupleCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Text(
                        s.t('account.bindTitle'),
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                      const SizedBox(height: 8),
                      Text(s.t('account.bindHint')),
                      const SizedBox(height: 16),
                      OutlinedButton.icon(
                        onPressed: state.busy
                            ? null
                            : controller.createInvitation,
                        icon: const Icon(Icons.link_rounded),
                        label: Text(s.t('account.createInvite')),
                      ),
                      if (state.invitation != null) ...[
                        const SizedBox(height: 12),
                        SelectableText(
                          state.invitation!,
                          style: TextStyle(
                            color: c.accent,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        const SizedBox(height: 6),
                        Text(
                          s.t('account.inviteExpiry'),
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                      ],
                      const SizedBox(height: 20),
                      TextField(
                        controller: invitation,
                        autocorrect: false,
                        enableSuggestions: false,
                        onChanged: (_) => setState(() {}),
                        decoration: InputDecoration(
                          labelText: s.t('account.inviteToken'),
                        ),
                      ),
                      const SizedBox(height: 12),
                      OutlinedButton.icon(
                        onPressed: state.busy
                            ? null
                            : () async {
                                final date = await showDatePicker(
                                  context: context,
                                  initialDate: start,
                                  firstDate: DateTime(1900),
                                  lastDate: DateTime.now(),
                                );
                                if (date != null && mounted) {
                                  setState(() => start = date);
                                }
                              },
                        icon: const Icon(Icons.calendar_month_outlined),
                        label: Text(
                          '${s.t('account.startAt')} · ${_date(start)}',
                        ),
                      ),
                      const SizedBox(height: 12),
                      TextField(
                        controller: timezone,
                        autocorrect: false,
                        decoration: InputDecoration(
                          labelText: s.t('account.timezone'),
                        ),
                      ),
                      const SizedBox(height: 16),
                      FilledButton(
                        onPressed: state.busy || invitation.text.trim().isEmpty
                            ? null
                            : () => controller.join(
                                invitation.text,
                                _date(start),
                                timezone.text,
                              ),
                        child: Text(s.t('account.join')),
                      ),
                    ],
                  ),
                ),
              if (state.couple != null)
                CoupleCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      SectionLabel(
                        s.t('account.bound'),
                        subtitle:
                            '${(state.couple!['partner'] as Map)['display_name']}',
                      ),
                      const SizedBox(height: 12),
                      Text(
                        '${s.t('account.startAt')} · ${state.couple!['start_at']}',
                      ),
                      Text(
                        '${s.t('account.timezone')} · ${state.couple!['timezone']}',
                      ),
                      const SizedBox(height: 16),
                      OutlinedButton(
                        onPressed: state.busy
                            ? null
                            : () async {
                                if (await confirm(
                                  'account.unbind',
                                  'account.unbindHint',
                                )) {
                                  await controller.unbind();
                                }
                              },
                        child: Text(s.t('account.unbind')),
                      ),
                    ],
                  ),
                ),
              const SizedBox(height: 22),
              SectionLabel(s.t('account.devices')),
              const SizedBox(height: 12),
              CoupleCard(
                padding: const EdgeInsets.all(12),
                child: Column(
                  children: [
                    for (final device in state.devices)
                      ListTile(
                        contentPadding: const EdgeInsets.symmetric(
                          horizontal: 4,
                          vertical: 4,
                        ),
                        leading: Icon(
                          device['platform'] == 'web'
                              ? Icons.web_outlined
                              : Icons.phone_android_outlined,
                          color: c.accent,
                        ),
                        title: Text(
                          '${device['label']}',
                          style: const TextStyle(fontSize: 14),
                        ),
                        subtitle: Text(
                          device['current'] == true
                              ? s.t('account.thisDevice')
                              : '${device['platform']}',
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                        trailing: IconButton(
                          tooltip: s.t('account.revokeDevice'),
                          icon: const Icon(Icons.logout_rounded, size: 20),
                          onPressed: state.busy
                              ? null
                              : () async {
                                  if (await confirm(
                                    'account.revokeDevice',
                                    'account.revokeHint',
                                  )) {
                                    await controller.revoke(
                                      device['id'] as String,
                                      device['current'] == true,
                                    );
                                  }
                                },
                        ),
                      ),
                  ],
                ),
              ),
            ],
            const SizedBox(height: 22),
            Text(
              s.t('account.scopeHint'),
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodySmall,
            ),
          ],
        ),
      ),
    );
  }

  String _date(DateTime date) =>
      '${date.year.toString().padLeft(4, '0')}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')}';
}
