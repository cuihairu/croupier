import 'package:croupier_mobile/app/app.dart';
import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:flutter/material.dart';
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
          apiClientFactoryProvider.overrideWithValue((baseUrl) {
            final client = ApiClient.create(
              baseUrl: baseUrl,
              sessionStore: store,
            );
            client.dio.httpClientAdapter = adapter;
            return client;
          }),
        ],
        child: const CroupierApp(),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('切换器：打开弹层列游戏，选 env 后持久化并更新顶栏', (WidgetTester tester) async {
    var scopePutBody = '';
    adapter.handler = (options, _) {
      final path = options.uri.path;
      if (path == '/api/v1/profile/games') {
        return jsonResponse(200, {
          'games': [
            {
              'gameId': 'demo',
              'gameName': '演示游戏',
              'envs': ['dev', 'prod'],
            },
          ],
        });
      }
      if (path == '/api/v1/profile/scope') {
        scopePutBody = adapter.lastBody ?? '';
        return jsonResponse(200, {'ok': true});
      }
      // MainShell 挂载后审批域请求：给合法空数据，避免噪音
      if (path == '/api/v1/approvals/') {
        return jsonResponse(200, {'approvals': <Object>[], 'total': 0});
      }
      if (path == '/api/v1/functions/descriptors') {
        return jsonResponse(200, {'items': <Object>[]});
      }
      return jsonResponse(404, {'error': 'not_found', 'message': path});
    };
    await store.save(
      const SessionData(
        token: 'jwt',
        user: {'username': 'admin'},
        serverUrl: 'http://gm.test',
      ),
    );
    await pumpApp(tester);
    // load() 已预选第一个可用 (game, env)：lastGameId 为空 → demo/dev
    expect(find.text('demo / dev'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('scope-switcher')));
    await tester.pumpAndSettle();
    expect(find.text('演示游戏'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('scope-env-demo-prod')));
    await tester.pumpAndSettle();

    // 弹层关闭，顶栏展示新 scope；PUT body 正确（捕获变量，lastRequest 已被
    // 后续审批域请求覆盖）
    expect(find.byKey(const ValueKey('scope-env-demo-prod')), findsNothing);
    expect(find.text('demo / prod'), findsOneWidget);
    expect(scopePutBody, contains('"env":"prod"'));
    expect((await store.load())?.env, 'prod');
  });

  testWidgets('弹层错误态：展示 message 并可重试', (WidgetTester tester) async {
    adapter.handler = (options, _) =>
        jsonResponse(500, {'error': 'internal', 'message': '服务不可用'});
    await store.save(
      const SessionData(token: 'jwt', serverUrl: 'http://gm.test'),
    );
    await pumpApp(tester);

    await tester.tap(find.byKey(const ValueKey('scope-switcher')));
    await tester.pumpAndSettle();

    // 弹层与底层审批列表同走 500，两处都展示服务端 message
    expect(find.text('服务不可用'), findsWidgets);
    expect(find.text('重试'), findsWidgets);
  });
}
