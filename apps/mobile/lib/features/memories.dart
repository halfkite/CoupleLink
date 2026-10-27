import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/strings.dart';
import '../core/theme.dart';
import '../ui/widgets.dart';

class MemoriesPage extends ConsumerWidget {
  const MemoriesPage({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final s = ref.watch(stringsProvider), c = context.colors;
    return ListView(
      key: const PageStorageKey('memories-scroll'),
      padding: EdgeInsets.zero,
      children: [
        const PageHeading(title: 'time.title', subtitle: 'time.subtitle'),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              CoupleCard(
                onTap: () => infoSheet(
                  context,
                  s.t('time.anniversaryDetail'),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        s.t('time.anniversary'),
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                      const SizedBox(height: 12),
                      Text(s.t('time.date')),
                      const SizedBox(height: 16),
                      Text(s.t('time.anniversaryHint')),
                    ],
                  ),
                ),
                color: c.heroStart,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Icon(
                          Icons.auto_awesome_rounded,
                          color: c.accent,
                          size: 18,
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            s.t('time.featured'),
                            style: TextStyle(fontSize: 11, color: c.accent),
                          ),
                        ),
                        const PreviewBadge(),
                      ],
                    ),
                    const SizedBox(height: 26),
                    Text(
                      s.t('time.anniversary'),
                      style: Theme.of(context).textTheme.titleLarge,
                    ),
                    const SizedBox(height: 8),
                    Text(
                      s.t('time.date'),
                      style: TextStyle(fontSize: 12, color: c.muted),
                    ),
                    const SizedBox(height: 26),
                    Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        FittedBox(
                          fit: BoxFit.scaleDown,
                          alignment: Alignment.centerLeft,
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Text(
                                '365',
                                style: TextStyle(
                                  fontSize: 54,
                                  height: 1,
                                  fontWeight: FontWeight.w300,
                                  color: c.ink,
                                ),
                              ),
                              const SizedBox(width: 8),
                              Text(
                                s.t('home.daysUnit'),
                                style: TextStyle(color: c.muted),
                              ),
                            ],
                          ),
                        ),
                        const SizedBox(height: 12),
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 12,
                            vertical: 7,
                          ),
                          decoration: BoxDecoration(
                            color: c.paper.withValues(alpha: .65),
                            borderRadius: BorderRadius.circular(12),
                          ),
                          child: Text(
                            s.t('time.today'),
                            style: TextStyle(fontSize: 11, color: c.accent),
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 16),
              CoupleCard(
                child: Row(
                  children: [
                    IconTile(
                      icon: Icons.train_outlined,
                      color: c.secondary,
                      size: 48,
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            s.t('time.meet'),
                            style: Theme.of(context).textTheme.titleMedium,
                          ),
                          const SizedBox(height: 5),
                          Text(
                            s.t('time.meetNote'),
                            style: Theme.of(context).textTheme.bodySmall,
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 28),
              SectionLabel(
                s.t('time.memories'),
                trailing: Icon(
                  Icons.favorite_border_rounded,
                  color: c.accent,
                  size: 20,
                ),
              ),
              const SizedBox(height: 16),
              CoupleCard(
                onTap: () => showAlbum(context, s),
                padding: const EdgeInsets.all(14),
                child: Column(
                  children: [
                    Row(
                      children: [
                        for (var i = 0; i < 3; i++)
                          Expanded(
                            child: Padding(
                              padding: EdgeInsets.only(left: i == 0 ? 0 : 8),
                              child: ClipRRect(
                                borderRadius: BorderRadius.circular(16),
                                child: SizedBox(
                                  height: 105,
                                  child: CustomPaint(painter: MemoryArt(i)),
                                ),
                              ),
                            ),
                          ),
                      ],
                    ),
                    const SizedBox(height: 16),
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 5),
                      child: SectionLabel(
                        s.t('time.album'),
                        subtitle: s.t('time.albumHint'),
                        trailing: Icon(
                          Icons.north_east_rounded,
                          color: c.muted,
                          size: 18,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 26),
              SectionLabel(
                s.t('time.timeline'),
                subtitle: s.t('time.timelineHint'),
              ),
              const SizedBox(height: 18),
              CoupleCard(
                child: Column(
                  children: [
                    for (var i = 3; i >= 1; i--)
                      Padding(
                        padding: EdgeInsets.only(bottom: i == 1 ? 0 : 24),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Container(
                              width: 10,
                              height: 10,
                              margin: const EdgeInsets.only(top: 7),
                              decoration: BoxDecoration(
                                shape: BoxShape.circle,
                                color: i == 3
                                    ? c.accent
                                    : c.secondary.withValues(alpha: .5),
                              ),
                            ),
                            const SizedBox(width: 16),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    s.t('time.memoryDate$i'),
                                    style: Theme.of(context)
                                        .textTheme
                                        .bodySmall,
                                  ),
                                  const SizedBox(height: 4),
                                  Text(
                                    s.t('time.memory$i'),
                                    style: const TextStyle(fontSize: 14),
                                  ),
                                ],
                              ),
                            ),
                            Icon(
                              Icons.favorite_rounded,
                              size: 13,
                              color: c.soft,
                            ),
                          ],
                        ),
                      ),
                  ],
                ),
              ),
              const SizedBox(height: 20),
              Center(
                child: Text(
                  s.t('app.demoNote'),
                  style: Theme.of(context).textTheme.bodySmall,
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

void showAlbum(BuildContext context, AppStrings s) => infoSheet(
  context,
  s.t('time.albumPreview'),
  Column(
    children: [
      Text(
        s.t('time.albumPreviewHint'),
        style: Theme.of(context).textTheme.bodySmall,
      ),
      const SizedBox(height: 16),
      GridView.count(
        crossAxisCount: 2,
        shrinkWrap: true,
        physics: const NeverScrollableScrollPhysics(),
        mainAxisSpacing: 12,
        crossAxisSpacing: 12,
        children: [
          for (var i = 0; i < 6; i++)
            ClipRRect(
              borderRadius: BorderRadius.circular(18),
              child: CustomPaint(painter: MemoryArt(i % 3)),
            ),
        ],
      ),
    ],
  ),
);

class MemoryArt extends CustomPainter {
  MemoryArt(this.variant);
  final int variant;
  @override
  void paint(Canvas canvas, Size size) {
    final palettes = [
      const [Color(0xFFE6B1BC), Color(0xFFF6D8BD)],
      const [Color(0xFFB8B9DD), Color(0xFFE6D5E2)],
      const [Color(0xFFA8C5BB), Color(0xFFE2E5C9)],
    ];
    final rect = Offset.zero & size;
    canvas.drawRect(
      rect,
      Paint()
        ..shader = LinearGradient(
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
          colors: palettes[variant],
        ).createShader(rect),
    );
    canvas.drawCircle(
      Offset(size.width * .65, size.height * .34),
      size.shortestSide * .13,
      Paint()..color = const Color(0xFFFFF0DB),
    );
    for (var i = 0; i < 2; i++) {
      final path = Path()
        ..moveTo(0, size.height * (.65 + i * .15))
        ..cubicTo(
          size.width * .3,
          size.height * (.25 + i * .2),
          size.width * .55,
          size.height * (1 + i * .1),
          size.width,
          size.height * .56,
        )
        ..lineTo(size.width, size.height)
        ..lineTo(0, size.height)
        ..close();
      canvas.drawPath(
        path,
        Paint()
          ..color = [
            const Color(0xFF8D8BAE),
            const Color(0xFF747291),
          ][i].withValues(alpha: .45),
      );
    }
  }

  @override
  bool shouldRepaint(covariant MemoryArt old) => old.variant != variant;
}
