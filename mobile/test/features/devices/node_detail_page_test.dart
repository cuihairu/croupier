import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/devices/node_detail_page.dart';
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

  Future<void> pumpDetail(WidgetTester tester, String nodeId) async {
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
        child: MaterialApp(home: NodeDetailPage(nodeId: nodeId)),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('三卡渲染：基础信息/资源/函数', (WidgetTester tester) async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'node': {
        'id': 'agent-1',
        'hostname': 'gm-node-a',
        'addr': '10.0.0.2:19091',
        'gameId': 'demo',
        'env': 'prod',
        'status': 'active',
        'lastSeen': '2026-09-30 10:00:00',
        'version': 'v0.9.3',
        'sdkLanguage': 'go',
        'sdkVersion': '1.2.0',
        'sdkName': 'croupier-sdk-go',
        'expiresInSec': 3600,
        'labels': {'role': 'gm'},
        'cpu': {'usagePercent': 12.5, 'cores': 8},
        'memory': {'totalBytes': 1024, 'usedBytes': 512, 'usagePercent': 50},
        'disks': [
          {
            'mountPoint': '/',
            'totalBytes': 2048,
            'usedBytes': 1024,
            'usagePercent': 50,
          },
        ],
        'functions': 12,
      },
    });
    await pumpDetail(tester, 'agent-1');

    expect(find.text('基础信息'), findsOneWidget);
    expect(find.text('资源'), findsOneWidget);
    expect(find.text('v0.9.3'), findsOneWidget);
    expect(find.textContaining('12.5%'), findsOneWidget);
    expect(find.textContaining('1KB'), findsWidgets);

    // 函数卡在测试视口外（ListView 惰性构建），滚动到可见再断言。
    await tester.scrollUntilVisible(
      find.text('函数清单请在 Web 端查看'),
      200,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.pumpAndSettle();
    expect(find.text('函数'), findsOneWidget);
    expect(find.text('12'), findsOneWidget);
    expect(find.text('函数清单请在 Web 端查看'), findsOneWidget);
  });

  testWidgets('无系统信息上报：资源卡显示「暂无上报」', (WidgetTester tester) async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'node': {'id': 'agent-2', 'status': 'stale'},
    });
    await pumpDetail(tester, 'agent-2');
    expect(find.text('暂无上报'), findsOneWidget);
  });

  testWidgets('404：错误面 + 重试', (WidgetTester tester) async {
    adapter.handler = (options, _) =>
        jsonResponse(404, {'error': 'not_found', 'message': '节点不存在'});
    await pumpDetail(tester, 'missing');
    expect(find.text('节点不存在'), findsOneWidget);
    expect(find.byKey(const ValueKey('node-detail-retry')), findsOneWidget);
  });
}
