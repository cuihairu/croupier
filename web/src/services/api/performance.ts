import { request } from '@umijs/max';

// Source: internal/platform/settings/layered.go PerformanceSettingsSnapshot
export type PerformanceSettingsSnapshot = {
  /** 均为 0 = 不启用该限制 */
  maxCpuPct: number;
  maxMemoryPct: number;
  maxDiskPct: number;
  maxConcurrent: number;
  maxThreadCount: number;
  /** 字节；0 = 沿用默认 */
  cacheSize: number;
  sources: Record<string, string>;
};

// Source: internal/api/ops/performance.go PerformanceRuntime / PerformanceHost / PerformanceOverload
export type PerformanceRuntime = {
  goMaxProcs: number;
  goroutines: number;
  heapAllocBytes: number;
  heapSysBytes: number;
  sysBytes: number;
  numGC: number;
  gcPauseMs: number;
  uptimeSeconds: number;
};

export type PerformanceHost = {
  cpuPercent: number;
  memoryUsedPct: number;
  memoryTotalBytes: number;
  memoryUsedBytes: number;
  diskPath: string;
  diskUsedPct: number;
  diskTotalBytes: number;
  diskUsedBytes: number;
};

export type PerformanceOverload = {
  cpu: boolean;
  memory: boolean;
  disk: boolean;
};

export type PerformanceSnapshot = {
  settings: PerformanceSettingsSnapshot;
  runtime: PerformanceRuntime;
  host: PerformanceHost;
  overload: PerformanceOverload;
};

// GET /api/v1/ops/performance —— 性能参数生效值 + 运行时快照（OPEN-ISSUES #53）
export async function fetchPerformance(): Promise<PerformanceSnapshot> {
  return request<PerformanceSnapshot>('/api/v1/ops/performance', { skipErrorHandler: true });
}

// PUT /api/v1/ops/performance —— 逐键写 perf.* L3 覆盖，返回更新后快照。
// 阈值仅注记不拦截请求（诚实边界见端点注释）。
export async function savePerformance(patch: Record<string, number>): Promise<PerformanceSnapshot> {
  return request<PerformanceSnapshot>('/api/v1/ops/performance', {
    method: 'PUT',
    data: patch,
    skipErrorHandler: true,
  });
}
