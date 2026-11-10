import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/analytics/analytics_dashboard_page.dart';
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
    store.save(const SessionData(
      token: 'jwt',
      user: {'username': 'admin'},
      gameId: 'demo',
      env: 'prod',
      serverUrl: 'http://gm.test',
    ));
  });

  Future<void> pumpPage(WidgetTester tester) async {
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
        child: const MaterialApp(home: AnalyticsDashboardPage()),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('渲染 ok/不可用/待建三类卡组（待建显示「数据源待建」）', (
    WidgetTester tester,
  ) async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'gameId': 'demo',
      'env': 'prod',
      'generatedAt': '2026-11-11T00:00:00Z',
      'domains': [
        {
          'domain': 'overview',
          'status': 'ok',
          'data': {
            'metrics': {'dau': 42, 'mau': 7.5},
          },
        },
        {
          'domain': 'payments',
          'status': 'unavailable',
          'error': 'payments db down',
        },
        {'domain': 'economy', 'status': 'pending_data_source'},
      ],
    });
    await pumpPage(tester);

    expect(find.text('宏观大盘'), findsOneWidget);
    expect(find.byKey(const ValueKey('analytics-status-overview')), findsOneWidget);
    expect(find.textContaining('dau 42'), findsOneWidget);
    expect(find.text('payments db down'), findsOneWidget);
    expect(find.text('数据源待建：后端尚未接入该域，无合成数据'), findsOneWidget);
    expect(find.byKey(const ValueKey('analytics-card-economy')), findsOneWidget);
  });

  testWidgets('空卡组：暂无数据占位', (WidgetTester tester) async {
    adapter.handler = (options, _) =>
        jsonResponse(200, {'gameId': 'demo', 'env': 'prod', 'domains': []});
    await pumpPage(tester);

    expect(find.byKey(const ValueKey('analytics-empty')), findsOneWidget);
  });

  testWidgets('错误态：message + 重试后恢复', (WidgetTester tester) async {
    adapter.handler = (options, _) =>
        jsonResponse(500, {'error': 'internal', 'message': '服务不可用'});
    await pumpPage(tester);

    expect(find.text('服务不可用'), findsOneWidget);
    expect(find.byKey(const ValueKey('analytics-retry')), findsOneWidget);

    adapter.handler = (options, _) => jsonResponse(200, {
      'gameId': 'demo',
      'env': 'prod',
      'domains': [
        {'domain': 'overview', 'status': 'ok', 'data': {'metrics': {}}},
      ],
    });
    await tester.tap(find.byKey(const ValueKey('analytics-retry')));
    await tester.pumpAndSettle();

    expect(find.text('宏观大盘'), findsOneWidget);
  });

  testWidgets('ok 卡点击进分域详情：facade query 拉数据渲染', (
    WidgetTester tester,
  ) async {
    adapter.handler = (options, _) {
      final path = options.uri.path;
      if (path == '/api/v1/analytics/facade/query') {
        return jsonResponse(200, {
          'domain': 'overview',
          'data': {
            'metrics': {'dau': 42},
          },
        });
      }
      return jsonResponse(200, {
        'gameId': 'demo',
        'env': 'prod',
        'domains': [
          {
            'domain': 'overview',
            'status': 'ok',
            'data': {'metrics': {'dau': 42}},
          },
        ],
      });
    };
    await pumpPage(tester);

    await tester.tap(find.text('宏观大盘'));
    await tester.pumpAndSettle();

    expect(find.text('分域查询'), findsNothing);
    expect(find.byKey(const ValueKey('analytics-domain-empty')), findsNothing);
    expect(find.textContaining('dau'), findsWidgets);
  });

  testWidgets('待建卡点击进占位详情：数据源待建（不发请求）', (
    WidgetTester tester,
  ) async {
    var queried = false;
    adapter.handler = (options, _) {
      if (options.uri.path == '/api/v1/analytics/facade/query') {
        queried = true;
      }
      return jsonResponse(200, {
        'gameId': 'demo',
        'env': 'prod',
        'domains': [
          {'domain': 'economy', 'status': 'pending_data_source'},
        ],
      });
    };
    await pumpPage(tester);

    await tester.tap(find.text('经济系统'));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('analytics-domain-pending')), findsOneWidget);
    expect(queried, isFalse);
  });
}
