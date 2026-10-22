/// 告警服务（设计稿 §2.2）：列表 / 静默 / 静默规则。
/// 注意：alerts 域**非 scoped**（无 X-Game-ID 头语义，路由挂 protected 组）。
library;

import '../../core/api/api_client.dart';

class AlertItem {
  const AlertItem({
    required this.id,
    this.type = '',
    this.level = '',
    this.message = '',
    this.source = '',
    this.status = '',
    this.createdAt = '',
  });

  final String id;
  final String type;
  final String level;
  final String message;
  final String source;
  final String status;
  final String createdAt;

  static AlertItem? fromJson(Map<String, Object?> json) {
    final id = json['id'];
    if (id is! String || id.isEmpty) return null;
    String str(String key) => json[key] is String ? json[key] as String : '';
    return AlertItem(
      id: id,
      type: str('type'),
      level: str('level'),
      message: str('message'),
      source: str('source'),
      status: str('status'),
      createdAt: str('createdAt'),
    );
  }
}

class AlertPage {
  const AlertPage({required this.items, required this.total});

  final List<AlertItem> items;
  final int total;
}

class SilenceRule {
  const SilenceRule({
    required this.id,
    this.alertType = '',
    this.startAt = '',
    this.endAt = '',
    this.createdBy = '',
  });

  final String id;
  final String alertType;
  final String startAt;
  final String endAt;
  final String createdBy;

  static SilenceRule? fromJson(Map<String, Object?> json) {
    final id = json['id'];
    if (id is! String || id.isEmpty) return null;
    String str(String key) => json[key] is String ? json[key] as String : '';
    return SilenceRule(
      id: id,
      alertType: str('alertType'),
      startAt: str('startAt'),
      endAt: str('endAt'),
      createdBy: str('createdBy'),
    );
  }
}

class AlertService {
  AlertService({required this.client});

  static const basePath = '/api/v1/alerts';

  final ApiClient client;

  /// 告警列表（level/status 服务端过滤，页大小与 Web 同默认 20）。
  Future<AlertPage> list({
    int page = 1,
    int pageSize = 20,
    String level = '',
    String status = '',
  }) async {
    final data = await client.get<Map<String, Object?>>(
      basePath,
      query: {
        'page': page,
        'pageSize': pageSize,
        if (level.isNotEmpty) 'level': level,
        if (status.isNotEmpty) 'status': status,
      },
    );
    final raw = data['items'];
    if (raw is! List) {
      throw StateError('invalid alerts payload');
    }
    final items = <AlertItem>[];
    for (final item in raw) {
      if (item is! Map) continue;
      final parsed = AlertItem.fromJson(Map<String, Object?>.from(item));
      if (parsed != null) items.add(parsed);
    }
    return AlertPage(
      items: items,
      total: data['total'] is int ? data['total'] as int : items.length,
    );
  }

  /// 静默：duration 分钟数、reason 必填（服务端 400 兜底）。
  Future<void> silence(
    String id, {
    required int durationMinutes,
    required String reason,
  }) async {
    await client.post<Map<String, Object?>>(
      '$basePath/$id/silence',
      body: {'duration': durationMinutes, 'reason': reason},
    );
  }

  /// 静默规则列表（只读展示）。
  Future<List<SilenceRule>> silences() async {
    final data = await client.get<Map<String, Object?>>('$basePath/silences');
    final raw = data['items'];
    if (raw is! List) {
      throw StateError('invalid silences payload');
    }
    final rules = <SilenceRule>[];
    for (final item in raw) {
      if (item is! Map) continue;
      final parsed = SilenceRule.fromJson(Map<String, Object?>.from(item));
      if (parsed != null) rules.add(parsed);
    }
    return rules;
  }
}
