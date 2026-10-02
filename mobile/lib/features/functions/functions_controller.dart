/// 函数目录 + 调用控制器（设计稿 §2.5）：
/// descriptors 目录加载 / 搜索过滤 / invoke（同步结果或异步任务）/
/// 任务轮询（5s，页面可见时）/ 取消。
library;

import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import '../../core/function/function_spec.dart';
import '../../core/function/function_spec_service.dart';
import '../scope/scope_controller.dart';
import 'function_invoke_service.dart';

class FunctionsState {
  const FunctionsState({
    this.specs = const [],
    this.loading = false,
    this.error,
    this.query = '',
    this.submitting = false,
    this.invokeError,
    this.invokeResult,
    this.task,
    this.lastFunctionId = '',
  });

  final List<FunctionSpec> specs;
  final bool loading;
  final String? error;
  final String query;

  /// 正在提交调用（按钮禁用）。
  final bool submitting;

  /// 调用错误（snackbar 用）。
  final String? invokeError;

  /// 同步调用结果 / 审批提交通知。
  final InvokeResult? invokeResult;

  /// 异步任务当前状态（非空时任务跟进页活跃）。
  final TaskStatus? task;

  final String lastFunctionId;

  bool get taskRunning => task != null && task!.isRunning;

  List<FunctionSpec> get filtered {
    final q = query.trim().toLowerCase();
    if (q.isEmpty) return specs;
    return specs
        .where(
          (s) =>
              s.id.toLowerCase().contains(q) ||
              s.displayName.toLowerCase().contains(q) ||
              s.displayDescription.toLowerCase().contains(q),
        )
        .toList(growable: false);
  }

  FunctionsState copyWith({
    List<FunctionSpec>? specs,
    bool? loading,
    String? error,
    bool clearError = false,
    String? query,
    bool? submitting,
    String? invokeError,
    bool clearInvokeError = false,
    InvokeResult? invokeResult,
    bool clearInvokeResult = false,
    TaskStatus? task,
    bool clearTask = false,
    String? lastFunctionId,
  }) {
    return FunctionsState(
      specs: specs ?? this.specs,
      loading: loading ?? this.loading,
      error: clearError ? null : (error ?? this.error),
      query: query ?? this.query,
      submitting: submitting ?? this.submitting,
      invokeError: clearInvokeError ? null : (invokeError ?? this.invokeError),
      invokeResult: clearInvokeResult
          ? null
          : (invokeResult ?? this.invokeResult),
      task: clearTask ? null : (task ?? this.task),
      lastFunctionId: lastFunctionId ?? this.lastFunctionId,
    );
  }
}

final functionsControllerProvider =
    NotifierProvider<FunctionsController, FunctionsState>(
      FunctionsController.new,
    );

class FunctionsController extends Notifier<FunctionsState> {
  Timer? _poll;

  @override
  FunctionsState build() {
    // scope 切换联动：descriptors 是 scoped 端点，切后目录重查。
    ref.listen(scopeControllerProvider, (prev, next) {
      if (prev?.selected != next.selected && next.selected != null) {
        refresh();
      }
    });
    ref.onDispose(() => _poll?.cancel());
    return const FunctionsState();
  }

  Future<FunctionSpecService?> _specServiceAsync() async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (serverUrl.isEmpty) return null;
    return FunctionSpecService(
      client: ref.read(apiClientFactoryProvider)(serverUrl),
    );
  }

  Future<FunctionInvokeService?> _invokeServiceAsync() async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (serverUrl.isEmpty) return null;
    return FunctionInvokeService(
      client: ref.read(apiClientFactoryProvider)(serverUrl),
    );
  }

  Future<void> refresh() async {
    if (state.loading) return;
    final service = await _specServiceAsync();
    if (service == null) {
      state = state.copyWith(loading: false, error: '未登录或缺少服务器地址');
      return;
    }
    state = state.copyWith(loading: true, clearError: true);
    try {
      final specs = await service.list();
      state = state.copyWith(specs: specs, loading: false);
    } on ApiError catch (e) {
      state = state.copyWith(loading: false, error: e.message);
    } on StateError catch (e) {
      state = state.copyWith(loading: false, error: e.message);
    }
  }

  void setQuery(String query) => state = state.copyWith(query: query);

  /// 调用函数。[payload] 由表单组装。返回结果经 state 暴露给页面。
  Future<void> invoke({
    required String functionId,
    required Map<String, Object?> payload,
    String mode = '',
    String route = '',
    String targetServiceId = '',
    String hashKey = '',
  }) async {
    if (state.submitting) return;
    state = state.copyWith(
      submitting: true,
      clearInvokeError: true,
      clearInvokeResult: true,
      lastFunctionId: functionId,
    );
    final service = await _invokeServiceAsync();
    if (service == null) {
      state = state.copyWith(submitting: false, invokeError: '未登录或缺少服务器地址');
      return;
    }
    try {
      final result = await service.invoke(
        functionId: functionId,
        payload: payload,
        mode: mode,
        route: route,
        targetServiceId: targetServiceId,
        hashKey: hashKey,
      );
      state = state.copyWith(submitting: false, invokeResult: result);
      if (result.isTask) {
        // 异步任务：立即拉一次并启动 5s 轮询（页面可见时；见 page）。
        state = state.copyWith(
          task: TaskStatus(id: result.taskId, status: 'running'),
        );
        await _pollTaskOnce();
        if (state.taskRunning) _startPolling();
      }
    } on ApiError catch (e) {
      state = state.copyWith(submitting: false, invokeError: e.message);
    } on StateError catch (e) {
      state = state.copyWith(submitting: false, invokeError: e.message);
    }
  }

  /// 页面可见性回调：resumed 启动轮询 / 不可见停止（后台零请求）。
  void onVisibility(bool visible) {
    if (visible) {
      if (state.taskRunning) _startPolling();
    } else {
      _poll?.cancel();
      _poll = null;
    }
  }

  void _startPolling() {
    _poll?.cancel();
    _poll = Timer.periodic(const Duration(seconds: 5), (_) => _pollTaskOnce());
  }

  Future<void> _pollTaskOnce() async {
    final current = state.task;
    if (current == null || !current.isRunning) {
      _poll?.cancel();
      _poll = null;
      return;
    }
    final service = await _invokeServiceAsync();
    if (service == null) return;
    try {
      final task = await service.taskDetail(current.id);
      state = state.copyWith(task: task);
      if (task.isDone) {
        _poll?.cancel();
        _poll = null;
      }
    } on ApiError {
      // 单次轮询失败不中断：下一 tick 重试（终态由用户手动刷新兜底）。
    } on StateError {
      // 同上。
    }
  }

  /// 取消异步任务（终态由后续轮询/刷新确认）。
  Future<void> cancelTask() async {
    final task = state.task;
    if (task == null) return;
    final service = await _invokeServiceAsync();
    if (service == null) return;
    try {
      await service.cancelTask(task.id);
    } on ApiError catch (e) {
      state = state.copyWith(invokeError: e.message);
    }
    await _pollTaskOnce();
  }

  /// 手动刷新任务状态（轮询失败后的兜底）。
  Future<void> refreshTask() => _pollTaskOnce();

  /// 清空调用结果 / 任务（返回目录或再次调用前）。
  void clearResult() {
    _poll?.cancel();
    _poll = null;
    state = state.copyWith(clearTask: true, clearInvokeResult: true);
  }
}
