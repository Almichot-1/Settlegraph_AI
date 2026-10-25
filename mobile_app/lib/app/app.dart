import 'dart:async';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../core/security/screen_shield_service.dart';
import '../features/auth/presentation/bloc/auth_event.dart';
import 'injection_container.dart';
import 'router/app_router.dart';
import 'theme/app_theme.dart';

class SettleGraphApp extends StatefulWidget {
  const SettleGraphApp({super.key, required this.dependencies});
  final AppDependencies dependencies;
  @override
  State<SettleGraphApp> createState() => _SettleGraphAppState();
}

class _SettleGraphAppState extends State<SettleGraphApp> {
  late final GoRouter _router;
  @override
  void initState() {
    super.initState();
    _router = createRouter(widget.dependencies);
    widget.dependencies.auth.add(const AuthStarted());
  }

  @override
  void dispose() {
    _router.dispose();
    unawaited(widget.dependencies.dispose());
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: widget.dependencies.appearance,
    builder: (context, _) => MaterialApp.router(
      title: 'SettleGraph',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.light,
      darkTheme: AppTheme.dark,
      themeMode: widget.dependencies.appearance.mode,
      routerConfig: _router,
      builder: (context, child) =>
          ScreenShield(child: child ?? const SizedBox.shrink()),
    ),
  );
}
