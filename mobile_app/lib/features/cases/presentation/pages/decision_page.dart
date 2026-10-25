import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';
import '../../../../app/injection_container.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../core/models/load_state.dart';
import '../../../../core/utils/validators.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../../../../core/widgets/error_view.dart';
import '../../../../core/widgets/loading_view.dart';
import '../../../../core/widgets/surfaces.dart';
import '../../data/models/case_model.dart';
import '../../data/repositories/cases_repository.dart';
import '../bloc/case_detail_cubit.dart';
import '../bloc/decision_cubit.dart';
import '../widgets/evidence_trail.dart';
import '../widgets/status_badge.dart';

class DecisionPage extends StatelessWidget {
  const DecisionPage({super.key, required this.dependencies, required this.id});
  final AppDependencies dependencies;
  final String id;
  @override
  Widget build(BuildContext context) => BlocProvider(
    create: (_) =>
        CaseDetailCubit(dependencies.get<CasesRepository>(), id)..refresh(),
    child: Scaffold(
      appBar: AppBar(title: const Text('Review case')),
      body: SafeArea(
        child: BlocBuilder<CaseDetailCubit, LoadState<CaseBundle>>(
          builder: (context, state) {
            if (state.failure != null) {
              return PageContent(
                children: [
                  ErrorView(
                    failure: state.failure!,
                    onRetry: context.read<CaseDetailCubit>().refresh,
                  ),
                ],
              );
            }
            if (state.value == null) {
              return const LoadingView(label: 'Preparing your review…');
            }
            return BlocProvider(
              create: (_) => DecisionCubit(
                dependencies.get<CasesRepository>(),
                state.value!.caseRecord,
              ),
              child: _DecisionForm(
                dependencies: dependencies,
                initial: state.value!.caseRecord,
              ),
            );
          },
        ),
      ),
    ),
  );
}

class _DecisionForm extends StatefulWidget {
  const _DecisionForm({required this.dependencies, required this.initial});
  final AppDependencies dependencies;
  final CaseModel initial;
  @override
  State<_DecisionForm> createState() => _DecisionFormState();
}

class _DecisionFormState extends State<_DecisionForm> {
  final _form = GlobalKey<FormState>();
  final _reason = TextEditingController();
  late String _decision;
  @override
  void initState() {
    super.initState();
    final pending = context.read<DecisionCubit>().pending;
    _decision =
        pending?.decision ?? (widget.initial.resolved ? 'REOPEN' : 'RESOLVE');
    _reason.text = pending?.reason ?? '';
  }

  @override
  void dispose() {
    _reason.dispose();
    super.dispose();
  }

  @override
  Widget build(
    BuildContext context,
  ) => BlocBuilder<DecisionCubit, DecisionState>(
    builder: (context, state) {
      final cubit = context.read<DecisionCubit>();
      final c = AppColors.of(context);
      final canReview =
          widget.dependencies.sessions.membership?.canReview == true &&
          state.record.supported;
      if (state.completed) {
        return PageContent(
          children: [
            const SizedBox(height: 48),
            Icon(LucideIcons.circleCheck, size: 52, color: c.positive),
            const PageHeading(
              'Decision recorded',
              'Your review is part of the case history.',
            ),
            StatusBadge(state.record.status),
            const SizedBox(height: 24),
            const Text(
              'Payment and settlement observations remain unchanged. Your decision updates the case and its audit history.',
            ),
            const SizedBox(height: 32),
            AppButton(label: 'Back to case', onPressed: () => context.pop()),
          ],
        );
      }
      return PageContent(
        children: [
          const SizedBox(height: 16),
          StatusBadge(state.record.status),
          const SizedBox(height: 20),
          Text(
            'Make the next\nstep clear.',
            style: Theme.of(context).textTheme.headlineLarge,
          ),
          const SizedBox(height: 14),
          Text(
            widget.initial.title,
            style: Theme.of(
              context,
            ).textTheme.bodyLarge?.copyWith(color: c.muted),
          ),
          const SizedBox(height: 24),
          if (!canReview)
            const Notice('Your role does not allow a decision on this case.'),
          if (state.failure != null &&
              !state.failure!.isConflict &&
              !state.uncertain)
            Notice(state.failure!.message, negative: true),
          if (state.uncertain)
            const Notice(
              'Submission not confirmed. Retry sends the same decision and retry key. Keep this action unchanged until its result is confirmed.',
            ),
          if (state.failure?.isConflict == true) ...[
            const Notice(
              'This case changed while you were reviewing it. Your reason is preserved. Inspect the updated evidence before submitting again.',
            ),
            if (state.conflict != null) ...[
              Text(
                'Latest case · version ${state.conflict!.caseRecord.version}',
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 12),
              EvidenceTrail(state.conflict!.evidence),
              const SizedBox(height: 12),
              for (final evidence in state.conflict!.evidence)
                TextButton(
                  onPressed: () => context.push(
                    '/evidence/${state.record.id}/${evidence.id}',
                  ),
                  child: Text('Review ${evidence.label.toLowerCase()} record'),
                ),
              AppButton(
                label: 'I have reviewed the updated evidence',
                secondary: true,
                onPressed: () {
                  final latest = state.conflict!.caseRecord;
                  setState(() {
                    _decision = latest.resolved ? 'REOPEN' : 'RESOLVE';
                  });
                  cubit.acknowledgeConflict();
                },
              ),
            ] else
              AppButton(
                label: 'Load updated evidence',
                secondary: true,
                loading: state.busy,
                onPressed: cubit.refreshConflict,
              ),
            const SizedBox(height: 24),
          ],
          Form(
            key: _form,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Your decision',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 12),
                RadioGroup<String>(
                  groupValue: _decision,
                  onChanged: (value) {
                    if (!state.busy &&
                        !state.uncertain &&
                        canReview &&
                        value != null) {
                      setState(() => _decision = value);
                    }
                  },
                  child: Column(
                    children: [
                      for (final option
                          in (state.record.resolved
                              ? [
                                  (
                                    'REOPEN',
                                    'Reopen case',
                                    'More review is needed.',
                                  ),
                                ]
                              : [
                                  (
                                    'RESOLVE',
                                    'Resolve case',
                                    'Record that your review is complete.',
                                  ),
                                  (
                                    'NEEDS_MORE_EVIDENCE',
                                    'Needs more evidence',
                                    'Keep this case open for further review.',
                                  ),
                                ]))
                        Padding(
                          padding: const EdgeInsets.only(bottom: 12),
                          child: AppCard(
                            padding: EdgeInsets.zero,
                            child: RadioListTile<String>(
                              value: option.$1,
                              enabled:
                                  !state.busy && !state.uncertain && canReview,
                              title: Text(option.$2),
                              subtitle: Text(option.$3),
                              contentPadding: const EdgeInsets.all(12),
                            ),
                          ),
                        ),
                    ],
                  ),
                ),
                const SizedBox(height: 12),
                AppTextField(
                  controller: _reason,
                  label: 'Reason for your decision',
                  hint: 'Describe the evidence you reviewed and why.',
                  maxLines: 5,
                  maxLength: 2000,
                  enabled: !state.busy && !state.uncertain && canReview,
                  validator: validateReason,
                ),
                const SizedBox(height: 18),
                Text(
                  'Recorded against version ${state.record.version}. Resolving a case does not confirm settlement or move funds.',
                  style: Theme.of(
                    context,
                  ).textTheme.bodySmall?.copyWith(color: c.muted),
                ),
                const SizedBox(height: 24),
                AppButton(
                  label: state.uncertain
                      ? 'Retry unconfirmed submission'
                      : 'Record decision',
                  loading: state.busy,
                  icon: LucideIcons.check,
                  onPressed:
                      !canReview ||
                          state.conflict != null ||
                          state.failure?.isConflict == true
                      ? null
                      : () {
                          if (_form.currentState!.validate()) {
                            cubit.submit(
                              _decision,
                              _reason.text,
                              canReview: canReview,
                            );
                          }
                        },
                ),
                const SizedBox(height: 20),
              ],
            ),
          ),
        ],
      );
    },
  );
}
