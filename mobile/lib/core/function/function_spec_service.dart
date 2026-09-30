/// 描述符索引服务：函数目录 + 动态表单 schema 的唯一拉取面。
///
/// 契约：`GET /api/v1/functions/descriptors`（scoped，需 X-Game-ID/X-Env）。
/// 响应形态兜底三态（对齐 web `getFunctionSummary`）：`{functions:[...]}` /
/// `{items:[...]}` / 裸数组；其余 fail-fast（防 SPA HTML 兜底当空目录）。
library;

import '../api/api_client.dart';
import 'function_spec.dart';

class FunctionSpecService {
  FunctionSpecService({required this.client});

  static const listPath = '/api/v1/functions/descriptors';

  final ApiClient client;

  Future<List<FunctionSpec>> list() async {
    // get<Object?> 而非 get<Map<...>>：裸数组形态在 Map 强转会抛 TypeError
    // 逃出 loadDescriptors 的 ApiError/StateError 静默网，此处按类型分发。
    final data = await client.get<Object?>(listPath);
    if (data is List) return parseList(data);
    if (data is Map) return parseSpecs(Map<String, Object?>.from(data));
    throw StateError('invalid descriptors payload');
  }

  /// 形态解析（对象态）：{functions} / {items} 两键兜底，其余 fail-fast
  /// （防 SPA HTML 兜底当空目录）。
  static List<FunctionSpec> parseSpecs(Map<String, Object?> data) {
    final functions = data['functions'];
    if (functions is List) return parseList(functions);
    final items = data['items'];
    if (items is List) return parseList(items);
    throw StateError('invalid descriptors payload');
  }

  /// 列表态解析：缺 id / 非 map 元素跳过，合法项进索引。
  static List<FunctionSpec> parseList(List<Object?> raw) {
    final specs = <FunctionSpec>[];
    for (final item in raw) {
      if (item is! Map) continue;
      final parsed = FunctionSpec.fromJson(Map<String, Object?>.from(item));
      if (parsed != null) specs.add(parsed);
    }
    return specs;
  }
}
