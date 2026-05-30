/**
 * 编辑操作审计日志
 *
 * 记录所有编辑操作，支持查询和导出。
 *
 * @module pages/WorkspaceEditor/utils/auditLogger
 */

import type { WorkspaceConfig } from '@/types/workspace';

/** 审计日志条目 */
export interface AuditLogEntry {
  /** 唯一标识 */
  id: string;
  /** 操作时间戳 */
  timestamp: number;
  /** 操作时间 ISO 字符串 */
  timestampISO: string;
  /** 操作类型 */
  action: AuditAction;
  /** 操作者 */
  actor?: string;
  /** 对象标识 */
  objectKey: string;
  /** 变更描述 */
  description: string;
  /** 变更详情（JSON） */
  details?: {
    /** 变更前（部分字段） */
    before?: Record<string, any>;
    /** 变更后（部分字段） */
    after?: Record<string, any>;
    /** 变更的字段列表 */
    changedFields?: string[];
  };
  /** IP 地址 */
  ipAddress?: string;
  /** 用户代理 */
  userAgent?: string;
}

/** 操作类型 */
export type AuditAction =
  | 'create' // 创建
  | 'update' // 更新
  | 'delete' // 删除
  | 'publish' // 发布
  | 'rollback' // 回滚
  | 'import' // 导入
  | 'export' // 导出
  | 'template_apply' // 应用模板
  | 'validate'; // 校验

/** 日志查询选项 */
export interface AuditLogQuery {
  /** 对象标识 */
  objectKey?: string;
  /** 操作类型 */
  action?: AuditAction;
  /** 操作者 */
  actor?: string;
  /** 开始时间 */
  startTime?: number;
  /** 结束时间 */
  endTime?: number;
  /** 最大返回数量 */
  limit?: number;
}

/** 日志统计 */
export interface AuditLogStats {
  totalEntries: number;
  actionsByType: Record<string, number>;
  entriesByObject: Record<string, number>;
  entriesByActor: Record<string, number>;
  recentActivity: AuditLogEntry[];
}

const STORAGE_KEY = 'workspace:audit:logs';
const MAX_LOGS = 1000; // 最多保留 1000 条日志

/**
 * 记录审计日志
 */
export function logAuditEvent(
  entry: Omit<AuditLogEntry, 'id' | 'timestamp' | 'timestampISO'>,
): AuditLogEntry {
  const logEntry: AuditLogEntry = {
    ...entry,
    id: `audit_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`,
    timestamp: Date.now(),
    timestampISO: new Date().toISOString(),
  };

  try {
    const logs = loadAllLogs();
    logs.unshift(logEntry);

    // 限制日志数量
    if (logs.length > MAX_LOGS) {
      logs.splice(MAX_LOGS);
    }

    localStorage.setItem(STORAGE_KEY, JSON.stringify(logs));
  } catch (error) {
    console.error('Failed to save audit log:', error);
  }

  return logEntry;
}

/**
 * 加载所有审计日志
 */
export function loadAllLogs(): AuditLogEntry[] {
  try {
    const data = localStorage.getItem(STORAGE_KEY);
    return data ? JSON.parse(data) : [];
  } catch {
    return [];
  }
}

/**
 * 查询审计日志
 */
export function queryAuditLogs(query: AuditLogQuery): AuditLogEntry[] {
  let logs = loadAllLogs();

  // 按对象过滤
  if (query.objectKey) {
    logs = logs.filter((log) => log.objectKey === query.objectKey);
  }

  // 按操作类型过滤
  if (query.action) {
    logs = logs.filter((log) => log.action === query.action);
  }

  // 按操作者过滤
  if (query.actor) {
    logs = logs.filter((log) => log.actor === query.actor);
  }

  // 按时间范围过滤
  if (query.startTime) {
    logs = logs.filter((log) => log.timestamp >= query.startTime!);
  }
  if (query.endTime) {
    logs = logs.filter((log) => log.timestamp <= query.endTime!);
  }

  // 限制返回数量
  if (query.limit) {
    logs = logs.slice(0, query.limit);
  }

  return logs;
}

/**
 * 获取日志统计
 */
export function getAuditLogStats(query?: AuditLogQuery): AuditLogStats {
  const logs = query ? queryAuditLogs(query) : loadAllLogs();

  const stats: AuditLogStats = {
    totalEntries: logs.length,
    actionsByType: {},
    entriesByObject: {},
    entriesByActor: {},
    recentActivity: logs.slice(0, 10),
  };

  logs.forEach((log) => {
    // 按操作类型统计
    stats.actionsByType[log.action] = (stats.actionsByType[log.action] || 0) + 1;

    // 按对象统计
    stats.entriesByObject[log.objectKey] = (stats.entriesByObject[log.objectKey] || 0) + 1;

    // 按操作者统计
    if (log.actor) {
      stats.entriesByActor[log.actor] = (stats.entriesByActor[log.actor] || 0) + 1;
    }
  });

  return stats;
}

/**
 * 导出审计日志为 CSV
 */
export function exportAuditLogsAsCSV(query?: AuditLogQuery): string {
  const logs = query ? queryAuditLogs(query) : loadAllLogs();

  const headers = ['时间', '操作', '操作者', '对象', '描述'];
  const rows = logs.map((log) => [
    log.timestampISO,
    getActionText(log.action),
    log.actor || '-',
    log.objectKey,
    log.description,
  ]);

  const csv = [headers, ...rows]
    .map((row) => row.map((cell) => `"${String(cell).replace(/"/g, '""')}"`).join(','))
    .join('\n');

  return csv;
}

/**
 * 导出审计日志为 JSON
 */
export function exportAuditLogsAsJSON(query?: AuditLogQuery): string {
  const logs = query ? queryAuditLogs(query) : loadAllLogs();
  return JSON.stringify(logs, null, 2);
}

/**
 * 清理旧日志（保留指定天数内的日志）
 */
export function cleanupOldLogs(daysToKeep: number = 90): number {
  const cutoffTime = Date.now() - daysToKeep * 24 * 60 * 60 * 1000;
  const logs = loadAllLogs();
  const filtered = logs.filter((log) => log.timestamp >= cutoffTime);

  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(filtered));
  } catch (error) {
    console.error('Failed to cleanup audit logs:', error);
  }

  return logs.length - filtered.length;
}

/**
 * 获取操作类型的显示文本
 */
export function getActionText(action: AuditAction): string {
  const actionTexts: Record<AuditAction, string> = {
    create: '创建',
    update: '更新',
    delete: '删除',
    publish: '发布',
    rollback: '回滚',
    import: '导入',
    export: '导出',
    template_apply: '应用模板',
    validate: '校验',
  };
  return actionTexts[action] || action;
}

/**
 * 获取操作类型的颜色
 */
export function getActionColor(action: AuditAction): string {
  const colors: Record<AuditAction, string> = {
    create: 'green',
    update: 'blue',
    delete: 'red',
    publish: 'purple',
    rollback: 'orange',
    import: 'cyan',
    export: 'geekblue',
    template_apply: 'magenta',
    validate: 'default',
  };
  return colors[action] || 'default';
}

/**
 * 获取操作类型图标
 */
export function getActionIcon(action: AuditAction): string {
  const icons: Record<AuditAction, string> = {
    create: '➕',
    update: '✏️',
    delete: '🗑️',
    publish: '🚀',
    rollback: '⏪',
    import: '📥',
    export: '📤',
    template_apply: '📋',
    validate: '✅',
  };
  return icons[action] || '•';
}

/**
 * 批量记录变更（用于一次操作影响多个对象）
 */
export function logBatchAuditEvents(
  entries: Omit<AuditLogEntry, 'id' | 'timestamp' | 'timestampISO'>[],
): AuditLogEntry[] {
  return entries.map((entry) => logAuditEvent(entry));
}

/**
 * 记录配置保存事件
 */
export function logConfigSave(
  objectKey: string,
  oldConfig: WorkspaceConfig | null,
  newConfig: WorkspaceConfig,
  actor?: string,
): AuditLogEntry {
  // 计算变更的字段
  const changedFields: string[] = [];

  if (oldConfig) {
    if (oldConfig.title !== newConfig.title) changedFields.push('title');
    if (oldConfig.description !== newConfig.description) changedFields.push('description');
    if (oldConfig.published !== newConfig.published) changedFields.push('published');
    if (oldConfig.layout?.type !== newConfig.layout?.type) changedFields.push('layoutType');
  }

  return logAuditEvent({
    action: oldConfig ? 'update' : 'create',
    objectKey,
    actor,
    description: oldConfig ? '更新配置' : '创建配置',
    details: {
      before: oldConfig
        ? { title: oldConfig.title, layoutType: oldConfig.layout?.type }
        : undefined,
      after: { title: newConfig.title, layoutType: newConfig.layout?.type },
      changedFields,
    },
  });
}

/**
 * 记录配置发布事件
 */
export function logConfigPublish(objectKey: string, actor?: string): AuditLogEntry {
  return logAuditEvent({
    action: 'publish',
    objectKey,
    actor,
    description: '发布配置',
  });
}

/**
 * 记录配置回滚事件
 */
export function logConfigRollback(
  objectKey: string,
  fromVersion: string,
  toVersion: string,
  actor?: string,
): AuditLogEntry {
  return logAuditEvent({
    action: 'rollback',
    objectKey,
    actor,
    description: `回滚配置: v${fromVersion} → v${toVersion}`,
  });
}

/**
 * 记录模板应用事件
 */
export function logTemplateApply(
  objectKey: string,
  templateName: string,
  actor?: string,
): AuditLogEntry {
  return logAuditEvent({
    action: 'template_apply',
    objectKey,
    actor,
    description: `应用模板: ${templateName}`,
  });
}

/**
 * 记录导入事件
 */
export function logConfigImport(
  objectKey: string,
  sourceObjectKey?: string,
  actor?: string,
): AuditLogEntry {
  return logAuditEvent({
    action: 'import',
    objectKey,
    actor,
    description: sourceObjectKey ? `导入配置: 从 ${sourceObjectKey}` : '导入配置',
  });
}

/**
 * 记录导出事件
 */
export function logConfigExport(
  objectKey: string,
  exportType: 'config' | 'published' | 'metadata' | 'backup',
  actor?: string,
): AuditLogEntry {
  const typeTexts: Record<typeof exportType, string> = {
    config: '配置',
    published: '已发布配置',
    metadata: '元信息',
    backup: '备份包',
  };

  return logAuditEvent({
    action: 'export',
    objectKey,
    actor,
    description: `导出${typeTexts[exportType]}`,
  });
}
