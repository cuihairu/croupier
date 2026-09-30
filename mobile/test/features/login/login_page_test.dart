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
          // 登录成功后主壳（scope 切换器/审批页）会用 apiClientFactory 发请求，
          // 同样必须落到 fake adapter（否则真 HttpClient 被 test binding 拦 400）
          apiClientFactoryProvider.overrideWithValue((baseUrl) {
            final client = ApiClient.create(
              baseUrl: baseUrl,
              sessionStore: store,
            );
            client.dio.httpClientAdapter = adapter;
            return client;
          }),
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
    var loginCalls = 0;
    adapter.handler = (options, _) {
      final path = options.uri.path;
      if (path == '/api/v1/auth/login') {
        loginCalls++;
        if (loginCalls == 1) {
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
      }
      // 登录成功后主壳挂载的后续请求：games 返回含 lastGameId 的列表（scope
      // 预选生效），审批域给合法空数据
      if (path == '/api/v1/profile/games') {
        return jsonResponse(200, {
          'games': [
            {
              'gameId': 'demo',
              'gameName': '演示',
              'envs': ['prod'],
            },
          ],
        });
      }
      if (path == '/api/v1/approvals/') {
        return jsonResponse(200, {'approvals': <Object>[], 'total': 0});
      }
      if (path == '/api/v1/functions/descriptors') {
        return jsonResponse(200, {'items': <Object>[]});
      }
      return jsonResponse(404, {'error': 'not_found', 'message': path});
    };
    await pumpApp(tester);

    await fillCredentials(tester);
    await tester.enterText(find.byKey(const ValueKey('login-totp')), '123456');
    await tester.tap(find.byKey(const ValueKey('login-submit')));
    await tester.pumpAndSettle();

    // App 根据会话探测切换到主壳；games 预选 lastGameId/lastEnv → 顶栏 demo/prod
    expect(find.byType(MainShell), findsOneWidget);
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
    expect(find.byType(MainShell), findsNothing);
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
    expect(find.byType(MainShell), findsOneWidget);

    await tester.tap(find.byTooltip('退出登录'));
    await tester.pumpAndSettle();

    expect(await store.load(), isNull);
    expect(find.byKey(const ValueKey('login-submit')), findsOneWidget);
  });
}
