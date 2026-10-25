import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';
import '../../../../app/injection_container.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../core/utils/date_formatter.dart';
import '../../../../core/widgets/empty_view.dart';
import '../../../../core/widgets/error_view.dart';
import '../../../../core/widgets/loading_view.dart';
import '../../../../core/widgets/surfaces.dart';
import '../../data/repositories/cases_repository.dart';
import '../bloc/cases_bloc.dart';
import '../bloc/cases_event.dart';
import '../bloc/cases_state.dart';
import '../models/case_status_presentation.dart';
import '../widgets/status_badge.dart';

class CasesPage extends StatelessWidget {
  const CasesPage({super.key, required this.dependencies});
  final AppDependencies dependencies;
  @override
  Widget build(BuildContext context) => BlocProvider(
    create: (_) =>
        CasesBloc(dependencies.get<CasesRepository>(), dependencies.sessions)
          ..add(const CasesRequested()),
    child: const _CasesView(),
  );
}

class _CasesView extends StatefulWidget {
  const _CasesView();
  @override
  State<_CasesView> createState() => _CasesViewState();
}

class _CasesViewState extends State<_CasesView> {
  Future<void> _refresh() async {
    final b = context.read<CasesBloc>();
    b.add(const CasesRequested());
    await b.stream.firstWhere((s) => !s.loading);
  }

  @override
  Widget build(BuildContext context) => BlocBuilder<CasesBloc, CasesState>(
    builder: (context, state) {
      final c = AppColors.of(context);
      final bloc = context.read<CasesBloc>();
      return PageContent(
        onRefresh: _refresh,
        children: [
          const PageHeading(
            'Your review desk',
            'A clear next step for every case.',
          ),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(22),
            decoration: BoxDecoration(
              color: c.brand,
              borderRadius: BorderRadius.circular(18),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Evidence first.',
                  style: Theme.of(context).textTheme.titleLarge?.copyWith(
                    color: c.onBrand,
                    fontSize: 22,
                  ),
                ),
                const SizedBox(height: 12),
                Text(
                  'Review the records. Make the call.',
                  style: Theme.of(
                    context,
                  ).textTheme.bodyMedium?.copyWith(color: c.onBrand),
                ),
              ],
            ),
          ),
          const SizedBox(height: 26),
          SegmentedButton<bool>(
            segments: const [
              ButtonSegment(value: true, label: Text('Assigned to me')),
              ButtonSegment(value: false, label: Text('All cases')),
            ],
            selected: {state.mine},
            onSelectionChanged: (v) => bloc.add(
              CasesFilterChanged(mine: v.first, status: state.status),
            ),
            showSelectedIcon: false,
          ),
          const SizedBox(height: 16),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final entry in <String?, String>{
                null: 'Any status',
                'OPEN': 'Open',
                'EVIDENCE_UPDATED': 'Updated',
                'RESOLVED': 'Resolved',
              }.entries)
                ChoiceChip(
                  label: Text(entry.value),
                  selected: state.status == entry.key,
                  showCheckmark: false,
                  onSelected: (_) => bloc.add(
                    CasesFilterChanged(mine: state.mine, status: entry.key),
                  ),
                ),
            ],
          ),
          SectionTitle(
            state.mine ? 'Assigned cases' : 'Case inbox',
            trailing: IconButton(
              tooltip: 'Refresh cases',
              onPressed: state.loading ? null : _refresh,
              icon: const Icon(LucideIcons.refreshCw, size: 18),
            ),
          ),
          if (state.failure != null) ...[
            if (state.items.isEmpty)
              ErrorView(failure: state.failure!, onRetry: _refresh)
            else
              Notice(
                'Showing the previous list. ${state.failure!.message}',
                negative: true,
              ),
          ],
          if (state.loading && state.items.isEmpty)
            const LoadingView(label: 'Finding your cases…')
          else if (state.items.isEmpty && state.failure == null)
            EmptyView(
              title: state.mine ? 'Your desk is clear' : 'No cases here',
              message: state.mine
                  ? 'No cases match your current assignment and filters.'
                  : 'Try another status or pull down to refresh.',
              action: state.mine ? 'View all cases' : 'Clear status filter',
              onAction: () => bloc.add(const CasesFilterChanged(mine: false)),
            )
          else if (state.items.isNotEmpty)
            AppCard(
              padding: EdgeInsets.zero,
              child: Column(
                children: [
                  for (var i = 0; i < state.items.length; i++) ...[
                    if (i > 0) const Divider(),
                    InkWell(
                      borderRadius: BorderRadius.circular(16),
                      onTap: () async {
                        await context.push('/cases/${state.items[i].id}');
                        if (context.mounted) bloc.add(const CasesRequested());
                      },
                      child: Padding(
                        padding: const EdgeInsets.all(20),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Wrap(
                              spacing: 10,
                              runSpacing: 8,
                              crossAxisAlignment: WrapCrossAlignment.center,
                              children: [
                                StatusBadge(state.items[i].status),
                                Text(
                                  '${state.items[i].severity.toLowerCase()} priority',
                                  style: Theme.of(context).textTheme.bodySmall
                                      ?.copyWith(color: c.muted),
                                ),
                              ],
                            ),
                            const SizedBox(height: 16),
                            Row(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Expanded(
                                  child: Text(
                                    state.items[i].title,
                                    style: Theme.of(
                                      context,
                                    ).textTheme.titleMedium,
                                  ),
                                ),
                                const SizedBox(width: 10),
                                const Icon(LucideIcons.arrowUpRight, size: 18),
                              ],
                            ),
                            const SizedBox(height: 8),
                            Text(
                              kindLabel(state.items[i].kind),
                              style: Theme.of(
                                context,
                              ).textTheme.bodyMedium?.copyWith(color: c.muted),
                            ),
                            const SizedBox(height: 22),
                            Wrap(
                              spacing: 16,
                              runSpacing: 8,
                              children: [
                                Text(
                                  '${state.items[i].evidenceCount} evidence records',
                                  style: Theme.of(context).textTheme.bodySmall,
                                ),
                                Text(
                                  'Updated ${formatDate(state.items[i].updatedAt)}',
                                  style: Theme.of(context).textTheme.bodySmall
                                      ?.copyWith(color: c.muted),
                                ),
                              ],
                            ),
                          ],
                        ),
                      ),
                    ),
                  ],
                ],
              ),
            ),
          if (state.cursor != null)
            Padding(
              padding: const EdgeInsets.only(top: 16),
              child: OutlinedButton(
                onPressed: state.loadingMore
                    ? null
                    : () => bloc.add(const CasesNextPage()),
                child: Text(state.loadingMore ? 'Loading…' : 'Load more cases'),
              ),
            ),
          const SizedBox(height: 22),
          Text(
            'Case decisions and settlement observations are recorded separately.',
            style: Theme.of(
              context,
            ).textTheme.bodySmall?.copyWith(color: c.muted),
          ),
        ],
      );
    },
  );
}
