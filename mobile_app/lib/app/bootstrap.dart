import 'package:flutter/material.dart';
import 'app.dart';
import 'injection_container.dart';

Future<void> bootstrap() async {
  WidgetsFlutterBinding.ensureInitialized();
  final dependencies = await AppDependencies.create();
  runApp(SettleGraphApp(dependencies: dependencies));
}
