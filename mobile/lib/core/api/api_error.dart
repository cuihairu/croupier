/// 统一错误对象（对齐后端响应契约与设计稿 §3.4）。
///
/// 后端错误体 `{error, message, details?}` + 标准 HTTP status；客户端逻辑
/// 分支一律按 [code]（机器可读），UI 文案按 [message]，表单回填按 [details]。
class ApiError implements Exception {
  const ApiError({
    required this.status,
    required this.code,
    required this.message,
    this.details,
  });

  /// HTTP status；网络层失败（未触达服务器）为 0。
  final int status;

  /// 稳定机器码（后端 `error` 字段）；网络层失败为 `network_error`。
  final String code;

  /// 用户可读文案（后端已本地化）。
  final String message;

  /// 结构化附加信息（字段级校验：字段→文案；原始 JSON 对象）。
  final Object? details;

  bool get isNetworkError => status == 0;

  factory ApiError.network([String message = '网络错误，请检查连接']) {
    return ApiError(status: 0, code: 'network_error', message: message);
  }

  /// 从 DioException 归一。非 4xx/5xx 的响应体解析失败不吞状态码。
  static ApiError fromDio(Object error) {
    final dioException = error as dynamic;
    final response = dioException.response as dynamic;
    if (response == null) {
      return ApiError.network();
    }
    final status = (response.statusCode as int?) ?? 0;
    final body = response.data;
    if (body is Map) {
      return ApiError(
        status: status,
        code: (body['error'] as String?) ?? 'http_$status',
        message: (body['message'] as String?) ?? '请求失败（$status）',
        details: body['details'],
      );
    }
    // 非契约形态（HTML 兜底页 / 空体等）：保 HTTP 语义，code 退化 http_<status>
    return ApiError(
      status: status,
      code: 'http_$status',
      message: '请求失败（$status）',
    );
  }

  @override
  String toString() => 'ApiError($status, $code, $message)';
}
