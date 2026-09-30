/// 审计服务（设计稿 §2.4）：GET /audit 只读检索。
/// 注意：audit 域**非 scoped**——游戏过滤走 gameId query 参数（§2.4）。
/// start/end 必须 RFC3339（含时区 offset），否则服务端 time.Parse 静默忽略。
library;

import '../../core/api/api_client.dart';

class AuditItem {
  const AuditItem({
    required this.id,
    this.action = '',
    this.userId = '',
    this.gameId = '',
    this.env = '',
    this.target = '',
    this.result = '',
    this.traceId = '',
    this.createdAt = '',
    this.metadata,
  });

  final String id;
  final String action;
  final String userId;
  final String gameId;
  final String env;
  final String target;
  final String result;
  final String traceId;
  final String createdAt;

  /// 原始 metadata 对象（展开行 JSON 折叠视图用）。
  final Map<String, Object?>? metadata;

  static AuditItem? fromJson(Map<String, Object?> json) {
    final id = json['id'];
    if (id is! String || id.isEmpty) return null;
    String str(String key) => json[key] is String ? json[key] as String : '';
    final metadata = json['metadata'];
    return AuditItem(
      id: id,
      action: str('action'),
      userId: str('userId'),
      gameId: str('gameId'),
      env: str('env'),
      target: str('target'),
      result: str('result'),
      traceId: str('traceId'),
      createdAt: str('createdAt'),
      metadata: metadata is Map ? Map<String, Object?>.from(metadata) : null,
    );
  }
}

class AuditPage {
  const AuditPage({required this.items, required this.total});

  final List<AuditItem> items;
  final int total;
}

/// RFC3339（含时区 offset）：后端 time.Parse(time.RFC3339) 只认这种形态，
/// 本地 DateTime.toIso8601String() 无 offset 会被静默忽略。
String toRfc3339(DateTime t) {
  final off = t.timeZoneOffset;
  final sign = off.isNegative ? '-' : '+';
  final abs = off.abs();
  String two(int n) => n.toString().padLeft(2, '0');
  final base =
      '${t.year}-${two(t.month)}-${two(t.day)}'
      'T${two(t.hour)}:${two(t.minute)}:${two(t.second)}';
  return '$base$sign${two(abs.inHours)}:${two(abs.inMinutes.remainder(60))}';
}

class AuditService {
  AuditService({required this.client});

  static const basePath = '/api/v1/audit';

  final ApiClient client;

  /// 检索。filters 为已归一参数（空值不带）。
  Future<AuditPage> list({
    int page = 1,
    int pageSize = 20,
    String actor = '',
    List<String> kinds = const [],
    String env = '',
    String ip = '',
    String gameId = '',
    DateTime? start,
    DateTime? end,
  }) async {
    final data = await client.get<Map<String, Object?>>(
      basePath,
      query: {
        'page': page,
        'pageSize': pageSize,
        if (actor.isNotEmpty) 'actor': actor,
        if (kinds.isNotEmpty) 'kinds': kinds.join(','),
        if (env.isNotEmpty) 'env': env,
        if (ip.isNotEmpty) 'ip': ip,
        if (gameId.isNotEmpty) 'gameId': gameId,
        if (start != null) 'start': toRfc3339(start),
        if (end != null) 'end': toRfc3339(end),
      },
    );
    final raw = data['items'];
    if (raw is! List) {
      throw StateError('invalid audit payload');
    }
    final items = <AuditItem>[];
    for (final item in raw) {
      if (item is! Map) continue;
      final parsed = AuditItem.fromJson(Map<String, Object?>.from(item));
      if (parsed != null) items.add(parsed);
    }
    return AuditPage(
      items: items,
      total: data['total'] is int ? data['total'] as int : items.length,
    );
  }
}
