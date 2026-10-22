/// 函数调用服务（设计稿 §2.5）：同步/异步 invoke + 异步任务跟进。
///
/// 契约来源：
/// - `POST /api/v1/functions/:id/invoke`（scoped）body
///   `{payload, mode?, route?, targetServiceId?, hashKey?}`；
///   gameId/env 由拦截器以 X-Game-ID/X-Env 注入，服务端忽略 body 内 scope
///   字段（防重定向到别的 agent scope）。
/// - `GET /api/v1/tasks/:id`（scoped）任务详情；
/// - `POST /api/v1/tasks/:id/cancel`（scoped）取消。
library;

import '../../core/api/api_client.dart';

/// 调用结果：同步直达 [result]；异步带 [taskId]；
/// approvalRequired 为真时带 [approvalId]（已提交审批，等第二审批人）。
class InvokeResult {
  const InvokeResult({
    this.taskId = '',
    this.result,
    this.approvalId = '',
    this.approvalRequired = false,
    this.traceId = '',
  });

  final String taskId;
  final Object? result;
  final String approvalId;
  final bool approvalRequired;
  final String traceId;

  bool get isTask => taskId.isNotEmpty;

  static InvokeResult fromJson(Map<String, Object?> json) {
    return InvokeResult(
      taskId: json['taskId'] is String ? json['taskId'] as String : '',
      result: json['result'],
      approvalId: json['approvalId'] is String
          ? json['approvalId'] as String
          : '',
      approvalRequired: json['approvalRequired'] == true,
      traceId: json['traceId'] is String ? json['traceId'] as String : '',
    );
  }
}

/// 异步任务状态（GET /tasks/:id）。
class TaskStatus {
  const TaskStatus({
    required this.id,
    this.functionId = '',
    this.status = '',
    this.progress = 0,
    this.message = '',
    this.result,
    this.error = '',
  });

  final String id;
  final String functionId;
  final String status;
  final int progress;
  final String message;
  final Object? result;
  final String error;

  /// 终态：success/failed/canceled 三种均停止轮询。
  bool get isDone =>
      status == 'success' || status == 'failed' || status == 'canceled';
  bool get isSuccess => status == 'success';
  bool get isFailed => status == 'failed';
  bool get isCanceled => status == 'canceled';
  bool get isRunning => !isDone;

  static TaskStatus? fromJson(Map<String, Object?> json) {
    final id = json['id'];
    if (id is! String || id.isEmpty) return null;
    return TaskStatus(
      id: id,
      functionId: json['functionId'] is String
          ? json['functionId'] as String
          : '',
      status: json['status'] is String ? json['status'] as String : '',
      progress: json['progress'] is int ? json['progress'] as int : 0,
      message: json['message'] is String ? json['message'] as String : '',
      result: json['result'],
      error: json['error'] is String ? json['error'] as String : '',
    );
  }
}

class FunctionInvokeService {
  FunctionInvokeService({required this.client});

  final ApiClient client;

  /// 调用函数。[mode] 空串时后端按函数执行类型默认（sync/task）。
  /// route 默认省略（服务端 lb）；targetServiceId/hashKey 为高级路由。
  Future<InvokeResult> invoke({
    required String functionId,
    Map<String, Object?> payload = const {},
    String mode = '',
    String route = '',
    String targetServiceId = '',
    String hashKey = '',
  }) async {
    final body = <String, Object?>{
      'payload': payload,
      if (mode.isNotEmpty) 'mode': mode,
      if (route.isNotEmpty) 'route': route,
      if (targetServiceId.isNotEmpty) 'targetServiceId': targetServiceId,
      if (hashKey.isNotEmpty) 'hashKey': hashKey,
    };
    final data = await client.post<Map<String, Object?>>(
      '/api/v1/functions/$functionId/invoke',
      body: body,
    );
    return InvokeResult.fromJson(data);
  }

  /// 任务详情（未命中 404 → ApiError；缺 id fail-fast）。
  Future<TaskStatus> taskDetail(String taskId) async {
    final data = await client.get<Map<String, Object?>>(
      '/api/v1/tasks/$taskId',
    );
    final parsed = TaskStatus.fromJson(data);
    if (parsed == null) throw StateError('invalid task payload');
    return parsed;
  }

  /// 取消任务（后端返回 200 `{message}`）。
  Future<void> cancelTask(String taskId) async {
    await client.post<Map<String, Object?>>('/api/v1/tasks/$taskId/cancel');
  }
}
