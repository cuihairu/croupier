/// 登录页状态机（设计稿 §3.2 双步）。
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import '../../core/auth/login_service.dart';

sealed class LoginUiState {
  const LoginUiState();
}

class LoginIdle extends LoginUiState {
  const LoginIdle();
}

class LoginSubmitting extends LoginUiState {
  const LoginSubmitting();
}

/// 第一步被 401 mfa_required 拒绝：展示动态码输入框
/// （凭据保留在表单内存 state，不落盘）。
class LoginNeedTotp extends LoginUiState {
  const LoginNeedTotp({required this.message});
  final String message;
}

/// 平台策略阻止（改密 / TOTP 绑定未完成）：弹窗引导回 Web。
class LoginBlocked extends LoginUiState {
  const LoginBlocked({required this.message});
  final String message;
}

class LoginFailure extends LoginUiState {
  const LoginFailure({required this.message});
  final String message;
}

class LoginSuccess extends LoginUiState {
  const LoginSuccess();
}

final loginControllerProvider = NotifierProvider<LoginController, LoginUiState>(
  LoginController.new,
);

class LoginController extends Notifier<LoginUiState> {
  @override
  LoginUiState build() => const LoginIdle();

  Future<void> submit({
    required String serverUrl,
    required String username,
    required String password,
    String? totpCode,
  }) async {
    state = const LoginSubmitting();
    final service = ref.read(loginServiceFactoryProvider)(serverUrl);
    try {
      await service.login(
        username: username,
        password: password,
        totpCode: totpCode,
        serverUrl: serverUrl,
      );
      // 会话已写入：invalidate 让 App 根重建切到已登录壳。
      ref.invalidate(sessionFutureProvider);
      state = const LoginSuccess();
    } on MfaRequiredException catch (e) {
      state = LoginNeedTotp(message: e.message);
    } on LoginBlockedException catch (e) {
      state = LoginBlocked(message: e.message);
    } on ApiError catch (e) {
      state = LoginFailure(message: e.message);
    }
  }
}
