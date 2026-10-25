import 'package:flutter/material.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';
import 'surfaces.dart';

class EmptyView extends StatelessWidget {
  const EmptyView({
    super.key,
    required this.title,
    required this.message,
    this.action,
    this.onAction,
  });
  final String title, message;
  final String? action;
  final VoidCallback? onAction;
  @override
  Widget build(BuildContext context) => AppCard(
    child: Column(
      children: [
        const Icon(LucideIcons.inbox, size: 32),
        const SizedBox(height: 20),
        Text(
          title,
          style: Theme.of(context).textTheme.titleLarge,
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: 8),
        Text(message, textAlign: TextAlign.center),
        if (action != null)
          TextButton(onPressed: onAction, child: Text(action!)),
      ],
    ),
  );
}
