import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/devices/devices_page.dart';
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

  Future<void> pumpDevices(WidgetTester tester) async {
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
        child: const MaterialApp(home: DevicesPage()),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('列表渲染：hostname/id·scope/心跳/函数/版本 + 状态徽标', (
    WidgetTester tester,
  ) async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'nodes': [
        {
          'id': 'agent-1',
          'hostname': 'gm-node-a',
          'gameId': 'demo',
          'env': 'prod',
          'status': 'active',
          'lastSeen': '2026-09-30 10:00:00',
          'version': 'v0.9.3',
          'functions': 12,
        },
        {
          'id': 'agent-2',
          'hostname': 'gm-node-b',
          'gameId': 'demo',
          'env': 'prod',
          'status': 'offline',
        },
      ],
    });
    await pumpDevices(tester);

    expect(find.text('gm-node-a'), findsOneWidget);
    expect(find.textContaining('agent-1 · demo/prod'), findsOneWidget);
    expect(find.textContaining('v0.9.3'), findsOneWidget);
    expect(find.text('在线'), findsOneWidget);
    expect(find.text('离线'), findsOneWidget);
    expect(find.byKey(const ValueKey('device-card-agent-1')), findsOneWidget);
  });

  testWidgets('label 芯片过滤', (WidgetTester tester) async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'nodes': [
        {
          'id': 'a1',
          'hostname': 'n1',
          'labels': {'role': 'gm'},
        },
        {
          'id': 'a2',
          'hostname': 'n2',
          'labels': {'role': 'log'},
        },
      ],
    });
    await pumpDevices(tester);

    expect(find.byKey(const ValueKey('device-card-a1')), findsOneWidget);
    expect(find.byKey(const ValueKey('device-card-a2')), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('label-chip-role=gm')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('device-card-a1')), findsOneWidget);
    expect(find.byKey(const ValueKey('device-card-a2')), findsNothing);
  });

  testWidgets('空表：暂无设备', (WidgetTester tester) async {
    adapter.handler = (options, _) => jsonResponse(200, {'nodes': []});
    await pumpDevices(tester);
    expect(find.text('暂无设备'), findsOneWidget);
  });

  testWidgets('加载失败：错误面 + 重试', (WidgetTester tester) async {
    adapter.handler = (options, _) =>
        jsonResponse(500, {'error': 'internal', 'message': 'boom'});
    await pumpDevices(tester);
    expect(find.text('boom'), findsOneWidget);

    adapter.handler = (options, _) => jsonResponse(200, {'nodes': []});
    await tester.tap(find.byKey(const ValueKey('devices-retry')));
    await tester.pumpAndSettle();
    expect(find.text('暂无设备'), findsOneWidget);
  });

  testWidgets('行点击进详情路由', (WidgetTester tester) async {
    adapter.handler = (options, _) {
      if (options.uri.path == '/api/v1/ops/nodes/agent-1') {
        return jsonResponse(200, {
          'node': {
            'id': 'agent-1',
            'hostname': 'gm-node-a',
            'status': 'active',
          },
        });
      }
      return jsonResponse(200, {
        'nodes': [
          {'id': 'agent-1', 'hostname': 'gm-node-a', 'status': 'active'},
        ],
      });
    };
    await pumpDevices(tester);

    await tester.tap(find.text('gm-node-a'));
    await tester.pumpAndSettle();
    expect(find.text('设备详情'), findsOneWidget);
    expect(find.text('基础信息'), findsOneWidget);
  });
}
