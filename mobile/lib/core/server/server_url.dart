/// 服务器地址校验与归一化：格式合法性 + 明确错误文案（不放行即抛回 UI）。
library;

/// 校验结果：[url] 非空 = 合法（已归一化）；否则 [error] 给用户可读文案。
class ServerUrlValidation {
  const ServerUrlValidation.ok(this.url) : error = null, assert(url != null);

  const ServerUrlValidation.fail(this.error) : url = null;

  final String? url;
  final String? error;

  bool get isValid => url != null;
}

/// 校验并归一化服务器地址：
/// - 空 → 「服务器地址不能为空」；
/// - 无法解析 / 无主机 → 格式错误；
/// - scheme 非 http(s) → 明确不支持；
/// - 归一化：去首尾空白与结尾多余的 `/`（保留端口与路径前缀语义）。
ServerUrlValidation validateServerUrl(String raw) {
  final text = raw.trim();
  if (text.isEmpty) {
    return const ServerUrlValidation.fail('服务器地址不能为空');
  }
  final uri = Uri.tryParse(text);
  if (uri == null || uri.host.isEmpty) {
    return const ServerUrlValidation.fail('地址格式不正确，示例：https://gm.example.com');
  }
  if (uri.scheme != 'http' && uri.scheme != 'https') {
    return const ServerUrlValidation.fail('仅支持 http / https 地址');
  }
  return ServerUrlValidation.ok(text.replaceFirst(RegExp(r'/+$'), ''));
}
