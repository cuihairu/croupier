/// 登录服务（设计稿 §3.2）：JWT + TOTP 双步。
///
/// 1. `POST /api/v1/auth/login` body `{username, password}`；
/// 2. 401 且 `error == 'mfa_required'` → 抛 [MfaRequiredException]，
///    UI 补动态码后携 `{username, password, totpCode}` 重试；
/// 3. 成功 `{token, user, lastGameId, lastEnv, mustChangePassword?,
///    mfaSetupRequired?}`：token 入 SessionStore，lastGameId/lastEnv
///    预选 scope；mustChangePassword / mfaSetupRequired 由 UI 引导回 Web
///    （移动端不做改密/绑定）。
library;

import '../api/api_client.dart';
import '../api/api_error.dart';
import '../storage/session_store.dart';

class MfaRequiredException implements Exception {
  const MfaRequiredException(this.message);
  final String message;

  @override
  String toString() => message;
}

class LoginResult {
  const LoginResult({
    required this.token,
    this.user = const {},
    this.lastGameId = '',
    this.lastEnv = '',
    this.mustChangePassword = false,
    this.mfaSetupRequired = false,
  });

  final String token;
  final Map<String, Object?> user;
  final String lastGameId;
  final String lastEnv;
  final bool mustChangePassword;
  final bool mfaSetupRequired;
}

class LoginService {
  LoginService({required this.client, required this.sessionStore});

  static const loginPath = '/api/v1/auth/login';

  final ApiClient client;
  final SessionStore sessionStore;

  /// 双步登录。成功即写会话（token + 预选 scope）并返回结果；
  /// 需要 TOTP 时抛 [MfaRequiredException]，其余失败抛 [ApiError]。
  Future<LoginResult> login({
    required String username,
    required String password,
    String? totpCode,
    String? serverUrl,
  }) async {
    final body = <String, Object?>{
      'username': username,
      'password': password,
      if (totpCode != null && totpCode.isNotEmpty) 'totpCode': totpCode,
    };

    final Map<String, Object?> data;
    try {
      data = await client.post<Map<String, Object?>>(loginPath, body: body);
    } on ApiError catch (e) {
      if (e.status == 401 && e.code == 'mfa_required') {
        throw const MfaRequiredException('请输入动态验证码');
      }
      rethrow;
    }

    final token = (data['token'] as String?) ?? '';
    if (token.isEmpty) {
      throw const ApiError(
        status: 500,
        code: 'invalid_login_response',
        message: '登录响应缺少 token',
      );
    }

    final result = LoginResult(
      token: token,
      user: data['user'] is Map<String, Object?>
          ? data['user'] as Map<String, Object?>
          : const {},
      lastGameId: (data['lastGameId'] as String?) ?? '',
      lastEnv: (data['lastEnv'] as String?) ?? '',
      mustChangePassword: data['mustChangePassword'] == true,
      mfaSetupRequired: data['mfaSetupRequired'] == true,
    );

    await sessionStore.save(
      SessionData(
        token: result.token,
        user: result.user,
        gameId: result.lastGameId,
        env: result.lastEnv,
        serverUrl: serverUrl ?? '',
      ),
    );
    return result;
  }

  /// 登出：整块清除会话。
  Future<void> logout() => sessionStore.clear();
}
