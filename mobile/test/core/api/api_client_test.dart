import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/api/api_error.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late ApiClient client;
  var unauthorizedCalls = 0;

  setUp(() {
    store = InMemorySessionStore();
    unauthorizedCalls = 0;
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    client = ApiClient.create(
      baseUrl: 'http://gm.test',
      sessionStore: store,
      onUnauthorized: () => unauthorizedCalls++,
    );
    client.dio.httpClientAdapter = adapter;
  });

  test('token 注入 Authorization 头', () async {
    await store.save(const SessionData(token: 'jwt-1'));
    await client.get<Map<String, Object?>>('/api/v1/ops/cluster');
    expect(adapter.lastRequest?.headers['Authorization'], 'Bearer jwt-1');
  });

  test('无会话不带 Authorization', () async {
    await client.get<Map<String, Object?>>('/api/v1/auth/providers');
    expect(adapter.lastRequest?.headers.containsKey('Authorization'), isFalse);
  });

  test('scoped 路径注入成对 scope 头，并剥除外部同名头（防注入）', () async {
    await store.save(
      const SessionData(token: 'jwt-1', gameId: 'demo', env: 'prod'),
    );
    await client.get<Map<String, Object?>>(
      '/api/v1/approvals/',
      query: {'status': 'pending'},
      options: Options(headers: {'X-Game-ID': 'evil', 'X-Env': 'evil'}),
    );
    expect(adapter.lastRequest?.headers['X-Game-ID'], 'demo');
    expect(adapter.lastRequest?.headers['X-Env'], 'prod');
  });

  test('非 scoped 路径（登录/审计）不注入 scope 头', () async {
    await store.save(
      const SessionData(token: 'jwt-1', gameId: 'demo', env: 'prod'),
    );
    await client.get<Map<String, Object?>>('/api/v1/auth/providers');
    expect(adapter.lastRequest?.headers.containsKey('X-Game-ID'), isFalse);
    expect(adapter.lastRequest?.headers.containsKey('X-Env'), isFalse);
  });

  test('scope 不成对不注入（服务端原子拒绝部分 scope）', () async {
    await store.save(const SessionData(token: 'jwt-1', gameId: 'demo'));
    await client.get<Map<String, Object?>>('/api/v1/approvals/');
    expect(adapter.lastRequest?.headers.containsKey('X-Game-ID'), isFalse);
  });

  test('401（业务端点）：清会话 + onUnauthorized 回调 + ApiError 401', () async {
    await store.save(const SessionData(token: 'stale'));
    adapter.handler = (options, _) =>
        jsonResponse(401, {'error': 'unauthorized', 'message': '未授权'});
    await expectLater(
      client.get<Map<String, Object?>>('/api/v1/tasks'),
      throwsA(isA<ApiError>().having((e) => e.status, 'status', 401)),
    );
    expect(await store.load(), isNull);
    expect(unauthorizedCalls, 1);
  });

  test('401（登录端点）：保留会话、不回调——mfa_required 走登录双步分支', () async {
    await store.save(const SessionData(token: 'stale'));
    adapter.handler = (options, _) =>
        jsonResponse(401, {'error': 'mfa_required', 'message': '需要动态验证码'});
    await expectLater(
      client.post<Map<String, Object?>>('/api/v1/auth/login', body: {}),
      throwsA(isA<ApiError>().having((e) => e.code, 'code', 'mfa_required')),
    );
    expect((await store.load())?.token, 'stale');
    expect(unauthorizedCalls, 0);
  });

  test('409 竞态错误体归一透传', () async {
    adapter.handler = (options, _) =>
        jsonResponse(409, {'error': 'conflict', 'message': '已被处理'});
    await expectLater(
      client.post<Map<String, Object?>>('/api/v1/approvals/1/approve'),
      throwsA(
        isA<ApiError>()
            .having((e) => e.code, 'code', 'conflict')
            .having((e) => e.message, 'message', '已被处理'),
      ),
    );
  });
}
