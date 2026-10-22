import 'package:croupier_mobile/core/api/api_client.dart';
import 'package:croupier_mobile/core/api/api_error.dart';
import 'package:croupier_mobile/core/server/server_probe.dart';
import 'package:croupier_mobile/core/storage/session_store.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers/fake_dio_adapter.dart';

void main() {
  late InMemorySessionStore store;
  late FakeDioAdapter adapter;
  late ApiClient client;

  setUp(() {
    store = InMemorySessionStore();
    adapter = FakeDioAdapter((options, _) => jsonResponse(200, {}));
    client = ApiClient.create(baseUrl: 'http://gm.test', sessionStore: store);
    client.dio.httpClientAdapter = adapter;
  });

  test('200 JSON 对象 → 探测通过', () async {
    adapter.handler = (options, _) =>
        jsonResponse(200, {'title': 'Croupier', 'edition': 'ce'});
    await expectLater(probeServer(client), completes);
    expect(adapter.lastRequest?.uri.path, '/api/v1/public/site');
  });

  test('200 HTML（SPA 兜底）不放行 → server_probe_failed', () async {
    adapter.handler = (options, _) => ResponseBody.fromString(
      '<!DOCTYPE html><html></html>',
      200,
      headers: {
        Headers.contentTypeHeader: <String>['text/html'],
      },
    );
    try {
      await probeServer(client);
      fail('should throw');
    } on ApiError catch (e) {
      expect(e.code, 'server_probe_failed');
      expect(e.status, 200);
    }
  });

  test('5xx → 透传 ApiError', () async {
    adapter.handler = (options, _) =>
        jsonResponse(502, {'error': 'bad_gateway', 'message': '网关错误'});
    try {
      await probeServer(client);
      fail('should throw');
    } on ApiError catch (e) {
      expect(e.status, 502);
      expect(e.message, '网关错误');
    }
  });

  test('连接失败 → network_error', () async {
    adapter.handler = (options, _) => throw DioException(
      requestOptions: options,
      type: DioExceptionType.connectionError,
      error: 'connection refused',
    );
    try {
      await probeServer(client);
      fail('should throw');
    } on ApiError catch (e) {
      expect(e.isNetworkError, isTrue);
    }
  });
}
