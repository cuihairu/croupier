import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/api/api_error.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/functions/function_invoke_service.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late FunctionInvokeService service;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    final client = ApiClient.create(
      baseUrl: 'http://gm.test',
      sessionStore: store,
    );
    client.dio.httpClientAdapter = adapter;
    service = FunctionInvokeService(client: client);
  });

  test('invoke 同步：路径/载荷形态 + invoke scoped 头', () async {
    await store.save(
      const SessionData(
        token: 'jwt',
        gameId: 'demo',
        env: 'prod',
        serverUrl: 'http://gm.test',
      ),
    );
    adapter.handler = (options, _) => jsonResponse(200, {
      'result': {'ok': true},
      'traceId': 't1',
    });
    final result = await service.invoke(
      functionId: 'player.kick',
      payload: {'playerId': 42},
    );
    expect(
      adapter.lastRequest?.uri.path,
      '/api/v1/functions/player.kick/invoke',
    );
    expect(adapter.lastRequest?.method, 'POST');
    expect(adapter.lastRequest?.headers['X-Game-ID'], 'demo');
    final body = adapter.lastBody!;
    expect(body, contains('"playerId":42'));
    // 默认省略 route/mode，不污染 body。
    expect(body, isNot(contains('"route"')));
    expect(body, isNot(contains('"mode"')));
    expect(result.isTask, isFalse);
    expect(result.traceId, 't1');
  });

  test('invoke 只带非空高级字段（route/target/hashKey/mode）', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'taskId': 'task-9'});
    final result = await service.invoke(
      functionId: 'mail.send',
      payload: const {},
      mode: 'async',
      route: 'hash',
      hashKey: 'player:1',
      targetServiceId: 'agent-a',
    );
    final body = adapter.lastBody!;
    expect(body, contains('"mode":"async"'));
    expect(body, contains('"route":"hash"'));
    expect(body, contains('"hashKey":"player:1"'));
    expect(body, contains('"targetServiceId":"agent-a"'));
    expect(result.isTask, isTrue);
    expect(result.taskId, 'task-9');
  });

  test('invoke approvalRequired 解析', () async {
    adapter.handler = (options, _) =>
        jsonResponse(200, {'approvalRequired': true, 'approvalId': 'ap-1'});
    final result = await service.invoke(functionId: 'p.kick');
    expect(result.approvalRequired, isTrue);
    expect(result.approvalId, 'ap-1');
    expect(result.isTask, isFalse);
  });

  test('taskDetail 解析 + 终态判定', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'id': 'task-1',
      'functionId': 'mail.send',
      'status': 'running',
      'progress': 30,
      'message': '处理中',
    });
    final task = await service.taskDetail('task-1');
    expect(adapter.lastRequest?.uri.path, '/api/v1/tasks/task-1');
    expect(task.isRunning, isTrue);
    expect(task.isDone, isFalse);
    expect(task.progress, 30);

    adapter.handler = (options, _) =>
        jsonResponse(200, {'id': 'task-1', 'status': 'success', 'result': 1});
    final done = await service.taskDetail('task-1');
    expect(done.isSuccess, isTrue);
    expect(done.isDone, isTrue);
  });

  test('taskDetail 缺 id fail-fast / 404 归一 ApiError', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'foo': 1});
    await expectLater(service.taskDetail('x'), throwsStateError);

    adapter.handler = (options, _) =>
        jsonResponse(404, {'error': 'not_found', 'message': '任务不存在'});
    try {
      await service.taskDetail('missing');
      fail('should throw');
    } on ApiError catch (e) {
      expect(e.status, 404);
      expect(e.code, 'not_found');
      expect(e.message, '任务不存在');
    }
  });

  test('cancelTask 走 POST /tasks/:id/cancel', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'message': '操作成功'});
    await service.cancelTask('task-7');
    expect(adapter.lastRequest?.uri.path, '/api/v1/tasks/task-7/cancel');
    expect(adapter.lastRequest?.method, 'POST');
  });
}
