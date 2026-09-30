import 'package:croupier_mobile/app/app.dart';
import 'package:flutter/material.dart';
import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/auth/login_service.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
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

  Future<void> pumpApp(WidgetTester tester) async {
    await tester.pumpWidget(
      ProviderScope(
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
        child: const CroupierApp(),
      ),
    );
    await tester.pumpAndSettle();
  }

  Future<void> fillCredentials(WidgetTester tester) async {
    await tester.enterText(
      find.byKey(const ValueKey('login-username')),
      'admin',
    );
    await tester.enterText(
      find.byKey(const ValueKey('login-password')),
      'secret',
    );
    await tester.tap(find.byKey(const ValueKey('login-submit')));
    await tester.pumpAndSettle();
  }

  testWidgets('首屏：凭据三件套，TOTP 框不出现', (WidgetTester tester) async {
    await pumpApp(tester);
    expect(find.byKey(const ValueKey('login-server')), findsOneWidget);
    expect(find.byKey(const ValueKey('login-username')), findsOneWidget);
    expect(find.byKey(const ValueKey('login-password')), findsOneWidget);
    expect(find.byKey(const ValueKey('login-totp')), findsNothing);
  });

  testWidgets('双步：第一步 401 mfa_required 后 TOTP 框出现', (
    WidgetTester tester,
  ) async {
    adapter.handler = (options, _) =>
        jsonResponse(401, {'error': 'mfa_required', 'message': '需要动态验证码'});
    await pumpApp(tester);

    await fillCredentials(tester);

    expect(find.byKey(const ValueKey('login-totp')), findsOneWidget);
    expect(find.text('动态验证码（TOTP）'), findsOneWidget);
  });

  testWidgets('双步第二跳：补 TOTP 后成功切已登录壳', (WidgetTester tester) async {
    var calls = 0;
    adapter.handler = (options, _) {
      calls++;
      if (calls == 1) {
        return jsonResponse(401, {
          'error': 'mfa_required',
          'message': '需要动态验证码',
        });
      }
      return jsonResponse(200, {
        'token': 'jwt-totp',
        'user': {'username': 'admin'},
        'lastGameId': 'demo',
        'lastEnv': 'prod',
      });
    };
    await pumpApp(tester);

    await fillCredentials(tester);
    await tester.enterText(find.byKey(const ValueKey('login-totp')), '123456');
    await tester.tap(find.byKey(const ValueKey('login-submit')));
    await tester.pumpAndSettle();

    // App 根据会话探测切换到已登录占位壳
    expect(find.byType(HomeStub), findsOneWidget);
    expect(find.text('demo / prod'), findsOneWidget);
    expect((await store.load())?.token, 'jwt-totp');
  });

  testWidgets('mustChangePassword：弹窗引导回 Web，不进 App', (
    WidgetTester tester,
  ) async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'token': 'jwt-flag',
      'user': {},
      'mustChangePassword': true,
    });
    await pumpApp(tester);

    await fillCredentials(tester);

    expect(find.text('无法在移动端完成登录'), findsOneWidget);
    expect(find.textContaining('改密'), findsWidgets);
    expect(find.byType(HomeStub), findsNothing);
  });

  testWidgets('凭据错误：内联展示服务端 message，停留登录页', (WidgetTester tester) async {
    adapter.handler = (options, _) =>
        jsonResponse(401, {'error': 'unauthorized', 'message': '用户名或密码错误'});
    await pumpApp(tester);

    await fillCredentials(tester);

    expect(find.text('用户名或密码错误'), findsOneWidget);
    expect(find.byKey(const ValueKey('login-totp')), findsNothing);
  });

  testWidgets('登出按钮清会话回登录页', (WidgetTester tester) async {
    await store.save(
      const SessionData(
        token: 'jwt-in',
        user: {'username': 'admin'},
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );
    await pumpApp(tester);
    expect(find.byType(HomeStub), findsOneWidget);

    await tester.tap(find.byTooltip('退出登录'));
    await tester.pumpAndSettle();

    expect(await store.load(), isNull);
    expect(find.byKey(const ValueKey('login-submit')), findsOneWidget);
  });
}
