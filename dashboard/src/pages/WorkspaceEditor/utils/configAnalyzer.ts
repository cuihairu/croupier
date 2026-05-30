/**
 * Workspace 配置性能分析工具
 *
 * 分析配置复杂度，预估渲染性能，复杂度过高时预警。
 *
 * @module pages/WorkspaceEditor/utils/configAnalyzer
 */

import type { TabConfig, TabLayout, FieldConfig, ColumnConfig } from '@/types/workspace';

export interface PerformanceAnalysisResult {
  score: number; // 0-100，越高越好
  level: 'good' | 'warning' | 'poor';
  metrics: PerformanceMetric[];
  suggestions: string[];
}

export interface PerformanceMetric {
  name: string;
  value: number;
  unit: string;
  threshold: { warning: number; poor: number };
  level: 'good' | 'warning' | 'poor';
}

/**
 * 分析 Tab 配置的性能
 */
export function analyzeTabPerformance(tab: TabConfig): PerformanceAnalysisResult {
  const metrics: PerformanceMetric[] = [];
  const suggestions: string[] = [];

  // 1. 分析函数数量
  const functionCount = tab.functions?.length || 0;
  metrics.push({
    name: '绑定函数数',
    value: functionCount,
    unit: '个',
    threshold: { warning: 5, poor: 10 },
    level: functionCount <= 5 ? 'good' : functionCount <= 10 ? 'warning' : 'poor',
  });
  if (functionCount > 8) {
    suggestions.push('绑定函数过多，建议拆分为多个 Tab 或使用编排向导管理函数调用关系');
  }

  // 2. 分析布局复杂度
  const layoutMetrics = analyzeLayoutComplexity(tab.layout);
  metrics.push(...layoutMetrics.metrics);
  suggestions.push(...layoutMetrics.suggestions);

  // 3. 计算总体得分
  const goodCount = metrics.filter((m) => m.level === 'good').length;
  const warningCount = metrics.filter((m) => m.level === 'warning').length;
  const poorCount = metrics.filter((m) => m.level === 'poor').length;

  const score = Math.max(0, Math.min(100, 100 - warningCount * 10 - poorCount * 25));

  let level: 'good' | 'warning' | 'poor' = 'good';
  if (poorCount > 0 || warningCount >= 2) {
    level = 'poor';
  } else if (warningCount > 0 || score < 90) {
    level = 'warning';
  }

  return {
    score,
    level,
    metrics,
    suggestions,
  };
}

/**
 * 分析布局复杂度
 */
function analyzeLayoutComplexity(layout: TabLayout): {
  metrics: PerformanceMetric[];
  suggestions: string[];
} {
  const metrics: PerformanceMetric[] = [];
  const suggestions: string[] = [];

  switch (layout.type) {
    case 'list':
      const listLayout = layout as any;
      const columnCount = listLayout.columns?.length || 0;
      metrics.push({
        name: '列表列数',
        value: columnCount,
        unit: '列',
        threshold: { warning: 8, poor: 15 },
        level: columnCount <= 8 ? 'good' : columnCount <= 15 ? 'warning' : 'poor',
      });
      if (columnCount > 12) {
        suggestions.push('列表列数过多会影响渲染性能和用户体验，建议隐藏次要列或使用详情页展示');
      }

      // 分析列配置复杂度
      let totalComplexity = 0;
      listLayout.columns?.forEach((col: ColumnConfig) => {
        if (col.render === 'custom') totalComplexity += 3;
        else if (col.render && col.render !== 'text') totalComplexity += 1;
        if (col.visibleCondition) totalComplexity += 2;
      });
      metrics.push({
        name: '列渲染复杂度',
        value: totalComplexity,
        unit: '分',
        threshold: { warning: 15, poor: 30 },
        level: totalComplexity <= 15 ? 'good' : totalComplexity <= 30 ? 'warning' : 'poor',
      });
      if (totalComplexity > 25) {
        suggestions.push('列渲染复杂度较高，建议简化自定义渲染和条件显隐逻辑');
      }
      break;

    case 'form':
    case 'form-detail':
      const formLayout = layout as any;
      const fields = formLayout.fields || formLayout.queryFields || [];
      const fieldCount = fields.length;
      metrics.push({
        name: '表单字段数',
        value: fieldCount,
        unit: '个',
        threshold: { warning: 15, poor: 30 },
        level: fieldCount <= 15 ? 'good' : fieldCount <= 30 ? 'warning' : 'poor',
      });
      if (fieldCount > 25) {
        suggestions.push('表单字段数过多，建议分组展示或拆分为多个步骤（向导模式）');
      }

      // 分析字段配置复杂度
      let formComplexity = 0;
      fields.forEach((field: FieldConfig) => {
        if (field.rules && field.rules.length > 0) formComplexity += field.rules.length;
        if (field.visibleWhen) formComplexity += 2;
        if (field.disabledWhen) formComplexity += 2;
        if (field.defaultValueExpression) formComplexity += 2;
        if (['select', 'radio', 'checkbox'].includes(field.type)) {
          formComplexity += (field.options?.length || 0) * 0.5;
        }
      });
      metrics.push({
        name: '表单交互复杂度',
        value: Math.round(formComplexity),
        unit: '分',
        threshold: { warning: 30, poor: 60 },
        level: formComplexity <= 30 ? 'good' : formComplexity <= 60 ? 'warning' : 'poor',
      });
      if (formComplexity > 50) {
        suggestions.push('表单交互逻辑复杂，建议简化联动规则或使用分组降低认知负担');
      }
      break;

    case 'detail':
      const detailLayout = layout as any;
      const sectionCount = detailLayout.sections?.length || 0;
      const detailFieldCount = (detailLayout.sections || []).reduce(
        (sum: number, s: any) => sum + (s.fields?.length || 0),
        0,
      );
      metrics.push({
        name: '详情分区数',
        value: sectionCount,
        unit: '个',
        threshold: { warning: 6, poor: 12 },
        level: sectionCount <= 6 ? 'good' : sectionCount <= 12 ? 'warning' : 'poor',
      });
      metrics.push({
        name: '详情字段数',
        value: detailFieldCount,
        unit: '个',
        threshold: { warning: 20, poor: 40 },
        level: detailFieldCount <= 20 ? 'good' : detailFieldCount <= 40 ? 'warning' : 'poor',
      });
      if (detailFieldCount > 35) {
        suggestions.push('详情字段数过多，建议使用折叠分组或分页展示');
      }
      break;

    case 'kanban':
    case 'timeline':
    case 'wizard':
    case 'dashboard':
    case 'grid':
      metrics.push({
        name: '布局类型',
        value: 1,
        unit: '',
        threshold: { warning: 1, poor: 2 },
        level: 'warning',
      });
      suggestions.push(`${layout.type} 布局属于高级布局，建议测试不同数据量下的性能表现`);
      break;
  }

  return { metrics, suggestions };
}

/**
 * 分析多个 Tab 的总体性能
 */
export function analyzeMultipleTabsPerformance(tabs: TabConfig[]): {
  overall: PerformanceAnalysisResult;
  byTab: Record<string, PerformanceAnalysisResult>;
} {
  const byTab: Record<string, PerformanceAnalysisResult> = {};
  let totalScore = 0;
  let allSuggestions: string[] = [];
  const tabCount = tabs.length;

  tabs.forEach((tab) => {
    const result = analyzeTabPerformance(tab);
    byTab[tab.key] = result;
    totalScore += result.score;
    result.suggestions.forEach((s) => {
      if (!allSuggestions.includes(s)) {
        allSuggestions.push(s);
      }
    });
  });

  const avgScore = tabCount > 0 ? Math.round(totalScore / tabCount) : 100;

  let level: 'good' | 'warning' | 'poor' = 'good';
  if (avgScore < 60) {
    level = 'poor';
  } else if (avgScore < 80) {
    level = 'warning';
  }

  return {
    overall: {
      score: avgScore,
      level,
      metrics: [
        {
          name: 'Tab 总数',
          value: tabCount,
          unit: '个',
          threshold: { warning: 8, poor: 15 },
          level: tabCount <= 8 ? 'good' : tabCount <= 15 ? 'warning' : 'poor',
        },
        {
          name: '平均性能得分',
          value: avgScore,
          unit: '分',
          threshold: { warning: 80, poor: 60 },
          level: avgScore >= 80 ? 'good' : avgScore >= 60 ? 'warning' : 'poor',
        },
      ],
      suggestions: allSuggestions,
    },
    byTab,
  };
}

/**
 * 获取性能等级的颜色
 */
export function getPerformanceLevelColor(level: 'good' | 'warning' | 'poor'): string {
  switch (level) {
    case 'good':
      return '#52c41a';
    case 'warning':
      return '#faad14';
    case 'poor':
      return '#ff4d4f';
    default:
      return '#d9d9d9';
  }
}

/**
 * 获取性能等级的文本
 */
export function getPerformanceLevelText(level: 'good' | 'warning' | 'poor'): string {
  switch (level) {
    case 'good':
      return '良好';
    case 'warning':
      return '一般';
    case 'poor':
      return '较差';
    default:
      return '未知';
  }
}
