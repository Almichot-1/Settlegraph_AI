import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

class AppearanceController extends ChangeNotifier {
  AppearanceController(this.preferences) {
    final saved = preferences?.getString('appearance');
    mode = ThemeMode.values.firstWhere(
      (m) => m.name == saved,
      orElse: () => ThemeMode.system,
    );
  }
  final SharedPreferences? preferences;
  late ThemeMode mode;
  Future<void> setMode(ThemeMode value) async {
    mode = value;
    notifyListeners();
    await preferences?.setString('appearance', value.name);
  }
}
