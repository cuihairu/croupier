/// 设备（Agent）服务（设计稿 §2.3）：GET /ops/nodes（整表）+
/// GET /ops/nodes/:nodeId（单设备，未命中 404）。全部 scoped。
library;

import '../../core/api/api_client.dart';
import '../monitoring/monitoring_format.dart';

/// 状态徽标四档归并（设计稿 §2.3）：
/// active/online→在线，drained→排空，stale→异常，offline→离线。
enum DeviceBand { online, drained, degraded, offline, unknown }

class NodeDevice {
  const NodeDevice({
    required this.id,
    this.hostname = '',
    this.addr = '',
    this.gameId = '',
    this.env = '',
    this.status = '',
    this.labels = const {},
    this.lastSeen = '',
    this.sdkLanguage = '',
    this.sdkVersion = '',
    this.sdkName = '',
    this.version = '',
    this.functions = 0,
    this.expiresInSec = 0,
    this.cpu,
    this.memory,
    this.disks = const [],
  });

  final String id;
  final String hostname;
  final String addr;
  final String gameId;
  final String env;
  final String status;
  final Map<String, String> labels;
  final String lastSeen;
  final String sdkLanguage;
  final String sdkVersion;
  final String sdkName;
  final String version;
  final int functions;
  final int expiresInSec;
  final DeviceCpu? cpu;
  final DeviceMemory? memory;
  final List<DeviceDisk> disks;

  static NodeDevice? fromJson(Map<String, Object?> json) {
    final id = json['id'];
    if (id is! String || id.isEmpty) return null;
    String str(String key) => json[key] is String ? json[key] as String : '';
    final labelsRaw = json['labels'];
    final labels = <String, String>{};
    if (labelsRaw is Map) {
      labelsRaw.forEach((key, value) {
        if (key is String && value is String) labels[key] = value;
      });
    }
    final disksRaw = json['disks'];
    final disks = <DeviceDisk>[];
    if (disksRaw is List) {
      for (final item in disksRaw) {
        if (item is! Map) continue;
        disks.add(DeviceDisk.fromJson(Map<String, Object?>.from(item)));
      }
    }
    return NodeDevice(
      id: id,
      hostname: str('hostname'),
      addr: str('addr'),
      gameId: str('gameId'),
      env: str('env'),
      status: str('status'),
      labels: labels,
      lastSeen: str('lastSeen'),
      sdkLanguage: str('sdkLanguage'),
      sdkVersion: str('sdkVersion'),
      sdkName: str('sdkName'),
      version: str('version'),
      functions: json['functions'] is int ? json['functions'] as int : 0,
      expiresInSec: json['expiresInSec'] is int
          ? json['expiresInSec'] as int
          : 0,
      cpu: json['cpu'] is Map
          ? DeviceCpu.fromJson(Map<String, Object?>.from(json['cpu'] as Map))
          : null,
      memory: json['memory'] is Map
          ? DeviceMemory.fromJson(
              Map<String, Object?>.from(json['memory'] as Map),
            )
          : null,
      disks: disks,
    );
  }

  DeviceBand get band => switch (status) {
    'active' || 'online' => DeviceBand.online,
    'drained' => DeviceBand.drained,
    'stale' => DeviceBand.degraded,
    'offline' => DeviceBand.offline,
    _ => DeviceBand.unknown,
  };

  /// 版本展示：agent 二进制版本优先，回退 SDK 版本（设计稿 §2.3）。
  String get displayVersion => version.isNotEmpty ? version : sdkVersion;

  String get lastSeenRelative => formatRelativeTime(lastSeen);
}

class DeviceCpu {
  const DeviceCpu({
    this.usagePercent = 0,
    this.cores = 0,
    this.load1m = 0,
    this.load5m = 0,
    this.load15m = 0,
  });

  final double usagePercent;
  final int cores;
  final double load1m;
  final double load5m;
  final double load15m;

  static DeviceCpu fromJson(Map<String, Object?> json) {
    return DeviceCpu(
      usagePercent: (json['usagePercent'] as num?)?.toDouble() ?? 0,
      cores: json['cores'] is int ? json['cores'] as int : 0,
      load1m: (json['load1m'] as num?)?.toDouble() ?? 0,
      load5m: (json['load5m'] as num?)?.toDouble() ?? 0,
      load15m: (json['load15m'] as num?)?.toDouble() ?? 0,
    );
  }
}

class DeviceMemory {
  const DeviceMemory({
    this.totalBytes = 0,
    this.usedBytes = 0,
    this.availableBytes = 0,
    this.usagePercent = 0,
  });

  final int totalBytes;
  final int usedBytes;
  final int availableBytes;
  final double usagePercent;

  static DeviceMemory fromJson(Map<String, Object?> json) {
    int intOf(String key) => json[key] is int ? json[key] as int : 0;
    return DeviceMemory(
      totalBytes: intOf('totalBytes'),
      usedBytes: intOf('usedBytes'),
      availableBytes: intOf('availableBytes'),
      usagePercent: (json['usagePercent'] as num?)?.toDouble() ?? 0,
    );
  }
}

class DeviceDisk {
  const DeviceDisk({
    this.mountPoint = '',
    this.fsType = '',
    this.totalBytes = 0,
    this.usedBytes = 0,
    this.usagePercent = 0,
  });

  final String mountPoint;
  final String fsType;
  final int totalBytes;
  final int usedBytes;
  final double usagePercent;

  static DeviceDisk fromJson(Map<String, Object?> json) {
    int intOf(String key) => json[key] is int ? json[key] as int : 0;
    return DeviceDisk(
      mountPoint: json['mountPoint'] is String
          ? json['mountPoint'] as String
          : '',
      fsType: json['fsType'] is String ? json['fsType'] as String : '',
      totalBytes: intOf('totalBytes'),
      usedBytes: intOf('usedBytes'),
      usagePercent: (json['usagePercent'] as num?)?.toDouble() ?? 0,
    );
  }
}

class NodeService {
  NodeService({required this.client});

  static const basePath = '/api/v1/ops/nodes';

  final ApiClient client;

  /// 设备整表（无分页，端点整表返回；客户端 label 过滤足够）。
  Future<List<NodeDevice>> list() async {
    final data = await client.get<Map<String, Object?>>(basePath);
    final raw = data['nodes'];
    if (raw is! List) {
      throw StateError('invalid nodes payload');
    }
    final nodes = <NodeDevice>[];
    for (final item in raw) {
      if (item is! Map) continue;
      final parsed = NodeDevice.fromJson(Map<String, Object?>.from(item));
      if (parsed != null) nodes.add(parsed);
    }
    return nodes;
  }

  /// 单设备详情（未命中 404 → ApiError）。
  Future<NodeDevice> detail(String nodeId) async {
    final data = await client.get<Map<String, Object?>>('$basePath/$nodeId');
    final raw = data['node'];
    if (raw is! Map) {
      throw StateError('invalid node payload');
    }
    final parsed = NodeDevice.fromJson(Map<String, Object?>.from(raw));
    if (parsed == null) {
      throw StateError('invalid node payload');
    }
    return parsed;
  }
}
