import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/api/api_error.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/monitoring/performance_service.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late PerformanceService service;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    final client = ApiClient.create(
      baseUrl: 'http://gm.test',
      sessionStore: store,
    );
    client.dio.httpClientAdapter = adapter;
    service = PerformanceService(client: client);
  });

  test('fetch 解析 runtime/host/overload 三块', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'settings': {'perf.gcPauseMs': 100},
      'runtime': {
        'goroutines': 42,
        'heapAllocBytes': 1536,
        'numGC': 7,
        'gcPauseMs': 12.5,
        'uptimeSeconds': 90061,
      },
      'host': {
        'cpuPercent': 12.5,
        'memoryUsedPct': 60,
        'memoryTotalBytes': 8589934592,
        'memoryUsedBytes': 5153960755,
        'diskUsedPct': 80,
        'diskTotalBytes': 100,
        'diskUsedBytes': 80,
      },
      'overload': {'cpu': false, 'memory': false, 'disk': true},
    });

    final snapshot = await service.fetch();
    expect(snapshot.runtime.goroutines, 42);
    expect(snapshot.runtime.heapAllocBytes, 1536);
    expect(snapshot.runtime.gcPauseMs, 12.5);
    expect(snapshot.host.cpuPercent, 12.5);
    expect(snapshot.host.memoryUsedPct, 60);
    expect(snapshot.overload.disk, isTrue);
    expect(snapshot.overload.cpu, isFalse);
  });

  test('fetch 请求正确路径且带 scope 头', () async {
    await store.save(
      const SessionData(token: 'jwt', gameId: 'demo', env: 'prod'),
    );
    adapter.handler = (options, _) =>
        jsonResponse(200, {'runtime': {}, 'host': {}, 'overload': {}});
    await service.fetch();
    expect(adapter.lastRequest?.uri.path, '/api/v1/ops/performance');
    expect(adapter.lastRequest?.headers['X-Game-ID'], 'demo');
    expect(adapter.lastRequest?.headers['X-Env'], 'prod');
  });

  test('块内单字段缺失按零值容错', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'runtime': {'goroutines': 3},
      'host': {'cpuPercent': 1.5},
      'overload': {},
    });
    final snapshot = await service.fetch();
    expect(snapshot.runtime.heapAllocBytes, 0);
    expect(snapshot.host.diskUsedPct, 0);
    expect(snapshot.overload.memory, isFalse);
  });

  test('非契约体 fail-fast（runtime 缺块）', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'foo': 'bar'});
    await expectLater(service.fetch(), throwsStateError);
  });

  test('SPA HTML 兜底不渲染为快照（归一 ApiError status 0）', () async {
    adapter.handler = (options, _) => ResponseBody.fromString(
      '<html>login</html>',
      200,
      headers: {
        'content-type': <String>['text/html'],
      },
    );
    // 200 + 非 JSON 体：dio 管线归一为无响应 ApiError(0)，
    // 断言语义 = HTML 永远不会当快照渲染（形态校验兜底）。
    try {
      await service.fetch();
      fail('should throw');
    } on ApiError catch (e) {
      expect(e.status, 0);
    }
  });

  test('HTTP 错误归一 ApiError', () async {
    adapter.handler = (options, _) =>
        jsonResponse(500, {'error': 'internal', 'message': 'boom'});
    try {
      await service.fetch();
      fail('should throw');
    } on ApiError catch (e) {
      expect(e.status, 500);
      expect(e.message, 'boom');
    }
  });
}
