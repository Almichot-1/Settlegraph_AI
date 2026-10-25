import 'package:flutter/material.dart';
import '../widgets/surfaces.dart';

/// Privacy cover for task-switcher snapshots; this is not screenshot prevention.
class ScreenShield extends StatefulWidget {
  const ScreenShield({super.key, required this.child});
  final Widget child;
  @override
  State<ScreenShield> createState() => _ScreenShieldState();
}

class _ScreenShieldState extends State<ScreenShield>
    with WidgetsBindingObserver {
  bool _covered = false;
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (mounted) setState(() => _covered = state != AppLifecycleState.resumed);
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => Stack(
    children: [
      widget.child,
      if (_covered)
        Positioned.fill(
          child: Material(
            color: Theme.of(context).scaffoldBackgroundColor,
            child: const Center(child: BrandLogo()),
          ),
        ),
    ],
  );
}
