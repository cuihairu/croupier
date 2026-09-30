import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/alerts/alerts_page.dart';
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

  Future<void> pumpAlerts(WidgetTester tester) async {
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
        child: const MaterialApp(home: AlertsPage()),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('列表渲染：message/type·source/level 色阶 + 状态', (
    WidgetTester tester,
  ) async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'items': [
        {
          'id': 'al-1',
          'type': 'agent_offline',
          'level': 'critical',
          'message': '节点失联',
          'source': 'registry',
          'status': 'firing',
          'createdAt': '2026-09-30 10:00',
        },
      ],
      'total': 1,
    });
    await pumpAlerts(tester);

    expect(find.text('节点失联'), findsOneWidget);
    expect(find.textContaining('agent_offline'), findsOneWidget);
    // 'critical' 同时出现在筛选芯片与卡片徽标；后者用卡片范围收窄断言。
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('alert-card-al-1')),
        matching: find.text('critical'),
      ),
      findsOneWidget,
    );
    expect(find.text('firing'), findsOneWidget);
    expect(find.byKey(const ValueKey('alert-card-al-1')), findsOneWidget);
  });

  testWidgets('筛选芯片切换触发重查', (WidgetTester tester) async {
    final queries = <Uri>[];
    adapter.handler = (options, _) {
      queries.add(options.uri);
      return jsonResponse(200, {'items': [], 'total': 0});
    };
    await pumpAlerts(tester);
    expect(find.text('暂无告警'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('alerts-level-critical')));
    await tester.pumpAndSettle();
    expect(queries.last.queryParameters['level'], 'critical');

    await tester.tap(find.byKey(const ValueKey('alerts-status-firing')));
    await tester.pumpAndSettle();
    expect(queries.last.queryParameters['status'], 'firing');
    expect(queries.last.queryParameters['page'], '1');
  });

  testWidgets('静默：reason 必填不关窗 → 填写后提交刷新', (WidgetTester tester) async {
    final posts = <Map<String, Object?>>[];
    adapter.handler = (options, _) {
      if (options.method == 'POST') {
        posts.add({'path': options.uri.path, 'body': options.uri.toString()});
        return jsonResponse(200, {'message': '操作成功'});
      }
      return jsonResponse(200, {
        'items': [
          {
            'id': 'al-1',
            'message': '节点失联',
            'level': 'critical',
            'status': 'firing',
          },
        ],
        'total': 1,
      });
    };
    await pumpAlerts(tester);

    await tester.tap(find.byKey(const ValueKey('alert-silence-al-1')));
    await tester.pumpAndSettle();

    // duration 预填 60；reason 空时确认按钮不生效（弹窗不关）。
    expect(
      tester
          .widget<TextField>(find.byKey(const ValueKey('silence-duration')))
          .controller!
          .text,
      '60',
    );
    await tester.tap(find.byKey(const ValueKey('silence-confirm')));
    await tester.pumpAndSettle();
    expect(find.text('静默告警'), findsOneWidget);

    await tester.enterText(
      find.byKey(const ValueKey('silence-reason')),
      '维护窗口',
    );
    await tester.tap(find.byKey(const ValueKey('silence-confirm')));
    await tester.pumpAndSettle();
    expect(find.text('已静默'), findsOneWidget);
    expect(posts, hasLength(1));
    expect((posts.single['path'] as String), '/api/v1/alerts/al-1/silence');
  });

  testWidgets('静默规则弹层：只读列表', (WidgetTester tester) async {
    adapter.handler = (options, _) {
      if (options.uri.path == '/api/v1/alerts/silences') {
        return jsonResponse(200, {
          'items': [
            {
              'id': 'sl-1',
              'alertType': 'agent_offline',
              'startAt': '2026-09-30 10:00',
              'endAt': '2026-09-30 12:00',
              'createdBy': 'admin',
            },
          ],
        });
      }
      return jsonResponse(200, {'items': [], 'total': 0});
    };
    await pumpAlerts(tester);

    await tester.tap(find.byKey(const ValueKey('alerts-silences-open')));
    await tester.pumpAndSettle();
    expect(find.text('静默规则'), findsOneWidget);
    expect(find.byKey(const ValueKey('silence-rule-sl-1')), findsOneWidget);
    expect(find.text('agent_offline'), findsOneWidget);
    expect(find.text('admin'), findsOneWidget);
  });

  testWidgets('加载失败：错误面 + 重试', (WidgetTester tester) async {
    adapter.handler = (options, _) =>
        jsonResponse(500, {'error': 'internal', 'message': 'boom'});
    await pumpAlerts(tester);
    expect(find.text('boom'), findsOneWidget);

    adapter.handler = (options, _) =>
        jsonResponse(200, {'items': [], 'total': 0});
    await tester.tap(find.byKey(const ValueKey('alerts-retry')));
    await tester.pumpAndSettle();
    expect(find.text('暂无告警'), findsOneWidget);
  });
}
