import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/scope/scope_controller.dart';
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

  ProviderContainer makeContainer({SessionData? session}) {
    final container = ProviderContainer(
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
    );
    addTearDown(container.dispose);
    if (session != null) {
      store.save(session);
    }
    return container;
  }

  final gamesPayload = {
    'games': [
      {
        'gameId': 'demo',
        'gameName': '演示',
        'envs': ['dev', 'prod'],
      },
      {
        'gameId': 'beta',
        'gameName': 'Beta',
        'envs': ['prod'],
      },
    ],
  };

  test('load：lastGameId/lastEnv 仍可选 → 预选保持', () async {
    adapter.handler = (options, _) => jsonResponse(200, gamesPayload);
    final container = makeContainer(
      session: const SessionData(
        token: 'jwt',
        gameId: 'beta',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );

    await container.read(scopeControllerProvider.notifier).load();

    final state = container.read(scopeControllerProvider);
    expect(state.loading, isFalse);
    expect(state.selected?.gameId, 'beta');
    expect(state.selected?.env, 'prod');
  });

  test('load：lastGameId 不在可选列表 → 落第一个可用 (game, env)', () async {
    adapter.handler = (options, _) => jsonResponse(200, gamesPayload);
    final container = makeContainer(
      session: const SessionData(
        token: 'jwt',
        gameId: 'gone',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );

    await container.read(scopeControllerProvider.notifier).load();

    final state = container.read(scopeControllerProvider);
    expect(state.selected?.gameId, 'demo');
    expect(state.selected?.env, 'dev');
  });

  test('load：无 serverUrl（未登录）→ error 态', () async {
    final container = makeContainer();

    await container.read(scopeControllerProvider.notifier).load();

    expect(container.read(scopeControllerProvider).error, isNotNull);
  });

  test('select：PUT 持久化成功 → 更新会话与 selected', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'ok': true});
    final container = makeContainer(
      session: const SessionData(
        token: 'jwt',
        gameId: 'demo',
        env: 'dev',
        serverUrl: 'http://gm.test',
      ),
    );

    await container
        .read(scopeControllerProvider.notifier)
        .select('demo', 'prod');

    final state = container.read(scopeControllerProvider);
    expect(state.selected, const ScopeSelection(gameId: 'demo', env: 'prod'));
    expect(state.error, isNull);
    final session = await store.load();
    expect(session?.gameId, 'demo');
    expect(session?.env, 'prod');
    expect(adapter.lastRequest?.path, '/api/v1/profile/scope');
  });

  test('select：PUT 失败（409）→ error 态且本地 scope 不变', () async {
    adapter.handler = (options, _) =>
        jsonResponse(409, {'error': 'conflict', 'message': '无权访问该环境'});
    final container = makeContainer(
      session: const SessionData(
        token: 'jwt',
        gameId: 'demo',
        env: 'dev',
        serverUrl: 'http://gm.test',
      ),
    );

    await container
        .read(scopeControllerProvider.notifier)
        .select('demo', 'prod');

    final state = container.read(scopeControllerProvider);
    expect(state.error, '无权访问该环境');
    expect(state.selected, isNull);
    expect((await store.load())?.env, 'dev');
  });
}
