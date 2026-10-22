import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/scope/scope_service.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late ScopeService service;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    final client = ApiClient.create(
      baseUrl: 'http://gm.test',
      sessionStore: store,
    );
    client.dio.httpClientAdapter = adapter;
    service = ScopeService(client: client, sessionStore: store);
  });

  test('fetchGames 解析 games 数组（gameId/gameName/envs）', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'games': [
        {
          'gameId': 'demo',
          'gameName': '演示游戏',
          'color': '#123456',
          'envs': ['dev', 'prod'],
        },
      ],
    });

    final games = await service.fetchGames();

    expect(games, hasLength(1));
    expect(games.first.gameId, 'demo');
    expect(games.first.gameName, '演示游戏');
    expect(games.first.envs, ['dev', 'prod']);
  });

  test('fetchGames 过滤缺 gameId 项与非法 env 项', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'games': [
        {'gameName': '无 id'},
        {
          'gameId': 'ok',
          'envs': ['dev', 42, null],
        },
      ],
    });

    final games = await service.fetchGames();

    expect(games, hasLength(1));
    expect(games.first.gameId, 'ok');
    expect(games.first.envs, ['dev']);
  });

  test('games 缺失或非 List → StateError（不静默当空列表）', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'foo': 1});
    await expectLater(service.fetchGames(), throwsStateError);

    adapter.handler = (options, _) => jsonResponse(200, {'games': 'oops'});
    await expectLater(service.fetchGames(), throwsStateError);
  });

  test('saveScope PUT body {gameId, env}', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'ok': true});

    await service.saveScope('demo', 'prod');

    expect(adapter.lastRequest?.path, ScopeService.scopePath);
    expect(adapter.lastBody, contains('"gameId":"demo"'));
    expect(adapter.lastBody, contains('"env":"prod"'));
  });
}
