import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:croupier_mobile/features/approvals/approval_service.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late ApprovalService service;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    final client = ApiClient.create(
      baseUrl: 'http://gm.test',
      sessionStore: store,
    );
    client.dio.httpClientAdapter = adapter;
    service = ApprovalService(client: client);
  });

  test('list 解析 {approvals,total}，缺 id 条目跳过', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'approvals': [
        {
          'id': 'a1',
          'functionId': 'player.kick',
          'actor': 'op1',
          'state': 'pending',
          'createdAt': '2026-09-30 10:00',
          'mode': 'invoke',
          'gameId': 'demo',
          'env': 'prod',
        },
        {'actor': '缺 id'},
      ],
      'total': 1,
      'page': 1,
      'size': 20,
    });

    final pageData = await service.list(page: 2);

    expect(pageData.items, hasLength(1));
    expect(pageData.items.first.functionId, 'player.kick');
    expect(pageData.items.first.actor, 'op1');
    expect(pageData.total, 1);
    expect(adapter.lastRequest?.path, '/api/v1/approvals/');
    expect(adapter.lastRequest?.queryParameters['page'], 2);
    expect(adapter.lastRequest?.queryParameters['status'], 'pending');
  });

  test('approvals 非 List → StateError（不静默当空列表）', () async {
    adapter.handler = (options, _) => jsonResponse(200, {'approvals': 'oops'});
    await expectLater(service.list(), throwsStateError);
  });

  test('detail 解析单条', () async {
    adapter.handler = (options, _) => jsonResponse(200, {
      'id': 'a1',
      'functionId': 'player.kick',
      'actor': 'op1',
      'state': 'pending',
      'payloadPreview': '{"playerId": 42}',
    });

    final item = await service.detail('a1');

    expect(item.id, 'a1');
    expect(item.payloadPreview, '{"playerId": 42}');
    expect(adapter.lastRequest?.path, '/api/v1/approvals/a1');
  });

  test('approve POST body 带 otp（step-up 预留）', () async {
    adapter.handler = (options, _) =>
        jsonResponse(200, {'id': 'a1', 'state': 'approved'});

    await service.approve('a1', otp: '123456');

    expect(adapter.lastRequest?.path, '/api/v1/approvals/a1/approve');
    expect(adapter.lastBody, contains('"otp":"123456"'));
  });

  test('reject POST body reason 必带', () async {
    adapter.handler = (options, _) =>
        jsonResponse(200, {'id': 'a1', 'state': 'rejected'});

    await service.reject('a1', reason: '不当操作');

    expect(adapter.lastRequest?.path, '/api/v1/approvals/a1/reject');
    expect(adapter.lastBody, contains('"reason":"不当操作"'));
  });
}
