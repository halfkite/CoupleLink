import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'core/state.dart';
import 'core/chat_controller.dart';
import 'core/remote_interactions.dart';
import 'core/background_notifications.dart';
import 'core/notifications.dart';
import 'core/chat_store.dart';
import 'core/connection.dart';
import 'features/interaction_feedback.dart';
import 'core/strings.dart';
import 'core/theme.dart';
import 'features/home.dart';
import 'features/map_page.dart';
import 'features/messages.dart';
import 'features/memories.dart';
import 'features/settings.dart';

class CoupleLinkApp extends ConsumerWidget {
  const CoupleLinkApp({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final settings = ref.watch(settingsProvider);
    return MaterialApp(
      title: 'CoupleLink',
      debugShowCheckedModeBanner: false,
      theme: buildTheme(settings.theme),
      themeAnimationDuration:
          settings.reduceMotion ||
              MediaQueryData.fromView(View.of(context)).disableAnimations
          ? Duration.zero
          : const Duration(milliseconds: 220),
      locale: settings.locale == 'en'
          ? const Locale('en')
          : const Locale('zh', 'CN'),
      supportedLocales: const [Locale('zh', 'CN'), Locale('en')],
      localizationsDelegates: GlobalMaterialLocalizations.delegates,
      builder: (context, child) => MediaQuery(
        data: MediaQuery.of(context).copyWith(
          disableAnimations:
              settings.reduceMotion || MediaQuery.disableAnimationsOf(context),
        ),
        child: child!,
      ),
      home: const SpaceShell(),
    );
  }
}

class SpaceShell extends ConsumerStatefulWidget {
  const SpaceShell({super.key});
  @override
  ConsumerState<SpaceShell> createState() => _SpaceShellState();
}

class _SpaceShellState extends ConsumerState<SpaceShell>
    with WidgetsBindingObserver {
  int index = 0;
  Future<void> _lifecycleWrite = Future.value();
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    notificationTap.addListener(_notificationTapped);
    if (notificationTap.value != null) {
      WidgetsBinding.instance.addPostFrameCallback(
        (_) => _notificationTapped(),
      );
    }
    _lifecycleWrite = setAppForeground(ref.read(preferencesProvider), true)
        .catchError((Object error, StackTrace stack) {
          FlutterError.reportError(
            FlutterErrorDetails(
              exception: error,
              stack: stack,
              library: 'CoupleLink lifecycle',
            ),
          );
        });
  }

  void _notificationTapped() {
    final target = notificationTap.value;
    notificationTap.value = null;
    if (mounted && target == 'messages') {
      setState(() => index = 2);
      ref.read(chatProvider.notifier).setChatVisible(true);
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    final foreground = state == AppLifecycleState.resumed;
    final prefs = ref.read(preferencesProvider);
    final connection = ref.read(connectionProvider);
    final store =
        !foreground && connection.couple != null && ref.read(chatProvider).ready
        ? ref.read(chatStoreProvider)
        : null;
    _lifecycleWrite = _lifecycleWrite
        .then((_) async {
          if (store != null) {
            await updateBackgroundBaseline(prefs, store, connection);
          }
          await setAppForeground(prefs, foreground);
        })
        .catchError((Object error, StackTrace stack) {
          FlutterError.reportError(
            FlutterErrorDetails(
              exception: error,
              stack: stack,
              library: 'CoupleLink lifecycle',
            ),
          );
        });
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    notificationTap.removeListener(_notificationTapped);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final chat = ref.watch(chatProvider);
    ref.listen(interactionProvider, (previous, next) {
      if (next.incoming != null && previous?.incoming == null) {
        final incoming = next.incoming!;
        WidgetsBinding.instance.addPostFrameCallback((_) async {
          if (!mounted ||
              ref.read(interactionProvider).incoming?.id != incoming.id) {
            return;
          }
          await showDialog<void>(
            context: context,
            barrierColor: Colors.black.withValues(alpha: .72),
            builder: (_) => InteractionFeedback(
              action: incoming.action,
              received: true,
              receivedOutcome: incoming.outcome,
            ),
          );
          if (mounted &&
              ref.read(interactionProvider).incoming?.id == incoming.id) {
            ref.read(interactionProvider.notifier).dismiss();
          }
        });
      }
    });
    final c = context.colors;
    final s = ref.watch(stringsProvider);
    final keys = ['home', 'map', 'messages', 'time', 'me'];
    final icons = [
      Icons.home_outlined,
      Icons.map_outlined,
      Icons.chat_bubble_outline_rounded,
      Icons.auto_awesome_outlined,
      Icons.person_outline_rounded,
    ];
    final filled = [
      Icons.home_rounded,
      Icons.map_rounded,
      Icons.chat_bubble_rounded,
      Icons.auto_awesome_rounded,
      Icons.person_rounded,
    ];
    final pages = [
      HomePage(onMap: () => setState(() => index = 1)),
      MapPage(active: index == 1),
      const MessagesPage(),
      const MemoriesPage(),
      const SettingsPage(),
    ];
    return ColoredBox(
      color: c.soft.withValues(alpha: .35),
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 600),
          child: Scaffold(
            body: SafeArea(
              bottom: false,
              child: Column(
                children: [
                  if (ref.watch(settingsProvider).persistFailed)
                    MaterialBanner(
                      content: Text(s.t('app.persistError')),
                      actions: [
                        TextButton(
                          onPressed: () => ref
                              .read(settingsProvider.notifier)
                              .update(ref.read(settingsProvider)),
                          child: Text(s.t('common.save')),
                        ),
                      ],
                    ),
                  Expanded(
                    child: IndexedStack(
                      index: index,
                      children: [
                        for (var i = 0; i < pages.length; i++)
                          TickerMode(enabled: i == index, child: pages[i]),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            bottomNavigationBar: DecoratedBox(
              decoration: BoxDecoration(
                color: c.paper,
                border: Border(top: BorderSide(color: c.line)),
              ),
              child: SafeArea(
                top: false,
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(8, 8, 8, 5),
                  child: Row(
                    children: [
                      for (var i = 0; i < keys.length; i++)
                        Expanded(
                          child: Semantics(
                            selected: index == i,
                            button: true,
                            label: s.t('nav.${keys[i]}'),
                            child: InkWell(
                              key: ValueKey('nav-${keys[i]}'),
                              borderRadius: BorderRadius.circular(18),
                              onTap: () {
                                setState(() => index = i);
                                ref
                                    .read(chatProvider.notifier)
                                    .setChatVisible(i == 2);
                              },
                              child: Padding(
                                padding: const EdgeInsets.symmetric(
                                  vertical: 6,
                                ),
                                child: Column(
                                  mainAxisSize: MainAxisSize.min,
                                  children: [
                                    AnimatedContainer(
                                      duration: context.reducedMotion
                                          ? Duration.zero
                                          : const Duration(milliseconds: 180),
                                      padding: const EdgeInsets.symmetric(
                                        horizontal: 16,
                                        vertical: 5,
                                      ),
                                      decoration: BoxDecoration(
                                        color: index == i
                                            ? c.soft
                                            : Colors.transparent,
                                        borderRadius: BorderRadius.circular(20),
                                      ),
                                      child: Badge(
                                        isLabelVisible: i == 2 && chat.unread,
                                        smallSize: 9,
                                        child: Icon(
                                          index == i ? filled[i] : icons[i],
                                          size: 22,
                                          color: index == i
                                              ? c.accent
                                              : c.muted,
                                        ),
                                      ),
                                    ),
                                    const SizedBox(height: 3),
                                    ExcludeSemantics(
                                      child: Text(
                                        s.t('nav.${keys[i]}'),
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: TextStyle(
                                          fontSize: 10,
                                          color: index == i
                                              ? c.accent
                                              : c.muted,
                                          fontWeight: index == i
                                              ? FontWeight.w700
                                              : FontWeight.w500,
                                        ),
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
              ),
            ),
          ),
        ),
      ),
    );
  }
}
