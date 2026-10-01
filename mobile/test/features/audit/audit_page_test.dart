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

  testWidgets('加载更多：第二页累积 + hasMore 收口后按钮消失', (WidgetTester tester) async {
    final queries = <Uri>[];
    adapter.handler = (options, _) {
      queries.add(options.uri);
      final page = options.uri.queryParameters['page'];
      return jsonResponse(200, {
        'items': [
          {
            'id': 'au-$page-1',
            'action': 'invoke',
            'createdAt': '2026-09-30 10:00',
          },
        ],
        'total': 2,
      });
    };
    await pumpAudit(tester);

    expect(find.textContaining('共 2 条'), findsOneWidget);
    expect(find.byKey(const ValueKey('audit-load-more')), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('audit-load-more')));
    await tester.pumpAndSettle();

    expect(queries, hasLength(2));
    expect(queries.last.queryParameters['page'], '2');
    expect(find.byKey(const ValueKey('audit-card-au-2-1')), findsOneWidget);
    expect(find.byKey(const ValueKey('audit-load-more')), findsNothing);
  });

  testWidgets('卡片展开：target/traceId/metadata 全空显「无附加信息」', (
    WidgetTester tester,
  ) async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'items': [
        // metadata 显式空 map：令 `metadata == null || metadata.isEmpty`
        // 的短路右侧求值（null 场景走短路，行覆盖缺 376）。
        {
          'id': 'au-bare',
          'action': 'invoke',
          'createdAt': '2026-09-30 10:00',
          'metadata': <String, Object>{},
        },
      ],
      'total': 1,
    });
    await pumpAudit(tester);

    await tester.tap(find.byKey(const ValueKey('audit-card-au-bare')));
    await tester.pumpAndSettle();
    expect(find.text('无附加信息'), findsOneWidget);
  });

  testWidgets('下拉刷新重放第一页', (WidgetTester tester) async {
    final queries = <Uri>[];
    adapter.handler = (options, _) {
      queries.add(options.uri);
      return jsonResponse(200, {
        'items': [
          {'id': 'au-1', 'action': 'invoke', 'createdAt': '2026-09-30 10:00'},
        ],
        'total': 1,
      });
    };
    await pumpAudit(tester);
    expect(queries, hasLength(1));

    await tester.fling(find.byType(ListView), const Offset(0, 400), 1000);
    await tester.pumpAndSettle();

    expect(queries, hasLength(2));
    expect(queries.last.queryParameters['page'], '1');
  });

  testWidgets('自定义 kind chip 再点移除（取消勾选）', (WidgetTester tester) async {
    final queries = <Uri>[];
    adapter.handler = (options, _) {
      queries.add(options.uri);
      return jsonResponse(200, {'items': [], 'total': 0});
    };
    await pumpAudit(tester);

    await tester.tap(find.byKey(const ValueKey('audit-filter-open')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const ValueKey('audit-kind-custom')),
      'zzz_op',
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('audit-kind-add')));
    await tester.pumpAndSettle();

    final chip = find.byKey(const ValueKey('audit-kind-custom-zzz_op'));
    await tester.dragUntilVisible(
      chip,
      find.byType(ListView).last,
      const Offset(0, -80),
    );
    await tester.pumpAndSettle();
    await tester.tap(chip);
    await tester.pumpAndSettle();
    expect(chip, findsNothing, reason: '再点自定义 chip 应整块移除');

    await tester.dragUntilVisible(
      find.byKey(const ValueKey('audit-filter-apply')),
      find.byType(ListView).last,
      const Offset(0, -120),
    );
    await tester.tap(find.byKey(const ValueKey('audit-filter-apply')));
    await tester.pumpAndSettle();
    expect(
      queries.last.queryParameters['kinds'],
      isNull,
      reason: '移除后 kinds 为空，请求不带 kinds 参数',
    );
  });

  testWidgets('时间区间：picker 回填 + 提交 RFC3339 归一（end 23:59:59）', (
    WidgetTester tester,
  ) async {
    final queries = <Uri>[];
    adapter.handler = (options, _) {
      queries.add(options.uri);
      return jsonResponse(200, {'items': [], 'total': 0});
    };
    await pumpAudit(tester);

    await tester.tap(find.byKey(const ValueKey('audit-filter-open')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('audit-filter-range')));
    await tester.pumpAndSettle();

    // 选择「今天」与「明天」——均在 end 月（右侧网格），且 ≤ lastDate(now+1)。
    // 这样在月初/月末/跨月时均稳定可选，避免写死 15/20 导致的脆弱性。
    final now = DateTime.now();
    final todayStr = now.day.toString();
    final tomorrowStr = now.add(const Duration(days: 1)).day.toString();
    await tester.tap(find.text(todayStr).last);
    await tester.pump();
    await tester.tap(find.text(tomorrowStr).last);
    await tester.pump();
    // M3 DateRangePickerDialog 确认按钮为 Save（saveButtonLabel）。
    await tester.tap(find.text('Save'));
    await tester.pumpAndSettle();

    // chip label 回填日期区间（_rangeDate 渲染）。
    final month = now.month.toString().padLeft(2, '0');
    final todayFmt = '${now.year}-$month-$todayStr';
    final tomorrow = now.add(const Duration(days: 1));
    final tomorrowMonth = tomorrow.month.toString().padLeft(2, '0');
    final tomorrowFmt = '${tomorrow.year}-$tomorrowMonth-$tomorrowStr';
    expect(find.textContaining('$todayFmt ~ $tomorrowFmt'), findsOneWidget);

    await tester.dragUntilVisible(
      find.byKey(const ValueKey('audit-filter-apply')),
      find.byType(ListView).last,
      const Offset(0, -120),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('audit-filter-apply')));
    await tester.pumpAndSettle();

    final start = queries.last.queryParameters['start'];
    final end = queries.last.queryParameters['end'];
    expect(
      start,
      startsWith(
        '$todayFmt'
        'T00:00:00',
      ),
    );
    expect(
      end,
      startsWith(
        '$tomorrowFmt'
        'T23:59:59',
      ),
    );
    // RFC3339 必须带时区 offset（否则服务端静默忽略）。
    expect(RegExp(r'[+-]\d{2}:\d{2}$').hasMatch(start!), isTrue);
    expect(RegExp(r'[+-]\d{2}:\d{2}$').hasMatch(end!), isTrue);
  });
}
