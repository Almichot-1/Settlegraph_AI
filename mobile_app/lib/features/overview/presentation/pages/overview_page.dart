import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';
import '../../../../app/injection_container.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../core/models/load_state.dart';
import '../../../../core/models/money.dart';
import '../../../../core/utils/date_formatter.dart';
import '../../../../core/utils/money_formatter.dart';
import '../../../../core/widgets/error_view.dart';
import '../../../../core/widgets/loading_view.dart';
import '../../../../core/widgets/surfaces.dart';
import '../../data/models/dashboard_model.dart';
import '../../data/repositories/overview_repository.dart';
import '../bloc/overview_cubit.dart';

class OverviewPage extends StatelessWidget {
  const OverviewPage({super.key, required this.dependencies});
  final AppDependencies dependencies;
  @override
  Widget build(BuildContext context) => BlocProvider(
    create: (_) =>
        OverviewCubit(dependencies.get<OverviewRepository>())..refresh(),
    child: BlocBuilder<OverviewCubit, LoadState<DashboardModel>>(
      builder: (context, state) {
        final cubit = context.read<OverviewCubit>();
        final c = AppColors.of(context);
        final data = state.value;
        return PageContent(
          onRefresh: cubit.refresh,
          children: [
            const PageHeading(
              'The bigger picture',
              'Payment and settlement observations, kept clear.',
            ),
            if (state.failure != null && data == null)
              ErrorView(failure: state.failure!, onRetry: cubit.refresh)
            else if (data == null)
              const LoadingView()
            else ...[
              if (state.failure != null)
                Notice(
                  'Showing the earlier snapshot. ${state.failure!.message}',
                  negative: true,
                ),
              if (data.coverage != 'COMPLETE')
                const Notice(
                  'Source data is incomplete or stale. These observations may not include every settlement.',
                ),
              Text(
                'AS OF ${formatDate(data.asOf)}',
                style: Theme.of(context).textTheme.labelMedium?.copyWith(
                  color: c.muted,
                  letterSpacing: 1,
                ),
              ),
              const SizedBox(height: 16),
              _metric(
                context,
                'Payments accepted',
                data.payments,
                'An accepted payment is separate from a settlement.',
                prominent: true,
              ),
              const SizedBox(height: 12),
              _metric(
                context,
                'Settlements observed',
                data.settlements,
                'Visible settlement records from the sources.',
              ),
              const SizedBox(height: 12),
              _metric(
                context,
                'Orders expected',
                data.orders,
                'Order amounts in this organization.',
              ),
              const SizedBox(height: 12),
              _metric(
                context,
                'Unmatched payments',
                data.unmatched,
                'Accepted payments awaiting an eligible match.',
              ),
              const SizedBox(height: 20),
              AppCard(
                padding: EdgeInsets.zero,
                child: ListTile(
                  contentPadding: const EdgeInsets.symmetric(
                    horizontal: 20,
                    vertical: 12,
                  ),
                  leading: const Icon(LucideIcons.inbox),
                  title: Text(
                    '${data.openCases} open ${data.openCases == 1 ? 'case' : 'cases'}',
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                  subtitle: const Text('Continue with the evidence'),
                  trailing: const Icon(LucideIcons.arrowUpRight),
                  onTap: () => context.go('/cases'),
                ),
              ),
              const SectionTitle('Source health'),
              for (final provider in data.providers)
                Padding(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: AppCard(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Expanded(
                              child: Text(
                                provider.name,
                                style: Theme.of(context).textTheme.titleMedium,
                              ),
                            ),
                            Icon(
                              provider.status == 'HEALTHY'
                                  ? LucideIcons.circleCheck
                                  : LucideIcons.circleAlert,
                              color: provider.status == 'HEALTHY'
                                  ? c.positive
                                  : c.attention,
                              size: 20,
                            ),
                          ],
                        ),
                        const SizedBox(height: 8),
                        Text(switch (provider.status) {
                          'HEALTHY' => 'Reporting normally',
                          'STALE' => 'Source data is stale',
                          'OUTAGE' => 'Source unavailable',
                          'NEVER_SYNCED' => 'No observations yet',
                          _ => 'Unknown source status',
                        }),
                        const SizedBox(height: 6),
                        Text(
                          provider.lastObservedAt == null
                              ? 'No last observation supplied'
                              : 'Last observed ${formatDate(provider.lastObservedAt!)}',
                          style: Theme.of(
                            context,
                          ).textTheme.bodySmall?.copyWith(color: c.muted),
                        ),
                      ],
                    ),
                  ),
                ),
              const SizedBox(height: 12),
              Text(
                'These amounts describe synthetic observations. They are not a live bank balance.',
                style: Theme.of(
                  context,
                ).textTheme.bodySmall?.copyWith(color: c.muted),
              ),
            ],
          ],
        );
      },
    ),
  );
  Widget _metric(
    BuildContext context,
    String title,
    Money money,
    String description, {
    bool prominent = false,
  }) {
    final c = AppColors.of(context);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(22),
      decoration: BoxDecoration(
        color: prominent ? c.brand : c.card,
        borderRadius: BorderRadius.circular(16),
        border: prominent ? null : Border.all(color: c.line),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: prominent ? c.onBrand : c.muted,
            ),
          ),
          const SizedBox(height: 12),
          Text(
            formatMoney(money),
            style: Theme.of(context).textTheme.headlineMedium?.copyWith(
              color: prominent ? c.onBrand : c.ink,
              fontFeatures: const [FontFeature.tabularFigures()],
            ),
          ),
          const SizedBox(height: 12),
          Text(
            description,
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
              color: prominent ? c.onBrand : c.muted,
            ),
          ),
        ],
      ),
    );
  }
}
