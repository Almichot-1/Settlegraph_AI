import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';
import '../../core/widgets/surfaces.dart';
import '../injection_container.dart';
import '../theme/app_colors.dart';

class MainAppShell extends StatelessWidget {
  const MainAppShell({
    super.key,
    required this.dependencies,
    required this.child,
    required this.location,
  });
  final AppDependencies dependencies;
  final Widget child;
  final String location;
  @override
  Widget build(BuildContext context) {
    final c = AppColors.of(context);
    final index = location.startsWith('/overview')
        ? 1
        : location.startsWith('/account')
        ? 2
        : 0;
    return Scaffold(
      appBar: AppBar(
        title: const BrandLogo(compact: true),
        actions: [
          Padding(
            padding: const EdgeInsets.only(right: 8),
            child: IconButton(
              tooltip: 'Your account',
              onPressed: () => context.go('/account'),
              icon: const Icon(LucideIcons.userRound, size: 20),
            ),
          ),
        ],
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(26),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(24, 0, 24, 10),
            child: Row(
              children: [
                Icon(LucideIcons.flaskConical, size: 12, color: c.muted),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    dependencies.config.environmentLabel,
                    style: Theme.of(
                      context,
                    ).textTheme.bodySmall?.copyWith(color: c.muted),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
      body: SafeArea(bottom: false, child: child),
      bottomNavigationBar: DecoratedBox(
        decoration: BoxDecoration(
          border: Border(top: BorderSide(color: c.line)),
        ),
        child: NavigationBar(
          selectedIndex: index,
          onDestinationSelected: (i) =>
              context.go(['/cases', '/overview', '/account'][i]),
          destinations: const [
            NavigationDestination(
              icon: Icon(LucideIcons.inbox, size: 22),
              label: 'Cases',
            ),
            NavigationDestination(
              icon: Icon(LucideIcons.chartNoAxesCombined, size: 22),
              label: 'Overview',
            ),
            NavigationDestination(
              icon: Icon(LucideIcons.settings2, size: 22),
              label: 'Account',
            ),
          ],
        ),
      ),
    );
  }
}
