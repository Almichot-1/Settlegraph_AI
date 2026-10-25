import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import '../../../../app/injection_container.dart';
import '../../../../core/models/load_state.dart';
import '../../../../core/utils/date_formatter.dart';
import '../../../../core/utils/money_formatter.dart';
import '../../../../core/widgets/error_view.dart';
import '../../../../core/widgets/loading_view.dart';
import '../../../../core/widgets/surfaces.dart';
import '../../data/repositories/cases_repository.dart';
import '../bloc/case_detail_cubit.dart';

class EvidencePage extends StatelessWidget {
  const EvidencePage({
    super.key,
    required this.dependencies,
    required this.caseId,
    required this.id,
  });
  final AppDependencies dependencies;
  final String caseId, id;
  @override
  Widget build(BuildContext context) => BlocProvider(
    create: (_) =>
        CaseDetailCubit(dependencies.get<CasesRepository>(), caseId)..refresh(),
    child: Scaffold(
      appBar: AppBar(title: const Text('Evidence record')),
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
              return const LoadingView(label: 'Finding source record…');
            }
            final records = state.value!.evidence.where((e) => e.id == id);
            if (records.isEmpty) {
              return const PageContent(
                children: [
                  PageHeading(
                    'Source unavailable',
                    'This citation is not present in the evidence available for this case. No substitute record has been used.',
                  ),
                ],
              );
            }
            final record = records.first;
            return PageContent(
              children: [
                PageHeading(
                  record.label,
                  'A source record linked to this case.',
                ),
                AppCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      KeyValue('Reference', record.reference),
                      if (record.amount != null)
                        KeyValue('Amount', formatMoney(record.amount!)),
                      KeyValue('Provider', record.providerId ?? 'Not supplied'),
                      KeyValue(
                        'Occurred at',
                        formatFullDate(record.occurredAt),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 24),
                const Notice(
                  'Synthetic source evidence. A record marked as observed is separate from a confirmed reconciliation match.',
                ),
                ExpansionTile(
                  title: const Text('Record details'),
                  childrenPadding: const EdgeInsets.all(16),
                  children: [
                    KeyValue('Evidence ID', record.id),
                    if (record.payloadHash != null)
                      KeyValue('Payload SHA-256', record.payloadHash!),
                  ],
                ),
              ],
            );
          },
        ),
      ),
    ),
  );
}
