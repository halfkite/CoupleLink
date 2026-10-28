import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/connection.dart';
import '../core/state.dart';
import '../core/strings.dart';
import 'account.dart';

const _serverListKey = 'couplelink.server.addresses.v1';

class ServerAddressesPage extends ConsumerStatefulWidget {
  const ServerAddressesPage({super.key});
  @override
  ConsumerState<ServerAddressesPage> createState() =>
      _ServerAddressesPageState();
}

class _ServerAddressesPageState extends ConsumerState<ServerAddressesPage> {
  final input = TextEditingController();
  List<String> addresses = [];
  String? message;
  bool busy = false;
  @override
  void initState() {
    super.initState();
    addresses =
        ref.read(preferencesProvider).getStringList(_serverListKey) ?? [];
    final current = ref.read(connectionProvider).origin;
    if (current.isNotEmpty && !addresses.contains(current)) {
      addresses = [current, ...addresses];
    }
  }

  @override
  void dispose() {
    input.dispose();
    super.dispose();
  }

  Future<void> _save(List<String> next) async {
    if (!await ref
        .read(preferencesProvider)
        .setStringList(_serverListKey, next)) {
      throw const ApiException('preferences_failed');
    }
    if (mounted) setState(() => addresses = next);
  }

  Future<void> _add() async {
    final text = input.text.trim(), uri = Uri.tryParse(text);
    if (uri == null ||
        !uri.hasAuthority ||
        uri.host.isEmpty ||
        !['http', 'https'].contains(uri.scheme) ||
        uri.userInfo.isNotEmpty ||
        uri.hasQuery ||
        uri.hasFragment ||
        (uri.path.isNotEmpty && uri.path != '/')) {
      setState(() => message = 'address.invalid');
      return;
    }
    final value = uri.replace(path: '').toString();
    try {
      if (!addresses.contains(value)) await _save([...addresses, value]);
      input.clear();
      if (mounted) setState(() => message = null);
    } catch (_) {
      if (mounted) setState(() => message = 'address.saveFailed');
    }
  }

  Future<void> _test(String address) async {
    setState(() {
      busy = true;
      message = null;
    });
    try {
      await testServerConnection(ref.read(httpProvider), address);
      message = 'account.probe.success';
    } on ApiException catch (e) {
      message = 'account.probe.${e.code}';
    } catch (_) {
      message = 'account.probe.unreachable';
    }
    if (mounted) setState(() => busy = false);
  }

  Future<void> _select(String address) async {
    setState(() {
      busy = true;
      message = null;
    });
    await ref.read(connectionProvider.notifier).changeServer(address);
    if (!mounted) return;
    final state = ref.read(connectionProvider);
    if (state.error != null) {
      setState(() {
        busy = false;
        message = 'account.error.${state.error}';
      });
      return;
    }
    setState(() => busy = false);
    Navigator.of(context)
        .push(MaterialPageRoute<void>(builder: (_) => const AccountPage()));
  }

  @override
  Widget build(BuildContext context) {
    final s = ref.watch(stringsProvider),
        current = ref.watch(connectionProvider).origin;
    return Scaffold(
      appBar: AppBar(title: Text(s.t('address.title'))),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Text(s.t('address.hint')),
          const SizedBox(height: 16),
          TextField(
            controller: input,
            keyboardType: TextInputType.url,
            decoration: InputDecoration(
              labelText: s.t('address.input'),
              hintText: 'http://192.168.137.1:18080',
            ),
          ),
          const SizedBox(height: 8),
          FilledButton(
            onPressed: busy ? null : _add,
            child: Text(s.t('address.add')),
          ),
          if (message != null)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 12),
              child: Semantics(
                liveRegion: true,
                child: Text(s.t(message!, {'hosts': configuredLocalHttpHosts})),
              ),
            ),
          for (final address in addresses)
            Card(
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    SelectableText(address),
                    if (address == current) Text(s.t('address.current')),
                    Wrap(
                      spacing: 8,
                      children: [
                        TextButton(
                          onPressed: busy ? null : () => _test(address),
                          child: Text(s.t('account.probe.button')),
                        ),
                        TextButton(
                          onPressed: busy ? null : () => _select(address),
                          child: Text(s.t('address.select')),
                        ),
                        TextButton(
                          onPressed: busy
                              ? null
                              : () async {
                                  try {
                                    await _save(
                                      addresses
                                          .where((a) => a != address)
                                          .toList(),
                                    );
                                  } catch (_) {
                                    if (mounted) {
                                      setState(
                                        () => message = 'address.saveFailed',
                                      );
                                    }
                                  }
                                },
                          child: Text(s.t('address.remove')),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }
}
