import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/audit/audit_page.dart';
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

  Future<void> pumpAudit(WidgetTester tester) async {
    await store.save(
      const SessionData(
        token: 'jwt',
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sessionStoreProvider.overrideWithValue(store),
          apiClientFactoryProvider.overrideWith((ref) {
            return (baseUrl) {
              final client = ApiClient.create(
                baseUrl: baseUrl,
                sessionStore: ref.read(sessionStoreProvider),
              );
              client.dio.httpClientAdapter = adapter;
              return client;
            };
          }),
        ],
        child: const MaterialApp(home: AuditPage()),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('列表渲染：action/userId·scope/结果 + 行展开 metadata/traceId', (
    WidgetTester tester,
  ) async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'items': [
        {
          'id': 'au-1',
          'action': 'approval_approve',
          'userId': 'admin',
          'gameId': 'demo',
          'env': 'prod',
          'target': 'ap-1',
          'result': 'success',
          'traceId': 'tr-9',
          'metadata': {'reason': 'ok'},
          'createdAt': '2026-09-30 10:00',
        },
      ],
      'total': 1,
    });
    await pumpAudit(tester);

    expect(find.text('approval_approve'), findsOneWidget);
    expect(find.textContaining('admin'), findsOneWidget);
    expect(find.textContaining('demo/prod'), findsOneWidget);
    expect(find.textContaining('共 1 条'), findsOneWidget);

    // 点行展开：traceId + metadata JSON。
    await tester.tap(find.text('approval_approve'));
    await tester.pumpAndSettle();
    expect(find.text('TraceID'), findsOneWidget);
    expect(find.textContaining('tr-9'), findsOneWidget);
    expect(find.textContaining('"reason": "ok"'), findsOneWidget);
  });

  testWidgets('筛选面板：kind 芯片多选 + 自定义 + 提交参数', (WidgetTester tester) async {
    final queries = <Uri>[];
    adapter.handler = (options, _) {
      queries.add(options.uri);
      return jsonResponse(200, {'items': [], 'total': 0});
    };
    await pumpAudit(tester);

    await tester.tap(find.byKey(const ValueKey('audit-filter-open')));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('audit-kind-invoke')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const ValueKey('audit-filter-actor')),
      'admin',
    );
    await tester.enterText(
      find.byKey(const ValueKey('audit-kind-custom')),
      'custom_kind',
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('audit-kind-add')));
    await tester.pumpAndSettle();
    // 自定义芯片出现后先取消再选回（覆盖 toggle 双向）。
    await tester.tap(
      find.byKey(const ValueKey('audit-kind-custom-custom_kind')),
    );
    await tester.pumpAndSettle();
    await tester.tap(
      find.byKey(const ValueKey('audit-kind-custom-custom_kind')),
    );
    await tester.pumpAndSettle();

    // apply 按钮在弹层视口下方，滚到可见再点。
    await tester.dragUntilVisible(
      find.byKey(const ValueKey('audit-filter-apply')),
      find.byType(ListView).last,
      const Offset(0, -120),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('audit-filter-apply')));
    await tester.pumpAndSettle();

    expect(queries.last.queryParameters['actor'], 'admin');
    expect(queries.last.queryParameters['kinds'], contains('invoke'));
    expect(queries.last.queryParameters['kinds'], contains('custom_kind'));
  });

  testWidgets('空表：暂无审计记录', (WidgetTester tester) async {
    adapter.handler = (options, _) =>
        jsonResponse(200, {'items': [], 'total': 0});
    await pumpAudit(tester);
    expect(find.text('暂无审计记录'), findsOneWidget);
  });

  testWidgets('加载失败：错误面 + 重试恢复', (WidgetTester tester) async {
    adapter.handler = (options, _) =>
        jsonResponse(500, {'error': 'internal', 'message': 'boom'});
    await pumpAudit(tester);
    expect(find.text('boom'), findsOneWidget);
    expect(find.byKey(const ValueKey('audit-retry')), findsOneWidget);

    adapter.handler = (options, _) =>
        jsonResponse(200, {'items': [], 'total': 0});
    await tester.tap(find.byKey(const ValueKey('audit-retry')));
    await tester.pumpAndSettle();
    expect(find.text('暂无审计记录'), findsOneWidget);
  });
}
