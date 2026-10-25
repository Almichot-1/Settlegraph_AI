import 'package:flutter/material.dart';

class AppTextField extends StatelessWidget {
  const AppTextField({
    super.key,
    required this.controller,
    required this.label,
    this.hint,
    this.validator,
    this.obscure = false,
    this.maxLines = 1,
    this.enabled = true,
    this.onChanged,
    this.maxLength,
    this.autofillHints,
  });
  final TextEditingController controller;
  final String label;
  final String? hint;
  final String? Function(String?)? validator;
  final bool obscure, enabled;
  final int maxLines;
  final int? maxLength;
  final ValueChanged<String>? onChanged;
  final Iterable<String>? autofillHints;
  @override
  Widget build(BuildContext context) => TextFormField(
    controller: controller,
    validator: validator,
    obscureText: obscure,
    enabled: enabled,
    maxLines: maxLines,
    maxLength: maxLength,
    onChanged: onChanged,
    autofillHints: autofillHints,
    textCapitalization: obscure
        ? TextCapitalization.none
        : TextCapitalization.sentences,
    decoration: InputDecoration(
      labelText: label,
      hintText: hint,
      alignLabelWithHint: maxLines > 1,
    ),
  );
}
