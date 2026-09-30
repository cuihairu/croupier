/// 设备列表控制器（设计稿 §2.3）：整表加载 / 刷新 / label 客户端过滤。
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/providers.dart';
import '../../core/api/api_error.dart';
import 'node_service.dart';

class DevicesState {
  const DevicesState({
    this.nodes = const [],
    this.loading = false,
    this.error,
    this.selectedLabels = const {},
  });

  final List<NodeDevice> nodes;
  final bool loading;
  final String? error;

  /// 选中的 label 芯片（'k=v' 形式），客户端过滤。
  final Set<String> selectedLabels;

  DevicesState copyWith({
    List<NodeDevice>? nodes,
    bool? loading,
    String? error,
    bool clearError = false,
    Set<String>? selectedLabels,
  }) {
    return DevicesState(
      nodes: nodes ?? this.nodes,
      loading: loading ?? this.loading,
      error: clearError ? null : (error ?? this.error),
      selectedLabels: selectedLabels ?? this.selectedLabels,
    );
  }

  /// 全部出现过的 label 芯片（去重排序）。
  List<String> get allLabels {
    final chips = <String>{};
    for (final node in nodes) {
      for (final entry in node.labels.entries) {
        chips.add('${entry.key}=${entry.value}');
      }
    }
    return chips.toList()..sort();
  }

  /// label 过滤后的设备列表（全部选中芯片都命中才保留）。
  List<NodeDevice> get filteredNodes {
    if (selectedLabels.isEmpty) return nodes;
    bool matches(NodeDevice node) {
      for (final chip in selectedLabels) {
        final idx = chip.indexOf('=');
        if (idx <= 0) return false;
        if (node.labels[chip.substring(0, idx)] != chip.substring(idx + 1)) {
          return false;
        }
      }
      return true;
    }

    return nodes.where(matches).toList();
  }
}

final devicesControllerProvider =
    NotifierProvider<DevicesController, DevicesState>(DevicesController.new);

class DevicesController extends Notifier<DevicesState> {
  @override
  DevicesState build() => const DevicesState();

  Future<NodeService?> _serviceAsync() async {
    final session = await ref.read(sessionStoreProvider).load();
    final serverUrl = session?.serverUrl ?? '';
    if (serverUrl.isEmpty) return null;
    return NodeService(client: ref.read(apiClientFactoryProvider)(serverUrl));
  }

  Future<void> refresh() async {
    if (state.loading) return;
    final service = await _serviceAsync();
    if (service == null) {
      state = state.copyWith(loading: false, error: '未登录或缺少服务器地址');
      return;
    }
    state = state.copyWith(loading: true, clearError: true);
    try {
      final nodes = await service.list();
      state = state.copyWith(nodes: nodes, loading: false);
    } on ApiError catch (e) {
      state = state.copyWith(loading: false, error: e.message);
    } on StateError catch (e) {
      state = state.copyWith(loading: false, error: e.message);
    }
  }

  /// 切换 label 芯片选中态（客户端过滤，不重查）。
  void toggleLabel(String chip) {
    final next = {...state.selectedLabels};
    if (!next.remove(chip)) next.add(chip);
    state = state.copyWith(selectedLabels: next);
  }
}
