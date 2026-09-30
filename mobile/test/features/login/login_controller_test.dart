import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/auth/login_service.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/login/login_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
  });

  ProviderContainer makeContainer() {
    final container = ProviderContainer(
      overrides: [
        sessionStoreProvider.overrideWithValue(store),
        loginServiceFactoryProvider.overrideWithValue((baseUrl) {
          final client = ApiClient.create(
            baseUrl: baseUrl,
            sessionStore: store,
          );
          client.dio.httpClientAdapter = adapter;
          return LoginService(client: client, sessionStore: store);
        }),
      ],
    );
    addTearDown(container.dispose);
    return container;
  }

  test('初始态 Idle', () {
    expect(makeContainer().read(loginControllerProvider), isA<LoginIdle>());
  });

  test('凭据成功 → LoginSuccess + 会话写入 + session 探测翻转为已登录', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'token': 'jwt-ok',
      'user': {'username': 'admin'},
      'lastGameId': 'demo',
      'lastEnv': 'prod',
    });
    final container = makeContainer();
    await container.read(sessionFutureProvider.future);

    await container
        .read(loginControllerProvider.notifier)
        .submit(
          serverUrl: 'http://gm.test',
          username: 'admin',
          password: 'secret',
        );

    expect(container.read(loginControllerProvider), isA<LoginSuccess>());
    expect((await store.load())?.token, 'jwt-ok');
    expect(
      (await container.read(sessionFutureProvider.future))?.gameId,
      'demo',
    );
  });

  test('401 mfa_required → NeedTotp（第二跳补动态码）', () async {
    adapter.handler = (options, _) =>
        jsonResponse(401, {'error': 'mfa_required', 'message': '需要动态验证码'});
    final container = makeContainer();

    await container
        .read(loginControllerProvider.notifier)
        .submit(
          serverUrl: 'http://gm.test',
          username: 'admin',
          password: 'secret',
        );

    final state = container.read(loginControllerProvider);
    expect(state, isA<LoginNeedTotp>());
    expect((state as LoginNeedTotp).message, '需要动态验证码');
    expect(await store.load(), isNull);
  });

  test('mustChangePassword → Blocked（引导回 Web）且不写会话', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'token': 'jwt-flag',
      'user': {},
      'mustChangePassword': true,
    });
    final container = makeContainer();

    await container
        .read(loginControllerProvider.notifier)
        .submit(serverUrl: 'http://gm.test', username: 'op', password: 'x');

    final state = container.read(loginControllerProvider);
    expect(state, isA<LoginBlocked>());
    expect((state as LoginBlocked).message, contains('改密'));
    expect(await store.load(), isNull);
  });

  test('ApiError → Failure 透传服务端 message', () async {
    adapter.handler = (options, _) =>
        jsonResponse(401, {'error': 'unauthorized', 'message': '用户名或密码错误'});
    final container = makeContainer();

    await container
        .read(loginControllerProvider.notifier)
        .submit(
          serverUrl: 'http://gm.test',
          username: 'admin',
          password: 'wrong',
        );

    final state = container.read(loginControllerProvider);
    expect(state, isA<LoginFailure>());
    expect((state as LoginFailure).message, '用户名或密码错误');
  });
}
