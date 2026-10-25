import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../core/widgets/loading_view.dart';
import '../../features/auth/presentation/pages/login_page.dart';
import '../../features/cases/presentation/pages/cases_page.dart';
import '../../features/cases/presentation/pages/case_detail_page.dart';
import '../../features/cases/presentation/pages/decision_page.dart';
import '../../features/cases/presentation/pages/evidence_page.dart';
import '../../features/investigations/presentation/pages/investigations_page.dart';
import '../../features/overview/presentation/pages/overview_page.dart';
import '../../features/account/presentation/pages/account_page.dart';
import '../injection_container.dart';
import '../widgets/main_app_shell.dart';
import 'route_names.dart';

GoRouter createRouter(AppDependencies d) => GoRouter(
  initialLocation: RouteNames.boot,
  refreshListenable: d.sessions,
  redirect: (context, state) {
    if (!d.sessions.initialized) {
      return state.matchedLocation == RouteNames.boot ? null : RouteNames.boot;
    }
    if (!d.sessions.authenticated) {
      return state.matchedLocation == RouteNames.login
          ? null
          : RouteNames.login;
    }
    if ([RouteNames.login, RouteNames.boot].contains(state.matchedLocation)) {
      return RouteNames.cases;
    }
    return null;
  },
  errorBuilder: (context, state) => Scaffold(
    appBar: AppBar(title: const Text('Page unavailable')),
    body: Center(
      child: TextButton(
        onPressed: () => context.go(RouteNames.cases),
        child: const Text('Back to workspace'),
      ),
    ),
  ),
  routes: [
    GoRoute(
      path: RouteNames.boot,
      builder: (_, _) => const Scaffold(body: SafeArea(child: LoadingView())),
    ),
    GoRoute(
      path: RouteNames.login,
      builder: (_, _) => LoginPage(dependencies: d),
    ),
    ShellRoute(
      builder: (context, state, child) =>
          MainAppShell(dependencies: d, location: state.uri.path, child: child),
      routes: [
        GoRoute(
          path: RouteNames.cases,
          builder: (_, _) => CasesPage(
            key: ValueKey('cases-${d.sessions.generation}'),
            dependencies: d,
          ),
        ),
        GoRoute(
          path: RouteNames.overview,
          builder: (_, _) => OverviewPage(
            key: ValueKey('overview-${d.sessions.generation}'),
            dependencies: d,
          ),
        ),
        GoRoute(
          path: RouteNames.account,
          builder: (_, _) => AccountPage(
            key: ValueKey('account-${d.sessions.generation}'),
            dependencies: d,
          ),
        ),
      ],
    ),
    GoRoute(
      path: '/cases/:id',
      builder: (_, s) => CaseDetailPage(
        key: ValueKey('${d.sessions.generation}-${s.pathParameters['id']}'),
        dependencies: d,
        id: s.pathParameters['id']!,
      ),
    ),
    GoRoute(
      path: '/review/:id',
      builder: (_, s) =>
          DecisionPage(dependencies: d, id: s.pathParameters['id']!),
    ),
    GoRoute(
      path: '/evidence/:caseId/:id',
      builder: (_, s) => EvidencePage(
        dependencies: d,
        caseId: s.pathParameters['caseId']!,
        id: s.pathParameters['id']!,
      ),
    ),
    GoRoute(
      path: '/analysis/:id',
      builder: (_, s) =>
          InvestigationsPage(dependencies: d, caseId: s.pathParameters['id']!),
    ),
  ],
);
