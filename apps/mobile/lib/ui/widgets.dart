import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/strings.dart';
import '../core/theme.dart';

class PreviewBadge extends ConsumerWidget {
  const PreviewBadge({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
    decoration: BoxDecoration(
      color: context.colors.soft,
      borderRadius: BorderRadius.circular(8),
    ),
    child: Text(
      ref.watch(stringsProvider).t('app.demo'),
      style: TextStyle(
        fontSize: 10,
        fontWeight: FontWeight.w700,
        letterSpacing: .5,
        color: context.colors.accent,
      ),
    ),
  );
}

class PageHeading extends ConsumerWidget {
  const PageHeading({
    super.key,
    required this.title,
    required this.subtitle,
    this.trailing,
    this.padding = const EdgeInsets.fromLTRB(20, 16, 20, 24),
  });
  final String title, subtitle;
  final Widget? trailing;
  final EdgeInsetsGeometry padding;
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final s = ref.watch(stringsProvider);
    return Padding(
      padding: padding,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.favorite_rounded,
                size: 18,
                color: context.colors.accent,
              ),
              const SizedBox(width: 7),
              Expanded(
                child: Text(
                  s.t('app.name'),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w800,
                    letterSpacing: .2,
                    color: context.colors.ink,
                  ),
                ),
              ),
              const Flexible(child: PreviewBadge()),
              ?trailing,
            ],
          ),
          const SizedBox(height: 24),
          Text(s.t(title), style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 7),
          Text(
            s.t(subtitle),
            style: TextStyle(
              color: context.colors.muted,
              fontSize: 12,
              height: 1.6,
            ),
          ),
        ],
      ),
    );
  }
}

class CoupleCard extends StatelessWidget {
  const CoupleCard({
    super.key,
    required this.child,
    this.onTap,
    this.onLongPress,
    this.padding = const EdgeInsets.all(20),
    this.color,
  });
  final Widget child;
  final VoidCallback? onTap, onLongPress;
  final EdgeInsetsGeometry padding;
  final Color? color;
  @override
  Widget build(BuildContext context) => DecoratedBox(
    decoration: BoxDecoration(
      borderRadius: BorderRadius.circular(24),
      boxShadow: [
        BoxShadow(
          color: context.colors.ink.withValues(alpha: .025),
          blurRadius: 20,
          offset: const Offset(0, 6),
        ),
      ],
    ),
    child: Material(
      color: color ?? context.colors.paper,
      borderRadius: BorderRadius.circular(24),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        onLongPress: onLongPress,
        child: Padding(padding: padding, child: child),
      ),
    ),
  );
}

class SectionLabel extends StatelessWidget {
  const SectionLabel(this.title, {super.key, this.subtitle, this.trailing});
  final String title;
  final String? subtitle;
  final Widget? trailing;
  @override
  Widget build(BuildContext context) => Row(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Expanded(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(title, style: Theme.of(context).textTheme.titleMedium),
            if (subtitle != null) ...[
              const SizedBox(height: 4),
              Text(subtitle!, style: Theme.of(context).textTheme.bodySmall),
            ],
          ],
        ),
      ),
      ?trailing,
    ],
  );
}

class PersonAvatar extends StatelessWidget {
  const PersonAvatar({super.key, required this.partner, this.size = 48});
  final bool partner;
  final double size;
  @override
  Widget build(BuildContext context) => Container(
    width: size,
    height: size,
    decoration: BoxDecoration(
      shape: BoxShape.circle,
      color: partner ? const Color(0xFFE3D9F1) : const Color(0xFFF5D5DA),
      border: Border.all(color: context.colors.paper, width: 3),
    ),
    child: Icon(
      partner ? Icons.face_3_rounded : Icons.face_6_rounded,
      color: partner ? const Color(0xFF8770A8) : const Color(0xFFC3798D),
      size: size * .68,
    ),
  );
}

class IconTile extends StatelessWidget {
  const IconTile({super.key, required this.icon, this.color, this.size = 42});
  final IconData icon;
  final Color? color;
  final double size;
  @override
  Widget build(BuildContext context) => Container(
    width: size,
    height: size,
    decoration: BoxDecoration(
      color: (color ?? context.colors.accent).withValues(alpha: .1),
      borderRadius: BorderRadius.circular(size * .32),
    ),
    child: Icon(icon, size: size * .48, color: color ?? context.colors.accent),
  );
}

class SettingTile extends StatelessWidget {
  const SettingTile({
    super.key,
    required this.icon,
    required this.title,
    this.subtitle,
    this.onTap,
    this.trailing,
  });
  final IconData icon;
  final String title;
  final String? subtitle;
  final VoidCallback? onTap;
  final Widget? trailing;
  @override
  Widget build(BuildContext context) => ListTile(
    contentPadding: const EdgeInsets.symmetric(horizontal: 4, vertical: 5),
    minVerticalPadding: 10,
    leading: IconTile(icon: icon),
    title: Text(title, style: const TextStyle(fontSize: 14)),
    subtitle: subtitle == null
        ? null
        : Text(subtitle!, style: Theme.of(context).textTheme.bodySmall),
    trailing:
        trailing ??
        Icon(Icons.chevron_right_rounded, color: context.colors.muted),
    onTap: onTap,
  );
}

void toast(BuildContext context, String message) {
  ScaffoldMessenger.of(context)
    ..hideCurrentSnackBar()
    ..showSnackBar(SnackBar(content: Text(message)));
}

Future<void> infoSheet(BuildContext context, String title, Widget child) =>
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      builder: (context) => Padding(
        padding: EdgeInsets.fromLTRB(
          24,
          0,
          24,
          24 + MediaQuery.viewInsetsOf(context).bottom,
        ),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(title, style: Theme.of(context).textTheme.titleLarge),
              const SizedBox(height: 16),
              child,
            ],
          ),
        ),
      ),
    );

const actionIcons = <String, IconData>{
  'miss_you': Icons.favorite_rounded,
  'poke': Icons.touch_app_rounded,
  'hug': Icons.diversity_1_rounded,
  'ring': Icons.notifications_active_rounded,
};
const actionKeys = <String, String>{
  'miss_you': 'interaction.miss',
  'poke': 'interaction.poke',
  'hug': 'interaction.hug',
  'ring': 'interaction.ring',
};
void previewGesture(BuildContext context, AppStrings s, String id) {
  if (id == 'ring') {
    infoSheet(
      context,
      s.t('interaction.ringTitle'),
      Text(s.t('interaction.ringHint')),
    );
    return;
  }
  showDialog<void>(
    context: context,
    builder: (_) => GesturePreview(action: id, strings: s),
  );
}

class GesturePreview extends StatefulWidget {
  const GesturePreview({
    super.key,
    required this.action,
    required this.strings,
  });
  final String action;
  final AppStrings strings;
  @override
  State<GesturePreview> createState() => _GesturePreviewState();
}

class _GesturePreviewState extends State<GesturePreview>
    with SingleTickerProviderStateMixin {
  late final AnimationController animation = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1100),
  );
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!context.reducedMotion) animation.forward();
  }

  @override
  void dispose() {
    animation.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => Dialog(
    child: Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          AnimatedBuilder(
            animation: animation,
            builder: (context, child) {
              final t = animation.value;
              return SizedBox(
                height: 140,
                child: Center(
                  child: Stack(
                    alignment: Alignment.center,
                    children: [
                      for (var i = 0; i < 5; i++)
                        Transform.translate(
                          offset: Offset(
                            math.cos(i * 1.3) * 54 * t,
                            -math.sin(i * 1.3).abs() * 42 * t,
                          ),
                          child: Opacity(
                            opacity: (1 - t).clamp(0, 1),
                            child: Icon(
                              Icons.favorite_rounded,
                              size: 15,
                              color: context.colors.accent,
                            ),
                          ),
                        ),
                      Transform.scale(
                        scale: context.reducedMotion
                            ? 1
                            : .9 + .15 * math.sin(t * math.pi * 3).abs(),
                        child: Icon(
                          actionIcons[widget.action],
                          size: 78,
                          color: context.colors.accent,
                        ),
                      ),
                    ],
                  ),
                ),
              );
            },
          ),
          Text(
            widget.strings.t('interaction.preview', {
              'action': widget.strings.t(actionKeys[widget.action]!),
            }),
            textAlign: TextAlign.center,
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: 12),
          Text(
            widget.strings.t('interaction.previewHint'),
            textAlign: TextAlign.center,
            style: Theme.of(context).textTheme.bodySmall,
          ),
          const SizedBox(height: 20),
          FilledButton(
            onPressed: () => Navigator.pop(context),
            child: Text(widget.strings.t('common.done')),
          ),
        ],
      ),
    ),
  );
}

class PetArt extends StatelessWidget {
  const PetArt({super.key, this.size = 104});
  final double size;
  @override
  Widget build(BuildContext context) => ExcludeSemantics(
    child: CustomPaint(
      size: Size.square(size),
      painter: _PetPainter(context.colors.accent),
    ),
  );
}

class _PetPainter extends CustomPainter {
  _PetPainter(this.accent);
  final Color accent;
  @override
  void paint(Canvas canvas, Size size) {
    canvas.scale(size.width / 120, size.height / 120);
    final paint = Paint()..color = const Color(0xFFE6DCE9);
    canvas.drawOval(const Rect.fromLTWH(20, 101, 80, 9), paint);
    paint.color = const Color(0xFFFFFAF4);
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(28, 53, 65, 52),
        const Radius.circular(26),
      ),
      paint,
    );
    final ears = Path()
      ..moveTo(28, 53)
      ..lineTo(27, 17)
      ..quadraticBezierTo(30, 12, 48, 33)
      ..lineTo(78, 33)
      ..quadraticBezierTo(100, 12, 99, 20)
      ..lineTo(94, 57)
      ..close();
    canvas.drawPath(ears, paint);
    canvas.drawOval(const Rect.fromLTWH(23, 30, 76, 61), paint);
    paint.color = const Color(0xFFF0B7CB);
    canvas.drawPath(
      Path()
        ..moveTo(32, 38)
        ..lineTo(32, 24)
        ..lineTo(44, 37)
        ..close(),
      paint,
    );
    canvas.drawPath(
      Path()
        ..moveTo(83, 37)
        ..lineTo(94, 23)
        ..lineTo(93, 42)
        ..close(),
      paint,
    );
    canvas.drawOval(const Rect.fromLTWH(31, 62, 13, 6), paint);
    canvas.drawOval(const Rect.fromLTWH(79, 62, 13, 6), paint);
    paint
      ..color = const Color(0xFF51454E)
      ..strokeWidth = 2.5
      ..strokeCap = StrokeCap.round
      ..style = PaintingStyle.stroke;
    canvas.drawArc(const Rect.fromLTWH(42, 49, 8, 9), .1, 2.9, false, paint);
    canvas.drawArc(const Rect.fromLTWH(72, 49, 8, 9), .1, 2.9, false, paint);
    canvas.drawPath(
      Path()
        ..moveTo(56, 66)
        ..quadraticBezierTo(60, 73, 64, 66),
      paint,
    );
    paint
      ..style = PaintingStyle.fill
      ..color = accent;
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(45, 83, 34, 7),
        const Radius.circular(4),
      ),
      paint,
    );
    canvas.drawCircle(
      const Offset(63, 90),
      4,
      Paint()..color = const Color(0xFFE4BC69),
    );
    canvas.drawOval(
      const Rect.fromLTWH(31, 95, 20, 13),
      Paint()..color = const Color(0xFFFFFAF4),
    );
    canvas.drawOval(
      const Rect.fromLTWH(74, 95, 20, 13),
      Paint()..color = const Color(0xFFFFFAF4),
    );
  }

  @override
  bool shouldRepaint(covariant _PetPainter oldDelegate) =>
      oldDelegate.accent != accent;
}
