import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';
import '../../../../app/injection_container.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../core/models/load_state.dart';
import '../../../../core/utils/date_formatter.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/empty_view.dart';
import '../../../../core/widgets/error_view.dart';
import '../../../../core/widgets/loading_view.dart';
import '../../../../core/widgets/surfaces.dart';
import '../../data/models/investigation_model.dart';
import '../../data/repositories/investigations_repository.dart';
import '../bloc/investigations_cubit.dart';

class InvestigationsPage extends StatefulWidget {
  const InvestigationsPage({
    super.key,
    required this.dependencies,
    required this.caseId,
  });
  final AppDependencies dependencies;
  final String caseId;
  @override
  State<InvestigationsPage> createState() => _InvestigationsPageState();
}

class _InvestigationsPageState extends State<InvestigationsPage>
    with WidgetsBindingObserver {
  late final InvestigationsCubit _cubit;
  @override
  void initState() {
    super.initState();
    _cubit = InvestigationsCubit(
      widget.dependencies.get<InvestigationsRepository>(),
      widget.caseId,
    )..refresh();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) =>
      _cubit.setActive(state == AppLifecycleState.resumed);
  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    unawaited(_cubit.close());
    super.dispose();
  }

  Widget _citations(List<String> ids) => Wrap(
    spacing: 8,
    runSpacing: 8,
    children: [
      for (var i = 0; i < ids.length; i++)
        OutlinedButton.icon(
          onPressed: () => context.push(
            '/evidence/${widget.caseId}/${Uri.encodeComponent(ids[i])}',
          ),
          icon: const Icon(LucideIcons.fileText, size: 16),
          label: Text('Source ${i + 1}'),
        ),
    ],
  );
  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('Investigation')),
    body: SafeArea(
      child: BlocBuilder<InvestigationsCubit, LoadState<List<InvestigationModel>>>(
        bloc: _cubit,
        builder: (context, state) {
          final c = AppColors.of(context);
          final results = state.value ?? [];
          final canCreate =
              widget.dependencies.sessions.membership?.canInvestigate == true;
          return PageContent(
            onRefresh: _cubit.refresh,
            children: [
              const PageHeading(
                'Understand the case',
                'Explanations grounded in the available records.',
              ),
              if (state.failure != null)
                ErrorView(failure: state.failure!, onRetry: _cubit.refresh),
              if (state.loading && state.value == null)
                const LoadingView(label: 'Loading investigation…'),
              if (!state.loading && results.isEmpty && state.failure == null)
                const EmptyView(
                  title: 'No investigation yet',
                  message:
                      'An owner or analyst can request an analysis of this case. You can still inspect evidence and record your review.',
                ),
              for (var index = 0; index < results.length; index++) ...[
                if (index > 0) const SectionTitle('Earlier investigation'),
                AppCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Wrap(
                        spacing: 8,
                        runSpacing: 8,
                        children: [
                          Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 10,
                              vertical: 6,
                            ),
                            decoration: BoxDecoration(
                              color: c.soft,
                              borderRadius: BorderRadius.circular(8),
                            ),
                            child: Text(
                              results[index].sourceLabel,
                              style: Theme.of(context).textTheme.labelMedium,
                            ),
                          ),
                          Text(
                            formatDate(results[index].createdAt),
                            style: Theme.of(
                              context,
                            ).textTheme.bodySmall?.copyWith(color: c.muted),
                          ),
                        ],
                      ),
                      const SizedBox(height: 20),
                      if (!results[index].supported)
                        const Notice(
                          'This analysis uses an unsupported status or source. Its result cannot be displayed.',
                        )
                      else if (results[index].pending) ...[
                        const Text(
                          'Analysis in progress',
                          style: TextStyle(fontWeight: FontWeight.w600),
                        ),
                        const SizedBox(height: 8),
                        const Text(
                          'You can continue reviewing the evidence. Pull down to check again if this takes longer than expected.',
                        ),
                      ] else if (!results[index].completed)
                        Notice(
                          results[index].status == 'REJECTED'
                              ? 'This analysis was rejected by the evidence checks. Use the source records for your review.'
                              : 'The investigation could not be completed. Your case and source evidence are available.',
                        )
                      else if (results[index].result != null) ...[
                        Text(
                          'Summary',
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                        const SizedBox(height: 10),
                        Text(
                          results[index].result!.summary,
                          style: Theme.of(context).textTheme.bodyLarge,
                        ),
                        if (results[index].result!.hypotheses.isNotEmpty)
                          const SectionTitle('Possible explanations'),
                        for (final hypothesis
                            in results[index].result!.hypotheses) ...[
                          Text(hypothesis.explanation),
                          const SizedBox(height: 12),
                          _citations(hypothesis.evidenceIds),
                          const SizedBox(height: 18),
                        ],
                        const SectionTitle('Sources'),
                        _citations(results[index].result!.evidenceIds),
                        if (results[index].result!.actions.isNotEmpty)
                          const SectionTitle('Suggested next steps'),
                        for (final action in results[index].result!.actions)
                          Padding(
                            padding: const EdgeInsets.only(bottom: 12),
                            child: Text('•  $action'),
                          ),
                        const SectionTitle('Limitations'),
                        for (final limitation
                            in results[index].result!.limitations)
                          Padding(
                            padding: const EdgeInsets.only(bottom: 10),
                            child: Text(
                              limitation,
                              style: Theme.of(
                                context,
                              ).textTheme.bodyMedium?.copyWith(color: c.muted),
                            ),
                          ),
                      ],
                    ],
                  ),
                ),
                const SizedBox(height: 16),
              ],
              if (results.isNotEmpty)
                const Notice(
                  'Analysis describes an evidence snapshot. Later records may change the picture. Your decision remains a separate human review.',
                ),
              if (canCreate) ...[
                const SizedBox(height: 10),
                AppButton(
                  label: 'Request a fresh analysis',
                  icon: LucideIcons.refreshCw,
                  secondary: true,
                  loading: state.loading,
                  onPressed: () => _cubit.create(permitted: canCreate),
                ),
              ],
            ],
          );
        },
      ),
    ),
  );
}
