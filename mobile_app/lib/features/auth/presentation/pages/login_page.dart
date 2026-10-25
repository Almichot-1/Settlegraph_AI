import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:lucide_icons_flutter/lucide_icons.dart';
import '../../../../app/injection_container.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../../../../core/widgets/surfaces.dart';
import '../bloc/auth_bloc.dart';
import '../bloc/auth_event.dart';
import '../bloc/auth_state.dart';

class LoginPage extends StatefulWidget {
  const LoginPage({super.key, required this.dependencies});
  final AppDependencies dependencies;
  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final _form = GlobalKey<FormState>();
  final _name = TextEditingController();
  final _code = TextEditingController();
  @override
  void dispose() {
    _name.dispose();
    _code.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final d = widget.dependencies;
    final c = AppColors.of(context);
    return Scaffold(
      body: SafeArea(
        child: BlocBuilder<AuthBloc, AuthState>(
          bloc: d.auth,
          builder: (context, state) => PageContent(
            children: [
              const SizedBox(height: 28),
              const BrandLogo(),
              const SizedBox(height: 60),
              Text(
                'A clearer view.\nA confident decision.',
                style: Theme.of(
                  context,
                ).textTheme.headlineLarge?.copyWith(fontSize: 38),
              ),
              const SizedBox(height: 20),
              Text(
                'Keep payments in view. Follow the evidence. Bring every case to a clear next step.',
                style: Theme.of(
                  context,
                ).textTheme.bodyLarge?.copyWith(color: c.muted),
              ),
              const SizedBox(height: 32),
              AppCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Icon(LucideIcons.route, size: 20, color: c.brand),
                        const SizedBox(width: 10),
                        Expanded(
                          child: Text(
                            'Built around the evidence',
                            style: Theme.of(context).textTheme.titleMedium,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 12),
                    Text(
                      'Payment observed  →  Settlement observed',
                      style: Theme.of(
                        context,
                      ).textTheme.bodyMedium?.copyWith(color: c.muted),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 32),
              if (state.message != null) Notice(state.message!, negative: true),
              Form(
                key: _form,
                child: Column(
                  children: [
                    AppTextField(
                      controller: _name,
                      label: 'Your name',
                      hint: 'How should we address you?',
                      enabled: !state.busy,
                      autofillHints: const [AutofillHints.name],
                      validator: (v) => (v?.trim().isEmpty ?? true)
                          ? 'Enter your name.'
                          : v!.trim().length > 80
                          ? 'Use 80 characters or fewer.'
                          : null,
                    ),
                    if (!d.config.isFixture) ...[
                      const SizedBox(height: 18),
                      AppTextField(
                        controller: _code,
                        label: 'Demo access code',
                        obscure: true,
                        enabled: !state.busy,
                        validator: (v) => (v?.isEmpty ?? true)
                            ? 'Enter the access code you were given.'
                            : null,
                      ),
                    ],
                    const SizedBox(height: 22),
                    AppButton(
                      label: d.config.isFixture
                          ? 'Explore sample workspace'
                          : 'Continue to workspace',
                      icon: LucideIcons.arrowRight,
                      loading: state.busy,
                      onPressed: () {
                        if (_form.currentState!.validate()) {
                          d.auth.add(AuthSignIn(_name.text, _code.text));
                        }
                      },
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 24),
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(LucideIcons.shieldCheck, size: 16, color: c.muted),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      d.config.isFixture
                          ? 'Sample data only. Explore a synthetic review without a connection.'
                          : 'Simulated funds and fictional institutions. Your access code opens the workspace with its assigned demo role.',
                      style: Theme.of(
                        context,
                      ).textTheme.bodySmall?.copyWith(color: c.muted),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
