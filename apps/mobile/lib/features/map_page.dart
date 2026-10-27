import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/strings.dart';
import '../core/theme.dart';
import '../ui/widgets.dart';

class MapPage extends ConsumerStatefulWidget {
  const MapPage({super.key, required this.active});
  final bool active;
  @override
  ConsumerState<MapPage> createState() => _MapPageState();
}

class _MapPageState extends ConsumerState<MapPage>
    with SingleTickerProviderStateMixin {
  late final AnimationController playback = AnimationController(
    vsync: this,
    duration: const Duration(seconds: 12),
  )..addListener(() => setState(() {}));
  String status = 'fresh', mode = 'timed';
  bool precise = true, record = false;
  final statuses = [
    'fresh',
    'stale',
    'locating',
    'denied',
    'disabled',
    'offline',
  ];
  @override
  void didUpdateWidget(covariant MapPage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!widget.active) playback.stop();
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (context.reducedMotion) playback.stop();
  }

  @override
  void dispose() {
    playback.dispose();
    super.dispose();
  }

  void play() {
    if (context.reducedMotion) {
      setState(() => playback.value = playback.value >= 1 ? 0 : 1);
      return;
    }
    if (playback.isAnimating) {
      playback.stop();
      setState(() {});
    } else {
      if (playback.value >= 1) playback.value = 0;
      playback.forward();
    }
  }

  @override
  Widget build(BuildContext context) {
    final s = ref.watch(stringsProvider), c = context.colors;
    final available = !['denied', 'disabled', 'locating'].contains(status);
    return Column(
      children: [
        const PageHeading(title: 'map.title', subtitle: 'map.subtitle'),
        Expanded(
          child: ClipRRect(
            borderRadius: const BorderRadius.vertical(top: Radius.circular(28)),
            child: Stack(
              children: [
                Positioned.fill(
                  child: CustomPaint(
                    painter: MapIllustration(
                      colors: c,
                      progress: playback.value,
                      showPins: available,
                      precise: precise,
                    ),
                  ),
                ),
                Positioned(
                  top: 18,
                  left: 0,
                  right: 0,
                  child: Center(
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 12,
                        vertical: 7,
                      ),
                      decoration: BoxDecoration(
                        color: c.paper.withValues(alpha: .9),
                        borderRadius: BorderRadius.circular(20),
                      ),
                      child: Text(
                        s.t('map.schematic'),
                        style: TextStyle(fontSize: 10, color: c.muted),
                      ),
                    ),
                  ),
                ),
                if (available)
                  Positioned(
                    top: 85,
                    left: 0,
                    right: 0,
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        if (precise) ...[
                          const PersonAvatar(partner: false, size: 40),
                          const SizedBox(width: 40),
                          const PersonAvatar(partner: true, size: 40),
                        ] else
                          Container(
                            padding: const EdgeInsets.all(12),
                            decoration: BoxDecoration(
                              color: c.paper.withValues(alpha: .9),
                              borderRadius: BorderRadius.circular(16),
                            ),
                            child: Text(
                              s.t('map.approximate'),
                              style: TextStyle(fontSize: 12, color: c.accent),
                            ),
                          ),
                      ],
                    ),
                  ),
                if (!available)
                  Positioned(
                    top: 90,
                    left: 24,
                    right: 24,
                    child: Center(
                      child: Text(
                        s.t('map.noLocation'),
                        style: TextStyle(color: c.muted, fontSize: 13),
                      ),
                    ),
                  ),
                DraggableScrollableSheet(
                  initialChildSize: .55,
                  minChildSize: .3,
                  maxChildSize: .92,
                  snap: true,
                  snapSizes: const [.3, .55, .92],
                  builder: (context, scroll) => Container(
                    decoration: BoxDecoration(
                      color: c.paper,
                      borderRadius: const BorderRadius.vertical(
                        top: Radius.circular(26),
                      ),
                      boxShadow: [
                        BoxShadow(
                          color: c.ink.withValues(alpha: .06),
                          blurRadius: 25,
                          offset: const Offset(0, -5),
                        ),
                      ],
                    ),
                    child: Material(
                      type: MaterialType.transparency,
                      child: ListView(
                        controller: scroll,
                        padding: const EdgeInsets.fromLTRB(20, 10, 20, 24),
                        children: [
                          Center(
                            child: Container(
                              width: 34,
                              height: 4,
                              decoration: BoxDecoration(
                                color: c.line,
                                borderRadius: BorderRadius.circular(5),
                              ),
                            ),
                          ),
                          const SizedBox(height: 18),
                          Row(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              IconTile(
                                icon: Icons.near_me_rounded,
                                color: c.secondary,
                              ),
                              const SizedBox(width: 12),
                              Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      !available
                                          ? s.t('map.$status')
                                          : precise
                                          ? s.t('map.distance')
                                          : s.t('map.approximate'),
                                      style: Theme.of(context)
                                          .textTheme
                                          .titleMedium,
                                    ),
                                    const SizedBox(height: 4),
                                    Text(
                                      s.t('home.updated'),
                                      style: Theme.of(context)
                                          .textTheme
                                          .bodySmall,
                                    ),
                                  ],
                                ),
                              ),
                              IconButton(
                                tooltip: s.t('map.stateSelect'),
                                onPressed: () => infoSheet(
                                  context,
                                  s.t('map.stateSelect'),
                                  Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Text(s.t('map.stateHint')),
                                      const SizedBox(height: 12),
                                      Wrap(
                                        spacing: 8,
                                        runSpacing: 8,
                                        children: [
                                          for (final value in statuses)
                                            ChoiceChip(
                                              label: Text(s.t('map.$value')),
                                              selected: status == value,
                                              onSelected: (_) {
                                                setState(() => status = value);
                                                Navigator.pop(context);
                                              },
                                            ),
                                        ],
                                      ),
                                    ],
                                  ),
                                ),
                                icon: const Icon(Icons.tune_rounded, size: 20),
                              ),
                            ],
                          ),
                          const SizedBox(height: 14),
                          Wrap(
                            spacing: 8,
                            runSpacing: 8,
                            children: [
                              Chip(
                                avatar: Icon(
                                  Icons.circle,
                                  size: 8,
                                  color: status == 'fresh'
                                      ? const Color(0xFF78A38E)
                                      : c.muted,
                                ),
                                label: Text(
                                  s.t('map.$status'),
                                  style: const TextStyle(fontSize: 11),
                                ),
                                side: BorderSide(color: c.line),
                              ),
                              Chip(
                                label: Text(
                                  s.t('common.demo'),
                                  style: const TextStyle(fontSize: 11),
                                ),
                                side: BorderSide(color: c.line),
                              ),
                            ],
                          ),
                          const SizedBox(height: 12),
                          Text(
                            s.t('map.sharing'),
                            style: Theme.of(context).textTheme.titleMedium,
                          ),
                          const SizedBox(height: 8),
                          Wrap(
                            spacing: 7,
                            runSpacing: 7,
                            children: [
                              for (final value in [
                                'off',
                                'once',
                                'timed',
                                'continuous',
                              ])
                                ChoiceChip(
                                  key: ValueKey('mode-$value'),
                                  label: Text(
                                    s.t('map.mode.$value'),
                                    style: const TextStyle(fontSize: 12),
                                  ),
                                  selected: mode == value,
                                  onSelected: (_) => setState(() {
                                    mode = value;
                                    status = value == 'off'
                                        ? 'disabled'
                                        : 'fresh';
                                  }),
                                ),
                            ],
                          ),
                          const SizedBox(height: 14),
                          Row(
                            children: [
                              Expanded(child: Text(s.t('map.precision'))),
                              Switch(
                                value: precise,
                                onChanged: (v) => setState(() => precise = v),
                              ),
                              const SizedBox(width: 5),
                              Text(
                                s.t(precise ? 'map.precise' : 'map.coarse'),
                                style: TextStyle(fontSize: 12, color: c.muted),
                              ),
                            ],
                          ),
                          SwitchListTile(
                            contentPadding: EdgeInsets.zero,
                            title: Text(
                              s.t('map.record'),
                              style: const TextStyle(fontSize: 14),
                            ),
                            subtitle: Text(
                              s.t('map.recordHint'),
                              style: Theme.of(context).textTheme.bodySmall,
                            ),
                            value: record,
                            onChanged: (v) => setState(() => record = v),
                          ),
                          const SizedBox(height: 14),
                          Divider(color: c.line),
                          const SizedBox(height: 12),
                          SectionLabel(
                            s.t('map.track'),
                            subtitle: s.t('map.trackDate'),
                          ),
                          const SizedBox(height: 8),
                          Slider(
                            value: playback.value,
                            onChanged: (v) {
                              playback.stop();
                              setState(() => playback.value = v);
                            },
                            semanticFormatterCallback: (v) =>
                                '${(v * 100).round()}%',
                          ),
                          Row(
                            children: [
                              Text(
                                s.t('map.timeStart'),
                                style: Theme.of(context).textTheme.bodySmall,
                              ),
                              const Spacer(),
                              Text(
                                s.t('map.timeEnd'),
                                style: Theme.of(context).textTheme.bodySmall,
                              ),
                            ],
                          ),
                          const SizedBox(height: 12),
                          OutlinedButton.icon(
                            onPressed: play,
                            icon: Icon(
                              playback.isAnimating
                                  ? Icons.pause_rounded
                                  : Icons.play_arrow_rounded,
                            ),
                            label: Text(
                              s.t(
                                playback.isAnimating ? 'map.pause' : 'map.play',
                              ),
                            ),
                          ),
                          const SizedBox(height: 20),
                          Container(
                            padding: const EdgeInsets.all(14),
                            decoration: BoxDecoration(
                              color: c.soft.withValues(alpha: .5),
                              borderRadius: BorderRadius.circular(16),
                            ),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Row(
                                  children: [
                                    Icon(
                                      Icons.shield_outlined,
                                      size: 18,
                                      color: c.accent,
                                    ),
                                    const SizedBox(width: 8),
                                    Expanded(
                                      child: Text(
                                        s.t('map.privacy'),
                                        style: const TextStyle(
                                          fontSize: 12,
                                          fontWeight: FontWeight.w600,
                                        ),
                                      ),
                                    ),
                                  ],
                                ),
                                const SizedBox(height: 8),
                                Text(
                                  s.t('map.privacyHint'),
                                  style: Theme.of(context).textTheme.bodySmall,
                                ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}

class MapIllustration extends CustomPainter {
  MapIllustration({
    required this.colors,
    this.progress = 0,
    this.showPins = true,
    this.precise = true,
  });
  final SpaceColors colors;
  final double progress;
  final bool showPins, precise;
  @override
  void paint(Canvas canvas, Size size) {
    final dark = colors.background.computeLuminance() < .1;
    canvas.drawRect(
      Offset.zero & size,
      Paint()..color = dark ? const Color(0xFF1E2530) : const Color(0xFFF0EEE8),
    );
    canvas.save();
    canvas.scale(size.width / 400, size.height / 600);
    final blocks = Paint()
      ..color = dark ? const Color(0xFF2A3440) : const Color(0xFFE5E3DA);
    for (var x = -1; x < 6; x++) {
      for (var y = -1; y < 7; y++) {
        canvas.drawRRect(
          RRect.fromRectAndRadius(
            Rect.fromLTWH(x * 89.0 + 12, y * 90.0 + 25, 65, 60),
            const Radius.circular(12),
          ),
          blocks,
        );
      }
    }
    final park = Paint()
      ..color = dark ? const Color(0xFF2C443C) : const Color(0xFFD9E6CF);
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(24, 122, 106, 124),
        const Radius.circular(26),
      ),
      park,
    );
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(280, 20, 110, 118),
        const Radius.circular(24),
      ),
      park,
    );
    for (var i = 0; i < 9; i++) {
      canvas.drawCircle(
        Offset(40 + (i % 3) * 32.0, 145 + (i ~/ 3) * 32.0),
        8,
        Paint()
          ..color = dark ? const Color(0xFF3B5C48) : const Color(0xFFC1D4B5),
      );
    }
    final river = Path()
      ..moveTo(340, -20)
      ..cubicTo(180, 130, 410, 180, 250, 310)
      ..cubicTo(70, 440, 270, 500, 90, 640);
    canvas.drawPath(
      river,
      Paint()
        ..color = (dark ? const Color(0xFF405369) : const Color(0xFFD5E5EF))
        ..style = PaintingStyle.stroke
        ..strokeWidth = 50,
    );
    final road = Paint()
      ..color = dark ? const Color(0xFF3F454F) : Colors.white
      ..strokeWidth = 14
      ..style = PaintingStyle.stroke;
    canvas.drawPath(
      Path()
        ..moveTo(-10, 105)
        ..lineTo(410, 72),
      road,
    );
    canvas.drawPath(
      Path()
        ..moveTo(-10, 275)
        ..lineTo(410, 230),
      road,
    );
    canvas.drawPath(
      Path()
        ..moveTo(140, -10)
        ..lineTo(185, 610),
      road,
    );
    canvas.drawPath(
      Path()
        ..moveTo(-10, 420)
        ..cubicTo(110, 350, 260, 440, 410, 360),
      road,
    );
    if (showPins) {
      final route = Path()
        ..moveTo(105, 125)
        ..lineTo(163, 119)
        ..lineTo(170, 218)
        ..quadraticBezierTo(220, 242, 280, 210);
      if (precise) {
        canvas.drawPath(
          route,
          Paint()
            ..color = colors.accent.withValues(alpha: .2)
            ..style = PaintingStyle.stroke
            ..strokeWidth = 5,
        );
        final metric = route.computeMetrics().first;
        canvas.drawPath(
          metric.extractPath(0, metric.length * progress),
          Paint()
            ..color = colors.accent
            ..style = PaintingStyle.stroke
            ..strokeWidth = 4
            ..strokeCap = StrokeCap.round,
        );
        if (progress > 0) {
          final p = metric
              .getTangentForOffset(metric.length * progress)!
              .position;
          canvas.drawCircle(p, 7, Paint()..color = colors.paper);
          canvas.drawCircle(p, 4, Paint()..color = colors.accent);
        }
        for (final p in [const Offset(105, 125), const Offset(280, 210)]) {
          canvas.drawCircle(
            p,
            18,
            Paint()..color = colors.accent.withValues(alpha: .13),
          );
          canvas.drawCircle(p, 7, Paint()..color = colors.accent);
          canvas.drawCircle(p, 3, Paint()..color = Colors.white);
        }
      } else {
        canvas.drawCircle(
          const Offset(190, 175),
          70,
          Paint()..color = colors.accent.withValues(alpha: .12),
        );
        canvas.drawCircle(
          const Offset(190, 175),
          70,
          Paint()
            ..color = colors.accent.withValues(alpha: .35)
            ..style = PaintingStyle.stroke
            ..strokeWidth = 1.5,
        );
      }
    }
    canvas.restore();
  }

  @override
  bool shouldRepaint(covariant MapIllustration old) =>
      old.colors != colors ||
      old.progress != progress ||
      old.showPins != showPins ||
      old.precise != precise;
}
