import 'package:flutter/material.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';
import '../../app/theme/app_colors.dart';

class AppCard extends StatelessWidget {
  const AppCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(20),
  });
  final Widget child;
  final EdgeInsetsGeometry padding;
  @override
  Widget build(BuildContext context) {
    final c = AppColors.of(context);
    return Container(
      width: double.infinity,
      padding: padding,
      decoration: BoxDecoration(
        color: c.card,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: c.line),
      ),
      child: child,
    );
  }
}

class Notice extends StatelessWidget {
  const Notice(this.message, {super.key, this.negative = false});
  final String message;
  final bool negative;
  @override
  Widget build(BuildContext context) {
    final c = AppColors.of(context);
    return Semantics(
      liveRegion: true,
      child: Container(
        width: double.infinity,
        margin: const EdgeInsets.only(bottom: 16),
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: negative ? c.negativeBg : c.attentionBg,
          borderRadius: BorderRadius.circular(12),
        ),
        child: Text(
          message,
          style: Theme.of(context).textTheme.bodyMedium?.copyWith(
            color: negative ? c.negative : c.attention,
          ),
        ),
      ),
    );
  }
}

class SectionTitle extends StatelessWidget {
  const SectionTitle(this.title, {super.key, this.trailing});
  final String title;
  final Widget? trailing;
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(top: 28, bottom: 14),
    child: Row(
      children: [
        Expanded(
          child: Text(title, style: Theme.of(context).textTheme.titleLarge),
        ),
        ?trailing,
      ],
    ),
  );
}

class PageHeading extends StatelessWidget {
  const PageHeading(this.title, this.subtitle, {super.key});
  final String title, subtitle;
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(top: 18, bottom: 28),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(title, style: Theme.of(context).textTheme.headlineLarge),
        const SizedBox(height: 10),
        Text(
          subtitle,
          style: Theme.of(
            context,
          ).textTheme.bodyLarge?.copyWith(color: AppColors.of(context).muted),
        ),
      ],
    ),
  );
}

class PageContent extends StatelessWidget {
  const PageContent({super.key, required this.children, this.onRefresh});
  final List<Widget> children;
  final Future<void> Function()? onRefresh;
  @override
  Widget build(BuildContext context) {
    final scroll = LayoutBuilder(
      builder: (context, constraints) => SingleChildScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(24, 0, 24, 32),
        child: Align(
          alignment: Alignment.topCenter,
          child: ConstrainedBox(
            constraints: BoxConstraints(
              maxWidth: 680,
              minHeight: constraints.maxHeight > 32
                  ? constraints.maxHeight - 32
                  : 0,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: children,
            ),
          ),
        ),
      ),
    );
    return onRefresh == null
        ? scroll
        : RefreshIndicator(onRefresh: onRefresh!, child: scroll);
  }
}

class BrandLogo extends StatelessWidget {
  const BrandLogo({super.key, this.compact = false});
  final bool compact;
  @override
  Widget build(BuildContext context) {
    final c = AppColors.of(context);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          width: compact ? 30 : 40,
          height: compact ? 30 : 40,
          decoration: BoxDecoration(
            color: c.brand,
            borderRadius: BorderRadius.circular(10),
          ),
          child: Icon(
            LucideIcons.layers2,
            color: c.onBrand,
            size: compact ? 18 : 23,
          ),
        ),
        const SizedBox(width: 10),
        Flexible(
          child: Text(
            'SettleGraph',
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
              fontSize: compact ? 17 : 21,
              letterSpacing: -.6,
            ),
          ),
        ),
      ],
    );
  }
}

class KeyValue extends StatelessWidget {
  const KeyValue(this.label, this.value, {super.key});
  final String label, value;
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: 10),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: Theme.of(
            context,
          ).textTheme.bodyMedium?.copyWith(color: AppColors.of(context).muted),
        ),
        const SizedBox(height: 4),
        SelectableText(value, style: Theme.of(context).textTheme.bodyLarge),
      ],
    ),
  );
}
