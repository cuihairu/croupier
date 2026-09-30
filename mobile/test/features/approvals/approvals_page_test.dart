import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/approvals/approvals_page.dart';
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
    store.save(
      const SessionData(
        token: 'jwt',
        user: {'username': 'admin'},
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );
  });

  Future<void> pumpShell(WidgetTester tester) async {
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
        child: const MaterialApp(home: ApprovalsPage()),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('列表渲染：functionId/actor/mode + 行点击进详情路由', (
    WidgetTester tester,
  ) async {
    adapter.handler = (options, _) {
      final path = options.uri.path;
      if (path == '/api/v1/approvals/') {
        return jsonResponse(200, {
          'approvals': [
            {
              'id': 'a1',
              'functionId': 'player.kick',
              'actor': 'op1',
              'state': 'pending',
              'mode': 'invoke',
              'createdAt': '2026-09-30 10:00',
            },
          ],
          'total': 1,
        });
      }
      return jsonResponse(200, {'items': <Object>[]});
    };
    await pumpShell(tester);

    expect(find.text('player.kick'), findsOneWidget);
    expect(find.text('op1 · invoke · 2026-09-30 10:00'), findsOneWidget);

    await tester.tap(find.text('player.kick'));
    await tester.pumpAndSettle();
    expect(find.text('审批详情'), findsOneWidget);
  });

  testWidgets('空列表：暂无审批占位', (WidgetTester tester) async {
    adapter.handler = (options, _) =>
        jsonResponse(200, {'approvals': [], 'total': 0});
    await pumpShell(tester);

    expect(find.text('暂无审批'), findsOneWidget);
  });

  testWidgets('错误态：展示 message + 重试', (WidgetTester tester) async {
    adapter.handler = (options, _) =>
        jsonResponse(500, {'error': 'internal', 'message': '服务不可用'});
    await pumpShell(tester);

    expect(find.text('服务不可用'), findsOneWidget);
    expect(find.text('重试'), findsOneWidget);
  });
}
