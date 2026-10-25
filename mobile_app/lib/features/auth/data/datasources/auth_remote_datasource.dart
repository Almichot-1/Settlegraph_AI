import '../../../../core/network/api_config.dart';
import '../models/user_model.dart';

class AuthRemoteDatasource {
  const AuthRemoteDatasource(this.api);
  final ApiClient api;
  Future<AuthSession> signIn(String name, String code) async =>
      AuthSession.fromJson(
        await api.post(
          '/v1/demo/sessions',
          {'display_name': name.trim()},
          headers: {'X-Demo-Access-Code': code},
        ),
      );
  Future<UserModel> me() async => UserModel.fromJson(await api.get('/v1/me'));
}
