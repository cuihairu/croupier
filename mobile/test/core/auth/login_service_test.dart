import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/api/api_error.dart';
import 'package:croupier_mobile/core/auth/login_service.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late LoginService service;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    final client = ApiClient.create(
      baseUrl: 'http://gm.test',
      sessionStore: store,
    );
    client.dio.httpClientAdapter = adapter;
    service = LoginService(client: client, sessionStore: store);
  });

  test('双步第一步：提交凭据，成功写会话并回填预选 scope', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'token': 'jwt-ok',
      'user': {'username': 'admin'},
      'lastGameId': 'demo',
      'lastEnv': 'prod',
    });

    final result = await service.login(username: 'admin', password: 'secret');

    expect(result.token, 'jwt-ok');
    expect(result.lastGameId, 'demo');
    expect(adapter.lastRequest?.path, LoginService.loginPath);
    expect(adapter.lastBody, contains('admin'));
    final session = await store.load();
    expect(session?.token, 'jwt-ok');
    expect(session?.gameId, 'demo');
    expect(session?.env, 'prod');
  });

  test('401 mfa_required → MfaRequiredException（UI 补动态码重试）', () async {
    adapter.handler = (options, _) =>
        jsonResponse(401, {'error': 'mfa_required', 'message': '需要动态验证码'});
    await expectLater(
      service.login(username: 'admin', password: 'secret'),
      throwsA(isA<MfaRequiredException>()),
    );
    expect(await store.load(), isNull);
  });

  test('第二跳：totpCode 随 body 提交', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'token': 'jwt-totp',
      'user': {},
      'lastGameId': '',
      'lastEnv': '',
    });
    await service.login(
      username: 'admin',
      password: 'secret',
      totpCode: '123456',
      serverUrl: 'http://gm.test',
    );
    expect(adapter.lastBody, contains('123456'));
    expect((await store.load())?.serverUrl, 'http://gm.test');
  });

  test('401 非 mfa_required → ApiError 原样透传', () async {
    adapter.handler = (options, _) =>
        jsonResponse(401, {'error': 'unauthorized', 'message': '用户名或密码错误'});
    await expectLater(
      service.login(username: 'admin', password: 'wrong'),
      throwsA(
        isA<ApiError>()
            .having((e) => e.code, 'code', 'unauthorized')
            .having((e) => e.message, 'message', '用户名或密码错误'),
      ),
    );
  });

  test('mustChangePassword / mfaSetupRequired 旗标透传（UI 引导回 Web）', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'token': 'jwt-flag',
      'user': {},
      'mustChangePassword': true,
      'mfaSetupRequired': true,
    });
    final result = await service.login(username: 'op', password: 'x');
    expect(result.mustChangePassword, isTrue);
    expect(result.mfaSetupRequired, isTrue);
  });

  test('200 但缺 token → invalid_login_response（不当成功处理）', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'user': {}});
    await expectLater(
      service.login(username: 'a', password: 'b'),
      throwsA(
        isA<ApiError>().having((e) => e.code, 'code', 'invalid_login_response'),
      ),
    );
  });

  test('logout 整块清会话', () async {
    await store.save(const SessionData(token: 'jwt-x'));
    await service.logout();
    expect(await store.load(), isNull);
  });
}
