import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';
import '../../../../app/injection_container.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../core/network/api_config.dart';
import '../../../../core/testing/fixture_api_client.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/surfaces.dart';
import '../../../auth/presentation/bloc/auth_event.dart';

class AccountPage extends StatelessWidget {
  const AccountPage({super.key, required this.dependencies});
  final AppDependencies dependencies;
  @override
  Widget build(BuildContext context) {
    final d = dependencies;
    final s = d.sessions.session!;
    final c = AppColors.of(context);
    return PageContent(
      children: [
        const PageHeading(
          'Your workspace',
          'A few essentials, all in one place.',
        ),
        AppCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              CircleAvatar(
                radius: 26,
                backgroundColor: c.soft,
                foregroundColor: c.ink,
                child: const Icon(LucideIcons.userRound, size: 24),
              ),
              const SizedBox(height: 18),
              Text(
                s.user.displayName,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: 6),
              Text(
                d.sessions.membership?.role.toLowerCase() ?? 'Unknown role',
                style: Theme.of(
                  context,
                ).textTheme.bodyMedium?.copyWith(color: c.muted),
              ),
              const SizedBox(height: 20),
              const Divider(),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                initialValue: s.organizationId,
                isExpanded: true,
                decoration: const InputDecoration(labelText: 'Organization'),
                items: [
                  for (final m in s.user.memberships)
                    DropdownMenuItem(
                      value: m.id,
                      child: Text(m.name, overflow: TextOverflow.ellipsis),
                    ),
                ],
                onChanged: (id) {
                  if (id != null && id != s.organizationId) {
                    context.go('/cases');
                    d.auth.add(AuthOrganizationChanged(id));
                  }
                },
              ),
            ],
          ),
        ),
        const SectionTitle('Appearance'),
        ListenableBuilder(
          listenable: d.appearance,
          builder: (context, _) => DropdownButtonFormField<ThemeMode>(
            initialValue: d.appearance.mode,
            decoration: const InputDecoration(labelText: 'Theme'),
            items: const [
              DropdownMenuItem(
                value: ThemeMode.system,
                child: Text('Follow device'),
              ),
              DropdownMenuItem(value: ThemeMode.light, child: Text('Light')),
              DropdownMenuItem(value: ThemeMode.dark, child: Text('Dark')),
            ],
            onChanged: (mode) {
              if (mode != null) d.appearance.setMode(mode);
            },
          ),
        ),
        const SectionTitle('About this workspace'),
        AppCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              KeyValue('Environment', d.config.environmentLabel),
              KeyValue('App version', d.version),
              const SizedBox(height: 10),
              const Text(
                'All institutions and funds in this demo are fictional. Review decisions do not change payment facts.',
              ),
            ],
          ),
        ),
        if (d.config.isFixture) ...[
          const SectionTitle('Sample walkthrough'),
          const Text(
            'Explore how a case changes when a later settlement observation arrives.',
          ),
          const SizedBox(height: 16),
          AppButton(
            label: 'Show settlement arrival',
            secondary: true,
            icon: LucideIcons.arrowDownLeft,
            onPressed: () {
              (d.get<ApiClient>() as FixtureApiClient).receiveSettlement();
              d.sessions.update(d.sessions.session);
              context.go('/cases');
            },
          ),
          const SizedBox(height: 12),
          AppButton(
            label: 'Reset sample story',
            secondary: true,
            onPressed: () {
              (d.get<ApiClient>() as FixtureApiClient).reset();
              d.sessions.update(d.sessions.session);
              context.go('/cases');
            },
          ),
        ],
        const SizedBox(height: 32),
        AppButton(
          label: 'Sign out',
          secondary: true,
          onPressed: () => d.auth.add(const AuthSignOut()),
        ),
      ],
    );
  }
}
