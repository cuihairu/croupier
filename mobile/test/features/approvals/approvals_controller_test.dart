import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/approvals/approvals_controller.dart';
import 'package:croupier_mobile/features/scope/scope_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

Map<String, Object?> _itemJson(String id) => {
  'id': id,
  'functionId': 'player.kick',
  'actor': 'op1',
  'state': 'pending',
  'createdAt': '2026-09-30 10:00',
};

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    store.save(
      const SessionData(
        token: 'jwt',
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );
  });

  ProviderContainer makeContainer() {
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
    return container;
  }

  test('refresh 拉第一页并写 items/total/page', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'approvals': [_itemJson('a1')],
      'total': 25,
    });
    final container = makeContainer();

    await container.read(approvalsControllerProvider.notifier).refresh();

    final state = container.read(approvalsControllerProvider);
    expect(state.items, hasLength(1));
    expect(state.items.first.id, 'a1');
    expect(state.total, 25);
    expect(state.page, 1);
    expect(state.loading, isFalse);
  });

  test('loadMore 追加第二页；无更多时不再请求', () async {
    var listCalls = 0;
    adapter.handler = (options, _) {
      if (options.uri.path != '/api/v1/approvals/') {
        return jsonResponse(200, {});
      }
      listCalls++;
      return jsonResponse(200, {
        'approvals': [_itemJson('a$listCalls')],
        'total': 2,
      });
    };
    final container = makeContainer();
    final notifier = container.read(approvalsControllerProvider.notifier);

    await notifier.refresh();
    await notifier.loadMore();

    expect(
      container
          .read(approvalsControllerProvider)
          .items
          .map((e) => e.id)
          .toList(),
      ['a1', 'a2'],
    );
    expect(container.read(approvalsControllerProvider).hasMore, isFalse);

    await notifier.loadMore();
    expect(listCalls, 2, reason: '无更多时不发请求');
  });

  test('switchStatus 换 status 重查第一页', () async {
    adapter.handler = (options, _) =>
        jsonResponse(200, {'approvals': <Object>[], 'total': 0});
    final container = makeContainer();
    final notifier = container.read(approvalsControllerProvider.notifier);

    await notifier.refresh();
    expect(adapter.lastRequest?.queryParameters['status'], 'pending');

    await notifier.switchStatus('approved');
    expect(adapter.lastRequest?.queryParameters['status'], 'approved');
    expect(container.read(approvalsControllerProvider).status, 'approved');
  });

  test('scope 切换联动：selected 变化自动 refresh（scoped 端点重查）', () async {
    var listCalls = 0;
    adapter.handler = (options, _) {
      final path = options.uri.path;
      if (path == '/api/v1/profile/games') {
        return jsonResponse(200, {
          'games': [
            {
              'gameId': 'demo',
              'gameName': '演示',
              'envs': ['dev', 'prod'],
            },
          ],
        });
      }
      if (path == '/api/v1/profile/scope') {
        return jsonResponse(200, {'ok': true});
      }
      if (path == '/api/v1/approvals/') {
        listCalls++;
        return jsonResponse(200, {'approvals': <Object>[], 'total': 0});
      }
      return jsonResponse(404, {'error': 'not_found'});
    };
    final container = makeContainer();
    await container.read(approvalsControllerProvider.notifier).refresh();
    expect(listCalls, 1);

    // scope 切换（select 全链：PUT + 会话更新 + selected 变化）→ 列表自动重查。
    // listen 回调触发的 refresh 是异步链，轮询等待完成。
    await container
        .read(scopeControllerProvider.notifier)
        .select('demo', 'prod');
    for (var i = 0; i < 50 && listCalls < 2; i++) {
      await Future<void>.delayed(const Duration(milliseconds: 5));
    }

    expect(listCalls, 2, reason: 'scope 切换应触发列表 refresh');
  });

  test('error 态：服务端 500 → error 透传', () async {
    adapter.handler = (options, _) =>
        jsonResponse(500, {'error': 'internal', 'message': '服务不可用'});
    final container = makeContainer();

    await container.read(approvalsControllerProvider.notifier).refresh();

    expect(container.read(approvalsControllerProvider).error, '服务不可用');
  });
}
