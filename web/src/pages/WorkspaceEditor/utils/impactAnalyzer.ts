/**
 * 配置变更影响分析工具
 *
 * 分析配置变更影响的范围和风险等级。
 *
 * @module pages/WorkspaceEditor/utils/impactAnalyzer
 */

import type { WorkspaceConfig, TabConfig, FieldConfig, ColumnConfig } from '@/types/workspace';

/** 影响级别 */
export type ImpactLevel = 'low' | 'medium' | 'high' | 'critical';

/** 影响类型 */
export type ImpactType =
  | 'field-added' // 新增字段
  | 'field-removed' // 删除字段
  | 'field-modified' // 修改字段
  | 'column-added' // 新增列
  | 'column-removed' // 删除列
  | 'column-modified' // 修改列
  | 'function-changed' // 函数变更
  | 'layout-changed' // 布局变更
  | 'tab-added' // 新增 Tab
  | 'tab-removed' // 删除 Tab
  | 'tab-modified'; // 修改 Tab

/** 影响项 */
export interface ImpactItem {
  type: ImpactType;
  level: ImpactLevel;
  description: string;
  affectedWorkspaces?: string[]; // 受影响的 workspace key 列表
  affectedUsers?: number; // 估计受影响用户数
  recommendation?: string; // 修复建议
}

/** 影响分析结果 */
export interface ImpactAnalysisResult {
  level: ImpactLevel; // 总体影响级别
  score: number; // 影响分数 0-100
  impacts: ImpactItem[]; // 详细影响列表
  summary: string; // 影响摘要
  warnings: string[]; // 警告信息
}

/**
 * 比较两个配置的差异并分析影响
 */
export function analyzeConfigImpact(
  oldConfig: WorkspaceConfig | null,
  newConfig: WorkspaceConfig,
  usageContext?: {
    /** 使用此配置的 workspace 列表（通常只有当前这一个） */
    dependentWorkspaces?: string[];
    /** 估计用户数 */
    estimatedUsers?: number;
  },
): ImpactAnalysisResult {
  const impacts: ImpactItem[] = [];
  const warnings: string[] = [];
  let totalScore = 0;

  // 如果没有旧配置，这是新创建
  if (!oldConfig) {
    return {
      level: 'low',
      score: 0,
      impacts: [],
      summary: '新创建的配置，无影响',
      warnings: [],
    };
  }

  // 分析 layout 变更
  if (oldConfig.layout?.type !== newConfig.layout?.type) {
    impacts.push({
      type: 'layout-changed',
      level: 'high',
      description: `顶层布局从 ${oldConfig.layout?.type || '-'} 变更为 ${
        newConfig.layout?.type || '-'
      }`,
      affectedWorkspaces: [newConfig.objectKey],
      affectedUsers: usageContext?.estimatedUsers,
      recommendation: '布局变更可能导致页面结构大幅变化，建议充分测试',
    });
    totalScore += 40;
  }

  // 分析 Tab 变更
  const oldTabs = oldConfig.layout?.type === 'tabs' ? oldConfig.layout.tabs || [] : [];
  const newTabs = newConfig.layout?.type === 'tabs' ? newConfig.layout.tabs || [] : [];

  const oldTabMap = new Map(oldTabs.map((t) => [t.key, t]));
  const newTabMap = new Map(newTabs.map((t) => [t.key, t]));

  // 检测新增的 Tab
  newTabs.forEach((tab) => {
    if (!oldTabMap.has(tab.key)) {
      impacts.push({
        type: 'tab-added',
        level: 'low',
        description: `新增 Tab: ${tab.title} (${tab.key})`,
        affectedWorkspaces: [newConfig.objectKey],
        affectedUsers: usageContext?.estimatedUsers,
      });
      totalScore += 10;
    }
  });

  // 检测删除的 Tab
  oldTabs.forEach((tab) => {
    if (!newTabMap.has(tab.key)) {
      impacts.push({
        type: 'tab-removed',
        level: 'critical',
        description: `删除 Tab: ${tab.title} (${tab.key})`,
        affectedWorkspaces: [newConfig.objectKey],
        affectedUsers: usageContext?.estimatedUsers,
        recommendation: '删除 Tab 会导致该页面所有功能失效，请确认操作',
      });
      totalScore += 50;
      warnings.push(`关键警告: 即将删除 Tab "${tab.title}"，这可能影响用户访问`);
    }
  });

  // 检测修改的 Tab
  newTabs.forEach((newTab) => {
    const oldTab = oldTabMap.get(newTab.key);
    if (!oldTab) return;

    // 分析 Tab 内的变更
    const tabImpacts = analyzeTabImpact(oldTab, newTab, newConfig.objectKey);
    impacts.push(...tabImpacts);
    totalScore += tabImpacts.reduce((sum, i) => sum + getImpactScore(i.level), 0);
  });

  // 分析函数变更
  const oldFunctions = new Set<string>();
  const newFunctions = new Set<string>();

  oldTabs.forEach((tab) => {
    tab.functions?.forEach((f) => oldFunctions.add(f));
    const layout = tab.layout as any;
    if (layout?.listFunction) oldFunctions.add(layout.listFunction);
    if (layout?.queryFunction) oldFunctions.add(layout.queryFunction);
    if (layout?.submitFunction) oldFunctions.add(layout.submitFunction);
    if (layout?.detailFunction) oldFunctions.add(layout.detailFunction);
    if (layout?.dataFunction) oldFunctions.add(layout.dataFunction);
  });

  newTabs.forEach((tab) => {
    tab.functions?.forEach((f) => newFunctions.add(f));
    const layout = tab.layout as any;
    if (layout?.listFunction) newFunctions.add(layout.listFunction);
    if (layout?.queryFunction) newFunctions.add(layout.queryFunction);
    if (layout?.submitFunction) newFunctions.add(layout.submitFunction);
    if (layout?.detailFunction) newFunctions.add(layout.detailFunction);
    if (layout?.dataFunction) newFunctions.add(layout.dataFunction);
  });

  // 检测函数移除
  oldFunctions.forEach((func) => {
    if (!newFunctions.has(func)) {
      impacts.push({
        type: 'function-changed',
        level: 'high',
        description: `移除函数绑定: ${func}`,
        affectedWorkspaces: [newConfig.objectKey],
        affectedUsers: usageContext?.estimatedUsers,
        recommendation: '移除函数可能导致相关功能无法使用',
      });
      totalScore += 30;
    }
  });

  // 计算总体影响级别
  const level = calculateImpactLevel(totalScore);
  const summary = generateImpactSummary(impacts, level);

  return {
    level,
    score: Math.min(totalScore, 100),
    impacts,
    summary,
    warnings,
  };
}

/**
 * 分析单个 Tab 的变更影响
 */
function analyzeTabImpact(
  oldTab: TabConfig,
  newTab: TabConfig,
  workspaceKey: string,
): ImpactItem[] {
  const impacts: ImpactItem[] = [];
  const oldLayout = oldTab.layout as any;
  const newLayout = newTab.layout as any;

  // 布局类型变更
  if (oldLayout?.type !== newLayout?.type) {
    impacts.push({
      type: 'layout-changed',
      level: 'high',
      description: `Tab "${newTab.title}" 布局类型从 ${oldLayout?.type || '-'} 变更为 ${
        newLayout?.type || '-'
      }`,
      affectedWorkspaces: [workspaceKey],
      recommendation: '布局类型变更会影响整个页面的渲染方式',
    });
  }

  // 分析字段变更
  if (newLayout?.type === 'form' || newLayout?.type === 'form-detail') {
    const oldFields = oldLayout?.fields || oldLayout?.queryFields || [];
    const newFields = newLayout?.fields || newLayout?.queryFields || [];

    const oldFieldMap = new Map(oldFields.map((f: FieldConfig) => [f.key, f]));
    const newFieldMap = new Map(newFields.map((f: FieldConfig) => [f.key, f]));

    // 新增字段
    newFields.forEach((field: FieldConfig) => {
      if (!oldFieldMap.has(field.key)) {
        impacts.push({
          type: 'field-added',
          level: 'low',
          description: `Tab "${newTab.title}" 新增字段: ${field.label || field.key}`,
          affectedWorkspaces: [workspaceKey],
        });
      }
    });

    // 删除字段
    oldFields.forEach((field: FieldConfig) => {
      if (!newFieldMap.has(field.key)) {
        const level = field.required ? 'high' : 'low';
        impacts.push({
          type: 'field-removed',
          level,
          description: `Tab "${newTab.title}" 删除字段: ${field.label || field.key}`,
          affectedWorkspaces: [workspaceKey],
          recommendation: field.required ? '删除必填字段可能导致表单提交失败' : undefined,
        });
      }
    });

    // 修改字段
    newFields.forEach((field: FieldConfig) => {
      const oldField = oldFieldMap.get(field.key);
      if (!oldField) return;

      if (field.type !== oldField.type) {
        impacts.push({
          type: 'field-modified',
          level: 'medium',
          description: `Tab "${newTab.title}" 字段 "${field.label || field.key}" 类型从 ${
            oldField.type
          } 变更为 ${field.type}`,
          affectedWorkspaces: [workspaceKey],
          recommendation: '字段类型变更可能导致数据格式问题',
        });
      }

      if (field.required && !oldField.required) {
        impacts.push({
          type: 'field-modified',
          level: 'medium',
          description: `Tab "${newTab.title}" 字段 "${field.label || field.key}" 变更为必填`,
          affectedWorkspaces: [workspaceKey],
          recommendation: '新增必填要求可能导致现有数据校验失败',
        });
      }

      if (!field.required && oldField.required) {
        impacts.push({
          type: 'field-modified',
          level: 'low',
          description: `Tab "${newTab.title}" 字段 "${field.label || field.key}" 变更为非必填`,
          affectedWorkspaces: [workspaceKey],
        });
      }
    });
  }

  // 分析列变更
  if (newLayout?.type === 'list' || newLayout?.type === 'form-detail') {
    const oldColumns = oldLayout?.columns || [];
    const newColumns = newLayout?.columns || [];

    const oldColumnMap = new Map(oldColumns.map((c: ColumnConfig) => [c.key, c]));
    const newColumnMap = new Map(newColumns.map((c: ColumnConfig) => [c.key, c]));

    // 新增列
    newColumns.forEach((column: ColumnConfig) => {
      if (!oldColumnMap.has(column.key)) {
        impacts.push({
          type: 'column-added',
          level: 'low',
          description: `Tab "${newTab.title}" 新增列: ${column.title || column.key}`,
          affectedWorkspaces: [workspaceKey],
        });
      }
    });

    // 删除列
    oldColumns.forEach((column: ColumnConfig) => {
      if (!newColumnMap.has(column.key)) {
        impacts.push({
          type: 'column-removed',
          level: 'medium',
          description: `Tab "${newTab.title}" 删除列: ${column.title || column.key}`,
          affectedWorkspaces: [workspaceKey],
          recommendation: '删除列可能影响用户查看数据',
        });
      }
    });
  }

  return impacts;
}

/**
 * 获取影响级别对应的分数
 */
function getImpactScore(level: ImpactLevel): number {
  const scores: Record<ImpactLevel, number> = {
    low: 10,
    medium: 25,
    high: 50,
    critical: 80,
  };
  return scores[level];
}

/**
 * 根据总分计算影响级别
 */
function calculateImpactLevel(score: number): ImpactLevel {
  if (score >= 80) return 'critical';
  if (score >= 50) return 'high';
  if (score >= 25) return 'medium';
  return 'low';
}

/**
 * 生成影响摘要
 */
function generateImpactSummary(impacts: ImpactItem[], level: ImpactLevel): string {
  if (impacts.length === 0) {
    return '配置无变更';
  }

  const levelCounts = impacts.reduce((acc, impact) => {
    acc[impact.level] = (acc[impact.level] || 0) + 1;
    return acc;
  }, {} as Record<ImpactLevel, number>);

  const parts: string[] = [];
  if (levelCounts.critical) parts.push(`${levelCounts.critical} 个关键变更`);
  if (levelCounts.high) parts.push(`${levelCounts.high} 个高风险变更`);
  if (levelCounts.medium) parts.push(`${levelCounts.medium} 个中等变更`);
  if (levelCounts.low) parts.push(`${levelCounts.low} 个低风险变更`);

  const levelText: Record<ImpactLevel, string> = {
    low: '低风险',
    medium: '中等风险',
    high: '高风险',
    critical: '关键风险',
  };

  return `检测到 ${impacts.length} 项变更 (${levelText[level]})：${parts.join('、')}`;
}

/**
 * 获取影响级别的颜色
 */
export function getImpactLevelColor(level: ImpactLevel): string {
  const colors: Record<ImpactLevel, string> = {
    low: 'green',
    medium: 'orange',
    high: 'red',
    critical: 'magenta',
  };
  return colors[level];
}

/**
 * 获取影响级别的文本
 */
export function getImpactLevelText(level: ImpactLevel): string {
  const texts: Record<ImpactLevel, string> = {
    low: '低风险',
    medium: '中等风险',
    high: '高风险',
    critical: '关键风险',
  };
  return texts[level];
}

/**
 * 获取影响类型的图标
 */
export function getImpactTypeIcon(type: ImpactType): string {
  const icons: Record<ImpactType, string> = {
    'field-added': '✅',
    'field-removed': '❌',
    'field-modified': '✏️',
    'column-added': '📊',
    'column-removed': '📉',
    'column-modified': '📋',
    'function-changed': '🔄',
    'layout-changed': '🏗️',
    'tab-added': '📑',
    'tab-removed': '🗑️',
    'tab-modified': '🔧',
  };
  return icons[type];
}
