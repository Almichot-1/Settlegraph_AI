import 'package:flutter/material.dart';
import '../../../../app/theme/app_colors.dart';
import '../models/case_status_presentation.dart';

class StatusBadge extends StatelessWidget {
  const StatusBadge(this.status, {super.key});
  final String status;
  @override
  Widget build(BuildContext context) {
    final c = AppColors.of(context);
    final good = status == 'RESOLVED';
    final attention = status == 'OPEN';
    final color = good
        ? c.positive
        : attention
        ? c.attention
        : c.ink;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: good
            ? c.positiveBg
            : attention
            ? c.attentionBg
            : c.soft,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        caseStatusLabel(status),
        style: Theme.of(context).textTheme.labelMedium?.copyWith(color: color),
      ),
    );
  }
}
