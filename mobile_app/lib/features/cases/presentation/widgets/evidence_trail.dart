import 'package:flutter/material.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../core/widgets/surfaces.dart';
import '../../data/models/evidence_model.dart';

class EvidenceTrail extends StatelessWidget {
  const EvidenceTrail(this.evidence, {super.key});
  final List<EvidenceModel> evidence;
  @override
  Widget build(BuildContext context) {
    final c = AppColors.of(context);
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Follow the evidence',
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: 8),
          Text(
            'Each step reflects the records available in this case.',
            style: Theme.of(
              context,
            ).textTheme.bodyMedium?.copyWith(color: c.muted),
          ),
          const SizedBox(height: 22),
          for (final type in ['ORDER', 'PAYMENT', 'SETTLEMENT']) ...[
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  width: 32,
                  height: 32,
                  decoration: BoxDecoration(
                    color: evidence.any((e) => e.recordType == type)
                        ? c.positiveBg
                        : c.soft,
                    shape: BoxShape.circle,
                  ),
                  child: Icon(
                    evidence.any((e) => e.recordType == type)
                        ? LucideIcons.check
                        : LucideIcons.minus,
                    size: 16,
                    color: evidence.any((e) => e.recordType == type)
                        ? c.positive
                        : c.muted,
                  ),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(switch (type) {
                        'ORDER' => 'Order',
                        'PAYMENT' => 'Payment',
                        _ => 'Settlement',
                      }, style: Theme.of(context).textTheme.titleMedium),
                      const SizedBox(height: 3),
                      Text(
                        evidence.any((e) => e.recordType == type)
                            ? 'Observed in source records'
                            : 'Not observed in available evidence',
                        style: Theme.of(
                          context,
                        ).textTheme.bodyMedium?.copyWith(color: c.muted),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            if (type != 'SETTLEMENT')
              Padding(
                padding: const EdgeInsets.only(left: 15),
                child: Container(width: 1, height: 20, color: c.line),
              ),
          ],
        ],
      ),
    );
  }
}
