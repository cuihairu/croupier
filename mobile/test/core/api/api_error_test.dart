import 'package:croupier_mobile/core/api/api_error.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  RequestOptions ro(String path) => RequestOptions(path: path);

  DioException dioError(int? status, Object? body) => DioException(
    requestOptions: ro('/api/v1/x'),
    response: status == null
        ? null
        : Response(
            requestOptions: ro('/api/v1/x'),
            statusCode: status,
            data: body,
          ),
  );

  test('契约错误体归一：error/message/details 透传', () {
    final e = ApiError.fromDio(
      dioError(409, {
        'error': 'conflict',
        'message': '审批已被他人处理',
        'details': {'state': 'approved'},
      }),
    );
    expect(e.status, 409);
    expect(e.code, 'conflict');
    expect(e.message, '审批已被他人处理');
    expect(e.details, isA<Map>());
  });

  test('非契约形态（HTML 兜底）：保状态码，code 退化 http_<status>', () {
    final e = ApiError.fromDio(dioError(404, '<!doctype html>…'));
    expect(e.status, 404);
    expect(e.code, 'http_404');
    expect(e.details, isNull);
  });

  test('无响应（网络层失败）→ network_error / status 0', () {
    final e = ApiError.fromDio(dioError(null, null));
    expect(e.isNetworkError, isTrue);
    expect(e.code, 'network_error');
    expect(e.status, 0);
  });
}
