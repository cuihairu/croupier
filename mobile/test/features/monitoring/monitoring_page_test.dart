import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/monitoring/monitoring_page.dart';
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

  Future<void> seedSession() async {
    await store.save(
      const SessionData(
        token: 'jwt',
        user: {'username': 'admin'},
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );
  }

  Future<void> pumpMonitoring(WidgetTester tester) async {
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
        child: const MaterialApp(home: Scaffold(body: MonitoringPage())),
      ),
    );
    await tester.pumpAndSettle();
  }

  final okPayload = {
    'runtime': {
      'goroutines': 42,
      'heapAllocBytes': 1536,
      'numGC': 7,
      'gcPauseMs': 12.5,
      'uptimeSeconds': 90061,
    },
    'host': {
      'cpuPercent': 12.5,
      'memoryUsedPct': 60,
      'memoryTotalBytes': 1024,
      'memoryUsedBytes': 614,
      'diskUsedPct': 80,
      'diskTotalBytes': 2048,
      'diskUsedBytes': 1638,
    },
    'overload': {'cpu': false, 'memory': false, 'disk': true},
  };

  testWidgets('渲染运行时与宿主机指标', (WidgetTester tester) async {
    await seedSession();
    adapter.handler = (options, _) => jsonResponse(200, okPayload);
    await pumpMonitoring(tester);

    expect(find.text('42'), findsOneWidget);
    expect(find.text('1.5KB'), findsOneWidget);
    expect(find.text('1天1小时'), findsOneWidget);
    expect(find.text('12.5%'), findsOneWidget);
  });

  testWidgets('过载行描红 +「过载」徽标', (WidgetTester tester) async {
    await seedSession();
    adapter.handler = (options, _) => jsonResponse(200, okPayload);
    await pumpMonitoring(tester);

    // disk 过载：徽标存在；cpu/memory 不过载：无徽标。
    expect(find.byKey(const ValueKey('overload-磁盘')), findsOneWidget);
    expect(find.byKey(const ValueKey('overload-CPU')), findsNothing);
    expect(find.byKey(const ValueKey('overload-内存')), findsNothing);
    final diskValue = tester.widget<Text>(find.textContaining('80%')).style;
    expect(diskValue?.color, Colors.red);
    expect(find.text('过载'), findsOneWidget);
  });

  testWidgets('加载失败：错误面 + 重试', (WidgetTester tester) async {
    await seedSession();
    adapter.handler = (options, _) =>
        jsonResponse(500, {'error': 'internal', 'message': 'boom'});
    await pumpMonitoring(tester);

    expect(find.text('boom'), findsOneWidget);
    expect(find.byKey(const ValueKey('monitor-retry')), findsOneWidget);

    adapter.handler = (options, _) => jsonResponse(200, okPayload);
    await tester.tap(find.byKey(const ValueKey('monitor-retry')));
    await tester.pumpAndSettle();
    expect(find.text('42'), findsOneWidget);
  });

  testWidgets('入口卡：设备/告警已启用，审计仍为禁用占位', (WidgetTester tester) async {
    await seedSession();
    adapter.handler = (options, _) => jsonResponse(200, okPayload);
    await pumpMonitoring(tester);

    expect(
      tester
          .widget<ListTile>(find.byKey(const ValueKey('monitor-entry-devices')))
          .enabled,
      isTrue,
    );
    expect(
      tester
          .widget<ListTile>(find.byKey(const ValueKey('monitor-entry-alerts')))
          .enabled,
      isTrue,
    );
    expect(
      tester
          .widget<ListTile>(find.byKey(const ValueKey('monitor-entry-audit')))
          .enabled,
      isFalse,
    );
    expect(find.text('M2 交付'), findsOneWidget);
  });

  testWidgets('点设备入口进入设备列表页', (WidgetTester tester) async {
    await seedSession();
    adapter.handler = (options, _) {
      if (options.uri.path == '/api/v1/ops/nodes') {
        return jsonResponse(200, {
          'nodes': [
            {
              'id': 'agent-1',
              'hostname': 'gm-node-a',
              'gameId': 'demo',
              'env': 'prod',
              'status': 'active',
            },
          ],
        });
      }
      return jsonResponse(200, okPayload);
    };
    await pumpMonitoring(tester);

    await tester.tap(find.byKey(const ValueKey('monitor-entry-devices')));
    await tester.pumpAndSettle();
    expect(find.text('设备（Agent）'), findsWidgets);
    expect(find.byKey(const ValueKey('device-card-agent-1')), findsOneWidget);
  });
}
