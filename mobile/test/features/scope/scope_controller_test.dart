import 'dart:async';
import 'dart:convert';

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

  test('load：旧作用域失效 → 预选同步持久化会话（AUD-062）', () async {
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

    // 持久化会话与界面预选一致：后续请求头不再使用已失效的旧 scope
    final session = await store.load();
    expect(session?.gameId, 'demo');
    expect(session?.env, 'dev');
  });

  test('load：旧作用域失效后的真实请求头使用预选 scope（AUD-062）', () async {
    var gamesCalled = false;
    adapter.handler = (options, _) {
      if (!gamesCalled) {
        gamesCalled = true;
        return jsonResponse(200, gamesPayload);
      }
      return jsonResponse(200, {'ok': true});
    };
    final container = makeContainer(
      session: const SessionData(
        token: 'jwt',
        gameId: 'gone',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );
    await container.read(scopeControllerProvider.notifier).load();

    // 用请求替身核对：load 落默认后，scoped 请求头 = 界面预选 (demo/dev)
    await container
        .read(apiClientFactoryProvider)('http://gm.test')
        .get<Map<String, Object?>>('/api/v1/approvals/');
    expect(adapter.lastRequest?.headers['X-Game-ID'], 'demo');
    expect(adapter.lastRequest?.headers['X-Env'], 'dev');
  });

  test('load：lastGameId 仍可选 → 会话保持不动（AUD-062 不误写）', () async {
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

    final session = await store.load();
    expect(session?.gameId, 'beta');
    expect(session?.env, 'prod');
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

  test('select：切换响应乱序，旧切换迟到不覆盖新切换（AUD-066）', () async {
    final oldAck = Completer<void>();
    adapter.handler = (options, raw) {
      final gameId =
          jsonDecode(utf8.decode(raw ?? const <int>[]))['gameId'] as String?;
      if (gameId == 'demo') {
        return oldAck.future.then((_) => jsonResponse(200, {'ok': true}));
      }
      return jsonResponse(200, {'ok': true});
    };
    final container = makeContainer(
      session: const SessionData(
        token: 'jwt',
        gameId: 'demo',
        env: 'dev',
        serverUrl: 'http://gm.test',
      ),
    );
    final notifier = container.read(scopeControllerProvider.notifier);

    final oldSwitch = notifier.select('demo', 'prod'); // 旧切换在途（门控）
    await notifier.select('beta', 'prod'); // 新切换立即完成并写回

    oldAck.complete(); // 旧切换此刻才迟到完成
    await oldSwitch;

    final state = container.read(scopeControllerProvider);
    expect(state.selected, const ScopeSelection(gameId: 'beta', env: 'prod'),
        reason: '最终 selected 为最新切换');
    final session = await store.load();
    expect(session?.gameId, 'beta', reason: '最终会话 scope 为最新切换');
    expect(session?.env, 'prod');
  });

  test('select：登出清会话后迟到完成不复活会话（AUD-066）', () async {
    final oldAck = Completer<void>();
    adapter.handler = (options, _) =>
        oldAck.future.then((_) => jsonResponse(200, {'ok': true}));
    final container = makeContainer(
      session: const SessionData(
        token: 'jwt',
        gameId: 'demo',
        env: 'dev',
        serverUrl: 'http://gm.test',
      ),
    );
    final notifier = container.read(scopeControllerProvider.notifier);

    final oldSwitch = notifier.select('demo', 'prod'); // 在途（门控）
    await store.clear(); // 登出：会话清空（仅保留 serverUrl）
    oldAck.complete();
    await oldSwitch;

    final session = await store.load();
    expect(session?.token, '', reason: '迟到写回不得复活 token');
    expect(session?.gameId, '', reason: '迟到写回不得写入旧 scope');
    expect(
      container.read(scopeControllerProvider).selected,
      isNull,
      reason: '会话已失效不更新界面预选',
    );
  });

  test('select：换服新会话期间迟到完成不覆盖新会话（AUD-066）', () async {
    final oldAck = Completer<void>();
    adapter.handler = (options, _) =>
        oldAck.future.then((_) => jsonResponse(200, {'ok': true}));
    final container = makeContainer(
      session: const SessionData(
        token: 'jwt',
        gameId: 'demo',
        env: 'dev',
        serverUrl: 'http://gm.test',
      ),
    );
    final notifier = container.read(scopeControllerProvider.notifier);

    final oldSwitch = notifier.select('demo', 'prod'); // 在途（门控）
    // 切换服务器：新会话（新地址 + 新 token + 各自 scope）落存储
    await store.save(const SessionData(
      token: 'jwt2',
      gameId: 'other-game',
      env: 'prod',
      serverUrl: 'http://other.test',
    ));
    oldAck.complete();
    await oldSwitch;

    final session = await store.load();
    expect(session?.serverUrl, 'http://other.test', reason: '新会话地址不动');
    expect(session?.token, 'jwt2', reason: '新会话 token 不被旧快照覆盖');
    expect(session?.gameId, 'other-game', reason: '新会话 scope 不被旧切换覆盖');
    expect(session?.env, 'prod');
    expect(
      container.read(scopeControllerProvider).selected,
      isNull,
      reason: '跨会话迟到完成不更新界面预选',
    );
  });
}
