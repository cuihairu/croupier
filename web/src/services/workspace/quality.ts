import type { WorkspaceConfig } from '@/types/workspace';
import { validateWorkspaceConfig } from '@/services/workspaceConfig';
import { validateTabConfig } from '@/pages/WorkspaceEditor/utils/configValidator';

const STABLE_TAB_LAYOUT_TYPES = new Set(['form-detail', 'list', 'form', 'detail']);
const BETA_TAB_LAYOUT_TYPES = new Set(['kanban', 'timeline', 'split', 'wizard', 'dashboard', 'grid']);
const EXPERIMENTAL_TAB_LAYOUT_TYPES = new Set(['custom', 'single']);

export type WorkspaceQualityItem = {
  level: 'blocking' | 'warning' | 'ready';
  title: string;
  detail: string;
};

export type WorkspaceTabQualityResult = {
  key: string;
  title: string;
  layoutType: string;
  level: 'blocking' | 'warning' | 'ready';
  summary: string;
  items: WorkspaceQualityItem[];
};

export type WorkspaceQualityReport = {
  score: number;
  headline: string;
  blockingCount: number;
  warningCount: number;
  readyCount: number;
  readyTabCount: number;
  pendingTabCount: number;
  summary: string;
  items: WorkspaceQualityItem[];
  tabs: WorkspaceTabQualityResult[];
};

export function buildWorkspaceQualityReport(
  config: WorkspaceConfig,
  availableFunctions: any[] = [],
): WorkspaceQualityReport {
  const allFunctionIds = (availableFunctions || [])
    .map((fn: any) => String(fn?.id || ''))
    .filter(Boolean);
  const workspaceValidation = validateWorkspaceConfig(config);
  const reportItems: WorkspaceQualityItem[] = [];

  if (workspaceValidation.errors.length > 0) {
    workspaceValidation.errors.slice(0, 8).forEach((error) => {
      reportItems.push({
        level: 'blocking',
        title: '基础配置未通过',
        detail: error,
      });
    });
  } else {
    reportItems.push({
      level: 'ready',
      title: '基础结构已通过',
      detail: '对象标识、标题、顶层 tabs 结构和基础布局字段已通过校验。',
    });
  }

  const tabs =
    config.layout?.type === 'tabs' && Array.isArray(config.layout.tabs) ? config.layout.tabs : [];

  const tabReports: WorkspaceTabQualityResult[] = tabs.map((tab) => {
    const layout = (tab.layout || {}) as any;
    const layoutType = String(layout.type || '-');
    const validation = validateTabConfig(tab, allFunctionIds);
    const items: WorkspaceQualityItem[] = [];
    const primaryFunction = tab.functions?.[0] || '';
    const hasCoreFunction = Boolean(
      layout.listFunction ||
        layout.submitFunction ||
        layout.detailFunction ||
        layout.queryFunction ||
        layout.dataFunction,
    );

    if (!primaryFunction) {
      items.push({
        level: 'blocking',
        title: '缺少主函数',
        detail: '当前页面还没有绑定主函数，无法稳定生成页面骨架与数据上下文。',
      });
    }

    if (!hasCoreFunction && STABLE_TAB_LAYOUT_TYPES.has(layoutType)) {
      items.push({
        level: 'blocking',
        title: '缺少核心函数',
        detail: `当前 ${layoutType} 页面还没有配置核心函数，发布后主流程会直接断掉。`,
      });
    }

    if (BETA_TAB_LAYOUT_TYPES.has(layoutType)) {
      items.push({
        level: 'warning',
        title: '使用 Beta 布局',
        detail: `${layoutType} 已可渲染，但仍属于 Beta 能力，发布前应重点核对真实数据态与空态。`,
      });
    }

    if (EXPERIMENTAL_TAB_LAYOUT_TYPES.has(layoutType)) {
      items.push({
        level: 'warning',
        title: '使用实验布局',
        detail: `${layoutType} 仍属实验能力，不建议作为默认主路径直接发布。`,
      });
    }

    if (
      ['kanban', 'timeline', 'split', 'wizard', 'dashboard', 'grid'].includes(layoutType) &&
      !layout.dataFunction &&
      !layout.listFunction &&
      !layout.detailFunction &&
      !layout.queryFunction &&
      !layout.submitFunction
    ) {
      items.push({
        level: 'warning',
        title: '可能仍依赖预览示例数据',
        detail: '当前高级布局没有绑定明确数据函数，运行态可能只能看到预览感很强的占位结果。',
      });
    }

    validation.errors.slice(0, 5).forEach((error) => {
      items.push({
        level: 'blocking',
        title: '页面配置未通过',
        detail: `${error.path}: ${error.message}`,
      });
    });

    validation.warnings.slice(0, 3).forEach((warning) => {
      items.push({
        level: 'warning',
        title: '页面配置提醒',
        detail: `${warning.path}: ${warning.message}`,
      });
    });

    if (items.length === 0) {
      items.push({
        level: 'ready',
        title: '页面已具备发布基础',
        detail: '主函数、核心函数和页面骨架已经齐备，可以继续检查预览与发布影响。',
      });
    }

    const hasBlocking = items.some((item) => item.level === 'blocking');
    const hasWarning = items.some((item) => item.level === 'warning');
    return {
      key: tab.key,
      title: tab.title || tab.key,
      layoutType,
      level: hasBlocking ? 'blocking' : hasWarning ? 'warning' : 'ready',
      summary: hasBlocking
        ? `${items.filter((item) => item.level === 'blocking').length} 个阻塞项待处理`
        : hasWarning
        ? `${items.filter((item) => item.level === 'warning').length} 个风险提示`
        : '已通过发布前检查',
      items,
    };
  });

  if (tabReports.length === 0) {
    reportItems.push({
      level: 'blocking',
      title: '缺少可见页面',
      detail: '当前工作台还没有任何 Tab，发布后不会形成可访问的业务入口。',
    });
  }

  const blockingCount =
    reportItems.filter((item) => item.level === 'blocking').length +
    tabReports.reduce((count, tab) => count + tab.items.filter((item) => item.level === 'blocking').length, 0);
  const warningCount =
    reportItems.filter((item) => item.level === 'warning').length +
    tabReports.reduce((count, tab) => count + tab.items.filter((item) => item.level === 'warning').length, 0);
  const readyCount =
    reportItems.filter((item) => item.level === 'ready').length +
    tabReports.reduce((count, tab) => count + tab.items.filter((item) => item.level === 'ready').length, 0);
  const readyTabCount = tabReports.filter((tab) => tab.level === 'ready').length;
  const pendingTabCount = tabReports.length - readyTabCount;
  const score = Math.max(0, Math.min(100, 100 - blockingCount * 18 - warningCount * 7));

  let headline = '可以进入发布流程';
  let summary = '当前草稿已具备基本发布条件，下一步重点核对预览和变更影响。';
  if (blockingCount > 0) {
    headline = '暂时不要发布';
    summary = `还有 ${blockingCount} 个阻塞项未处理，先回编辑区补齐主函数、核心函数或页面骨架。`;
  } else if (warningCount > 0) {
    headline = '可以发布，但有明显风险';
    summary = `没有硬阻塞，但还有 ${warningCount} 个风险提示，建议先做一次预览和运行态核对。`;
  }

  return {
    score,
    headline,
    blockingCount,
    warningCount,
    readyCount,
    readyTabCount,
    pendingTabCount,
    summary,
    items: reportItems,
    tabs: tabReports,
  };
}
