import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';
import '../../../../app/injection_container.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../core/models/load_state.dart';
import '../../../../core/utils/date_formatter.dart';
import '../../../../core/utils/money_formatter.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/error_view.dart';
import '../../../../core/widgets/loading_view.dart';
import '../../../../core/widgets/surfaces.dart';
import '../../data/repositories/cases_repository.dart';
import '../bloc/case_detail_cubit.dart';
import '../widgets/evidence_trail.dart';
import '../widgets/status_badge.dart';

class CaseDetailPage extends StatefulWidget {
  const CaseDetailPage({
    super.key,
    required this.dependencies,
    required this.id,
  });
  final AppDependencies dependencies;
  final String id;
  @override
  State<CaseDetailPage> createState() => _CaseDetailPageState();
}

class _CaseDetailPageState extends State<CaseDetailPage>
    with WidgetsBindingObserver {
  late final CaseDetailCubit _cubit;
  Timer? _timer;
  int _polls = 0;
  @override
  void initState() {
    super.initState();
    _cubit = CaseDetailCubit(
      widget.dependencies.get<CasesRepository>(),
      widget.id,
    )..refresh();
    WidgetsBinding.instance.addObserver(this);
    _startPolling();
  }

  void _startPolling() {
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 10), (t) {
      if (++_polls > 30) {
        t.cancel();
        return;
      }
      if (ModalRoute.of(context)?.isCurrent == true && !_cubit.state.loading) {
        unawaited(_cubit.refresh());
      }
    });
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    _timer?.cancel();
    if (state == AppLifecycleState.resumed) {
      _polls = 0;
      unawaited(_cubit.refresh());
      _startPolling();
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _timer?.cancel();
    unawaited(_cubit.close());
    super.dispose();
  }

  @override
  Widget build(
    BuildContext context,
  ) => BlocBuilder<CaseDetailCubit, LoadState<CaseBundle>>(
    bloc: _cubit,
    builder: (context, state) {
      final record = state.value?.caseRecord;
      final c = AppColors.of(context);
      final membership = widget.dependencies.sessions.membership;
      return Scaffold(
        appBar: AppBar(
          title: const Text('Case details'),
          actions: [
            IconButton(
              tooltip: 'Refresh case',
              onPressed: state.loading ? null : _cubit.refresh,
              icon: const Icon(LucideIcons.refreshCw, size: 20),
            ),
          ],
        ),
        bottomNavigationBar:
            record != null && record.supported && membership?.canReview == true
            ? SafeArea(
                top: false,
                child: Container(
                  padding: const EdgeInsets.fromLTRB(24, 12, 24, 16),
                  decoration: BoxDecoration(
                    color: c.card,
                    border: Border(top: BorderSide(color: c.line)),
                  ),
                  child: AppButton(
                    label: record.resolved
                        ? 'Review resolved case'
                        : 'Review case',
                    icon: LucideIcons.arrowRight,
                    onPressed: state.loading
                        ? null
                        : () async {
                            await context.push('/review/${record.id}');
                            if (mounted) await _cubit.refresh();
                          },
                  ),
                ),
              )
            : null,
        body: SafeArea(
          child: PageContent(
            onRefresh: _cubit.refresh,
            children: [
              if (state.failure != null && state.value == null)
                ErrorView(failure: state.failure!, onRetry: _cubit.refresh)
              else if (record == null)
                const LoadingView(label: 'Opening case…')
              else ...[
                if (state.failure != null)
                  Notice(
                    'Showing an earlier snapshot. ${state.failure!.message}',
                    negative: true,
                  ),
                const SizedBox(height: 16),
                Wrap(
                  spacing: 12,
                  runSpacing: 8,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    StatusBadge(record.status),
                    Text(
                      '${record.severity.toLowerCase()} priority',
                      style: Theme.of(
                        context,
                      ).textTheme.bodyMedium?.copyWith(color: c.muted),
                    ),
                  ],
                ),
                const SizedBox(height: 22),
                Text(
                  record.title,
                  style: Theme.of(context).textTheme.headlineMedium,
                ),
                const SizedBox(height: 14),
                Text(
                  record.summary,
                  style: Theme.of(
                    context,
                  ).textTheme.bodyLarge?.copyWith(color: c.muted),
                ),
                const SizedBox(height: 14),
                Text(
                  'Version ${record.version} · Updated ${formatDate(record.updatedAt)}',
                  style: Theme.of(
                    context,
                  ).textTheme.bodySmall?.copyWith(color: c.muted),
                ),
                const SizedBox(height: 26),
                if (!record.supported)
                  const Notice(
                    'This case uses an unsupported status or type. Review actions are unavailable until the app supports it.',
                  ),
                if (record.kind == 'SOURCE_UNAVAILABLE')
                  const Notice(
                    'Source data is incomplete. An unavailable source does not establish a missing settlement.',
                  ),
                if (record.status == 'EVIDENCE_UPDATED')
                  const Notice(
                    'New evidence has arrived. Review the latest records before making a decision.',
                  ),
                EvidenceTrail(state.value!.evidence),
                const SectionTitle('Source records'),
                if (state.value!.evidence.isEmpty)
                  const AppCard(
                    child: Text(
                      'No evidence records are available for this case.',
                    ),
                  ),
                for (final evidence in state.value!.evidence)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 10),
                    child: AppCard(
                      padding: EdgeInsets.zero,
                      child: ListTile(
                        contentPadding: const EdgeInsets.symmetric(
                          horizontal: 18,
                          vertical: 8,
                        ),
                        leading: Icon(switch (evidence.recordType) {
                          'PAYMENT' => LucideIcons.arrowUpRight,
                          'SETTLEMENT' => LucideIcons.arrowDownLeft,
                          'ORDER' => LucideIcons.fileText,
                          _ => LucideIcons.radio,
                        }, size: 22),
                        title: Text(
                          evidence.label,
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                        subtitle: Text(
                          evidence.amount == null
                              ? evidence.reference
                              : formatMoney(evidence.amount!),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                        trailing: const Icon(
                          LucideIcons.chevronRight,
                          size: 18,
                        ),
                        onTap: () => context.push(
                          '/evidence/${record.id}/${evidence.id}',
                        ),
                      ),
                    ),
                  ),
                const SectionTitle('Investigation'),
                AppCard(
                  padding: EdgeInsets.zero,
                  child: ListTile(
                    contentPadding: const EdgeInsets.all(18),
                    leading: const Icon(LucideIcons.notebookText),
                    title: const Text('Read the analysis'),
                    subtitle: const Text(
                      'Explanations, sources and limitations',
                    ),
                    trailing: const Icon(LucideIcons.arrowUpRight, size: 18),
                    onTap: () => context.push('/analysis/${record.id}'),
                  ),
                ),
                SectionTitle(
                  'Case history',
                  trailing: Text(
                    '${state.value!.events.length} events',
                    style: Theme.of(
                      context,
                    ).textTheme.bodySmall?.copyWith(color: c.muted),
                  ),
                ),
                if (state.value!.events.isEmpty)
                  const Text('No history is available yet.'),
                for (final event in state.value!.events)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 24),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Padding(
                          padding: const EdgeInsets.only(top: 5),
                          child: Icon(
                            event.type == 'RESOLVED'
                                ? LucideIcons.circleCheck
                                : LucideIcons.history,
                            size: 18,
                            color: c.muted,
                          ),
                        ),
                        const SizedBox(width: 14),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                event.message,
                                style: Theme.of(context).textTheme.bodyMedium,
                              ),
                              const SizedBox(height: 6),
                              Text(
                                '${formatDate(event.occurredAt)} · ${event.actor} · v${event.version}',
                                style: Theme.of(
                                  context,
                                ).textTheme.bodySmall?.copyWith(color: c.muted),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                if (membership?.canAssign == true &&
                    record.assignedTo !=
                        widget.dependencies.sessions.session?.user.id)
                  AppButton(
                    label: 'Assign to me',
                    secondary: true,
                    loading: state.loading,
                    onPressed: _cubit.assign,
                  ),
                const SizedBox(height: 18),
                Text(
                  '${widget.dependencies.config.environmentLabel} · Records describe synthetic funds.',
                  style: Theme.of(
                    context,
                  ).textTheme.bodySmall?.copyWith(color: c.muted),
                ),
              ],
            ],
          ),
        ),
      );
    },
  );
}
