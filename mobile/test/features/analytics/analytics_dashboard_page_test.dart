import 'dart:async';

import 'package:croupier_mobile/app/providers.dart';
import 'package:dio/dio.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/analytics/analytics_dashboard_page.dart';
import 'package:croupier_mobile/features/scope/scope_controller.dart';
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

  testWidgets('渲染 ok/不可用/待建三类卡组（待建显示「数据源待建」）', (WidgetTester tester) async {
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
    expect(
      find.byKey(const ValueKey('analytics-status-overview')),
      findsOneWidget,
    );
    expect(find.textContaining('dau 42'), findsOneWidget);
    expect(find.text('payments db down'), findsOneWidget);
    expect(find.text('数据源待建：后端尚未接入该域，无合成数据'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('analytics-card-economy')),
      findsOneWidget,
    );
  });

  testWidgets('超时卡：显示「超时」chip 与预算错误文本（DATA-030）', (WidgetTester tester) async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'gameId': 'demo',
      'env': 'prod',
      'generatedAt': '2026-11-11T00:00:00Z',
      'domains': [
        {
          'domain': 'realtime',
          'status': 'timeout',
          'error': 'facade: domain summary exceeded its 5s budget',
        },
        {
          'domain': 'overview',
          'status': 'ok',
          'data': {
            'metrics': {'dau': 9},
          },
        },
      ],
    });
    await pumpPage(tester);

    expect(find.text('超时'), findsOneWidget);
    expect(find.textContaining('exceeded its 5s budget'), findsOneWidget);
    expect(find.textContaining('dau 9'), findsOneWidget);
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
        {
          'domain': 'overview',
          'status': 'ok',
          'data': {'metrics': {}},
        },
      ],
    });
    await tester.tap(find.byKey(const ValueKey('analytics-retry')));
    await tester.pumpAndSettle();

    expect(find.text('宏观大盘'), findsOneWidget);
  });

  testWidgets('ok 卡点击进分域详情：facade query 拉数据渲染', (WidgetTester tester) async {
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
            'data': {
              'metrics': {'dau': 42},
            },
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

  testWidgets('待建卡点击进占位详情：数据源待建（不发请求）', (WidgetTester tester) async {
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

    expect(
      find.byKey(const ValueKey('analytics-domain-pending')),
      findsOneWidget,
    );
    expect(queried, isFalse);
  });

  group('AUD-003 作用域切换清理与过期响应', () {
    Map<String, Object?> summaryBody(String gameId, String domain) => {
      'gameId': gameId,
      'env': 'prod',
      'generatedAt': '2026-11-11T00:00:00Z',
      'domains': [
        {
          'domain': domain,
          'status': 'ok',
          'data': {
            'metrics': {'dau': 1},
          },
        },
      ],
    };

    // 受控作用域控制器：只驱动 selected 变化（AUD-003 的缺陷在 analytics 对
    // scope 变化的反应，不测 ScopeController 的持久化面）。
    late _ScopeStub scopeCtrl;
    Future<ProviderContainer> pumpWithScope(
      WidgetTester tester,
      FutureOr<ResponseBody> Function(RequestOptions, List<int>?) handler, {
      bool settle = true,
    }) async {
      scopeCtrl = _ScopeStub(store);
      adapter.handler = handler;
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
            scopeControllerProvider.overrideWith(() => scopeCtrl),
          ],
          child: const MaterialApp(home: AnalyticsDashboardPage()),
        ),
      );
      // settle=false：请求挂起时转圈动画会令 pumpAndSettle 永不返回。
      if (settle) {
        await tester.pumpAndSettle();
      } else {
        await tester.pump(const Duration(milliseconds: 50));
      }
      return ProviderScope.containerOf(
        tester.element(find.byType(AnalyticsDashboardPage)),
      );
    }

    testWidgets('切换作用域：旧作用域卡片被清空，只显示新作用域数据', (WidgetTester tester) async {
      final container = await pumpWithScope(tester, (options, _) {
        final gameId = options.uri.queryParameters['gameId'];
        return jsonResponse(
          200,
          summaryBody(
            gameId ?? 'demo',
            gameId == 'demo2' ? 'behavior' : 'overview',
          ),
        );
      });

      expect(find.text('宏观大盘'), findsOneWidget); // demo 初始作用域

      await (container.read(scopeControllerProvider.notifier) as _ScopeStub)
          .setSelected('demo2', 'prod');
      await tester.pumpAndSettle();

      expect(find.text('宏观大盘'), findsNothing); // 旧作用域卡片已清
      expect(find.text('玩家行为'), findsOneWidget); // 新作用域卡片
    });

    testWidgets('切换作用域后请求失败：显示错误与重试，不保留旧作用域卡片', (WidgetTester tester) async {
      final container = await pumpWithScope(tester, (options, _) {
        if (options.uri.queryParameters['gameId'] == 'demo2') {
          return jsonResponse(500, {'error': 'internal', 'message': '新作用域不可用'});
        }
        return jsonResponse(200, summaryBody('demo', 'overview'));
      });

      expect(find.text('宏观大盘'), findsOneWidget);

      await (container.read(scopeControllerProvider.notifier) as _ScopeStub)
          .setSelected('demo2', 'prod');
      await tester.pumpAndSettle();

      expect(find.text('宏观大盘'), findsNothing); // 旧卡片不保留
      expect(find.text('新作用域不可用'), findsOneWidget); // 错误 + 重试
      expect(find.byKey(const ValueKey('analytics-retry')), findsOneWidget);
    });

    testWidgets('乱序响应：慢的旧作用域响应在切换后到达，被丢弃不覆盖新数据', (WidgetTester tester) async {
      final gate = Completer<ResponseBody>();
      var demoRequested = false;
      final container = await pumpWithScope(tester, (options, _) {
        final gameId = options.uri.queryParameters['gameId'];
        if (gameId == 'demo') {
          demoRequested = true;
          return gate.future; // 旧作用域请求挂起
        }
        return jsonResponse(200, summaryBody('demo2', 'behavior'));
      }, settle: false);

      expect(demoRequested, isTrue); // 初始 demo 请求已发出但挂起
      expect(
        find.byType(CircularProgressIndicator),
        findsOneWidget, // 加载中，无卡片
      );

      await (container.read(scopeControllerProvider.notifier) as _ScopeStub)
          .setSelected('demo2', 'prod');
      await tester.pumpAndSettle();
      expect(find.text('玩家行为'), findsOneWidget); // 新作用域已渲染

      // 慢的旧作用域响应此刻才到达——必须被丢弃。
      gate.complete(jsonResponse(200, summaryBody('demo', 'overview')));
      await tester.pumpAndSettle();

      expect(find.text('宏观大盘'), findsNothing); // 过期响应未覆盖
      expect(find.text('玩家行为'), findsOneWidget);
    });
  });
}

/// 仅用于测试：驱动 selected，同时按真实 select() 的口径更新会话
/// （analytics 控制器从会话读取 scope），绕过 ScopeController 的网络持久化面。
class _ScopeStub extends ScopeController {
  _ScopeStub(this._store);

  final InMemorySessionStore _store;

  Future<void> setSelected(String gameId, String env) async {
    final session = await _store.load();
    if (session != null) {
      await _store.save(session.copyWith(gameId: gameId, env: env));
    }
    state = state.copyWith(
      selected: ScopeSelection(gameId: gameId, env: env),
    );
  }
}
