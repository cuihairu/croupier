import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/api/api_error.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/alerts/alert_service.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late AlertService service;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    final client = ApiClient.create(
      baseUrl: 'http://gm.test',
      sessionStore: store,
    );
    client.dio.httpClientAdapter = adapter;
    service = AlertService(client: client);
  });

  test('list 解析 {items,total}，缺 id 条目跳过；筛选参数透传', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'items': [
        {
          'id': 'al-1',
          'type': 'agent_offline',
          'level': 'critical',
          'message': '节点失联',
          'source': 'registry',
          'status': 'firing',
          'createdAt': '2026-09-30 10:00:00',
        },
        {'message': '缺 id'},
      ],
      'total': 1,
      'page': 1,
      'pageSize': 20,
    });

    final page = await service.list(level: 'critical', status: 'firing');
    expect(page.items, hasLength(1));
    expect(page.items.single.level, 'critical');
    expect(page.total, 1);

    final query = adapter.lastRequest?.uri.queryParameters;
    expect(query?['level'], 'critical');
    expect(query?['status'], 'firing');
    expect(query?['pageSize'], '20');
  });

  test('筛选为空时不带 level/status 参数', () async {
    adapter.handler = (options, _) =>
        jsonResponse(200, {'items': [], 'total': 0});
    await service.list();
    final query = adapter.lastRequest?.uri.queryParameters;
    expect(query?.containsKey('level'), isFalse);
    expect(query?.containsKey('status'), isFalse);
  });

  test('silence 提交 {duration,reason}', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'message': '操作成功'});
    await service.silence('al-1', durationMinutes: 120, reason: '维护窗口');
    expect(adapter.lastRequest?.uri.path, '/api/v1/alerts/al-1/silence');
    expect(adapter.lastBody, contains('"duration":120'));
    expect(adapter.lastBody, contains('"reason":"维护窗口"'));
  });

  test('silences 解析 {items:[...]}', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'items': [
        {
          'id': 'sl-1',
          'alertType': 'agent_offline',
          'startAt': '2026-09-30 10:00',
          'endAt': '2026-09-30 12:00',
          'createdBy': 'admin',
        },
      ],
    });
    final rules = await service.silences();
    expect(rules, hasLength(1));
    expect(rules.single.alertType, 'agent_offline');
    expect(rules.single.createdBy, 'admin');
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
      expect(e.message, 'boom');
    }
  });
}
