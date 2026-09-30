import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/monitoring/monitoring_controller.dart';
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

  final okPayload = {
    'runtime': {'goroutines': 42, 'uptimeSeconds': 3600},
    'host': {'cpuPercent': 5},
    'overload': {'disk': true},
  };

  test('refresh 成功置快照', () async {
    await store.save(
      const SessionData(
        token: 'jwt',
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );
    adapter.handler = (options, _) => jsonResponse(200, okPayload);
    final notifier = container.read(monitoringControllerProvider.notifier);
    await notifier.refresh();
    final state = container.read(monitoringControllerProvider);
    expect(state.error, isNull);
    expect(state.loading, isFalse);
    expect(state.snapshot?.runtime.goroutines, 42);
    expect(state.snapshot?.overload.disk, isTrue);
  });

  test('未登录置错误面', () async {
    final notifier = container.read(monitoringControllerProvider.notifier);
    await notifier.refresh();
    expect(container.read(monitoringControllerProvider).error, '未登录或缺少服务器地址');
  });

  test('HTTP 错误置 error，重试可恢复', () async {
    await store.save(
      const SessionData(
        token: 'jwt',
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );
    adapter.handler = (options, _) => jsonResponse(500, {'message': 'boom'});
    final notifier = container.read(monitoringControllerProvider.notifier);
    await notifier.refresh();
    expect(container.read(monitoringControllerProvider).error, 'boom');

    adapter.handler = (options, _) => jsonResponse(200, okPayload);
    await notifier.refresh();
    final state = container.read(monitoringControllerProvider);
    expect(state.error, isNull);
    expect(state.snapshot?.runtime.goroutines, 42);
  });

  test('形态不合法（缺块）置 error', () async {
    await store.save(
      const SessionData(
        token: 'jwt',
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );
    adapter.handler = (options, _) => jsonResponse(200, {'foo': 1});
    await container.read(monitoringControllerProvider.notifier).refresh();
    expect(
      container.read(monitoringControllerProvider).error,
      contains('invalid performance payload'),
    );
  });
}
