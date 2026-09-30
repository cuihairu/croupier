/// 审批服务（设计稿 §2.1）：列表 / 详情 / 批准 / 拒绝。
/// 全部 scoped 端点（X-Game-ID/X-Env 由 ApiClient 拦截器注入）。
library;

import '../../core/api/api_client.dart';

class ApprovalItem {
  const ApprovalItem({
    required this.id,
    required this.functionId,
    required this.actor,
    required this.state,
    required this.createdAt,
    this.gameId = '',
    this.env = '',
    this.mode = '',
    this.payloadPreview = '',
    this.approver = '',
  });

  final String id;
  final String functionId;
  final String actor;
  final String state;
  final String createdAt;
  final String gameId;
  final String env;
  final String mode;
  final String payloadPreview;
  final String approver;

  static ApprovalItem? fromJson(Map<String, Object?> json) {
    final id = json['id'];
    if (id is! String || id.isEmpty) return null;
    return ApprovalItem(
      id: id,
      functionId: json['functionId'] is String
          ? json['functionId'] as String
          : '',
      actor: json['actor'] is String ? json['actor'] as String : '',
      state: json['state'] is String ? json['state'] as String : '',
      createdAt: json['createdAt'] is String ? json['createdAt'] as String : '',
      gameId: json['gameId'] is String ? json['gameId'] as String : '',
      env: json['env'] is String ? json['env'] as String : '',
      mode: json['mode'] is String ? json['mode'] as String : '',
      payloadPreview: json['payloadPreview'] is String
          ? json['payloadPreview'] as String
          : '',
      approver: json['approver'] is String ? json['approver'] as String : '',
    );
  }
}

class ApprovalPage {
  const ApprovalPage({required this.items, required this.total});

  final List<ApprovalItem> items;
  final int total;
}

class ApprovalService {
  ApprovalService({required this.client});

  static const listPath = '/api/v1/approvals/';

  final ApiClient client;

  /// 审批列表。响应形态不合法 fail-fast（防 SPA HTML 兜底当空列表）。
  Future<ApprovalPage> list({
    int page = 1,
    int pageSize = 20,
    String status = 'pending',
  }) async {
    final data = await client.get<Map<String, Object?>>(
      listPath,
      query: {'page': page, 'pageSize': pageSize, 'status': status},
    );
    final raw = data['approvals'];
    if (raw is! List) {
      throw StateError('invalid approvals payload');
    }
    final items = <ApprovalItem>[];
    for (final item in raw) {
      if (item is! Map) continue;
      final parsed = ApprovalItem.fromJson(Map<String, Object?>.from(item));
      if (parsed != null) items.add(parsed);
    }
    return ApprovalPage(
      items: items,
      total: data['total'] is int ? data['total'] as int : items.length,
    );
  }

  /// 详情（复用列表条模型 + 展示层取 payloadPreview/approver）。
  Future<ApprovalItem> detail(String id) async {
    final data = await client.get<Map<String, Object?>>('$listPath$id');
    final parsed = ApprovalItem.fromJson(data);
    if (parsed == null) {
      throw StateError('invalid approval payload');
    }
    return parsed;
  }

  /// 批准。otp 为 step-up 预留（当前后端不读 body，字段补齐即生效）。
  Future<void> approve(String id, {String? otp}) async {
    await client.post<Map<String, Object?>>(
      '$listPath$id/approve',
      body: {if (otp != null && otp.isNotEmpty) 'otp': otp},
    );
  }

  /// 拒绝。reason 必填（UI 前置校验；服务端 400 兜底）。
  Future<void> reject(String id, {required String reason}) async {
    await client.post<Map<String, Object?>>(
      '$listPath$id/reject',
      body: {'reason': reason},
    );
  }
}
