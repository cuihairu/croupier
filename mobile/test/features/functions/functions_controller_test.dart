import 'dart:async';

import 'package:croupier_mobile/app/providers.dart';
import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/functions/functions_controller.dart';
import 'package:croupier_mobile/features/scope/scope_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

/// 等待目录刷新落定（loading=false）——测试内轮询上限 500ms。
Future<FunctionsState> waitCatalogLoaded(ProviderContainer container) async {
  for (var i = 0; i < 500; i++) {
    final s = container.read(functionsControllerProvider);
    if (!s.loading) return s;
    await Future<void>.delayed(const Duration(milliseconds: 1));
  }
  fail('catalog refresh did not settle');
}

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

  Future<void> login() => store.save(
    const SessionData(
      token: 'jwt',
      gameId: 'demo',
      env: 'prod',
      serverUrl: 'http://gm.test',
    ),
  );

  final descriptors = {
    'functions': [
      {
        'id': 'player.kick',
        'summary': {'zh-CN': '踢人'},
        'risk': 'high',
        'approval': {'required': true},
        'execution': 'sync',
      },
      {
        'id': 'mail.batch',
        'summary': {'zh-CN': '批量邮件'},
        'execution': 'task',
      },
    ],
  };

  test('refresh 加载目录 + 搜索过滤', () async {
    await login();
    adapter.handler = (options, _) => jsonResponse(200, descriptors);
    final notifier = container.read(functionsControllerProvider.notifier);
    await notifier.refresh();
    var state = container.read(functionsControllerProvider);
    expect(state.specs, hasLength(2));
    expect(state.filtered, hasLength(2));

    notifier.setQuery('kick');
    state = container.read(functionsControllerProvider);
    expect(state.filtered.single.id, 'player.kick');

    notifier.setQuery('踢人');
    state = container.read(functionsControllerProvider);
    expect(state.filtered.single.id, 'player.kick');
  });

  test('未登录置错误面', () async {
    final notifier = container.read(functionsControllerProvider.notifier);
    await notifier.refresh();
    expect(container.read(functionsControllerProvider).error, '未登录或缺少服务器地址');
  });

  test('refresh：切换作用域清空旧目录并按新作用域重查（AUD-063）', () async {
    await login();
    final scopeSeen = <String>[];
    adapter.handler = (options, _) {
      if (options.uri.path == '/api/v1/profile/scope') {
        return jsonResponse(200, {'ok': true});
      }
      scopeSeen.add('${options.headers['X-Game-ID']}/${options.headers['X-Env']}');
      return options.headers['X-Game-ID'] == 'demo'
          ? jsonResponse(200, {
            'functions': [
              {'id': 'demo.only'},
            ],
          })
          : jsonResponse(200, {
            'functions': [
              {'id': 'beta.only'},
            ],
          });
    };
    final notifier = container.read(functionsControllerProvider.notifier);
    await notifier.refresh();
    expect(
      container.read(functionsControllerProvider).specs.single.id,
      'demo.only',
    );

    // 切换作用域 → 监听器触发清目录 + 新查询
    await container
        .read(scopeControllerProvider.notifier)
        .select('beta', 'prod');

    var state = container.read(functionsControllerProvider);
    expect(state.specs, isEmpty, reason: '切换即清空旧目录');
    expect(state.loading, isTrue, reason: '新查询在途');

    state = await waitCatalogLoaded(container);
    expect(state.specs.single.id, 'beta.only', reason: '新目录来自新作用域');
    expect(state.loading, isFalse);
    expect(state.error, isNull);
    expect(
      scopeSeen,
      ['demo/prod', 'beta/prod'],
      reason: '重查请求头使用切换后的作用域',
    );
  });

  test('refresh：A→B 后旧请求迟到成功不回灌旧目录（AUD-063）', () async {
    await login();
    final oldAck = Completer<void>();
    var oldSeen = false;
    adapter.handler = (options, _) {
      if (options.uri.path == '/api/v1/profile/scope') {
        return jsonResponse(200, {'ok': true});
      }
      if (options.headers['X-Game-ID'] == 'demo') {
        oldSeen = true;
        return oldAck.future.then(
          (_) => jsonResponse(200, {
            'functions': [
              {'id': 'demo.stale'},
            ],
          }),
        );
      }
      return jsonResponse(200, {
        'functions': [
          {'id': 'beta.only'},
        ],
      });
    };
    final notifier = container.read(functionsControllerProvider.notifier);
    final oldRefresh = notifier.refresh(); // A 请求在途（被门控挂起）
    while (!oldSeen) {
      await Future<void>.delayed(const Duration(milliseconds: 1));
    }

    // 旧请求仍在途时切换作用域：不被阻塞，新查询照发
    await container
        .read(scopeControllerProvider.notifier)
        .select('beta', 'prod');
    final state = await waitCatalogLoaded(container);
    expect(state.specs.single.id, 'beta.only');
    expect(state.loading, isFalse);
    expect(state.error, isNull);

    oldAck.complete(); // 旧 A 请求此刻才迟到完成
    await oldRefresh;
    final after = container.read(functionsControllerProvider);
    expect(after.specs.single.id, 'beta.only', reason: '迟到旧目录不回灌');
    expect(after.loading, isFalse);
    expect(after.error, isNull);
  });

  test('refresh：A→B 后旧请求迟到失败不污染新作用域错误面（AUD-063）', () async {
    await login();
    final oldAck = Completer<void>();
    var oldSeen = false;
    adapter.handler = (options, _) {
      if (options.uri.path == '/api/v1/profile/scope') {
        return jsonResponse(200, {'ok': true});
      }
      if (options.headers['X-Game-ID'] == 'demo') {
        oldSeen = true;
        return oldAck.future.then(
          (_) =>
              jsonResponse(500, {'error': 'internal', 'message': '旧请求失败'}),
        );
      }
      return jsonResponse(200, {
        'functions': [
          {'id': 'beta.only'},
        ],
      });
    };
    final notifier = container.read(functionsControllerProvider.notifier);
    final oldRefresh = notifier.refresh();
    while (!oldSeen) {
      await Future<void>.delayed(const Duration(milliseconds: 1));
    }

    await container
        .read(scopeControllerProvider.notifier)
        .select('beta', 'prod');
    final state = await waitCatalogLoaded(container);
    expect(state.error, isNull);
    expect(state.specs.single.id, 'beta.only');

    oldAck.complete();
    await oldRefresh;
    final after = container.read(functionsControllerProvider);
    expect(after.error, isNull, reason: '旧请求错误不写入新作用域状态');
    expect(after.loading, isFalse);
    expect(after.specs.single.id, 'beta.only');
  });

  test('invoke 同步：置 invokeResult，非任务不建 task', () async {
    await login();
    adapter.handler = (options, _) => jsonResponse(200, {'result': 1});
    final notifier = container.read(functionsControllerProvider.notifier);
    await notifier.invoke(
      functionId: 'player.kick',
      payload: const {'playerId': 1},
    );
    final state = container.read(functionsControllerProvider);
    expect(state.submitting, isFalse);
    expect(state.invokeResult?.isTask, isFalse);
    expect(state.task, isNull);
    expect(
      adapter.lastRequest?.uri.path,
      '/api/v1/functions/player.kick/invoke',
    );
  });

  test('invoke 异步：taskId → 建 task 并立即拉一次详情', () async {
    await login();
    var detailCalls = 0;
    adapter.handler = (options, _) {
      if (options.uri.path == '/api/v1/functions/mail.batch/invoke') {
        return jsonResponse(200, {'taskId': 'task-1'});
      }
      detailCalls += 1;
      return jsonResponse(200, {
        'id': 'task-1',
        'status': 'running',
        'progress': 10,
      });
    };
    final notifier = container.read(functionsControllerProvider.notifier);
    await notifier.invoke(
      functionId: 'mail.batch',
      payload: const {},
      mode: 'async',
    );
    final state = container.read(functionsControllerProvider);
    expect(state.invokeResult?.isTask, isTrue);
    expect(state.task?.id, 'task-1');
    expect(state.task?.isRunning, isTrue);
    expect(detailCalls, 1, reason: 'invoke 后立即拉一次详情');
    notifier.clearResult();
  });

  test('invoke 失败置 invokeError', () async {
    await login();
    adapter.handler = (options, _) =>
        jsonResponse(403, {'error': 'forbidden', 'message': '无权限'});
    final notifier = container.read(functionsControllerProvider.notifier);
    await notifier.invoke(functionId: 'player.kick', payload: const {});
    expect(container.read(functionsControllerProvider).invokeError, '无权限');
  });

  test('refreshTask 终态停止轮询（done）', () async {
    await login();
    var running = true;
    adapter.handler = (options, _) {
      if (options.uri.path.endsWith('/invoke')) {
        return jsonResponse(200, {'taskId': 'task-2'});
      }
      return jsonResponse(200, {
        'id': 'task-2',
        'status': running ? 'running' : 'success',
        'progress': running ? 50 : 100,
        'result': running ? null : {'ok': true},
      });
    };
    final notifier = container.read(functionsControllerProvider.notifier);
    await notifier.invoke(
      functionId: 'mail.batch',
      payload: const {},
      mode: 'async',
    );
    expect(container.read(functionsControllerProvider).task?.isRunning, isTrue);

    running = false;
    await notifier.refreshTask();
    final task = container.read(functionsControllerProvider).task;
    expect(task?.isSuccess, isTrue);
    expect(task?.isDone, isTrue);
  });

  test('clearResult 清任务', () async {
    await login();
    adapter.handler = (options, _) {
      if (options.uri.path.endsWith('/invoke')) {
        return jsonResponse(200, {'taskId': 'task-3'});
      }
      return jsonResponse(200, {'id': 'task-3', 'status': 'running'});
    };
    final notifier = container.read(functionsControllerProvider.notifier);
    await notifier.invoke(
      functionId: 'mail.batch',
      payload: const {},
      mode: 'async',
    );
    expect(container.read(functionsControllerProvider).task, isNotNull);
    notifier.clearResult();
    expect(container.read(functionsControllerProvider).task, isNull);
  });
}
