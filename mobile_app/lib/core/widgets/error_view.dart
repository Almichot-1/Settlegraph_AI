import 'package:flutter/material.dart';
import '../error/failures.dart';
import 'surfaces.dart';

class ErrorView extends StatelessWidget {
  const ErrorView({super.key, required this.failure, required this.onRetry});
  final Failure failure;
  final VoidCallback onRetry;
  @override
  Widget build(BuildContext context) => AppCard(
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          failure.status == 404
              ? 'Case unavailable'
              : 'Could not load this view',
          style: Theme.of(context).textTheme.titleLarge,
        ),
        const SizedBox(height: 12),
        Text(failure.message),
        if (failure.requestId != null) ...[
          const SizedBox(height: 8),
          Text(
            'Reference: ${failure.requestId}',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
        const SizedBox(height: 8),
        TextButton(onPressed: onRetry, child: const Text('Try again')),
      ],
    ),
  );
}
