import 'dart:async';

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

  test('switchStatus 后旧页 append 迟到不并入新筛选（AUD-064）', () async {
    final oldAck = Completer<void>();
    var oldSeen = false;
    adapter.handler = (options, _) {
      if (options.uri.path != '/api/v1/approvals/') {
        return jsonResponse(200, {'ok': true});
      }
      final status = options.uri.queryParameters['status'];
      final page = options.uri.queryParameters['page'];
      if (status == 'pending' && page == '2') {
        oldSeen = true;
        return oldAck.future.then(
          (_) => jsonResponse(200, {
            'approvals': [_itemJson('stale-p2')],
            'total': 25,
          }),
        );
      }
      if (status == 'approved') {
        return jsonResponse(200, {
          'approvals': [_itemJson('approved-1')],
          'total': 1,
        });
      }
      return jsonResponse(200, {
        'approvals': [_itemJson('pending-1')],
        'total': 25,
      });
    };
    final container = makeContainer();
    final notifier = container.read(approvalsControllerProvider.notifier);

    await notifier.refresh(); // pending 第 1 页
    final oldAppend = notifier.loadMore(); // 旧页 append 在途（门控挂起）
    while (!oldSeen) {
      await Future<void>.delayed(const Duration(milliseconds: 1));
    }

    // 旧 append 仍挂起时切筛选：新首页立即返回
    await notifier.switchStatus('approved');
    var state = container.read(approvalsControllerProvider);
    expect(state.items.single.id, 'approved-1');
    expect(state.loading, isFalse);

    oldAck.complete(); // 旧 append 此刻才迟到返回
    await oldAppend;
    state = container.read(approvalsControllerProvider);
    expect(state.items.single.id, 'approved-1', reason: '旧页不并入新筛选');
    expect(state.loadingMore, isFalse);
    expect(state.error, isNull);
  });

  test('switchStatus 后旧筛选首页迟到不覆盖新列表（AUD-064）', () async {
    final oldAck = Completer<void>();
    var oldSeen = false;
    adapter.handler = (options, _) {
      if (options.uri.path != '/api/v1/approvals/') {
        return jsonResponse(200, {'ok': true});
      }
      final status = options.uri.queryParameters['status'];
      if (status == 'pending') {
        oldSeen = true;
        return oldAck.future.then(
          (_) => jsonResponse(200, {
            'approvals': [_itemJson('stale-pending')],
            'total': 25,
          }),
        );
      }
      return jsonResponse(200, {
        'approvals': [_itemJson('approved-1')],
        'total': 1,
      });
    };
    final container = makeContainer();
    final notifier = container.read(approvalsControllerProvider.notifier);

    final oldFirst = notifier.refresh(); // pending 首页在途（门控挂起）
    while (!oldSeen) {
      await Future<void>.delayed(const Duration(milliseconds: 1));
    }

    await notifier.switchStatus('approved');
    var state = container.read(approvalsControllerProvider);
    expect(state.items.single.id, 'approved-1');
    expect(state.status, 'approved');

    oldAck.complete();
    await oldFirst;
    state = container.read(approvalsControllerProvider);
    expect(state.items.single.id, 'approved-1', reason: '旧筛选首页迟到不覆盖');
    expect(state.status, 'approved');
    expect(state.loading, isFalse);
    expect(state.error, isNull);
  });

  test('scope 切换后旧作用域 append 迟到不并入（AUD-064）', () async {
    final oldAck = Completer<void>();
    var oldSeen = false;
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
            {
              'gameId': 'beta',
              'gameName': 'Beta',
              'envs': ['prod'],
            },
          ],
        });
      }
      if (path == '/api/v1/profile/scope') {
        return jsonResponse(200, {'ok': true});
      }
      if (path == '/api/v1/functions/descriptors') {
        return jsonResponse(200, {'functions': <Object>[]});
      }
      if (path == '/api/v1/approvals/') {
        if (options.uri.queryParameters['page'] == '2') {
          oldSeen = true;
          return oldAck.future.then(
            (_) => jsonResponse(200, {
              'approvals': [_itemJson('stale-demo-p2')],
              'total': 25,
            }),
          );
        }
        final game = options.headers['X-Game-ID'];
        return jsonResponse(200, {
          'approvals': [_itemJson(game == 'beta' ? 'beta-1' : 'demo-1')],
          'total': game == 'beta' ? 1 : 25,
        });
      }
      return jsonResponse(404, {'error': 'not_found'});
    };
    final container = makeContainer();
    final notifier = container.read(approvalsControllerProvider.notifier);

    await notifier.refresh(); // demo 第 1 页
    final oldAppend = notifier.loadMore(); // demo 第 2 页在途（门控挂起）
    while (!oldSeen) {
      await Future<void>.delayed(const Duration(milliseconds: 1));
    }

    // 旧 append 仍挂起时切换作用域：联动 refresh 新首页立即返回
    await container
        .read(scopeControllerProvider.notifier)
        .select('beta', 'prod');
    // 联动 refresh 是异步链：轮询等待 beta 首页真正落地
    for (var i = 0; i < 200; i++) {
      final s = container.read(approvalsControllerProvider);
      if (s.items.isNotEmpty && s.items.first.id == 'beta-1') break;
      await Future<void>.delayed(const Duration(milliseconds: 1));
    }
    var state = container.read(approvalsControllerProvider);
    expect(state.items.single.id, 'beta-1', reason: '新作用域首页已落地');

    oldAck.complete();
    await oldAppend;
    state = container.read(approvalsControllerProvider);
    expect(state.items.single.id, 'beta-1', reason: '旧作用域 append 不并入');
    expect(state.loadingMore, isFalse);
    expect(state.error, isNull);
  });

  test('scope 切换后旧作用域描述符迟到不回灌（AUD-064）', () async {
    final oldAck = Completer<void>();
    var oldSeen = false;
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
            {
              'gameId': 'beta',
              'gameName': 'Beta',
              'envs': ['prod'],
            },
          ],
        });
      }
      if (path == '/api/v1/profile/scope') {
        return jsonResponse(200, {'ok': true});
      }
      if (path == '/api/v1/approvals/') {
        return jsonResponse(200, {'approvals': <Object>[], 'total': 0});
      }
      if (path == '/api/v1/functions/descriptors') {
        if (options.headers['X-Game-ID'] == 'demo') {
          oldSeen = true;
          return oldAck.future.then(
            (_) => jsonResponse(200, {
              'functions': [
                {'id': 'demo.func', 'risk': 'danger'},
              ],
            }),
          );
        }
        return jsonResponse(200, {
          'functions': [
            {'id': 'beta.func', 'risk': 'safe'},
          ],
        });
      }
      return jsonResponse(404, {'error': 'not_found'});
    };
    final container = makeContainer();
    final notifier = container.read(approvalsControllerProvider.notifier);

    final oldDesc = notifier.loadDescriptors(); // demo 描述符在途（门控挂起）
    while (!oldSeen) {
      await Future<void>.delayed(const Duration(milliseconds: 1));
    }

    // 旧描述符仍挂起时切换作用域：联动 reloadDescriptors 按新 scope 重建
    await container
        .read(scopeControllerProvider.notifier)
        .select('beta', 'prod');
    for (var i = 0;
        i < 200 &&
            container.read(approvalsControllerProvider).descs.isEmpty;
        i++) {
      await Future<void>.delayed(const Duration(milliseconds: 1));
    }
    var descs = container.read(approvalsControllerProvider).descs;
    expect(descs.keys, ['beta.func'], reason: '新作用域标签已落地');

    oldAck.complete();
    await oldDesc;
    descs = container.read(approvalsControllerProvider).descs;
    expect(descs.keys, ['beta.func'], reason: '旧作用域描述符迟到不回灌');
  });

  test(
    'loadDescriptors 解析 {functions} 并建 functionId→risk/approval 索引',
    () async {
      adapter.handler = (options, _) => jsonResponse(200, {
        'functions': [
          {
            'id': 'player.kick',
            'risk': 'danger',
            'approval': {'required': true, 'policyKey': 'two.person'},
          },
          {'id': 'player.info', 'risk': 'safe'},
        ],
      });
      final container = makeContainer();

      await container
          .read(approvalsControllerProvider.notifier)
          .loadDescriptors();

      final descs = container.read(approvalsControllerProvider).descs;
      expect(descs.keys.toSet(), {'player.kick', 'player.info'});
      expect(descs['player.kick']!.isHighRisk, isTrue);
      expect(descs['player.kick']!.approvalRequired, isTrue);
      expect(descs['player.kick']!.approvalPolicyKey, 'two.person');
      expect(descs['player.info']!.isHighRisk, isFalse);
    },
  );

  test('loadDescriptors 失败静默（标签缺失不阻塞列表）', () async {
    adapter.handler = (options, _) =>
        jsonResponse(500, {'error': 'internal', 'message': 'boom'});
    final container = makeContainer();

    await container
        .read(approvalsControllerProvider.notifier)
        .loadDescriptors();

    expect(container.read(approvalsControllerProvider).descs, isEmpty);
    expect(container.read(approvalsControllerProvider).error, isNull);
  });

  test('loadDescriptors 非法形态静默（StateError 不外泄）', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'data': <Object>[]});
    final container = makeContainer();

    await container
        .read(approvalsControllerProvider.notifier)
        .loadDescriptors();

    expect(container.read(approvalsControllerProvider).descs, isEmpty);
  });

  test('error 态：服务端 500 → error 透传', () async {
    adapter.handler = (options, _) =>
        jsonResponse(500, {'error': 'internal', 'message': '服务不可用'});
    final container = makeContainer();

    await container.read(approvalsControllerProvider.notifier).refresh();

    expect(container.read(approvalsControllerProvider).error, '服务不可用');
  });
}
