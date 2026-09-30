import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/api/api_error.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/audit/audit_service.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late AuditService service;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    final client = ApiClient.create(
      baseUrl: 'http://gm.test',
      sessionStore: store,
    );
    client.dio.httpClientAdapter = adapter;
    service = AuditService(client: client);
  });

  test('list 解析 {items,total}，缺 id 条目跳过；metadata 透传', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'items': [
        {
          'id': 'au-1',
          'action': 'invoke',
          'userId': 'admin',
          'gameId': 'demo',
          'env': 'prod',
          'target': 'player.kick',
          'result': 'success',
          'traceId': 'tr-1',
          'metadata': {
            'args': {'pid': 7},
          },
          'createdAt': '2026-09-30 10:00:00',
        },
        {'action': '缺 id'},
      ],
      'total': 1,
      'page': 1,
      'pageSize': 20,
    });
    final page = await service.list();
    expect(page.items, hasLength(1));
    final item = page.items.single;
    expect(item.action, 'invoke');
    expect(item.traceId, 'tr-1');
    expect(item.metadata?['args'], isMap);
    expect(page.total, 1);
  });

  test('筛选参数：kinds 逗号串、空值不带、gameId 走 query（非 scoped 头）', () async {
    await store.save(
      const SessionData(token: 'jwt', gameId: 'demo', env: 'prod'),
    );
    adapter.handler = (options, _) =>
        jsonResponse(200, {'items': [], 'total': 0});
    await service.list(
      actor: 'admin',
      kinds: ['invoke', 'approval_approve'],
      env: 'prod',
      ip: '10.0.0.1',
      gameId: 'demo',
    );
    final query = adapter.lastRequest?.uri.queryParameters;
    expect(query?['actor'], 'admin');
    expect(query?['kinds'], 'invoke,approval_approve');
    expect(query?['env'], 'prod');
    expect(query?['ip'], '10.0.0.1');
    expect(query?['gameId'], 'demo');
    expect(query?.containsKey('start'), isFalse);
    // audit 非 scoped：即便会话有 scope 也不注入 X-Game-ID 头。
    expect(adapter.lastRequest?.headers.containsKey('X-Game-ID'), isFalse);
  });

  test('时间区间转 RFC3339（含时区 offset）', () async {
    adapter.handler = (options, _) =>
        jsonResponse(200, {'items': [], 'total': 0});
    await service.list(
      start: DateTime(2026, 9, 1, 0, 0, 0),
      end: DateTime(2026, 9, 30, 23, 59, 59),
    );
    final query = adapter.lastRequest?.uri.queryParameters;
    // 本机时区不论 ±，必须带 offset 才能过服务端 time.Parse(RFC3339)。
    expect(
      query?['start'],
      matches(RegExp(r'^2026-09-01T00:00:00[+-]\d{2}:\d{2}$')),
    );
    expect(
      query?['end'],
      matches(RegExp(r'^2026-09-30T23:59:59[+-]\d{2}:\d{2}$')),
    );
  });

  test('非契约体 fail-fast + HTTP 错误归一', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'foo': 1});
    await expectLater(service.list(), throwsStateError);

    adapter.handler = (options, _) =>
        jsonResponse(500, {'error': 'internal', 'message': 'boom'});
    try {
      await service.list();
      fail('should throw');
    } on ApiError catch (e) {
      expect(e.status, 500);
    }
  });
}
