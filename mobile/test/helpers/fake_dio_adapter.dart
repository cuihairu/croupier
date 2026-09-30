/// 测试用 dio 假适配器：按请求回调返回预置响应，捕获请求供断言。
library;

import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';

class FakeDioAdapter implements HttpClientAdapter {
  FakeDioAdapter(this.handler);

  /// 可在用例中途替换（如先 200 后 401 的会话失效场景）。
  ResponseBody Function(RequestOptions options, List<int>? rawBody) handler;

  /// 最近一次请求（简单场景断言用）。
  RequestOptions? lastRequest;
  String? lastBody;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastRequest = options;
    final chunks = requestStream == null
        ? const <Uint8List>[]
        : await requestStream.toList();
    final raw = chunks.expand((c) => c).toList();
    lastBody = raw.isEmpty ? null : String.fromCharCodes(raw);
    return handler(options, raw);
  }

  @override
  void close({bool force = false}) {}
}

/// JSON 响应体快捷构造。
ResponseBody jsonResponse(int status, Map<String, Object?> body) {
  return ResponseBody.fromString(
    jsonEncode(body),
    status,
    headers: {
      Headers.contentTypeHeader: <String>[Headers.jsonContentType],
    },
  );
}
