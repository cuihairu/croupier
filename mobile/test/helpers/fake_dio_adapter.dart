/// 测试用 dio 假适配器：按请求回调返回预置响应，捕获请求供断言。
library;

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';

class FakeDioAdapter implements HttpClientAdapter {
  FakeDioAdapter(this.handler);

  /// 可在用例中途替换（如先 200 后 401 的会话失效场景）；返回 FutureOr
  /// 支持挂起响应（Completer 门控，测 loading 态竞态分支）。
  FutureOr<ResponseBody> Function(RequestOptions options, List<int>? rawBody)
  handler;

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
    // dio 出站 body 是 UTF-8 字节流；按 CharCodes 解读会打碎多字节中文。
    lastBody = raw.isEmpty ? null : utf8.decode(raw);
    return await handler(options, raw);
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
