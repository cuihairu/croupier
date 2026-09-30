import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/devices/devices_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late ProviderContainer container;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    container = ProviderContainer(
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
    );
    addTearDown(container.dispose);
  });

  Future<void> seedSession() => store.save(
    const SessionData(
      token: 'jwt',
      gameId: 'demo',
      env: 'prod',
      serverUrl: 'http://gm.test',
    ),
  );

  final nodesPayload = {
    'nodes': [
      {
        'id': 'a1',
        'hostname': 'node-a',
        'labels': {'role': 'gm'},
      },
      {
        'id': 'a2',
        'hostname': 'node-b',
        'labels': {'role': 'log'},
      },
    ],
  };

  test('refresh 拉整表 + allLabels 去重', () async {
    await seedSession();
    adapter.handler = (options, _) => jsonResponse(200, nodesPayload);
    await container.read(devicesControllerProvider.notifier).refresh();
    final state = container.read(devicesControllerProvider);
    expect(state.nodes, hasLength(2));
    expect(state.allLabels, ['role=gm', 'role=log']);
  });

  test('toggleLabel 客户端过滤', () async {
    await seedSession();
    adapter.handler = (options, _) => jsonResponse(200, nodesPayload);
    final notifier = container.read(devicesControllerProvider.notifier);
    await notifier.refresh();
    notifier.toggleLabel('role=gm');
    expect(
      container.read(devicesControllerProvider).filteredNodes,
      hasLength(1),
    );
    // 再点一次取消过滤。
    notifier.toggleLabel('role=gm');
    expect(
      container.read(devicesControllerProvider).filteredNodes,
      hasLength(2),
    );
  });

  test('多芯片 AND 过滤', () async {
    await seedSession();
    adapter.handler = (options, _) => jsonResponse(200, {
      'nodes': [
        {
          'id': 'a1',
          'labels': {'role': 'gm', 'region': 'cn'},
        },
        {
          'id': 'a2',
          'labels': {'role': 'gm'},
        },
      ],
    });
    final notifier = container.read(devicesControllerProvider.notifier);
    await notifier.refresh();
    notifier.toggleLabel('role=gm');
    notifier.toggleLabel('region=cn');
    expect(
      container.read(devicesControllerProvider).filteredNodes.single.id,
      'a1',
    );
  });

  test('未登录置错误面', () async {
    await container.read(devicesControllerProvider.notifier).refresh();
    expect(container.read(devicesControllerProvider).error, '未登录或缺少服务器地址');
  });

  test('HTTP 错误置 error', () async {
    await seedSession();
    adapter.handler = (options, _) => jsonResponse(500, {'message': 'boom'});
    await container.read(devicesControllerProvider.notifier).refresh();
    expect(container.read(devicesControllerProvider).error, 'boom');
  });
}
