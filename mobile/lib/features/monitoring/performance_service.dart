/// 性能快照服务（设计稿 §2.2）：GET /ops/performance（scoped）。
/// 响应契约：{settings, runtime, host, overload}——移动端只消费
/// runtime / host / overload 三块；settings 为服务端参数回显不展示。
library;

import '../../core/api/api_client.dart';

class PerformanceSnapshot {
  const PerformanceSnapshot({
    required this.runtime,
    required this.host,
    required this.overload,
  });

  final PerformanceRuntime runtime;
  final PerformanceHost host;
  final PerformanceOverload overload;

  /// 形态不合法返 null（runtime/host/overload 任一缺块即视为非法，
  /// 防非契约体被当快照渲染）；块内单字段缺失按零值容错。
  static PerformanceSnapshot? fromJson(Map<String, Object?> json) {
    final runtime = json['runtime'];
    final host = json['host'];
    final overload = json['overload'];
    if (runtime is! Map || host is! Map || overload is! Map) return null;
    return PerformanceSnapshot(
      runtime: PerformanceRuntime.fromJson(Map<String, Object?>.from(runtime)),
      host: PerformanceHost.fromJson(Map<String, Object?>.from(host)),
      overload: PerformanceOverload.fromJson(
        Map<String, Object?>.from(overload),
      ),
    );
  }
}

class PerformanceRuntime {
  const PerformanceRuntime({
    this.goroutines = 0,
    this.heapAllocBytes = 0,
    this.numGC = 0,
    this.gcPauseMs = 0,
    this.uptimeSeconds = 0,
  });

  final int goroutines;
  final int heapAllocBytes;
  final int numGC;
  final double gcPauseMs;
  final int uptimeSeconds;

  static PerformanceRuntime fromJson(Map<String, Object?> json) {
    return PerformanceRuntime(
      goroutines: json['goroutines'] is int ? json['goroutines'] as int : 0,
      heapAllocBytes: json['heapAllocBytes'] is int
          ? json['heapAllocBytes'] as int
          : 0,
      numGC: json['numGC'] is int ? json['numGC'] as int : 0,
      gcPauseMs: (json['gcPauseMs'] as num?)?.toDouble() ?? 0,
      uptimeSeconds: json['uptimeSeconds'] is int
          ? json['uptimeSeconds'] as int
          : 0,
    );
  }
}

class PerformanceHost {
  const PerformanceHost({
    this.cpuPercent = 0,
    this.memoryUsedPct = 0,
    this.memoryTotalBytes = 0,
    this.memoryUsedBytes = 0,
    this.diskUsedPct = 0,
    this.diskTotalBytes = 0,
    this.diskUsedBytes = 0,
  });

  final double cpuPercent;
  final double memoryUsedPct;
  final int memoryTotalBytes;
  final int memoryUsedBytes;
  final double diskUsedPct;
  final int diskTotalBytes;
  final int diskUsedBytes;

  static PerformanceHost fromJson(Map<String, Object?> json) {
    return PerformanceHost(
      cpuPercent: (json['cpuPercent'] as num?)?.toDouble() ?? 0,
      memoryUsedPct: (json['memoryUsedPct'] as num?)?.toDouble() ?? 0,
      memoryTotalBytes: json['memoryTotalBytes'] is int
          ? json['memoryTotalBytes'] as int
          : 0,
      memoryUsedBytes: json['memoryUsedBytes'] is int
          ? json['memoryUsedBytes'] as int
          : 0,
      diskUsedPct: (json['diskUsedPct'] as num?)?.toDouble() ?? 0,
      diskTotalBytes: json['diskTotalBytes'] is int
          ? json['diskTotalBytes'] as int
          : 0,
      diskUsedBytes: json['diskUsedBytes'] is int
          ? json['diskUsedBytes'] as int
          : 0,
    );
  }
}

class PerformanceOverload {
  const PerformanceOverload({
    this.cpu = false,
    this.memory = false,
    this.disk = false,
  });

  final bool cpu;
  final bool memory;
  final bool disk;

  static PerformanceOverload fromJson(Map<String, Object?> json) {
    return PerformanceOverload(
      cpu: json['cpu'] == true,
      memory: json['memory'] == true,
      disk: json['disk'] == true,
    );
  }
}

class PerformanceService {
  PerformanceService({required this.client});

  static const path = '/api/v1/ops/performance';

  final ApiClient client;

  /// 快照。响应形态不合法 fail-fast（防 SPA HTML 兜底当快照）。
  Future<PerformanceSnapshot> fetch() async {
    final data = await client.get<Map<String, Object?>>(path);
    final parsed = PerformanceSnapshot.fromJson(data);
    if (parsed == null) {
      throw StateError('invalid performance payload');
    }
    return parsed;
  }
}
