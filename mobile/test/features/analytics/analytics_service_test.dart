import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/analytics/analytics_service.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late AnalyticsService service;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    final client = ApiClient.create(
      baseUrl: 'http://gm.test',
      sessionStore: store,
    );
    client.dio.httpClientAdapter = adapter;
    service = AnalyticsService(client: client);
  });

  test('summary 解析 domains 卡组，坏形态条目跳过', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'gameId': 'demo',
      'env': 'prod',
      'generatedAt': '2026-11-11T00:00:00Z',
      'domains': [
        {
          'domain': 'overview',
          'status': 'ok',
          'data': {
            'metrics': {'dau': 42, 'mau': 7},
          },
        },
        {
          'domain': 'economy',
          'status': 'pending_data_source',
        },
        {
          'domain': 'payments',
          'status': 'unavailable',
          'error': 'payments db down',
        },
        {'domain': '缺 status'},
      ],
    });

    final summary = await service.summary(gameId: 'demo', env: 'prod');

    expect(summary.domains, hasLength(3));
    expect(summary.domains[0].domain, 'overview');
    expect(summary.domains[0].isOk, isTrue);
    expect(summary.domains[1].isPending, isTrue);
    expect(summary.domains[2].isUnavailable, isTrue);
    expect(summary.domains[2].error, 'payments db down');
    expect(adapter.lastRequest?.path, '/api/v1/analytics/facade/summary');
    expect(adapter.lastRequest?.queryParameters['gameId'], 'demo');
    expect(adapter.lastRequest?.queryParameters['env'], 'prod');
  });

  test('domains 非 List → StateError（不静默当空卡组）', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'domains': 'oops'});
    await expectLater(
      service.summary(gameId: 'demo'),
      throwsStateError,
    );
  });

  test('query POST body 携带 domain/filter/timeRange，返回 data', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'domain': 'overview',
      'data': {
        'metrics': {'dau': 42},
      },
    });

    final data = await service.query(
      domain: 'overview',
      gameId: 'demo',
      env: 'prod',
      startDate: '2026-11-01',
      endDate: '2026-11-11',
    );

    expect(adapter.lastRequest?.path, '/api/v1/analytics/facade/query');
    final body = adapter.lastBody!;
    expect(body, contains('"domain":"overview"'));
    expect(body, contains('"gameId":"demo"'));
    expect(body, contains('"startDate":"2026-11-01"'));
    expect((data as Map)['metrics']['dau'], 42);
  });

  test('query 响应缺 data 键 → StateError', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'domain': 'overview'});
    await expectLater(
      service.query(domain: 'overview', gameId: 'demo'),
      throwsStateError,
    );
  });

  test('分域请求注入 scope 头（analytics 是 scoped 端）', () async {
    await store.save(const SessionData(
      token: 'jwt',
      user: {'username': 'admin'},
      gameId: 'demo',
      env: 'prod',
      serverUrl: 'http://gm.test',
    ));
    adapter.handler = (options, _) => jsonResponse(200, {
      'gameId': 'demo',
      'env': 'prod',
      'domains': [
        {'domain': 'overview', 'status': 'ok'},
      ],
    });

    await service.summary(gameId: 'demo', env: 'prod');

    expect(adapter.lastRequest?.headers['X-Game-ID'], 'demo');
    expect(adapter.lastRequest?.headers['X-Env'], 'prod');
  });
}
