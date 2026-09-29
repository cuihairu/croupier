import { request } from '@umijs/max';

// Source: internal/api/ops/logs.go LogsSettingsView
export type LogsSettingsView = {
  /** log.retentionDays L3 覆盖；0 = 跟随配置文件 */
  retentionDays: number;
  sources: Record<string, string>;
};

// Source: internal/api/ops/logs.go LogsEffectiveView
export type LogsEffectiveView = {
  /** 0 = 永久保留 */
  executionLogDays: number;
  taskLogDays: number;
};

// Source: internal/api/ops/logs.go ServerLogView（只读，轮转参数走配置文件 + 重启）
export type ServerLogView = {
  output: string;
  file: string;
  directory: string;
  maxSizeMB: number;
  maxBackups: number;
  maxAgeDays: number;
  compress: boolean;
  fileCount: number;
};

// Source: internal/api/ops/logs.go LogTableView
export type LogTableView = {
  table: string;
  /** 查询失败/库不可达为 -1 */
  rows: number;
  oldestAt: string;
};

export type LogsSnapshot = {
  settings: LogsSettingsView;
  effective: LogsEffectiveView;
  serverLog: ServerLogView;
  tables: LogTableView[];
};

// GET /api/v1/ops/logs —— 日志维护快照（OPEN-ISSUES #54）
export async function fetchLogsMaintenance(): Promise<LogsSnapshot> {
  return request<LogsSnapshot>('/api/v1/ops/logs', { skipErrorHandler: true });
}

// PUT /api/v1/ops/logs —— 逐键写 log.* L3 覆盖，返回更新后快照
export async function saveLogsSettings(patch: Record<string, number>): Promise<LogsSnapshot> {
  return request<LogsSnapshot>('/api/v1/ops/logs', {
    method: 'PUT',
    data: patch,
    skipErrorHandler: true,
  });
}

export type LogsCleanupScope = 'execution' | 'task' | 'all';

export type LogsCleanupResult = {
  scope: LogsCleanupScope;
  cutoff: string;
  executionLogsDeleted: number;
  taskRunsDeleted: number;
  taskEventsDeleted: number;
};

// POST /api/v1/ops/logs/cleanup —— 按时间手动清理留痕日志（audit 不参与）
export async function cleanupLogs(
  scope: LogsCleanupScope,
  beforeHours: number,
): Promise<LogsCleanupResult> {
  return request<LogsCleanupResult>('/api/v1/ops/logs/cleanup', {
    method: 'POST',
    data: { scope, beforeHours },
    skipErrorHandler: true,
  });
}
