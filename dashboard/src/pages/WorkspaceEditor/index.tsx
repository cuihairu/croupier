import React, { useState, useEffect, useCallback, useRef } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import {
  Alert,
  Badge,
  Button,
  Card,
  Dropdown,
  Grid,
  message,
  Drawer,
  Descriptions,
  Input,
  Collapse,
  List,
  Modal,
  Segmented,
  Select,
  Space,
  Spin,
  Tag,
  Tooltip,
  Typography,
  Row,
  Col,
} from 'antd';
import {
  BarsOutlined,
  CheckCircleOutlined,
  MenuUnfoldOutlined,
  MoreOutlined,
  EyeOutlined,
  SaveOutlined,
  FileOutlined,
  DownloadOutlined,
  UploadOutlined,
  UndoOutlined,
  RedoOutlined,
  HistoryOutlined,
  ThunderboltOutlined,
  BorderOutlined,
  PlayCircleOutlined,
  TeamOutlined,
  FolderOpenOutlined,
} from '@ant-design/icons';
import { useAccess, useParams } from '@umijs/max';
import type { WorkspaceConfig, WorkspaceVersionRecord } from '@/types/workspace';
import {
  getWorkspaceVersionDetail,
  exportWorkspaceBackupBundle,
  exportPublishedWorkspaceConfig,
  exportWorkspaceMetadata,
  exportWorkspaceConfig,
  importWorkspaceConfig,
  loadWorkspaceConfig,
  listWorkspaceVersions,
  rollbackWorkspaceVersion,
  saveWorkspaceConfig,
  validateWorkspaceConfig,
} from '@/services/workspaceConfig';
import { validateTabConfig } from './utils/configValidator';
import { analyzeMultipleTabsPerformance } from './utils/configAnalyzer';
import { trackWorkspaceEvent } from '@/services/workspace/telemetry';
import { getWorkspaceErrorMessage } from '@/services/workspace/errors';
import { listDescriptors } from '@/services/api/functions';
import { buildWorkspaceQualityReport } from '@/services/workspace/quality';
import FunctionList from './components/FunctionList';
import LayoutDesigner from './components/LayoutDesigner';
import ConfigPreview from './components/ConfigPreview';
import TemplateManager, { SHOWCASE_TEMPLATES, type Template } from './components/TemplateManager';
import DiffViewer from './components/DiffViewer';
import PerformancePanel from './components/PerformancePanel';
import LayoutThumbnail, { MultiTabThumbnail } from './components/LayoutThumbnail';
import GuideTour, { shouldShowGuide, QuickGuideButton } from './components/GuideTour';
import ConfigTestTool from './components/ConfigTestTool';
import AuditLogPanel, { AuditLogDetailModal } from './components/AuditLogPanel';
import CollaborationPanel from './components/CollaborationPanel';
import DraftPanel from './components/DraftPanel';
import EditorEmptyState from './components/EditorEmptyState';
import { getCollaborationManager } from './utils/collaborationManager';
import { AutoSaveDraft } from './utils/draftManager';
import { useSimpleHistory } from './hooks/useHistory';

/** 两种模式：2=函数+设计器（默认），3=函数+设计器+预览 */
type ViewMode = 2 | 3;
const STABLE_TAB_LAYOUT_TYPES = new Set(['form-detail', 'list', 'form', 'detail']);
const BETA_TAB_LAYOUT_TYPES = new Set(['kanban', 'timeline', 'split', 'wizard', 'dashboard', 'grid']);
type VersionTimeRange = 'all' | '7d' | '30d' | '90d';

function resolveWorkspaceStatus(config?: WorkspaceConfig | null): string {
  if (!config) return 'unknown';
  if (config.status) return config.status;
  return config.published ? 'published' : 'draft';
}

function summarizeVersion(version: WorkspaceVersionRecord): string {
  const cfg = version.config;
  const status = resolveWorkspaceStatus(cfg);
  const tabs = cfg?.layout?.type === 'tabs' ? cfg.layout.tabs?.length || 0 : 0;
  const topLayout = cfg?.layout?.type || '-';
  return [`状态: ${status}`, `顶层布局: ${topLayout}`, `标签页: ${tabs}`].join(' · ');
}

function summarizeDiff(
  currentConfig: WorkspaceConfig | null,
  targetVersion: WorkspaceVersionRecord,
): string {
  const currentStatus = resolveWorkspaceStatus(currentConfig);
  const targetStatus = resolveWorkspaceStatus(targetVersion.config);
  const currentLayout = currentConfig?.layout?.type || '-';
  const targetLayout = targetVersion.config?.layout?.type || '-';
  const currentTabs =
    currentConfig?.layout?.type === 'tabs' ? currentConfig.layout.tabs?.length || 0 : 0;
  const targetTabs =
    targetVersion.config?.layout?.type === 'tabs'
      ? targetVersion.config.layout.tabs?.length || 0
      : 0;

  const changed: string[] = [];
  if (currentStatus !== targetStatus) changed.push(`状态 ${currentStatus} -> ${targetStatus}`);
  if (currentLayout !== targetLayout) changed.push(`布局 ${currentLayout} -> ${targetLayout}`);
  if (currentTabs !== targetTabs) changed.push(`标签页 ${currentTabs} -> ${targetTabs}`);

  return changed.length > 0 ? changed.join(' · ') : '与当前草稿结构一致';
}

function buildVersionDiff(
  currentConfig: WorkspaceConfig | null,
  targetConfig?: WorkspaceConfig | null,
): {
  fieldChanges: string[];
  layoutChanges: string[];
  tabChanges: string[];
} {
  if (!currentConfig || !targetConfig) {
    return {
      fieldChanges: ['缺少可对比的配置快照'],
      layoutChanges: [],
      tabChanges: [],
    };
  }

  const fieldChanges: string[] = [];
  const layoutChanges: string[] = [];
  const tabChanges: string[] = [];

  const currentStatus = resolveWorkspaceStatus(currentConfig);
  const targetStatus = resolveWorkspaceStatus(targetConfig);
  if (currentStatus !== targetStatus) {
    fieldChanges.push(`状态: ${currentStatus} -> ${targetStatus}`);
  }
  if ((currentConfig.title || '') !== (targetConfig.title || '')) {
    fieldChanges.push(`标题: ${currentConfig.title || '-'} -> ${targetConfig.title || '-'}`);
  }
  if ((currentConfig.description || '') !== (targetConfig.description || '')) {
    fieldChanges.push('描述: 已变更');
  }
  if (Boolean(currentConfig.published) !== Boolean(targetConfig.published)) {
    fieldChanges.push(
      `发布标记: ${Boolean(currentConfig.published)} -> ${Boolean(targetConfig.published)}`,
    );
  }

  const currentLayout = currentConfig.layout?.type || '-';
  const targetLayout = targetConfig.layout?.type || '-';
  if (currentLayout !== targetLayout) {
    layoutChanges.push(`顶层布局: ${currentLayout} -> ${targetLayout}`);
  }

  const currentTabs = currentConfig.layout?.type === 'tabs' ? currentConfig.layout.tabs || [] : [];
  const targetTabs = targetConfig.layout?.type === 'tabs' ? targetConfig.layout.tabs || [] : [];
  const currentMap = new Map(currentTabs.map((tab) => [tab.key, tab]));
  const targetMap = new Map(targetTabs.map((tab) => [tab.key, tab]));

  targetTabs.forEach((tab) => {
    if (!currentMap.has(tab.key)) {
      tabChanges.push(`新增 Tab: ${tab.key}`);
    }
  });
  currentTabs.forEach((tab) => {
    if (!targetMap.has(tab.key)) {
      tabChanges.push(`删除 Tab: ${tab.key}`);
    }
  });
  targetTabs.forEach((tab) => {
    const currentTab = currentMap.get(tab.key);
    if (!currentTab) return;
    if ((currentTab.title || '') !== (tab.title || '')) {
      tabChanges.push(`Tab ${tab.key} 标题变更`);
    }
    if (currentTab.layout?.type !== tab.layout?.type) {
      tabChanges.push(`Tab ${tab.key} 布局: ${currentTab.layout?.type} -> ${tab.layout?.type}`);
    }
    const currentFns = new Set(currentTab.functions || []);
    const targetFns = new Set(tab.functions || []);
    const addedFns = Array.from(targetFns).filter((fn) => !currentFns.has(fn));
    const removedFns = Array.from(currentFns).filter((fn) => !targetFns.has(fn));
    if (addedFns.length > 0) {
      tabChanges.push(`Tab ${tab.key} 新增函数: ${addedFns.join(', ')}`);
    }
    if (removedFns.length > 0) {
      tabChanges.push(`Tab ${tab.key} 删除函数: ${removedFns.join(', ')}`);
    }
  });

  if (fieldChanges.length === 0) fieldChanges.push('字段未变化');
  if (layoutChanges.length === 0) layoutChanges.push('布局未变化');
  if (tabChanges.length === 0) tabChanges.push('Tab 结构未变化');

  return { fieldChanges, layoutChanges, tabChanges };
}

function getVersionTimeWindow(range: VersionTimeRange): { from?: string; to?: string } | undefined {
  if (range === 'all') return undefined;
  const now = Date.now();
  const durationMap: Record<Exclude<VersionTimeRange, 'all'>, number> = {
    '7d': 7,
    '30d': 30,
    '90d': 90,
  };
  const days = durationMap[range as Exclude<VersionTimeRange, 'all'>];
  return {
    from: new Date(now - days * 24 * 60 * 60 * 1000).toISOString(),
    to: new Date(now).toISOString(),
  };
}

function filterVersionsByTime(
  versions: WorkspaceVersionRecord[],
  range: VersionTimeRange,
): WorkspaceVersionRecord[] {
  if (range === 'all') return versions;
  const window = getVersionTimeWindow(range);
  if (!window?.from) return versions;
  const fromTs = new Date(window.from).getTime();
  return versions.filter((item) => {
    if (!item.createdAt) return true;
    return new Date(item.createdAt).getTime() >= fromTs;
  });
}

function collectRequiredFunctions(candidate: WorkspaceConfig): string[] {
  if (candidate.layout?.type !== 'tabs' || !Array.isArray(candidate.layout.tabs)) return [];
  const result = new Set<string>();
  candidate.layout.tabs.forEach((tab) => {
    tab.functions?.forEach((fn) => fn && result.add(fn));
    const layout = tab.layout as any;
    if (layout?.listFunction) result.add(layout.listFunction);
    if (layout?.queryFunction) result.add(layout.queryFunction);
    if (layout?.submitFunction) result.add(layout.submitFunction);
    if (layout?.detailFunction) result.add(layout.detailFunction);
    if (layout?.dataFunction) result.add(layout.dataFunction);

    const tryCollectComp = (comp: any) => {
      if (!comp?.config) return;
      if (comp.config.listFunction) result.add(comp.config.listFunction);
      if (comp.config.queryFunction) result.add(comp.config.queryFunction);
      if (comp.config.submitFunction) result.add(comp.config.submitFunction);
      if (comp.config.detailFunction) result.add(comp.config.detailFunction);
      if (comp.config.dataFunction) result.add(comp.config.dataFunction);
    };
    if (Array.isArray(layout?.panels)) {
      layout.panels.forEach((p: any) => tryCollectComp(p?.component));
    }
    if (Array.isArray(layout?.steps)) {
      layout.steps.forEach((s: any) => tryCollectComp(s?.component));
    }
    if (Array.isArray(layout?.items)) {
      layout.items.forEach((it: any) => tryCollectComp(it?.component));
    }
  });
  return Array.from(result);
}

function autoBindTemplateFunctions(
  candidate: WorkspaceConfig,
  availableFunctionIds: string[],
): WorkspaceConfig {
  if (candidate.layout?.type !== 'tabs' || !Array.isArray(candidate.layout.tabs)) return candidate;
  if (!Array.isArray(availableFunctionIds) || availableFunctionIds.length === 0) return candidate;

  const pickFn = (preferred: string[] = []) => {
    for (const id of preferred) {
      if (id && availableFunctionIds.includes(id)) return id;
    }
    return availableFunctionIds[0];
  };

  const bindIntoConfig = (cfg: any, preferred: string[]) => {
    if (!cfg) return cfg;
    const next = { ...cfg };
    const t = String(next?.type || '');
    const isAvailable = (id?: string) => !!id && availableFunctionIds.includes(id);

    if ((t === 'list' || 'listFunction' in next) && !isAvailable(next.listFunction)) {
      next.listFunction = pickFn(preferred);
    }
    if (t === 'list' && (!Array.isArray(next.columns) || next.columns.length === 0)) {
      next.columns = [
        { key: 'id', title: 'ID' },
        { key: 'name', title: '名称' },
        { key: 'status', title: '状态' },
      ];
    }

    if ((t === 'form-detail' || 'queryFunction' in next) && !isAvailable(next.queryFunction)) {
      next.queryFunction = pickFn(preferred);
    }
    if (t === 'form-detail' && !Array.isArray(next.queryFields)) {
      next.queryFields = [];
    }

    if ((t === 'form' || 'submitFunction' in next) && !isAvailable(next.submitFunction)) {
      next.submitFunction = pickFn(preferred);
    }
    if (t === 'form' && (!Array.isArray(next.fields) || next.fields.length === 0)) {
      next.fields = [{ key: 'name', label: '名称', type: 'input', required: true }];
    }

    if ((t === 'detail' || 'detailFunction' in next) && !isAvailable(next.detailFunction)) {
      next.detailFunction = pickFn(preferred);
    }
    if (t === 'detail' && (!Array.isArray(next.sections) || next.sections.length === 0)) {
      next.sections = [
        {
          title: '基础信息',
          fields: [
            { key: 'id', label: 'ID' },
            { key: 'name', label: '名称' },
          ],
        },
      ];
    }

    if (
      (t === 'kanban' || t === 'timeline' || 'dataFunction' in next) &&
      !isAvailable(next.dataFunction)
    ) {
      next.dataFunction = pickFn(preferred);
    }
    return next;
  };

  const nextTabs = candidate.layout.tabs.map((tab: any) => {
    const preferred =
      Array.isArray(tab?.functions) && tab.functions.length > 0 ? tab.functions : [];
    const fallback = pickFn(preferred);
    const nextTab = { ...tab };
    if (!Array.isArray(nextTab.functions) || nextTab.functions.length === 0) {
      nextTab.functions = [fallback];
    }
    const layout = nextTab.layout as any;
    if (!layout) return nextTab;
    let nextLayout = bindIntoConfig(layout, nextTab.functions);

    if (Array.isArray(layout.panels)) {
      nextLayout = {
        ...nextLayout,
        panels: layout.panels.map((p: any) => ({
          ...p,
          component: p?.component
            ? { ...p.component, config: bindIntoConfig(p.component.config, nextTab.functions) }
            : p?.component,
        })),
      };
    }
    if (Array.isArray(layout.steps)) {
      nextLayout = {
        ...nextLayout,
        steps: layout.steps.map((s: any) => ({
          ...s,
          component: s?.component
            ? { ...s.component, config: bindIntoConfig(s.component.config, nextTab.functions) }
            : s?.component,
        })),
      };
    }
    if (Array.isArray(layout.items)) {
      nextLayout = {
        ...nextLayout,
        items: layout.items.map((it: any) => ({
          ...it,
          component: it?.component
            ? { ...it.component, config: bindIntoConfig(it.component.config, nextTab.functions) }
            : it?.component,
        })),
      };
    }

    nextTab.layout = nextLayout;
    return nextTab;
  });

  return {
    ...candidate,
    layout: {
      ...candidate.layout,
      tabs: nextTabs,
    },
  };
}

function isTemplateNonBlockingValidationError(error: string): boolean {
  return /(listFunction|submitFunction|detailFunction|queryFunction|dataFunction) 不能为空/.test(
    error,
  );
}

function buildTemplateAppliedConfig(
  currentConfig: WorkspaceConfig,
  template: Template,
): WorkspaceConfig {
  const templateConfig = ((template.config as WorkspaceConfig | undefined) ||
    {}) as WorkspaceConfig;
  const nextTitle = String(
    templateConfig.title || currentConfig.title || template.name || '',
  ).trim();
  const nextDescription =
    templateConfig.description !== undefined
      ? templateConfig.description
      : currentConfig.description;

  return {
    ...currentConfig,
    ...templateConfig,
    objectKey: String(currentConfig.objectKey || '').trim(),
    title: nextTitle || `${currentConfig.objectKey} 管理`,
    description: nextDescription,
    layout: templateConfig.layout || currentConfig.layout || { type: 'tabs', tabs: [] },
    status: templateConfig.status || currentConfig.status || 'draft',
    published: Boolean(templateConfig.published ?? currentConfig.published),
    meta: {
      ...currentConfig.meta,
      ...templateConfig.meta,
      updatedAt: new Date().toISOString(),
    },
  };
}

function buildWorkspaceValidationGuidance(errors: string[]): string[] {
  const hints: string[] = [];
  const joined = errors.join('\n');

  if (/layout\.(listFunction|submitFunction|detailFunction|queryFunction)/.test(joined)) {
    hints.push(
      '先为当前页面选择核心函数。列表页要定 listFunction，表单页要定 submitFunction，详情页要定 detailFunction，查询详情页要定 queryFunction。',
    );
  }
  if (/functions|至少绑定|未绑定函数|tab\.functions/i.test(joined)) {
    hints.push('至少给页面绑定一个主函数。页面骨架、自动补字段和自动补列都依赖主函数。');
  }
  if (/columns|列表布局需要至少一个列配置/i.test(joined)) {
    hints.push('列表页还缺列配置。可以先选列表函数，再用“自动填充”生成基础列。');
  }
  if (/fields|表单布局需要至少一个字段配置/i.test(joined)) {
    hints.push('表单页还缺字段。先定提交函数，再用“自动填充”或字段库补核心字段。');
  }
  if (/queryFields|表单-详情布局需要至少一个查询字段配置/i.test(joined)) {
    hints.push('查询详情页还缺查询字段。先定查询函数，再补一个最小查询入口。');
  }
  if (/sections|详情布局需要至少一个分区配置/i.test(joined)) {
    hints.push('详情页还缺分区。先定详情函数，再按阅读顺序添加至少一个信息分区。');
  }

  return hints.slice(0, 4);
}

function buildTabReadinessSummary(config: WorkspaceConfig | null, availableFunctions: any[]) {
  if (config?.layout?.type !== 'tabs' || !Array.isArray(config.layout.tabs)) {
    return {
      total: 0,
      readyCount: 0,
      pendingCount: 0,
      items: [] as Array<{
        key: string;
        title: string;
        status: 'ready' | 'pending';
        summary: string;
      }>,
    };
  }

  const functionIds = (availableFunctions || [])
    .map((fn: any) => String(fn?.id || ''))
    .filter(Boolean);

  const items = config.layout.tabs.map((tab) => {
    const result = validateTabConfig(tab, functionIds);
    const primaryFunction = tab.functions?.[0] || '';
    const layout = (tab.layout || {}) as any;
    const coreFunction =
      layout.listFunction ||
      layout.submitFunction ||
      layout.detailFunction ||
      layout.queryFunction ||
      layout.dataFunction ||
      '';
    const blockers: string[] = [];
    if (!primaryFunction) blockers.push('缺主函数');
    if (!coreFunction && ['list', 'form', 'detail', 'form-detail'].includes(layout.type)) {
      blockers.push('缺核心函数');
    }
    if (result.errors.some((error) => error.path.includes('layout.columns'))) blockers.push('缺列');
    if (
      result.errors.some(
        (error) =>
          error.path.includes('layout.fields') || error.path.includes('layout.queryFields'),
      )
    ) {
      blockers.push('缺字段');
    }
    if (result.errors.some((error) => error.path.includes('layout.sections'))) {
      blockers.push('缺分区');
    }

    return {
      key: tab.key,
      title: tab.title || tab.key,
      status: result.valid ? ('ready' as const) : ('pending' as const),
      summary: result.valid
        ? `主函数 ${primaryFunction || '未设置'}，可继续预览或准备发布`
        : blockers.length > 0
        ? blockers.join(' / ')
        : `${result.errors.length} 个待处理问题`,
    };
  });

  const readyCount = items.filter((item) => item.status === 'ready').length;
  return {
    total: items.length,
    readyCount,
    pendingCount: items.length - readyCount,
    items,
  };
}

function getLayoutCapabilityMeta(layoutType?: string): {
  label: string;
  tone: 'success' | 'processing' | 'warning';
} {
  if (layoutType && STABLE_TAB_LAYOUT_TYPES.has(layoutType)) {
    return { label: '正式', tone: 'success' };
  }
  if (layoutType && BETA_TAB_LAYOUT_TYPES.has(layoutType)) {
    return { label: 'Beta', tone: 'processing' };
  }
  return { label: '实验', tone: 'warning' };
}

function buildAppliedTemplateSummary(
  template: Template,
  config: WorkspaceConfig,
  availableFunctions: any[],
): {
  templateName: string;
  showcase: boolean;
  tabCount: number;
  tabTitles: string[];
  score: number;
  headline: string;
  checklist: string[];
  scenario?: string;
  requiredFunctions: string[];
  riskNotes: string[];
} {
  const tabs =
    config.layout?.type === 'tabs' && Array.isArray(config.layout.tabs) ? config.layout.tabs : [];
  const report = buildWorkspaceQualityReport(config, availableFunctions);
  return {
    templateName: template.name,
    showcase: Boolean(template.showcase),
    tabCount: tabs.length,
    tabTitles: tabs.map((tab) => tab.title || tab.key).filter(Boolean).slice(0, 4),
    score: report.score,
    headline: template.showcase ? '官方样板已接入当前草稿' : '模板已接入当前草稿',
    checklist: (template.applyChecklist || []).slice(0, 3),
    scenario: template.scenario,
    requiredFunctions: (template.requiredFunctions || []).slice(0, 4),
    riskNotes: (template.riskNotes || []).slice(0, 3),
  };
}

export default function WorkspaceEditor() {
  const access = useAccess() as any;
  const params = useParams<{ objectKey: string }>();
  const objectKey = params.objectKey || '';
  const screens = Grid.useBreakpoint();
  const isCompactScreen = !screens.xl;

  const [config, setConfig] = useState<WorkspaceConfig | null>(null);
  const [availableFunctions, setAvailableFunctions] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [isFreshWorkspace, setIsFreshWorkspace] = useState(false);
  const [viewMode, setViewMode] = useState<ViewMode>(2);
  const [collapsed, setCollapsed] = useState(false);
  const [versionsVisible, setVersionsVisible] = useState(false);
  const [versionsLoading, setVersionsLoading] = useState(false);
  const [rollingVersionId, setRollingVersionId] = useState('');
  const [versions, setVersions] = useState<WorkspaceVersionRecord[]>([]);
  const [selectedVersion, setSelectedVersion] = useState<WorkspaceVersionRecord | null>(null);
  const [compareVersions, setCompareVersions] = useState<{
    old: WorkspaceVersionRecord;
    new: WorkspaceVersionRecord;
  } | null>(null);
  const [versionDetailLoading, setVersionDetailLoading] = useState(false);
  const [versionDetailId, setVersionDetailId] = useState('');
  const [versionTimeRange, setVersionTimeRange] = useState<VersionTimeRange>('all');
  const [importVisible, setImportVisible] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importContent, setImportContent] = useState('');
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  // 模板管理
  const [templateModalVisible, setTemplateModalVisible] = useState(false);
  const [templateBrowseMode, setTemplateBrowseMode] = useState<'all' | 'showcase'>('all');
  const [lastAppliedTemplateSummary, setLastAppliedTemplateSummary] = useState<ReturnType<
    typeof buildAppliedTemplateSummary
  > | null>(null);
  const [performanceModalVisible, setPerformanceModalVisible] = useState(false);
  const [thumbnailModalVisible, setThumbnailModalVisible] = useState(false);
  const [configTestVisible, setConfigTestVisible] = useState(false);
  const [auditLogVisible, setAuditLogVisible] = useState(false);
  const [activeTabKey, setActiveTabKey] = useState<string | undefined>(undefined);

  // 新手引导
  const [guideTourVisible, setGuideTourVisible] = useState(false);
  const [guideStep, setGuideStep] = useState(0);

  // 协作面板
  const [collaborationVisible, setCollaborationVisible] = useState(false);

  const [functionLibraryVisible, setFunctionLibraryVisible] = useState(false);

  // 草稿管理
  const [draftVisible, setDraftVisible] = useState(false);
  const [publishCheckVisible, setPublishCheckVisible] = useState(false);
  const autoSaveRef = useRef<AutoSaveDraft | null>(null);
  const autoPreviewTriggeredRef = useRef(false);

  // 使用历史记录 Hook
  const history = useSimpleHistory<WorkspaceConfig | null>(null, 100);

  useEffect(() => {
    trackWorkspaceEvent('workspace_page_open', {
      page: 'workspace_editor',
      objectKey,
    });
    loadData();
  }, [objectKey]);

  // 新手引导自动启动检测
  useEffect(() => {
    // 延迟检测，确保页面已完全加载
    const timer = setTimeout(() => {
      if (shouldShowGuide()) {
        setGuideTourVisible(true);
      }
    }, 1500);
    return () => clearTimeout(timer);
  }, []);

  // 组件卸载时清理协作管理器
  useEffect(() => {
    return () => {
      const manager = getCollaborationManager();
      manager.leave();
    };
  }, []);

  // 自动保存草稿
  useEffect(() => {
    if (!config || !objectKey) return;

    // 初始化自动保存
    if (!autoSaveRef.current) {
      autoSaveRef.current = new AutoSaveDraft(objectKey, () => config, {
        delay: 30000, // 30秒
        onSave: (draft) => {
          console.log('自动保存草稿:', draft.id);
        },
      });
    }

    // 配置变化时触发自动保存
    autoSaveRef.current.trigger();

    return () => {
      autoSaveRef.current?.dispose();
      autoSaveRef.current = null;
    };
  }, [config, objectKey]);

  const loadData = async () => {
    if (!objectKey) {
      message.error('缺少对象标识');
      return;
    }
    setLoading(true);
    try {
      const workspaceConfig = await loadWorkspaceConfig(objectKey);
      trackWorkspaceEvent('workspace_load', { objectKey });
      setIsFreshWorkspace(!workspaceConfig);
      const initialConfig = workspaceConfig || {
        objectKey,
        title: `${objectKey} 管理`,
        layout: { type: 'tabs', tabs: [] },
      };
      setConfig(initialConfig);
      setLastAppliedTemplateSummary(null);
      history.reset(initialConfig);

      const descriptors = await listDescriptors();
      const functions = descriptors.filter(
        (d) => !d.entity || d.entity === objectKey || d.id.startsWith(`${objectKey}.`),
      );
      setAvailableFunctions(functions.length > 0 ? functions : descriptors);
    } catch (error: any) {
      trackWorkspaceEvent('workspace_load_error', {
        objectKey,
        error: error?.message || String(error),
      });
      setIsFreshWorkspace(false);
      message.error(getWorkspaceErrorMessage(error, '加载失败'));
    } finally {
      setLoading(false);
    }
  };

  // 处理配置变更（带历史记录）
  const handleConfigChange = useCallback(
    (newConfig: WorkspaceConfig, description: string = '更新配置') => {
      setConfig(newConfig);
      history.setState(newConfig, description);
    },
    [history],
  );

  // 撤销
  const handleUndo = useCallback(() => {
    history.undo();
    setConfig(history.state);
  }, [history]);

  // 重做
  const handleRedo = useCallback(() => {
    history.redo();
    setConfig(history.state);
  }, [history]);

  const handleCreateFirstTab = useCallback(() => {
    if (!config) return;
    if (config.layout?.type !== 'tabs') {
      handleConfigChange(
        {
          ...config,
          layout: {
            type: 'tabs',
            tabs: [
              {
                key: `tab_${Date.now()}`,
                title: '基础页',
                functions: [],
                defaultActive: true,
                layout: {
                  type: 'list',
                  listFunction: '',
                  columns: [],
                },
              },
            ],
          },
        },
        '创建首个 Tab',
      );
      return;
    }
    if ((config.layout.tabs || []).length > 0) return;
    handleConfigChange(
      {
        ...config,
        layout: {
          ...config.layout,
          tabs: [
            {
              key: `tab_${Date.now()}`,
              title: '基础页',
              functions: [],
              defaultActive: true,
              layout: {
                type: 'list',
                listFunction: '',
                columns: [],
              },
            },
          ],
        },
      },
      '创建首个 Tab',
    );
  }, [config, handleConfigChange]);

  // 保存
  const handleSave = async () => {
    if (!config) return;
    if (!access?.canWorkspaceEdit) {
      message.error('无编辑权限');
      return;
    }
    const validation = validateWorkspaceConfig(config);
    if (!validation.valid) {
      const guidance = buildWorkspaceValidationGuidance(validation.errors || []);
      Modal.error({
        title: '配置校验失败',
        content: (
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            {guidance.length > 0 ? (
              <Alert
                type="warning"
                showIcon
                message="当前主要卡在这些步骤"
                description={
                  <div>
                    {guidance.map((hint, index) => (
                      <div key={`${index}-${hint}`}>{`${index + 1}. ${hint}`}</div>
                    ))}
                  </div>
                }
              />
            ) : null}
            <div>
              {(validation.errors || []).slice(0, 8).map((error, index) => (
                <div key={`${index}-${error}`}>{`${index + 1}. ${error}`}</div>
              ))}
            </div>
          </Space>
        ),
      });
      return;
    }

    // Tab 级别详细校验
    if (config.layout?.type === 'tabs' && config.layout.tabs) {
      const functionIds = (availableFunctions || [])
        .map((fn: any) => String(fn?.id || ''))
        .filter(Boolean);
      const tabErrors: string[] = [];
      const tabWarnings: string[] = [];

      config.layout.tabs.forEach((tab) => {
        const result = validateTabConfig(tab, functionIds);
        if (!result.valid) {
          tabErrors.push(`Tab [${tab.key}]:`);
          result.errors.forEach((error) => {
            tabErrors.push(`  - ${error.path}: ${error.message}`);
          });
        }
        if (result.warnings.length > 0) {
          tabWarnings.push(`Tab [${tab.key}]:`);
          result.warnings.forEach((warning) => {
            tabWarnings.push(`  - ${warning.path}: ${warning.message}`);
          });
        }
      });

      if (tabErrors.length > 0) {
        Modal.error({
          title: 'Tab 配置校验失败',
          width: 600,
          content: (
            <div style={{ maxHeight: 400, overflow: 'auto' }}>
              {tabErrors.slice(0, 20).map((error, index) => (
                <div key={index} style={{ fontSize: 12, fontFamily: 'monospace' }}>
                  {error}
                </div>
              ))}
            </div>
          ),
        });
        return;
      }

      if (tabWarnings.length > 0) {
        Modal.confirm({
          title: 'Tab 配置警告',
          width: 600,
          content: (
            <div style={{ maxHeight: 300, overflow: 'auto' }}>
              <div>以下配置可能存在问题，建议检查：</div>
              {tabWarnings.slice(0, 10).map((warning, index) => (
                <div key={index} style={{ fontSize: 12, fontFamily: 'monospace', marginTop: 4 }}>
                  {warning}
                </div>
              ))}
            </div>
          ),
          okText: '继续保存',
          cancelText: '取消',
          onOk: () => {
            doSave();
          },
        });
        return;
      }
    }

    doSave();
  };

  const doSave = async () => {
    if (!config) return;
    const currentConfig = config;
    setSaving(true);
    try {
      await saveWorkspaceConfig(currentConfig);
      trackWorkspaceEvent('workspace_save', {
        objectKey: currentConfig.objectKey,
        tabs: currentConfig.layout?.type === 'tabs' ? currentConfig.layout.tabs?.length || 0 : 0,
      });
      message.success('保存成功');
    } catch (error: any) {
      trackWorkspaceEvent('workspace_save_error', {
        objectKey: currentConfig.objectKey,
        error: error?.message || String(error),
      });
      message.error(getWorkspaceErrorMessage(error, '保存失败'));
    } finally {
      setSaving(false);
    }
  };

  const handleExport = useCallback(async () => {
    if (!objectKey) return;
    if (!access?.canWorkspaceRead) {
      message.error('无导出权限');
      return;
    }
    try {
      const content = await exportWorkspaceConfig(objectKey);
      const blob = new Blob([content], { type: 'application/json;charset=utf-8' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `${objectKey}.workspace.json`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      message.success('配置已导出');
    } catch (error: any) {
      message.error(getWorkspaceErrorMessage(error, '导出失败'));
    }
  }, [objectKey, access?.canWorkspaceRead]);

  const handleExportPublished = useCallback(async () => {
    if (!objectKey) return;
    if (!access?.canWorkspaceRead) {
      message.error('无导出权限');
      return;
    }
    try {
      const content = await exportPublishedWorkspaceConfig(objectKey);
      const blob = new Blob([content], { type: 'application/json;charset=utf-8' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `${objectKey}.workspace.published.json`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      message.success('已发布配置已导出');
    } catch (error: any) {
      message.error(getWorkspaceErrorMessage(error, '导出已发布版本失败'));
    }
  }, [objectKey, access?.canWorkspaceRead]);

  const handleExportMetadata = useCallback(async () => {
    if (!objectKey) return;
    if (!access?.canWorkspaceRead) {
      message.error('无导出权限');
      return;
    }
    try {
      const content = await exportWorkspaceMetadata(objectKey);
      const blob = new Blob([content], { type: 'application/json;charset=utf-8' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `${objectKey}.workspace.metadata.json`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      message.success('元信息已导出');
    } catch (error: any) {
      message.error(getWorkspaceErrorMessage(error, '导出元信息失败'));
    }
  }, [objectKey, access?.canWorkspaceRead]);

  const handleExportBackupBundle = useCallback(async () => {
    if (!objectKey) return;
    if (!access?.canWorkspaceRead) {
      message.error('无导出权限');
      return;
    }
    try {
      const content = await exportWorkspaceBackupBundle(objectKey);
      const blob = new Blob([content], { type: 'application/json;charset=utf-8' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `${objectKey}.workspace.backup.json`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      trackWorkspaceEvent('workspace_backup_export', { objectKey });
      message.success('备份包已导出');
    } catch (error: any) {
      trackWorkspaceEvent('workspace_backup_export_error', {
        objectKey,
        error: error?.message || String(error),
      });
      message.error(getWorkspaceErrorMessage(error, '导出备份包失败'));
    }
  }, [objectKey, access?.canWorkspaceRead]);

  const applyImportedConfig = useCallback(
    async (content: string) => {
      if (!objectKey) return;
      setImporting(true);
      try {
        const saved = await importWorkspaceConfig(content, {
          targetObjectKey: objectKey,
          forceDraft: true,
        });
        setConfig(saved);
        history.reset(saved);
        trackWorkspaceEvent('workspace_import', {
          objectKey,
          importedObjectKey: saved.objectKey,
        });
        setImportVisible(false);
        setImportContent('');
        message.success('导入成功，已保存为当前对象草稿');
      } catch (error: any) {
        trackWorkspaceEvent('workspace_import_error', {
          objectKey,
          error: error?.message || String(error),
        });
        message.error(getWorkspaceErrorMessage(error, '导入失败'));
      } finally {
        setImporting(false);
      }
    },
    [objectKey, history],
  );

  const handleImport = useCallback(async () => {
    if (!objectKey) return;
    if (!access?.canWorkspaceEdit) {
      message.error('无导入权限');
      return;
    }
    const content = importContent.trim();
    if (!content) {
      message.error('请先粘贴或选择配置 JSON');
      return;
    }
    try {
      const parsed = JSON.parse(content) as WorkspaceConfig;
      const normalizedConfig: WorkspaceConfig = {
        ...parsed,
        objectKey,
        status: 'draft',
        published: false,
        publishedAt: undefined,
        publishedBy: undefined,
      };
      const validation = validateWorkspaceConfig(normalizedConfig);
      if (!validation.valid) {
        Modal.error({
          title: '导入前校验失败',
          content: (
            <div>
              {(validation.errors || []).slice(0, 8).map((error, index) => (
                <div key={`${index}-${error}`}>{`${index + 1}. ${error}`}</div>
              ))}
            </div>
          ),
        });
        return;
      }

      const sourceKey = parsed.objectKey || '';
      if (sourceKey && sourceKey !== objectKey) {
        Modal.confirm({
          title: '检测到对象冲突',
          content: `导入文件 objectKey 为 ${sourceKey}，当前页面对象为 ${objectKey}。将按当前对象覆盖保存为草稿，是否继续？`,
          onOk: async () => {
            await applyImportedConfig(content);
          },
        });
        return;
      }

      await applyImportedConfig(content);
    } catch (error) {
      message.error('导入内容不是合法 JSON');
    }
  }, [objectKey, importContent, access?.canWorkspaceEdit, applyImportedConfig]);

  const handleSelectImportFile = useCallback(() => {
    if (!access?.canWorkspaceEdit) {
      message.error('无导入权限');
      return;
    }
    fileInputRef.current?.click();
  }, [access?.canWorkspaceEdit]);

  const handleImportFileChange = useCallback(async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      setImportContent(text);
      message.success(`已读取文件: ${file.name}`);
    } catch {
      message.error('读取文件失败');
    } finally {
      event.target.value = '';
    }
  }, []);

  // 模板选择处理
  const handleTemplateSelect = useCallback(
    (template: Template) => {
      if (!config) return;

      const layout = (template.config as any)?.layout;
      if (
        layout?.type !== 'tabs' ||
        (Array.isArray(layout?.tabs) &&
          layout.tabs.some((tab: any) => !STABLE_TAB_LAYOUT_TYPES.has(tab?.layout?.type)))
      ) {
        trackWorkspaceEvent('workspace_template_apply_error', {
          objectKey: config.objectKey,
          template: template.name,
          reason: 'unsupported_layout',
        });
        message.error('当前正式模板仅支持 tabs + form-detail/list/form/detail');
        return;
      }

      const draftConfig = buildTemplateAppliedConfig(config, template);
      const allAvailableIds = (availableFunctions || [])
        .map((fn: any) => String(fn?.id || ''))
        .filter(Boolean);
      const newConfig = autoBindTemplateFunctions(draftConfig, allAvailableIds);

      const availableFunctionIds = new Set(
        (availableFunctions || []).map((fn: any) => String(fn?.id || '')).filter(Boolean),
      );
      const requiredFunctions = collectRequiredFunctions(newConfig);
      const missingFunctions = requiredFunctions.filter((id) => !availableFunctionIds.has(id));
      if (missingFunctions.length > 0) {
        trackWorkspaceEvent('workspace_template_apply_error', {
          objectKey: config.objectKey,
          template: template.name,
          reason: 'missing_functions',
          missingCount: missingFunctions.length,
        });
        Modal.warning({
          title: '模板预检查未通过',
          content: `以下函数未找到: ${missingFunctions.slice(0, 8).join(', ')}`,
        });
        return;
      }

      const validation = validateWorkspaceConfig(newConfig);
      if (!validation.valid) {
        const blockingErrors = (validation.errors || []).filter(
          (error) => !isTemplateNonBlockingValidationError(error),
        );
        const onlyMissingFunctionBindings = blockingErrors.length === 0;
        if (onlyMissingFunctionBindings) {
          handleConfigChange(newConfig, `应用模板: ${template.name}`);
          setTemplateModalVisible(false);
          setTemplateBrowseMode('all');
          if (newConfig.layout?.type === 'tabs' && newConfig.layout.tabs?.length) {
            setActiveTabKey(newConfig.layout.tabs[0].key);
            setViewMode(3);
          }
          trackWorkspaceEvent('workspace_template_apply', {
            objectKey: config.objectKey,
            template: template.name,
            degraded: true,
          });
          const summary = buildAppliedTemplateSummary(template, newConfig, availableFunctions);
          setLastAppliedTemplateSummary(summary);
          Modal.warning({
            title: summary.headline,
            content: (
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <Typography.Text>
                  {`已生成 ${summary.tabCount} 个页面骨架${
                    summary.tabTitles.length ? `：${summary.tabTitles.join(' / ')}` : ''
                  }。`}
                </Typography.Text>
                <Typography.Text type="secondary">
                  {`当前发布评分 ${summary.score}/100。模板已应用，但部分函数仍待绑定后再保存。`}
                </Typography.Text>
                <Typography.Text type="secondary">
                  下一步先在中间编辑区补齐主函数与核心函数，再检查预览结果。
                </Typography.Text>
              </Space>
            ),
          });
          return;
        }
        trackWorkspaceEvent('workspace_template_apply_error', {
          objectKey: config.objectKey,
          template: template.name,
          reason: 'config_validation_failed',
        });
        Modal.warning({
          title: '模板预检查未通过',
          content: `配置不兼容: ${blockingErrors.slice(0, 6).join('；')}`,
        });
        return;
      }

      handleConfigChange(newConfig, `应用模板: ${template.name}`);
      setTemplateModalVisible(false);
      setTemplateBrowseMode('all');
      if (newConfig.layout?.type === 'tabs' && newConfig.layout.tabs?.length) {
        setActiveTabKey(newConfig.layout.tabs[0].key);
        setViewMode(3);
      }
      trackWorkspaceEvent('workspace_template_apply', {
        objectKey: config.objectKey,
        template: template.name,
      });
      const summary = buildAppliedTemplateSummary(template, newConfig, availableFunctions);
      setLastAppliedTemplateSummary(summary);
      Modal.success({
        title: summary.headline,
        content: (
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            <Typography.Text>
              {`已生成 ${summary.tabCount} 个页面骨架${
                summary.tabTitles.length ? `：${summary.tabTitles.join(' / ')}` : ''
              }。`}
            </Typography.Text>
            <Typography.Text type="secondary">
              {`当前发布评分 ${summary.score}/100，编辑器已切到更适合继续细化的视图。`}
            </Typography.Text>
            <Typography.Text type="secondary">
              下一步优先检查预览、补齐字段或函数绑定，再执行发布前检查。
            </Typography.Text>
          </Space>
        ),
      });
    },
    [config, availableFunctions, handleConfigChange],
  );

  const loadVersions = useCallback(async () => {
    if (!objectKey) return;
    setVersionsLoading(true);
    try {
      const timeWindow = getVersionTimeWindow(versionTimeRange);
      const rows = await listWorkspaceVersions(objectKey, timeWindow);
      const normalizedRows = Array.isArray(rows) ? rows : [];
      const visibleRows = filterVersionsByTime(normalizedRows, versionTimeRange);
      trackWorkspaceEvent('workspace_versions_load', {
        objectKey,
        count: visibleRows.length,
        range: versionTimeRange,
      });
      setVersions(visibleRows);
    } catch (error: any) {
      trackWorkspaceEvent('workspace_versions_load_error', {
        objectKey,
        error: error?.message || String(error),
        range: versionTimeRange,
      });
      message.warning(getWorkspaceErrorMessage(error, '版本接口暂不可用'));
      setVersions([]);
    } finally {
      setVersionsLoading(false);
    }
  }, [objectKey, versionTimeRange]);

  const handleRollback = useCallback(
    (version: WorkspaceVersionRecord) => {
      if (!objectKey) return;
      if (!access?.canWorkspaceRollback) {
        message.error('无回滚权限');
        return;
      }
      Modal.confirm({
        title: '确认回滚',
        content: (
          <div>
            <div>{`目标版本: v${version.version}`}</div>
            <div>{`版本摘要: ${summarizeVersion(version)}`}</div>
            <div>{`当前版本: ${
              typeof config?.version === 'number' ? `v${config.version}` : '-'
            }`}</div>
            <div>{`差异摘要: ${summarizeDiff(config, version)}`}</div>
            <div style={{ marginTop: 8, color: '#cf1322' }}>
              此操作会覆盖当前草稿，请确认后执行。
            </div>
            <div style={{ marginTop: 4, color: '#cf1322' }}>
              回滚后若重新发布，控制台展示行为可能随目标版本发生变化。
            </div>
          </div>
        ),
        onOk: async () => {
          try {
            setRollingVersionId(version.id);
            await rollbackWorkspaceVersion(objectKey, version.id);
            await loadData();
            await loadVersions();
            trackWorkspaceEvent('workspace_rollback', {
              objectKey,
              versionId: version.id,
              version: version.version,
            });
            message.success(`已回滚到版本 v${version.version}`);
          } catch (error: any) {
            trackWorkspaceEvent('workspace_rollback_error', {
              objectKey,
              versionId: version.id,
              error: error?.message || String(error),
            });
            message.error(getWorkspaceErrorMessage(error, '回滚失败'));
          } finally {
            setRollingVersionId('');
          }
        },
      });
    },
    [objectKey, loadVersions, access?.canWorkspaceRollback, config?.version],
  );

  const handleOpenVersionDetail = useCallback(
    async (version: WorkspaceVersionRecord) => {
      if (!objectKey) return;
      if (!access?.canWorkspaceRead) {
        message.error('无查看版本权限');
        return;
      }
      setVersionDetailId(version.id);
      setVersionDetailLoading(true);
      try {
        const detail = await getWorkspaceVersionDetail(objectKey, version.id);
        setSelectedVersion(detail || version);
      } catch (error: any) {
        // 兼容后端未启用详情接口的场景：降级使用列表中的快照
        message.warning(getWorkspaceErrorMessage(error, '版本详情接口暂不可用，已展示列表快照'));
        setSelectedVersion(version);
      } finally {
        setVersionDetailLoading(false);
        setVersionDetailId('');
      }
    },
    [objectKey, access?.canWorkspaceRead],
  );

  useEffect(() => {
    if (!versionsVisible) return;
    loadVersions().catch(() => {});
  }, [versionsVisible, versionTimeRange, loadVersions]);

  // 键盘快捷键
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ctrl/Cmd + S: 保存
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault();
        handleSave();
      }
      // Ctrl/Cmd + Z: 撤销
      if ((e.ctrlKey || e.metaKey) && e.key === 'z' && !e.shiftKey) {
        e.preventDefault();
        handleUndo();
      }
      // Ctrl/Cmd + Y 或 Ctrl/Cmd + Shift + Z: 重做
      if ((e.ctrlKey || e.metaKey) && (e.key === 'y' || (e.key === 'z' && e.shiftKey))) {
        e.preventDefault();
        handleRedo();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleSave, handleUndo, handleRedo, viewMode]);

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '100px 0' }}>
        <Spin size="large" />
      </div>
    );
  }

  const tabCount = config?.layout?.type === 'tabs' ? config.layout.tabs?.length || 0 : 0;
  const singleAvailableFunction = availableFunctions.length === 1 ? availableFunctions[0] : null;
  const currentStatus = resolveWorkspaceStatus(config);
  const previewAsDrawer = isCompactScreen && viewMode === 3;
  const functionPanelWidth = collapsed ? '48px' : '300px';
  const contentColumns = isCompactScreen
    ? 'minmax(0, 1fr)'
    : viewMode === 3
    ? `${functionPanelWidth} minmax(0, 1fr) minmax(360px, 420px)`
    : `${functionPanelWidth} minmax(0, 1fr)`;

  const openWorkspaceTool = (key: string) => {
    switch (key) {
      case 'template':
        setTemplateModalVisible(true);
        break;
      case 'import':
        setImportVisible(true);
        break;
      case 'versions':
        setVersionsVisible(true);
        break;
      case 'performance':
        setPerformanceModalVisible(true);
        break;
      case 'thumbnail':
        setThumbnailModalVisible(true);
        break;
      case 'test':
        setConfigTestVisible(true);
        break;
      case 'audit':
        setAuditLogVisible(true);
        break;
      case 'collaboration':
        setCollaborationVisible(true);
        break;
      case 'draft':
        setDraftVisible(true);
        break;
      case 'function-library':
        setFunctionLibraryVisible(true);
        break;
      case 'export':
        handleExport().catch?.(() => {});
        break;
      case 'export-published':
        handleExportPublished().catch?.(() => {});
        break;
      case 'export-metadata':
        handleExportMetadata().catch?.(() => {});
        break;
      case 'export-backup':
        handleExportBackupBundle().catch?.(() => {});
        break;
      default:
        break;
    }
  };

  const toolMenuItems = [
    {
      key: 'template',
      icon: <FileOutlined />,
      label: '正式模板',
    },
    {
      key: 'import',
      icon: <UploadOutlined />,
      label: '导入配置',
      disabled: !access?.canWorkspaceEdit,
    },
    {
      key: 'preview',
      icon: <EyeOutlined />,
      label: viewMode === 3 ? '关闭预览' : '打开预览',
    },
    {
      type: 'divider' as const,
    },
    {
      type: 'group' as const,
      label: '编辑辅助',
      children: [
        {
          key: 'versions',
          icon: <HistoryOutlined />,
          label: '版本历史',
          disabled: !access?.canWorkspaceRead,
        },
        {
          key: 'function-library',
          icon: <FolderOpenOutlined />,
          label: '函数面板',
          disabled: !isCompactScreen,
        },
        {
          key: 'draft',
          icon: <FileOutlined />,
          label: '草稿管理',
        },
        {
          key: 'thumbnail',
          icon: <BorderOutlined />,
          label: '布局缩略图',
        },
      ],
    },
    {
      type: 'group' as const,
      label: '诊断与协作',
      children: [
        {
          key: 'performance',
          icon: <ThunderboltOutlined />,
          label: '性能分析',
        },
        {
          key: 'test',
          icon: <PlayCircleOutlined />,
          label: '配置测试',
        },
        {
          key: 'audit',
          icon: <FileOutlined />,
          label: '审计日志',
        },
        {
          key: 'collaboration',
          icon: <TeamOutlined />,
          label: '协作状态',
        },
      ],
    },
    {
      type: 'group' as const,
      label: '导出',
      children: [
        {
          key: 'export',
          icon: <DownloadOutlined />,
          label: '导出当前配置',
          disabled: !access?.canWorkspaceRead,
        },
        {
          key: 'export-published',
          icon: <DownloadOutlined />,
          label: '导出已发布配置',
          disabled: !access?.canWorkspaceRead,
        },
        {
          key: 'export-metadata',
          icon: <DownloadOutlined />,
          label: '导出元信息',
          disabled: !access?.canWorkspaceRead,
        },
        {
          key: 'export-backup',
          icon: <DownloadOutlined />,
          label: '导出备份包',
          disabled: !access?.canWorkspaceRead,
        },
      ],
    },
  ];

  const workflowStatus =
    availableFunctions.length === 0
      ? {
          type: 'warning' as const,
          step: '1. 先挑主函数',
          title: '当前还没有主函数',
          description:
            '没有函数时，后面的布局和预览都缺少上下文。建议先从函数面板选择一个主函数，再开始配置 Tab。',
          action: isCompactScreen ? (
            <Button
              size="small"
              icon={<BarsOutlined />}
              onClick={() => setFunctionLibraryVisible(true)}
            >
              打开函数面板
            </Button>
          ) : null,
        }
      : tabCount === 0
      ? {
          type: 'warning' as const,
          step: '2. 先创建首个页面',
          title: '函数已经有了，但还没有页面',
          description:
            '函数已经准备好，但还没有承载界面的 Tab。建议先在中间区域创建一个 Tab，再继续选择布局。',
          action: null,
        }
      : viewMode !== 3
      ? {
          type: 'info' as const,
          step: '3. 检查页面预览',
          title: '页面已经能继续细化，建议顺手看一眼预览',
          description: '主配置已经具备基础结构，可以切到预览模式快速确认界面层级和关键流转。',
          action: (
            <Button size="small" icon={<EyeOutlined />} onClick={() => setViewMode(3)}>
              切换到预览
            </Button>
          ),
        }
      : {
          type: 'success' as const,
          step: '3. 预览检查中',
          title: '当前正在边看边改',
          description: '可以一边核对预览，一边回到编辑模式微调字段、布局和交互细节。',
          action: (
            <Button size="small" icon={<BarsOutlined />} onClick={() => setViewMode(2)}>
              返回编辑
            </Button>
          ),
        };

  const tabReadiness = buildTabReadinessSummary(config, availableFunctions);
  const publishCheck = React.useMemo(() => {
    if (!config) {
      return {
        score: 0,
        headline: '当前还没有可检查的草稿',
        blockingCount: 1,
        warningCount: 0,
        readyCount: 0,
        readyTabCount: 0,
        pendingTabCount: 0,
        summary: '先加载或创建对象工作台草稿，再执行发布前检查。',
        items: [
          {
            level: 'blocking' as const,
            title: '缺少工作台草稿',
            detail: '当前页面还没有可供发布检查的配置。',
          },
        ],
        tabs: [],
      };
    }
    return buildWorkspaceQualityReport(config, availableFunctions);
  }, [config, availableFunctions]);

  const activeTabTitle =
    config?.layout?.type === 'tabs'
      ? config.layout.tabs?.find((tab) => tab.key === activeTabKey)?.title ||
        config.layout.tabs?.find((tab) => tab.defaultActive)?.title ||
        config.layout.tabs?.[0]?.title
      : undefined;

  useEffect(() => {
    if (config?.layout?.type !== 'tabs' || !config.layout.tabs?.length) {
      setActiveTabKey(undefined);
      return;
    }
    if (activeTabKey && config.layout.tabs.some((tab) => tab.key === activeTabKey)) {
      return;
    }
    const nextActiveKey =
      config.layout.tabs.find((tab) => tab.defaultActive)?.key || config.layout.tabs[0]?.key;
    setActiveTabKey(nextActiveKey);
  }, [activeTabKey, config]);

  useEffect(() => {
    if (autoPreviewTriggeredRef.current) return;
    if (!config || config.layout?.type !== 'tabs' || !config.layout.tabs?.length) return;
    const firstTab = config.layout.tabs[0] as any;
    if (!firstTab?.functions?.length) return;
    const layout = firstTab.layout || {};
    const hasRenderableStructure =
      Boolean(
        layout.listFunction ||
          layout.submitFunction ||
          layout.detailFunction ||
          layout.queryFunction ||
          layout.dataFunction,
      ) ||
      (Array.isArray(layout.columns) && layout.columns.length > 0) ||
      (Array.isArray(layout.fields) && layout.fields.length > 0) ||
      (Array.isArray(layout.queryFields) && layout.queryFields.length > 0) ||
      (Array.isArray(layout.sections) && layout.sections.length > 0);
    if (!hasRenderableStructure) return;
    autoPreviewTriggeredRef.current = true;
    setViewMode(3);
    message.info('已为你切到预览，可直接检查首个页面效果');
  }, [config, message]);

  const firstPageAction =
    config && singleAvailableFunction && tabCount === 0 ? (
      <Button
        type="primary"
        onClick={() => {
          handleConfigChange(
            {
              ...(config as WorkspaceConfig),
              layout: {
                type: 'tabs',
                tabs: [
                  {
                    key: `tab_${Date.now()}`,
                    title:
                      singleAvailableFunction.displayName?.zh ||
                      singleAvailableFunction.displayName?.en ||
                      '基础页',
                    functions: [singleAvailableFunction.id],
                    defaultActive: true,
                    layout: {
                      type: 'form',
                    },
                  },
                ],
              },
            },
            '根据唯一函数创建首个 Tab',
          );
        }}
        disabled={!access?.canWorkspaceEdit}
      >
        {`1. 用 ${
          singleAvailableFunction.displayName?.zh || singleAvailableFunction.id
        } 创建首个页面`}
      </Button>
    ) : (
      <Button onClick={handleCreateFirstTab} disabled={tabCount > 0 || !access?.canWorkspaceEdit}>
        1. 新建首个页面
      </Button>
    );

  const openTemplateGallery = useCallback(
    (mode: 'all' | 'showcase' = 'all') => {
      setTemplateBrowseMode(mode);
      setTemplateModalVisible(true);
    },
    [],
  );

  return (
    <>
      <PageContainer
        title={`对象工作台编排: ${objectKey}`}
        subTitle={
          isFreshWorkspace
            ? '当前还没有现成页面草稿。先创建首个页面，再继续绑定函数和细化布局。'
            : '这里负责把函数装配成业务页面。先绑主函数，再补 Tab 骨架，最后预览并决定是否发布到运行控制台。'
        }
        extra={[
          <Segmented
            key="view-mode"
            size="middle"
            value={viewMode}
            onChange={(value) => setViewMode(value as ViewMode)}
            options={[
              {
                value: 2,
                label: (
                  <Space size={6}>
                    <BarsOutlined />
                    <span>专注编辑</span>
                  </Space>
                ),
              },
              {
                value: 3,
                label: (
                  <Space size={6}>
                    <EyeOutlined />
                    <span>边改边看</span>
                  </Space>
                ),
              },
            ]}
          />,
          <Dropdown
            key="more"
            trigger={['click']}
            menu={{
              items: toolMenuItems,
              onClick: ({ key }) => {
                if (key === 'preview') {
                  setViewMode((prev) => (prev === 3 ? 2 : 3));
                  return;
                }
                openWorkspaceTool(String(key));
              },
            }}
          >
            <Button icon={<MoreOutlined />}>工具</Button>
          </Dropdown>,
          <Button
            key="publish-check"
            icon={<CheckCircleOutlined />}
            onClick={() => setPublishCheckVisible(true)}
          >
            发布前检查
          </Button>,
          <Button
            key="save"
            type="primary"
            icon={<SaveOutlined />}
            onClick={handleSave}
            loading={saving}
            disabled={!access?.canWorkspaceEdit}
          >
            保存
          </Button>,
        ]}
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Card
            styles={{
              body: {
                padding: 22,
                background:
                  'linear-gradient(135deg, rgba(22,119,255,0.1) 0%, rgba(82,196,26,0.05) 50%, rgba(250,173,20,0.05) 100%)',
              },
            }}
          >
            <Space direction="vertical" size={18} style={{ width: '100%' }}>
              <Space wrap size={[8, 8]}>
                <Tag color="blue">对象工作台编排</Tag>
                <Tag>{objectKey}</Tag>
                <Tag color={currentStatus === 'published' ? 'success' : 'default'}>
                  {currentStatus === 'published'
                    ? '已发布'
                    : currentStatus === 'archived'
                    ? '已归档'
                    : '草稿'}
                </Tag>
                <Tag>{`Tab ${tabCount}`}</Tag>
                <Tag>{`函数 ${availableFunctions.length}`}</Tag>
                {typeof config?.version === 'number' ? (
                  <Tag>{`版本 v${config.version}`}</Tag>
                ) : null}
              </Space>
              <Space direction="vertical" size={6} style={{ width: '100%' }}>
                <Typography.Title level={4} style={{ margin: 0 }}>
                  {isFreshWorkspace
                    ? `先为 ${objectKey} 建立首个业务页面`
                    : `继续完善 ${objectKey} 的页面装配与发布准备`}
                </Typography.Title>
                <Typography.Text type="secondary">
                  编辑器负责把函数能力组织成真实业务页面。优先顺序是：选择主函数，生成首个页面骨架，检查预览，再执行发布前检查。
                </Typography.Text>
              </Space>
              <Row gutter={[12, 12]}>
                <Col xs={24} sm={12} xl={6}>
                  <Card size="small" style={{ height: '100%', background: 'rgba(255,255,255,0.8)' }}>
                    <Space direction="vertical" size={4}>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        当前对象
                      </Typography.Text>
                      <Typography.Text strong>{objectKey}</Typography.Text>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        当前编辑入口绑定到单一对象页面装配流。
                      </Typography.Text>
                    </Space>
                  </Card>
                </Col>
                <Col xs={24} sm={12} xl={6}>
                  <Card size="small" style={{ height: '100%', background: 'rgba(255,255,255,0.8)' }}>
                    <Space direction="vertical" size={4}>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        页面规模
                      </Typography.Text>
                      <Typography.Text strong>{`${tabCount} 个 Tab`}</Typography.Text>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        {tabCount > 0 ? '先把当前页面做完整，再继续扩更多页面。' : '先形成首个可运行页面骨架。'}
                      </Typography.Text>
                    </Space>
                  </Card>
                </Col>
                <Col xs={24} sm={12} xl={6}>
                  <Card size="small" style={{ height: '100%', background: 'rgba(255,255,255,0.8)' }}>
                    <Space direction="vertical" size={4}>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        可用函数
                      </Typography.Text>
                      <Typography.Text strong>{availableFunctions.length}</Typography.Text>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        先为页面绑定主函数，再补核心函数和字段结构。
                      </Typography.Text>
                    </Space>
                  </Card>
                </Col>
                <Col xs={24} sm={12} xl={6}>
                  <Card size="small" style={{ height: '100%', background: 'rgba(255,255,255,0.8)' }}>
                    <Space direction="vertical" size={4}>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        发布评分
                      </Typography.Text>
                      <Typography.Text strong>{`${publishCheck.score}/100`}</Typography.Text>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        {publishCheck.blockingCount > 0
                          ? `还有 ${publishCheck.blockingCount} 个阻塞项待处理`
                          : publishCheck.warningCount > 0
                          ? `还有 ${publishCheck.warningCount} 个风险提示待确认`
                          : '当前已具备基础发布条件'}
                      </Typography.Text>
                    </Space>
                  </Card>
                </Col>
              </Row>
              <Card size="small" style={{ background: 'rgba(255,255,255,0.78)' }}>
                <Space direction="vertical" size={10} style={{ width: '100%' }}>
                  <Space wrap size={[8, 8]}>
                    <Typography.Text strong>主流程操作</Typography.Text>
                    <Tag bordered={false} color="processing">
                      1. 先挑主函数
                    </Tag>
                    <Tag bordered={false} color={tabCount > 0 ? 'success' : 'processing'}>
                      2. 起页面骨架
                    </Tag>
                    <Tag bordered={false} color={viewMode === 3 ? 'success' : 'processing'}>
                      3. 看预览并微调
                    </Tag>
                    <Tag
                      bordered={false}
                      color={publishCheck.blockingCount > 0 ? 'warning' : 'success'}
                    >
                      4. 发布前检查
                    </Tag>
                  </Space>
                  <Space wrap size={[8, 8]}>
                    {config && tabCount === 0 ? firstPageAction : null}
                    <Button icon={<FolderOpenOutlined />} onClick={() => openTemplateGallery('showcase')}>
                      {config && tabCount === 0 ? '官方演示样板' : '查看官方样板'}
                    </Button>
                    {isCompactScreen ? (
                      <Button icon={<BarsOutlined />} onClick={() => setFunctionLibraryVisible(true)}>
                        {config && tabCount === 0 ? '2. 打开函数面板' : '1. 打开函数面板'}
                      </Button>
                    ) : null}
                    <Button
                      onClick={handleCreateFirstTab}
                      disabled={tabCount > 0 || !access?.canWorkspaceEdit}
                    >
                      {config && tabCount === 0 ? '3. 新建首个 Tab' : '2. 新建首个 Tab'}
                    </Button>
                    <Button
                      icon={<EyeOutlined />}
                      onClick={() => setViewMode(3)}
                      disabled={tabCount === 0}
                    >
                      {config && tabCount === 0 ? '4. 检查预览' : '3. 检查预览'}
                    </Button>
                    <Button
                      icon={<CheckCircleOutlined />}
                      onClick={() => setPublishCheckVisible(true)}
                    >
                      发布前检查
                    </Button>
                    <Button
                      type="primary"
                      icon={<SaveOutlined />}
                      onClick={handleSave}
                      loading={saving}
                      disabled={!access?.canWorkspaceEdit}
                    >
                      保存草稿
                    </Button>
                  </Space>
                </Space>
              </Card>
              <Card
                size="small"
                style={{ background: 'rgba(255,255,255,0.78)' }}
                title="官方样板起步"
                extra={<Typography.Text type="secondary">直接套用高质量起始骨架</Typography.Text>}
              >
                <Row gutter={[12, 12]}>
                  {SHOWCASE_TEMPLATES.slice(0, 3).map((template) => (
                    <Col xs={24} md={8} key={template.id}>
                      <Card
                        size="small"
                        hoverable
                        style={{ height: '100%' }}
                        styles={{ body: { padding: 14 } }}
                        onClick={() => handleTemplateSelect(template)}
                      >
                        <Space direction="vertical" size={8} style={{ width: '100%' }}>
                          <Space wrap size={[6, 6]}>
                            <Tag color="magenta">官方样板</Tag>
                            <Tag>{template.category}</Tag>
                          </Space>
                          <Typography.Text strong>{template.name}</Typography.Text>
                          <Typography.Text type="secondary">
                            {template.scenario || template.description}
                          </Typography.Text>
                          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                            {template.applyChecklist?.[0] || '应用后先检查首屏和函数绑定'}
                          </Typography.Text>
                          <Space wrap size={[8, 8]}>
                            <Button
                              size="small"
                              onClick={(event) => {
                                event.stopPropagation();
                                openTemplateGallery('showcase');
                              }}
                            >
                              看更多
                            </Button>
                            <Button
                              size="small"
                              type="primary"
                              onClick={(event) => {
                                event.stopPropagation();
                                handleTemplateSelect(template);
                              }}
                            >
                              直接套用
                            </Button>
                          </Space>
                        </Space>
                      </Card>
                    </Col>
                  ))}
                </Row>
              </Card>
              <Space wrap size={[8, 8]}>
                <Typography.Text type="secondary">快捷操作:</Typography.Text>
                <Space.Compact>
                  <Tooltip title="撤销 (Ctrl+Z)">
                    <Button
                      icon={<UndoOutlined />}
                      disabled={!history.canUndo}
                      onClick={handleUndo}
                    >
                      撤销
                    </Button>
                  </Tooltip>
                  <Tooltip title="重做 (Ctrl+Y)">
                    <Button
                      icon={<RedoOutlined />}
                      disabled={!history.canRedo}
                      onClick={handleRedo}
                    >
                      重做
                    </Button>
                  </Tooltip>
                </Space.Compact>
                {isCompactScreen ? (
                  <Button icon={<BarsOutlined />} onClick={() => setFunctionLibraryVisible(true)}>
                    打开函数面板
                  </Button>
                ) : null}
              </Space>
              <Alert
                type={workflowStatus.type}
                showIcon
                message={`${workflowStatus.step} · ${workflowStatus.title}`}
                action={workflowStatus.action}
                description={
                  <Space wrap size={[8, 8]}>
                    <Tag bordered={false} color="processing">
                      1. 先挑主函数
                    </Tag>
                    <Tag bordered={false} color={tabCount > 0 ? 'success' : 'processing'}>
                      2. 起页面骨架
                    </Tag>
                    <Tag bordered={false} color={viewMode === 3 ? 'success' : 'processing'}>
                      3. 看预览并微调
                    </Tag>
                    <Typography.Text type="secondary">{workflowStatus.description}</Typography.Text>
                    <Typography.Text type="secondary">
                      高级操作已经收进“高级工具”，避免干扰主编辑流。
                    </Typography.Text>
                  </Space>
                }
              />
              {lastAppliedTemplateSummary ? (
                <Alert
                  type={lastAppliedTemplateSummary.showcase ? 'success' : 'info'}
                  showIcon
                  message={`${lastAppliedTemplateSummary.headline} · ${lastAppliedTemplateSummary.templateName}`}
                  action={
                    <Space wrap size={[8, 8]}>
                      <Button size="small" icon={<EyeOutlined />} onClick={() => setViewMode(3)}>
                        检查预览
                      </Button>
                      <Button size="small" icon={<CheckCircleOutlined />} onClick={() => setPublishCheckVisible(true)}>
                        继续检查
                      </Button>
                    </Space>
                  }
                  description={
                    <Space wrap size={[8, 8]}>
                      <Tag color="processing">{`评分 ${lastAppliedTemplateSummary.score}/100`}</Tag>
                      <Tag color="blue">{`生成 ${lastAppliedTemplateSummary.tabCount} 个页面`}</Tag>
                      {lastAppliedTemplateSummary.scenario ? (
                        <Tag color="purple">{lastAppliedTemplateSummary.scenario}</Tag>
                      ) : null}
                      {lastAppliedTemplateSummary.tabTitles.map((title) => (
                        <Tag key={`applied-${title}`}>{title}</Tag>
                      ))}
                      {lastAppliedTemplateSummary.requiredFunctions.map((item) => (
                        <Tag key={`required-${item}`} color="cyan">
                          {`需核对 ${item}`}
                        </Tag>
                      ))}
                      {lastAppliedTemplateSummary.checklist.map((item) => (
                        <Tag key={`check-${item}`} color="processing">
                          {item}
                        </Tag>
                      ))}
                      {lastAppliedTemplateSummary.riskNotes.map((item) => (
                        <Tag key={`risk-${item}`} color="warning">
                          {item}
                        </Tag>
                      ))}
                      <Typography.Text type="secondary">
                        下一步先检查预览、补齐字段和函数绑定，再执行发布前检查。
                      </Typography.Text>
                    </Space>
                  }
                />
              ) : null}
              {tabReadiness.total > 0 ? (
                <Alert
                  type={tabReadiness.pendingCount === 0 ? 'success' : 'warning'}
                  showIcon
                  message={
                    tabReadiness.pendingCount === 0
                      ? `所有页面都已具备基础发布条件 (${tabReadiness.readyCount}/${tabReadiness.total})`
                      : `还有 ${tabReadiness.pendingCount} 个页面未准备好发布`
                  }
                  description={
                    <Space wrap size={[8, 8]}>
                      {tabReadiness.items.map((item) => (
                        <Tag key={item.key} color={item.status === 'ready' ? 'success' : 'warning'}>
                          {`${item.title}: ${item.summary}`}
                        </Tag>
                      ))}
                    </Space>
                  }
                />
              ) : null}
              <Alert
                type={
                  publishCheck.blockingCount > 0
                    ? 'error'
                    : publishCheck.warningCount > 0
                    ? 'warning'
                    : 'success'
                }
                showIcon
                message={`发布前检查 · ${publishCheck.headline}`}
                action={
                  <Button size="small" icon={<CheckCircleOutlined />} onClick={() => setPublishCheckVisible(true)}>
                    查看详情
                  </Button>
                }
                description={
                  <Space wrap size={[8, 8]}>
                    <Tag color={publishCheck.blockingCount > 0 ? 'error' : 'success'}>
                      {`评分 ${publishCheck.score}/100`}
                    </Tag>
                    <Tag color={publishCheck.blockingCount > 0 ? 'error' : 'default'}>
                      {`阻塞 ${publishCheck.blockingCount}`}
                    </Tag>
                    <Tag color={publishCheck.warningCount > 0 ? 'warning' : 'default'}>
                      {`风险 ${publishCheck.warningCount}`}
                    </Tag>
                    <Tag color="processing">{`通过项 ${publishCheck.readyCount}`}</Tag>
                    <Typography.Text type="secondary">{publishCheck.summary}</Typography.Text>
                  </Space>
                }
              />
              {isFreshWorkspace && tabCount === 0 ? (
                <Alert
                  type="success"
                  showIcon
                  message={`正在为 ${objectKey} 创建首个页面`}
                  action={firstPageAction}
                  description={
                    availableFunctions.length > 0
                      ? `系统已经为这个对象筛到 ${availableFunctions.length} 个相关函数。先起一个首个页面，再到中间区域继续补布局和字段。`
                      : '当前还没筛到可直接使用的相关函数。可以先打开函数面板确认函数，或者先建一个空页面骨架。'
                  }
                />
              ) : null}
            </Space>
          </Card>

          <div
            style={{
              display: 'grid',
              gridTemplateColumns: contentColumns,
              gap: 12,
              alignItems: 'stretch',
              minHeight: 'calc(100vh - 280px)',
            }}
          >
            {!isCompactScreen && (
              <div
                style={{
                  minWidth: 0,
                  overflow: 'hidden',
                  transition: 'width 0.2s',
                  display: 'flex',
                  flexDirection: 'column',
                }}
              >
                {collapsed ? (
                  <div
                    style={{
                      width: 48,
                      height: '100%',
                      border: '1px solid #f0f0f0',
                      borderRadius: 8,
                      display: 'flex',
                      flexDirection: 'column',
                      alignItems: 'center',
                      paddingTop: 12,
                      backgroundColor: '#fafafa',
                      cursor: 'pointer',
                    }}
                    onClick={() => setCollapsed(false)}
                  >
                    <Tooltip title="展开函数面板" placement="right">
                      <MenuUnfoldOutlined style={{ color: '#666' }} />
                    </Tooltip>
                    <div
                      style={{
                        marginTop: 16,
                        writingMode: 'vertical-rl',
                        fontSize: 12,
                        color: '#999',
                        letterSpacing: 2,
                      }}
                    >
                      函数面板
                    </div>
                  </div>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
                    <Card
                      size="small"
                      style={{ flexShrink: 0 }}
                      styles={{
                        body: {
                          padding: 14,
                          background:
                            'linear-gradient(180deg, rgba(22,119,255,0.05) 0%, rgba(22,119,255,0.01) 100%)',
                        },
                      }}
                    >
                      <Space direction="vertical" size={4} style={{ width: '100%' }}>
                        <Space wrap size={[8, 8]}>
                          <Tag color="blue">函数源</Tag>
                          <Tag>{`可用 ${availableFunctions.length}`}</Tag>
                        </Space>
                        <Typography.Text strong>选择能力供给</Typography.Text>
                        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                          先在这里筛选和拖拽函数，再到中间区域完成当前 Tab 的布局配置。
                        </Typography.Text>
                      </Space>
                    </Card>
                    <div style={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
                      <div
                        style={{
                          height: '100%',
                          border: '1px solid #f0f0f0',
                          borderRadius: 14,
                          overflow: 'hidden',
                          background: '#fff',
                        }}
                      >
                        <FunctionList
                          functions={availableFunctions}
                          onCollapse={() => setCollapsed(true)}
                        />
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}

            <div style={{ minWidth: 0, display: 'flex', flexDirection: 'column' }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 10, height: '100%' }}>
                <Card
                  size="small"
                  style={{ flexShrink: 0 }}
                  styles={{
                    body: {
                      padding: 14,
                      background:
                        'linear-gradient(180deg, rgba(250,173,20,0.08) 0%, rgba(250,173,20,0.02) 100%)',
                    },
                  }}
                >
                  <Space direction="vertical" size={4} style={{ width: '100%' }}>
                    <Space wrap size={[8, 8]}>
                      <Tag color="processing">当前 Tab 编辑</Tag>
                      <Tag>{activeTabTitle || '待选择 Tab'}</Tag>
                      <Tag>{viewMode === 3 ? '可边改边看预览' : '编辑模式'}</Tag>
                    </Space>
                    <Typography.Text strong>配置当前页面骨架</Typography.Text>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      中间区域负责当前 Tab 的函数、布局和详细配置。建议一次只处理一个
                      Tab，完成后再切换。
                    </Typography.Text>
                  </Space>
                </Card>
                <div
                  style={{
                    flex: 1,
                    minHeight: 0,
                    border: '1px solid #f0f0f0',
                    borderRadius: 14,
                    overflow: 'hidden',
                    background: '#fff',
                  }}
                >
                  <LayoutDesigner
                    config={config}
                    onChange={(newConfig) => handleConfigChange(newConfig, '更新布局')}
                    descriptors={availableFunctions}
                    activeTabKey={activeTabKey}
                    onActiveTabChange={setActiveTabKey}
                  />
                </div>
              </div>
            </div>

            {!previewAsDrawer && viewMode === 3 && (
              <div style={{ minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 10, height: '100%' }}>
                  <Card
                    size="small"
                    style={{ flexShrink: 0 }}
                    styles={{
                      body: {
                        padding: 14,
                        background:
                          'linear-gradient(180deg, rgba(82,196,26,0.08) 0%, rgba(82,196,26,0.02) 100%)',
                      },
                    }}
                  >
                    <Space direction="vertical" size={4} style={{ width: '100%' }}>
                      <Space wrap size={[8, 8]}>
                        <Tag color="success">预览结果</Tag>
                        <Tag>{activeTabTitle || '当前草稿'}</Tag>
                      </Space>
                      <Typography.Text strong>验证运行结果</Typography.Text>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        这里用来验证当前草稿的界面结果。优先检查页面层级、主流程和字段可读性，再回到编辑区微调。
                      </Typography.Text>
                    </Space>
                  </Card>
                  <div
                    style={{
                      flex: 1,
                      minHeight: 0,
                      border: '1px solid #f0f0f0',
                      borderRadius: 14,
                      overflow: 'hidden',
                      background: '#fff',
                    }}
                  >
                    <ConfigPreview
                      config={config}
                      currentTabKey={activeTabKey}
                      currentTabTitle={activeTabTitle}
                      appliedTemplateSummary={lastAppliedTemplateSummary}
                      onBackToEditor={() => setViewMode(2)}
                    />
                  </div>
                </div>
              </div>
            )}
          </div>
        </Space>

        {/* 模板管理弹窗 */}
        <TemplateManager
          visible={templateModalVisible}
          onClose={() => setTemplateModalVisible(false)}
          onSelect={handleTemplateSelect}
          currentConfig={config as Record<string, any>}
          initialBrowseMode={templateBrowseMode}
        />

        <Modal
          title="导入页面配置 JSON"
          open={importVisible}
          onCancel={() => setImportVisible(false)}
          onOk={() => handleImport().catch(() => {})}
          confirmLoading={importing}
          okText="校验并导入"
          cancelText="取消"
        >
          <Space direction="vertical" size={10} style={{ width: '100%' }}>
            <div style={{ color: '#666' }}>导入后将强制保存为当前对象草稿，不会直接发布。</div>
            <Space size={8}>
              <Button onClick={handleSelectImportFile} disabled={!access?.canWorkspaceEdit}>
                选择 JSON 文件
              </Button>
              <div style={{ color: '#999', fontSize: 12 }}>支持 .json 文件或粘贴 JSON 内容</div>
            </Space>
            <Input.TextArea
              value={importContent}
              onChange={(e) => setImportContent(e.target.value)}
              placeholder="请粘贴页面配置 JSON"
              autoSize={{ minRows: 10, maxRows: 18 }}
            />
            <input
              ref={fileInputRef}
              type="file"
              accept=".json,application/json"
              style={{ display: 'none' }}
              onChange={handleImportFileChange}
            />
          </Space>
        </Modal>

        <Drawer
          title="版本历史"
          open={versionsVisible}
          onClose={() => setVersionsVisible(false)}
          width={520}
          extra={
            <Space size={8}>
              <Select<VersionTimeRange>
                size="small"
                style={{ width: 120 }}
                value={versionTimeRange}
                onChange={(value) => setVersionTimeRange(value)}
                options={[
                  { label: '全部时间', value: 'all' },
                  { label: '最近 7 天', value: '7d' },
                  { label: '最近 30 天', value: '30d' },
                  { label: '最近 90 天', value: '90d' },
                ]}
              />
              <Button
                size="small"
                onClick={() => loadVersions().catch(() => {})}
                loading={versionsLoading}
              >
                刷新
              </Button>
            </Space>
          }
        >
          <List
            loading={versionsLoading}
            dataSource={versions}
            locale={{ emptyText: '暂无版本记录或接口未启用' }}
            renderItem={(item) => (
              <List.Item
                actions={[
                  <Button
                    key="compare"
                    size="small"
                    onClick={() =>
                      setCompareVersions({
                        old: item,
                        new: {
                          id: 'current',
                          version: 'current',
                          objectKey: config?.objectKey || objectKey,
                          config,
                        } as any,
                      })
                    }
                  >
                    对比当前
                  </Button>,
                  <Button
                    key="detail"
                    size="small"
                    onClick={() => handleOpenVersionDetail(item)}
                    loading={versionDetailLoading && versionDetailId === item.id}
                    disabled={!access?.canWorkspaceRead}
                  >
                    详情
                  </Button>,
                  <Button
                    key="rollback"
                    size="small"
                    danger
                    disabled={!access?.canWorkspaceRollback}
                    loading={rollingVersionId === item.id}
                    onClick={() => handleRollback(item)}
                  >
                    回滚
                  </Button>,
                ]}
              >
                <List.Item.Meta
                  title={
                    <Space size={8}>
                      <span>{`v${item.version}`}</span>
                      <Tag color="blue">{item.id}</Tag>
                      {item.isCurrentDraft && <Tag color="gold">当前草稿</Tag>}
                      {item.isCurrentPublished && <Tag color="green">当前发布</Tag>}
                    </Space>
                  }
                  description={[
                    item.createdAt
                      ? `时间: ${new Date(item.createdAt).toLocaleString('zh-CN')}`
                      : '',
                    item.createdBy ? `操作人: ${item.createdBy}` : '',
                    item.comment ? `备注: ${item.comment}` : '',
                    summarizeVersion(item),
                    `差异: ${summarizeDiff(config, item)}`,
                    (() => {
                      const diff = buildVersionDiff(config, item.config);
                      const total =
                        diff.fieldChanges.filter((line) => line !== '字段未变化').length +
                        diff.layoutChanges.filter((line) => line !== '布局未变化').length +
                        diff.tabChanges.filter((line) => line !== 'Tab 结构未变化').length;
                      return `Diff 项数: ${total}`;
                    })(),
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                />
              </List.Item>
            )}
          />
        </Drawer>

        <Modal
          title={selectedVersion ? `版本详情 v${selectedVersion.version}` : '版本详情'}
          open={Boolean(selectedVersion)}
          width={860}
          footer={null}
          onCancel={() => setSelectedVersion(null)}
        >
          {selectedVersion && (
            <Space direction="vertical" size={12} style={{ width: '100%' }}>
              <Descriptions size="small" bordered column={2}>
                <Descriptions.Item label="版本 ID">{selectedVersion.id}</Descriptions.Item>
                <Descriptions.Item label="对象">{selectedVersion.objectKey}</Descriptions.Item>
                <Descriptions.Item label="时间">
                  {selectedVersion.createdAt
                    ? new Date(selectedVersion.createdAt).toLocaleString('zh-CN')
                    : '-'}
                </Descriptions.Item>
                <Descriptions.Item label="操作人">
                  {selectedVersion.createdBy || '-'}
                </Descriptions.Item>
                <Descriptions.Item label="状态">
                  {resolveWorkspaceStatus(selectedVersion.config)}
                </Descriptions.Item>
                <Descriptions.Item label="标签页">
                  {selectedVersion.config?.layout?.type === 'tabs'
                    ? selectedVersion.config.layout.tabs?.length || 0
                    : 0}
                </Descriptions.Item>
              </Descriptions>
              <div style={{ border: '1px solid #f0f0f0', borderRadius: 6, padding: 12 }}>
                <div style={{ marginBottom: 8, fontWeight: 500 }}>配置 JSON</div>
                <pre
                  style={{
                    margin: 0,
                    maxHeight: 420,
                    overflow: 'auto',
                    background: '#fafafa',
                    border: '1px solid #f0f0f0',
                    padding: 12,
                    borderRadius: 6,
                    fontSize: 12,
                    lineHeight: 1.5,
                  }}
                >
                  {JSON.stringify(selectedVersion.config, null, 2)}
                </pre>
              </div>
              <div style={{ border: '1px solid #f0f0f0', borderRadius: 6, padding: 12 }}>
                <div style={{ marginBottom: 8, fontWeight: 500 }}>与当前草稿 Diff</div>
                <div
                  style={{
                    maxHeight: 320,
                    overflow: 'auto',
                    border: '1px solid #f0f0f0',
                    borderRadius: 6,
                    padding: 8,
                    background: '#fafafa',
                  }}
                >
                  <DiffViewer
                    title={undefined}
                    oldData={selectedVersion.config}
                    newData={config}
                    expanded={false}
                  />
                </div>
              </div>
            </Space>
          )}
        </Modal>

        <Modal
          title={
            compareVersions ? `版本对比: v${compareVersions.old.version} → 当前草稿` : '版本对比'
          }
          open={Boolean(compareVersions)}
          width={900}
          footer={<Button onClick={() => setCompareVersions(null)}>关闭</Button>}
          onCancel={() => setCompareVersions(null)}
        >
          {compareVersions && (
            <DiffViewer
              title={`v${compareVersions.old.version} vs 当前草稿`}
              oldData={compareVersions.old.config}
              newData={compareVersions.new.config}
              expanded={true}
            />
          )}
        </Modal>

        <Modal
          title="配置性能分析"
          open={performanceModalVisible}
          onCancel={() => setPerformanceModalVisible(false)}
          footer={<Button onClick={() => setPerformanceModalVisible(false)}>关闭</Button>}
          width={700}
        >
          {config?.layout?.type === 'tabs' && config.layout.tabs && (
            <Space direction="vertical" style={{ width: '100%' }} size={16}>
              <PerformancePanel
                title="总体性能"
                result={analyzeMultipleTabsPerformance(config.layout.tabs).overall}
              />
              <Collapse
                ghost
                size="small"
                items={config.layout.tabs.map((tab) => ({
                  key: tab.key,
                  label: `Tab: ${tab.title} (${tab.key})`,
                  children: (
                    <PerformancePanel
                      title={undefined}
                      result={
                        analyzeMultipleTabsPerformance(config.layout.tabs || []).byTab[tab.key]
                      }
                    />
                  ),
                }))}
              />
            </Space>
          )}
        </Modal>

        <Modal
          title="布局结构缩略图"
          open={thumbnailModalVisible}
          onCancel={() => setThumbnailModalVisible(false)}
          footer={<Button onClick={() => setThumbnailModalVisible(false)}>关闭</Button>}
          width={600}
        >
          {config?.layout?.type === 'tabs' && config.layout.tabs ? (
            <MultiTabThumbnail
              tabs={config.layout.tabs}
              onTabClick={(tabKey) => {
                setThumbnailModalVisible(false);
                // 可以添加滚动到对应 Tab 的逻辑
              }}
            />
          ) : config?.layout ? (
            <LayoutThumbnail
              tab={
                {
                  key: config.objectKey,
                  title: config.title,
                  functions: [],
                  layout: config.layout,
                } as any
              }
            />
          ) : (
            <EditorEmptyState
              title="当前没有可展示的配置"
              description="先保存或导入一个草稿，再查看这里的结构化内容。"
            />
          )}
        </Modal>

        {/* 配置测试工具 */}
        {config?.layout?.type === 'tabs' && config.layout.tabs && config.layout.tabs.length > 0 && (
          <ConfigTestTool
            visible={configTestVisible}
            onClose={() => setConfigTestVisible(false)}
            tab={config.layout.tabs[0]}
            descriptors={availableFunctions}
          />
        )}

        {/* 审计日志 */}
        <AuditLogPanel
          visible={auditLogVisible}
          onClose={() => setAuditLogVisible(false)}
          objectKey={objectKey}
        />

        {/* 协作面板 */}
        <Drawer
          title="发布前检查"
          placement="right"
          width={520}
          open={publishCheckVisible}
          onClose={() => setPublishCheckVisible(false)}
          styles={{
            body: { padding: 12 },
          }}
        >
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Card
              size="small"
              styles={{
                body: {
                  padding: 16,
                  background:
                    publishCheck.blockingCount > 0
                      ? 'linear-gradient(180deg, rgba(255,77,79,0.08) 0%, rgba(255,77,79,0.02) 100%)'
                      : publishCheck.warningCount > 0
                      ? 'linear-gradient(180deg, rgba(250,173,20,0.08) 0%, rgba(250,173,20,0.02) 100%)'
                      : 'linear-gradient(180deg, rgba(82,196,26,0.08) 0%, rgba(82,196,26,0.02) 100%)',
                },
              }}
            >
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <Space wrap size={[8, 8]}>
                  <Tag color="blue">{objectKey}</Tag>
                  <Tag color={publishCheck.blockingCount > 0 ? 'error' : publishCheck.warningCount > 0 ? 'warning' : 'success'}>
                    {publishCheck.headline}
                  </Tag>
                </Space>
                <Typography.Title level={5} style={{ margin: 0 }}>
                  {`当前发布评分 ${publishCheck.score}/100`}
                </Typography.Title>
                <Typography.Text type="secondary">{publishCheck.summary}</Typography.Text>
                <Space wrap size={[8, 8]}>
                  <Badge status={publishCheck.blockingCount > 0 ? 'error' : 'success'} text={`阻塞项 ${publishCheck.blockingCount}`} />
                  <Badge status={publishCheck.warningCount > 0 ? 'warning' : 'default'} text={`风险项 ${publishCheck.warningCount}`} />
                  <Badge status="processing" text={`通过项 ${publishCheck.readyCount}`} />
                </Space>
              </Space>
            </Card>

            {publishCheck.items.length > 0 ? (
              <Card size="small" title="全局检查结果">
                <Space direction="vertical" size={10} style={{ width: '100%' }}>
                  {publishCheck.items.map((item, index) => (
                    <Alert
                      key={`${item.title}-${index}`}
                      type={
                        item.level === 'blocking'
                          ? 'error'
                          : item.level === 'warning'
                          ? 'warning'
                          : 'success'
                      }
                      showIcon
                      message={item.title}
                      description={item.detail}
                    />
                  ))}
                </Space>
              </Card>
            ) : null}

            <Card size="small" title="逐页检查">
              <Collapse
                ghost
                items={publishCheck.tabs.map((tab) => {
                  const capability = getLayoutCapabilityMeta(tab.layoutType);
                  return {
                    key: tab.key,
                    label: (
                      <Space wrap size={[8, 8]}>
                        <Typography.Text strong>{tab.title}</Typography.Text>
                        <Tag>{tab.layoutType}</Tag>
                        <Tag color={capability.tone}>{capability.label}</Tag>
                        <Badge
                          status={
                            tab.level === 'blocking'
                              ? 'error'
                              : tab.level === 'warning'
                              ? 'warning'
                              : 'success'
                          }
                          text={tab.summary}
                        />
                      </Space>
                    ),
                    children: (
                      <Space direction="vertical" size={10} style={{ width: '100%' }}>
                        {tab.items.map((item, index) => (
                          <Alert
                            key={`${tab.key}-${index}-${item.title}`}
                            type={
                              item.level === 'blocking'
                                ? 'error'
                                : item.level === 'warning'
                                ? 'warning'
                                : 'success'
                            }
                            showIcon
                            message={item.title}
                            description={item.detail}
                          />
                        ))}
                      </Space>
                    ),
                  };
                })}
              />
            </Card>
          </Space>
        </Drawer>

        <Drawer
          title="协作状态"
          placement="right"
          width={380}
          open={collaborationVisible}
          onClose={() => setCollaborationVisible(false)}
          styles={{
            body: { padding: 12 },
          }}
        >
          <CollaborationPanel objectKey={objectKey} isAdmin={access?.canWorkspaceAdmin || false} />
        </Drawer>
        <Drawer
          title="函数面板"
          placement="left"
          width={Math.min(typeof window !== 'undefined' ? window.innerWidth - 24 : 420, 420)}
          open={functionLibraryVisible}
          onClose={() => setFunctionLibraryVisible(false)}
          destroyOnClose
        >
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Card
              size="small"
              styles={{
                body: {
                  padding: 14,
                  background:
                    'linear-gradient(180deg, rgba(22,119,255,0.05) 0%, rgba(22,119,255,0.01) 100%)',
                },
              }}
            >
              <Space direction="vertical" size={4} style={{ width: '100%' }}>
                <Space wrap size={[8, 8]}>
                  <Tag color="blue">函数源</Tag>
                  <Tag>{`可用 ${availableFunctions.length}`}</Tag>
                </Space>
                <Typography.Text strong>先选能力，再配页面</Typography.Text>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  先在这里确认对象可用函数，再回到编辑区完成当前 Tab 的绑定和布局配置。
                </Typography.Text>
              </Space>
            </Card>
            <FunctionList functions={availableFunctions} />
          </Space>
        </Drawer>

        <Drawer
          title="配置预览"
          placement="right"
          width={Math.min(typeof window !== 'undefined' ? window.innerWidth - 24 : 460, 460)}
          open={previewAsDrawer}
          onClose={() => setViewMode(2)}
          destroyOnClose
        >
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Card
              size="small"
              styles={{
                body: {
                  padding: 14,
                  background:
                    'linear-gradient(180deg, rgba(82,196,26,0.08) 0%, rgba(82,196,26,0.02) 100%)',
                },
              }}
            >
              <Space direction="vertical" size={4} style={{ width: '100%' }}>
                <Space wrap size={[8, 8]}>
                  <Tag color="success">预览结果</Tag>
                  <Tag>{activeTabTitle || '当前草稿'}</Tag>
                </Space>
                <Typography.Text strong>先看结果，再回去微调</Typography.Text>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  这里优先检查页面层级、关键流程和字段可读性。确认问题后，再切回编辑区微调。
                </Typography.Text>
              </Space>
            </Card>
            <ConfigPreview
              config={config}
              currentTabKey={activeTabKey}
              currentTabTitle={activeTabTitle}
              appliedTemplateSummary={lastAppliedTemplateSummary}
              onBackToEditor={() => setViewMode(2)}
            />
          </Space>
        </Drawer>

        {/* 草稿面板 */}
        <Drawer
          title="草稿管理"
          placement="right"
          width={400}
          open={draftVisible}
          onClose={() => setDraftVisible(false)}
          styles={{
            body: { padding: 12 },
          }}
        >
          <DraftPanel
            objectKey={objectKey}
            onRestore={(restoredConfig) => {
              handleConfigChange(restoredConfig, '恢复草稿');
              setDraftVisible(false);
              message.success('已恢复草稿');
            }}
          />
        </Drawer>

        {/* 新手引导 */}
        <QuickGuideButton
          onStart={() => {
            setGuideTourVisible(true);
            setGuideStep(0);
          }}
        />
        <GuideTour
          visible={guideTourVisible}
          currentStep={guideStep}
          onStepChange={setGuideStep}
          onClose={() => setGuideTourVisible(false)}
        />
      </PageContainer>
    </>
  );
}
