/// 服务器连通性探测：GET /api/v1/public/site（公开端点，无需登录）。
/// 2xx 且响应为 JSON 对象才算通——防把返回 200 HTML 的 SPA 兜底页当成功。
library;

import '../api/api_client.dart';
import '../api/api_error.dart';

/// [client] 由调用方经 `apiClientFactoryProvider`（或测试 fake）按
/// 待探测地址装配，本函数只做探测与形态校验；失败抛 [ApiError]。
Future<void> probeServer(ApiClient client) async {
  final data = await client.get<Object?>('/api/v1/public/site');
  if (data is! Map) {
    throw const ApiError(
      status: 200,
      code: 'server_probe_failed',
      message: '连接的不是 Croupier 服务（响应不是 JSON）',
    );
  }
}
