/// 分析门面服务（MOB-ANA-001 §4）：GET summary 一次拉全部域摘要卡片 /
/// POST query 统一分域查询。契约键 lowerCamelCase；解析 fail-fast
/// （防 SPA HTML 兜底被当空卡组）。
library;

import '../../core/api/api_client.dart';

/// 域摘要卡片（对齐后端 DomainCard）。
class DomainCardData {
  const DomainCardData({
    required this.domain,
    required this.status,
    this.data,
    this.error = '',
  });

  static const statusOk = 'ok';
  static const statusUnavailable = 'unavailable';
  static const statusPending = 'pending_data_source';

  final String domain;
  final String status;
  final Object? data;
  final String error;

  bool get isOk => status == statusOk;
  bool get isPending => status == statusPending;
  bool get isUnavailable => status == statusUnavailable;

  static DomainCardData? fromJson(Map<String, Object?> json) {
    final domain = json['domain'];
    final status = json['status'];
    if (domain is! String || domain.isEmpty) return null;
    if (status is! String || status.isEmpty) return null;
    return DomainCardData(
      domain: domain,
      status: status,
      data: json['data'],
      error: json['error'] is String ? json['error'] as String : '',
    );
  }
}

/// summary 聚合响应（对齐后端 FacadeSummaryResponse）。
class AnalyticsSummary {
  const AnalyticsSummary({
    required this.gameId,
    required this.env,
    required this.generatedAt,
    required this.domains,
  });

  final String gameId;
  final String env;
  final String generatedAt;
  final List<DomainCardData> domains;
}

class AnalyticsService {
  AnalyticsService({required this.client});

  static const summaryPath = '/api/v1/analytics/facade/summary';
  static const queryPath = '/api/v1/analytics/facade/query';

  final ApiClient client;

  /// 全域摘要卡组。domains 非 List → StateError（不静默当空卡组）；
  /// 单卡形态不合法跳过（与 approvals 列表同语义）。
  Future<AnalyticsSummary> summary({
    required String gameId,
    String env = '',
  }) async {
    final data = await client.get<Map<String, Object?>>(
      summaryPath,
      query: {'gameId': gameId, if (env.isNotEmpty) 'env': env},
    );
    final raw = data['domains'];
    if (raw is! List) {
      throw StateError('invalid analytics summary payload');
    }
    final cards = <DomainCardData>[];
    for (final item in raw) {
      if (item is! Map) continue;
      final parsed = DomainCardData.fromJson(Map<String, Object?>.from(item));
      if (parsed != null) cards.add(parsed);
    }
    return AnalyticsSummary(
      gameId: data['gameId'] is String ? data['gameId'] as String : gameId,
      env: data['env'] is String ? data['env'] as String : env,
      generatedAt: data['generatedAt'] is String
          ? data['generatedAt'] as String
          : '',
      domains: cards,
    );
  }

  /// 统一分域查询：POST {domain, filter:{gameId,env,...}, timeRange}。
  /// 响应 {domain, data}；data 缺键视为形态不合法 fail-fast。
  Future<Object?> query({
    required String domain,
    required String gameId,
    String env = '',
    Map<String, String> filter = const {},
    String startDate = '',
    String endDate = '',
  }) async {
    final data = await client.post<Map<String, Object?>>(queryPath, body: {
      'domain': domain,
      'filter': {
        'gameId': gameId,
        if (env.isNotEmpty) 'env': env,
        ...filter,
      },
      'timeRange': {
        if (startDate.isNotEmpty) 'startDate': startDate,
        if (endDate.isNotEmpty) 'endDate': endDate,
      },
    });
    if (!data.containsKey('data')) {
      throw StateError('invalid analytics query payload');
    }
    return data['data'];
  }
}
