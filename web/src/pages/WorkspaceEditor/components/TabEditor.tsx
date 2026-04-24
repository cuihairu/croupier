import React, { useReducer, useMemo, useEffect, useRef, useState } from 'react';
import {
  Alert,
  Card,
  Collapse,
  Modal,
  Space,
  Tag,
  Typography,
  message,
  Button,
  Segmented,
} from 'antd';
import type { TabConfig, ColumnConfig, FieldConfig } from '@/types/workspace';
import type { FunctionDescriptor } from '@/services/api/functions';
import { descriptorToLayout } from '../utils/schemaToLayout';
import TabBasicInfo from './TabEditor/TabBasicInfo';
import TabFunctionManager from './TabEditor/TabFunctionManager';
import LayoutTypeSelector from './TabEditor/LayoutTypeSelector';
import OrchestrationWizard from './TabEditor/OrchestrationWizard';
import ColumnEditorModal from './TabEditor/ColumnEditorModal';
import FieldEditorModal from './TabEditor/FieldEditorModal';
import LayoutConfigRenderer from './TabEditor/LayoutConfigRenderer';
import {
  createDefaultLayout,
  buildOrchestrationLayout,
  buildDefaultOrchestratorBindings,
  getRolesForOrchestratorMode,
  mergeLayoutByMissing,
  buildLayoutDiffPreview,
  buildOrchestrationRiskTips,
  assessOrchestrationRiskLevel,
  type OrchestratorBindings,
} from './TabEditor/orchestrationUtils';
import {
  detectRecommendedScenario,
  createScenarioLayout,
  type ScenarioRecommendation,
} from './TabEditor/scenarioUtils';
import { healTabLayoutWithTemplate } from './TabEditor/healLayoutUtils';
import { useOrchestrationHistory } from './TabEditor/useOrchestrationHistory';

type OrchestratorMode = 'list' | 'form-detail';
type OrchestratorRole = 'list' | 'detail' | 'submit' | 'query' | 'data';
type OrchestratorApplyMode = 'overwrite' | 'merge';
type QuickLayoutMode = 'list' | 'form' | 'detail' | 'form-detail';

type TabEditorState = {
  editingColumn: ColumnConfig | null;
  editingField: FieldConfig | null;
  columnModalOpen: boolean;
  fieldModalOpen: boolean;
  layoutWizardDescriptor: FunctionDescriptor | null;
  orchestratorOpen: boolean;
  orchestratorMode: OrchestratorMode;
  orchestratorBindings: OrchestratorBindings | null;
  orchestratorApplyMode: OrchestratorApplyMode;
};

type TabEditorAction =
  | { type: 'openColumnEditor'; payload: ColumnConfig | null }
  | { type: 'closeColumnEditor' }
  | { type: 'openFieldEditor'; payload: FieldConfig | null }
  | { type: 'closeFieldEditor' }
  | { type: 'openLayoutWizard'; payload: FunctionDescriptor }
  | { type: 'closeLayoutWizard' }
  | { type: 'openOrchestrator' }
  | { type: 'closeOrchestrator' }
  | { type: 'setOrchestratorMode'; payload: OrchestratorMode }
  | { type: 'setOrchestratorBindings'; payload: OrchestratorBindings | null }
  | { type: 'setOrchestratorApplyMode'; payload: OrchestratorApplyMode };

const initialTabEditorState: TabEditorState = {
  editingColumn: null,
  editingField: null,
  columnModalOpen: false,
  fieldModalOpen: false,
  layoutWizardDescriptor: null,
  orchestratorOpen: false,
  orchestratorMode: 'form-detail',
  orchestratorBindings: null,
  orchestratorApplyMode: 'overwrite',
};

function tabEditorReducer(state: TabEditorState, action: TabEditorAction): TabEditorState {
  switch (action.type) {
    case 'openColumnEditor':
      return { ...state, editingColumn: action.payload, columnModalOpen: true };
    case 'closeColumnEditor':
      return { ...state, editingColumn: null, columnModalOpen: false };
    case 'openFieldEditor':
      return { ...state, editingField: action.payload, fieldModalOpen: true };
    case 'closeFieldEditor':
      return { ...state, editingField: null, fieldModalOpen: false };
    case 'openLayoutWizard':
      return { ...state, layoutWizardDescriptor: action.payload };
    case 'closeLayoutWizard':
      return { ...state, layoutWizardDescriptor: null };
    case 'openOrchestrator':
      return { ...state, orchestratorOpen: true };
    case 'closeOrchestrator':
      return { ...state, orchestratorOpen: false };
    case 'setOrchestratorMode':
      return { ...state, orchestratorMode: action.payload };
    case 'setOrchestratorBindings':
      return { ...state, orchestratorBindings: action.payload };
    case 'setOrchestratorApplyMode':
      return { ...state, orchestratorApplyMode: action.payload };
    default:
      return state;
  }
}

export interface TabEditorProps {
  tab: TabConfig;
  onChange: (tab: TabConfig) => void;
  descriptors?: FunctionDescriptor[];
}

function StepSectionHeader({
  step,
  title,
  summary,
  status,
  statusColor,
}: {
  step: number;
  title: string;
  summary: string;
  status: string;
  statusColor: string;
}) {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 12,
        width: '100%',
      }}
    >
      <Space direction="vertical" size={2} style={{ minWidth: 0 }}>
        <Typography.Text strong>{`${step}. ${title}`}</Typography.Text>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {summary}
        </Typography.Text>
      </Space>
      <Tag color={statusColor} bordered={false} style={{ margin: 0, flexShrink: 0 }}>
        {status}
      </Tag>
    </div>
  );
}

export default function TabEditor({ tab, onChange, descriptors = [] }: TabEditorProps) {
  const [uiState, dispatch] = useReducer(tabEditorReducer, initialTabEditorState);
  const autoWizardDismissedRef = useRef(false);
  const [activeSections, setActiveSections] = useState<string[]>(['basic', 'functions']);
  const [editorMode, setEditorMode] = useState<'basic' | 'advanced'>('basic');

  // Safe tab with defaults
  const safeTab = {
    ...tab,
    functions: tab?.functions || [],
    layout: tab?.layout || { type: 'form' },
  };

  const {
    undoStack: orchestratorUndoStack,
    redoStack: orchestratorRedoStack,
    pushToHistory: pushOrchestrationHistory,
    undo: undoOrchestration,
    redo: redoOrchestration,
    clearRedoStack,
  } = useOrchestrationHistory({
    maxStackSize: 10,
    onLayoutChange: (layout) => onChange({ ...safeTab, layout }),
    getCurrentLayout: () => safeTab.layout,
    enableHotkeys: true,
  });

  // Handlers
  const handleBasicChange = (field: string, value: any) => {
    onChange({ ...safeTab, [field]: value });
  };

  const handleLayoutTypeChange = (type: string) => {
    const primaryFunctionId = safeTab.functions[0];
    const primaryDescriptor = descriptors.find((d) => d.id === primaryFunctionId);
    const defaultLayout = createDefaultLayout(type, primaryFunctionId, primaryDescriptor);
    onChange({ ...safeTab, layout: defaultLayout });
  };

  const handleApplyScenario = (scenarioId: string) => {
    const primaryFunctionId = safeTab.functions[0] || '';
    const scenarioLayout = createScenarioLayout(scenarioId, primaryFunctionId);
    if (!scenarioLayout) {
      message.warning('未识别的场景模板');
      return;
    }
    onChange({ ...safeTab, layout: scenarioLayout });
    message.success('已应用场景模板');
  };

  const handleAutoLayout = () => {
    if (safeTab.functions.length === 0) {
      message.warning('请先添加函数');
      return;
    }
    const firstFuncId = safeTab.functions[0];
    const descriptor = descriptors.find((d) => d.id === firstFuncId);
    if (!descriptor) {
      message.warning('未找到函数描述符，无法自动推导');
      return;
    }
    const autoLayout = descriptorToLayout(descriptor);
    onChange({ ...safeTab, layout: autoLayout });
    message.success(`已自动生成 ${autoLayout.type} 布局`);
  };

  const handleHealLayout = () => {
    const nextTab = healTabLayoutWithTemplate(safeTab, descriptors);
    onChange(nextTab);
    message.success('已补全当前布局缺失配置');
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    const funcData = e.dataTransfer.getData('function');
    if (!funcData) return;
    try {
      const func: FunctionDescriptor = JSON.parse(funcData);
      if (!safeTab.functions.includes(func.id)) {
        const newFunctions = [...safeTab.functions, func.id];
        if (safeTab.functions.length === 0) {
          const autoLayout = descriptorToLayout(func);
          onChange({ ...safeTab, functions: newFunctions, layout: autoLayout });
          message.success(`已添加函数并自动生成 ${autoLayout.type} 布局`);
        } else {
          const nextTab = healTabLayoutWithTemplate(
            { ...safeTab, functions: newFunctions },
            descriptors,
          );
          onChange(nextTab);
          message.success('函数已添加并自动补全布局缺失配置');
        }
      } else {
        message.warning('函数已存在');
      }
    } catch {
      message.error('添加函数失败');
    }
  };

  const handleRemoveFunction = (functionId: string) => {
    const nextFunctions = safeTab.functions.filter((f) => f !== functionId);
    const nextLayout = { ...(safeTab.layout as any) };
    if (nextLayout.listFunction === functionId) nextLayout.listFunction = '';
    if (nextLayout.submitFunction === functionId) nextLayout.submitFunction = '';
    if (nextLayout.detailFunction === functionId) nextLayout.detailFunction = '';
    if (nextLayout.queryFunction === functionId) nextLayout.queryFunction = '';
    const nextTab = healTabLayoutWithTemplate(
      { ...safeTab, functions: nextFunctions, layout: nextLayout },
      descriptors,
    );
    onChange(nextTab);
  };

  const handleApplyFunctionLayout = (descriptor: FunctionDescriptor, mode: QuickLayoutMode) => {
    const layout = createDefaultLayout(mode, descriptor.id, descriptor);
    const nextFunctions = safeTab.functions.includes(descriptor.id)
      ? safeTab.functions
      : [...safeTab.functions, descriptor.id];
    onChange({ ...safeTab, functions: nextFunctions, layout });
    dispatch({ type: 'closeLayoutWizard' });
    message.success(`已基于 ${descriptor.displayName?.zh || descriptor.id} 生成 ${mode} 布局`);
  };

  // Orchestration
  const orchestrationPlan = useMemo(
    () =>
      buildOrchestrationLayout(
        uiState.orchestratorMode,
        safeTab.functions,
        descriptors,
        uiState.orchestratorBindings,
      ),
    [uiState.orchestratorMode, safeTab.functions, descriptors, uiState.orchestratorBindings],
  );

  const defaultBindings = useMemo(
    () => buildDefaultOrchestratorBindings(safeTab.functions, descriptors),
    [safeTab.functions, descriptors],
  );

  useEffect(() => {
    if (!uiState.orchestratorOpen) return;
    dispatch({ type: 'setOrchestratorBindings', payload: defaultBindings });
  }, [uiState.orchestratorOpen, defaultBindings]);

  const activeOrchestratorRoles = useMemo(
    () => getRolesForOrchestratorMode(uiState.orchestratorMode),
    [uiState.orchestratorMode],
  );

  const invalidOrchestratorRoles = useMemo(() => {
    if (!uiState.orchestratorBindings) return [] as OrchestratorRole[];
    const functionSet = new Set(safeTab.functions);
    return activeOrchestratorRoles.filter(
      (role) => !functionSet.has(uiState.orchestratorBindings![role]),
    ) as OrchestratorRole[];
  }, [uiState.orchestratorBindings, safeTab.functions, activeOrchestratorRoles]);

  const displayedAssignments = useMemo(() => {
    if (!orchestrationPlan) return [];
    const prefixes = new Set(activeOrchestratorRoles.map((r) => `${r} -> `));
    return orchestrationPlan.assignments.filter((line) =>
      Array.from(prefixes).some((prefix) => line.startsWith(prefix)),
    );
  }, [orchestrationPlan, activeOrchestratorRoles]);

  const previewNextLayout = useMemo(() => {
    if (!orchestrationPlan) return null;
    return uiState.orchestratorApplyMode === 'merge'
      ? mergeLayoutByMissing(safeTab.layout as any, orchestrationPlan.layout)
      : orchestrationPlan.layout;
  }, [orchestrationPlan, uiState.orchestratorApplyMode, safeTab.layout]);

  const layoutDiffPreview = useMemo(() => {
    if (!previewNextLayout) return [];
    return buildLayoutDiffPreview(safeTab.layout as any, previewNextLayout);
  }, [previewNextLayout, safeTab.layout]);

  const orchestrationRiskTips = useMemo(
    () =>
      buildOrchestrationRiskTips(
        uiState.orchestratorApplyMode,
        safeTab.layout as any,
        previewNextLayout,
        layoutDiffPreview,
      ),
    [uiState.orchestratorApplyMode, safeTab.layout, previewNextLayout, layoutDiffPreview],
  );

  const handleApplyOrchestration = () => {
    if (!orchestrationPlan) {
      message.warning('当前函数不足，无法生成编排方案');
      return;
    }
    if (invalidOrchestratorRoles.length > 0) {
      message.warning(`存在失效角色绑定: ${invalidOrchestratorRoles.join(', ')}，请先调整`);
      return;
    }
    const riskLevel = assessOrchestrationRiskLevel(
      uiState.orchestratorApplyMode,
      safeTab.layout as any,
      previewNextLayout,
      layoutDiffPreview,
    );
    const doApply = () => {
      pushOrchestrationHistory(safeTab.layout);
      const nextLayout =
        uiState.orchestratorApplyMode === 'merge'
          ? mergeLayoutByMissing(safeTab.layout as any, orchestrationPlan.layout)
          : orchestrationPlan.layout;
      onChange({ ...safeTab, layout: nextLayout });
      dispatch({ type: 'closeOrchestrator' });
      message.success(
        `已应用多函数编排：${uiState.orchestratorMode}（${
          uiState.orchestratorApplyMode === 'merge' ? '仅补空字段' : '覆盖当前'
        }）`,
      );
    };

    if (riskLevel === 'high') {
      Modal.confirm({
        title: '高风险变更确认',
        content: '本次编排将产生高影响改动（覆盖/关键绑定变化/布局切换），确认继续应用？',
        okText: '确认应用',
        cancelText: '取消',
        okButtonProps: { danger: true },
        onOk: doApply,
      });
      return;
    }

    if (riskLevel === 'medium') {
      Modal.confirm({
        title: '变更确认',
        content: '本次编排将更新当前布局配置，是否继续？',
        okText: '继续',
        cancelText: '取消',
        onOk: doApply,
      });
      return;
    }

    doApply();
  };

  const handleUndoOrchestration = () => {
    undoOrchestration(safeTab.layout);
  };

  const handleRedoOrchestration = () => {
    redoOrchestration(safeTab.layout);
  };

  // Clear redo stack on external layout change
  useEffect(() => {
    if (orchestratorRedoStack.length > 0) {
      const lastRedo = orchestratorRedoStack[orchestratorRedoStack.length - 1];
      if (JSON.stringify(lastRedo) === JSON.stringify(safeTab.layout)) {
        return;
      }
      clearRedoStack();
    }
  }, [safeTab.layout, orchestratorRedoStack, clearRedoStack]);

  // Scenario recommendation
  const recommendedScenario = useMemo((): ScenarioRecommendation | null => {
    const funcDescriptors = safeTab.functions
      .map((id) => descriptors.find((d) => d.id === id))
      .filter(Boolean) as FunctionDescriptor[];
    if (funcDescriptors.length === 0) return null;
    return detectRecommendedScenario(funcDescriptors);
  }, [safeTab.functions, descriptors]);

  const shouldPromptLayoutWizard = useMemo(() => {
    if (safeTab.functions.length !== 1) return false;
    if (uiState.layoutWizardDescriptor) return false;
    if (autoWizardDismissedRef.current) return false;
    const layout = safeTab.layout as any;
    const hasConfiguredStructure =
      Boolean(
        layout?.listFunction ||
          layout?.submitFunction ||
          layout?.detailFunction ||
          layout?.queryFunction,
      ) ||
      (Array.isArray(layout?.columns) && layout.columns.length > 0) ||
      (Array.isArray(layout?.fields) && layout.fields.length > 0) ||
      (Array.isArray(layout?.queryFields) && layout.queryFields.length > 0) ||
      (Array.isArray(layout?.sections) && layout.sections.length > 0);
    return !hasConfiguredStructure;
  }, [safeTab.functions, safeTab.layout, uiState.layoutWizardDescriptor]);

  useEffect(() => {
    if (!shouldPromptLayoutWizard) return;
    const descriptor = descriptors.find((d) => d.id === safeTab.functions[0]);
    if (!descriptor) return;
    dispatch({ type: 'openLayoutWizard', payload: descriptor });
  }, [shouldPromptLayoutWizard, descriptors, safeTab.functions]);

  // Column/Field editor handlers
  const handleColumnSave = (values: ColumnConfig) => {
    const keyLower = String(values?.key || '').toLowerCase();
    const numericHint =
      values.render === 'money' ||
      keyLower.includes('count') ||
      keyLower.includes('num') ||
      keyLower.includes('amount') ||
      keyLower.includes('price') ||
      keyLower.includes('score') ||
      keyLower.includes('level') ||
      keyLower.includes('total');
    const normalizedValues: ColumnConfig = {
      ...values,
      align: values.align || (numericHint ? 'right' : undefined),
    };

    const layout = safeTab.layout as any;
    const cols: ColumnConfig[] = layout.columns || [];
    if (uiState.editingColumn) {
      onChange({
        ...safeTab,
        layout: {
          ...layout,
          columns: cols.map((c) =>
            c.key === uiState.editingColumn!.key ? { ...c, ...normalizedValues } : c,
          ),
        },
      });
    } else {
      onChange({ ...safeTab, layout: { ...layout, columns: [...cols, normalizedValues] } });
    }
    dispatch({ type: 'closeColumnEditor' });
  };

  const handleFieldSave = (values: FieldConfig) => {
    const layout = safeTab.layout as any;
    const fieldsKey = layout.type === 'form' ? 'fields' : 'queryFields';
    const fields: FieldConfig[] = layout[fieldsKey] || [];
    if (uiState.editingField) {
      onChange({
        ...safeTab,
        layout: {
          ...layout,
          [fieldsKey]: fields.map((f) =>
            f.key === uiState.editingField!.key ? { ...f, ...values } : f,
          ),
        },
      });
    } else {
      onChange({ ...safeTab, layout: { ...layout, [fieldsKey]: [...fields, values] } });
    }
    dispatch({ type: 'closeFieldEditor' });
  };

  const handleOpenColumnEditor = (column: ColumnConfig | null) => {
    dispatch({ type: 'openColumnEditor', payload: column });
  };

  const handleOpenFieldEditor = (field: FieldConfig | null) => {
    dispatch({ type: 'openFieldEditor', payload: field });
  };

  const tabSummary = useMemo(() => {
    const functionCount = safeTab.functions.length;
    const layoutType = safeTab.layout?.type || 'form';
    const fieldCount = Array.isArray((safeTab.layout as any)?.fields)
      ? (safeTab.layout as any).fields.length
      : Array.isArray((safeTab.layout as any)?.queryFields)
      ? (safeTab.layout as any).queryFields.length
      : 0;
    const columnCount = Array.isArray((safeTab.layout as any)?.columns)
      ? (safeTab.layout as any).columns.length
      : 0;
    return { functionCount, layoutType, fieldCount, columnCount };
  }, [safeTab]);

  const sectionGuide =
    tabSummary.functionCount === 0
      ? '先绑定函数，再生成页面骨架，系统才能更准确地帮你补齐配置。'
      : '函数已绑定，可以继续生成页面骨架，再细化字段、列和详情结构。';

  const currentStep =
    tabSummary.functionCount === 0
      ? 2
      : tabSummary.columnCount + tabSummary.fieldCount === 0
      ? 3
      : 4;

  const defaultActiveKeys = useMemo(() => {
    if (tabSummary.functionCount === 0) {
      return ['basic', 'functions'];
    }
    return ['basic', 'functions', 'layout', 'config'];
  }, [tabSummary.functionCount]);

  useEffect(() => {
    setActiveSections((current) => {
      const next = current.length > 0 ? current : defaultActiveKeys;
      return Array.from(new Set(next));
    });
  }, [defaultActiveKeys]);

  const openSection = (sectionKey: string) => {
    setActiveSections((current) => {
      const next = current.filter(Boolean);
      if (next.includes(sectionKey)) return next;
      return [...next, sectionKey];
    });
  };

  const focusStep = (sectionKey: string) => {
    openSection(sectionKey);
    if (sectionKey === 'functions' || sectionKey === 'layout' || sectionKey === 'config') {
      setActiveSections((current) => {
        const ordered = ['basic', 'functions', 'layout', 'config'].filter(
          (key) => key === sectionKey || current.includes(key),
        );
        return Array.from(new Set(ordered));
      });
    }
  };

  const quickNextAction =
    currentStep === 2
      ? {
          label: '去绑定主函数',
          onClick: () => focusStep('functions'),
        }
      : currentStep === 3
      ? {
          label: safeTab.functions.length === 1 ? '生成基础页面骨架' : '去选择页面骨架',
          onClick: () => {
            focusStep('layout');
            if (safeTab.functions.length === 1) {
              const descriptor = descriptors.find((d) => d.id === safeTab.functions[0]);
              if (descriptor) {
                dispatch({ type: 'openLayoutWizard', payload: descriptor });
              }
            }
          },
        }
      : {
          label: '去补页面细节',
          onClick: () => focusStep('config'),
        };

  const advancedCapabilitySummary =
    safeTab.functions.length > 1
      ? '当前已绑定多个函数，可以使用多函数编排向导生成更复杂的页面结构。'
      : '高级模式会暴露多函数编排、布局补全和编排撤销等能力，默认多数页面不需要先用它。';

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      <Card
        size="small"
        styles={{
          body: {
            padding: 16,
            background:
              'linear-gradient(180deg, rgba(22,119,255,0.04) 0%, rgba(255,255,255,0.98) 100%)',
          },
        }}
      >
        <Space direction="vertical" size={10} style={{ width: '100%' }}>
          <Space wrap size={[8, 8]}>
            <Tag color="blue">{safeTab.title || safeTab.key || '未命名 Tab'}</Tag>
            <Tag>{`函数 ${tabSummary.functionCount}`}</Tag>
            <Tag>{`布局 ${tabSummary.layoutType}`}</Tag>
            {tabSummary.columnCount > 0 ? <Tag>{`列 ${tabSummary.columnCount}`}</Tag> : null}
            {tabSummary.fieldCount > 0 ? <Tag>{`字段 ${tabSummary.fieldCount}`}</Tag> : null}
            {safeTab.defaultActive ? <Tag color="success">默认页</Tag> : null}
            {safeTab.locked ? <Tag color="warning">已锁定</Tag> : null}
          </Space>
          <Typography.Text type="secondary">{sectionGuide}</Typography.Text>
          <Card size="small">
            <Space direction="vertical" size={10} style={{ width: '100%' }}>
              <Space wrap size={[8, 8]}>
                <Tag color={currentStep >= 4 ? 'success' : 'processing'}>
                  {currentStep === 2
                    ? '下一步：绑定主函数'
                    : currentStep === 3
                    ? '下一步：生成页面骨架'
                    : currentStep === 4
                    ? '下一步：补页面细节'
                    : '下一步：确认基本信息'}
                </Tag>
                <Tag>{`当前布局 ${tabSummary.layoutType}`}</Tag>
                {tabSummary.functionCount > 0 ? (
                  <Tag color="success">{`已绑函数 ${tabSummary.functionCount}`}</Tag>
                ) : (
                  <Tag color="orange">当前待绑函数</Tag>
                )}
                {tabSummary.columnCount + tabSummary.fieldCount > 0 ? (
                  <Tag color="green">{`已配置 ${
                    tabSummary.columnCount + tabSummary.fieldCount
                  } 项`}</Tag>
                ) : (
                  <Tag color="blue">待补核心字段</Tag>
                )}
              </Space>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {currentStep === 2
                  ? '先给当前页面挂上查询、表单或详情函数，再决定它到底是列表页、表单页还是详情页。'
                  : currentStep === 3
                  ? '优先用自动生成或推荐场景起一个能跑的结构，不要一开始就钻进高级编排。'
                  : currentStep === 4
                  ? '页面已经有基础结构，接下来重点检查列、字段和分区是否符合运营使用习惯。'
                  : '先确认这个页面的标题、图标和默认页设置。'}
              </Typography.Text>
              <Space wrap size={[8, 8]}>
                <Button size="small" type="primary" onClick={quickNextAction.onClick}>
                  {quickNextAction.label}
                </Button>
              </Space>
            </Space>
          </Card>
          <Space wrap>
            <Tag color={currentStep <= 1 ? 'processing' : 'default'}>1. 基本信息</Tag>
            <Tag color={currentStep <= 2 ? 'processing' : currentStep > 2 ? 'success' : 'default'}>
              2. 绑定函数
            </Tag>
            <Tag color={currentStep <= 3 ? 'processing' : currentStep > 3 ? 'success' : 'default'}>
              3. 生成页面骨架
            </Tag>
            <Tag color={currentStep >= 4 ? 'processing' : 'default'}>4. 补页面细节</Tag>
          </Space>
          <Card size="small" style={{ background: 'rgba(255,255,255,0.82)' }}>
            <Space direction="vertical" size={10} style={{ width: '100%' }}>
              <Space wrap size={[8, 8]} style={{ justifyContent: 'space-between', width: '100%' }}>
                <Typography.Text strong>编辑模式</Typography.Text>
                <Segmented
                  size="small"
                  value={editorMode}
                  onChange={(value) => setEditorMode(value as 'basic' | 'advanced')}
                  options={[
                    { label: '基础模式', value: 'basic' },
                    { label: '高级模式', value: 'advanced' },
                  ]}
                />
              </Space>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {editorMode === 'basic'
                  ? '默认只保留大多数用户最常用的骨架生成和页面细化流，避免首屏信息过载。'
                  : '高级模式会展开多函数编排和进阶修复工具，适合复杂页面或实施团队精细调优。'}
              </Typography.Text>
            </Space>
          </Card>
        </Space>
      </Card>

      <Collapse
        activeKey={activeSections}
        onChange={(keys) => setActiveSections(Array.isArray(keys) ? (keys as string[]) : [keys])}
        items={[
          {
            key: 'basic',
            label: (
              <StepSectionHeader
                step={1}
                title="基本信息"
                summary="页面名、图标、默认页"
                status="基础设置"
                statusColor="default"
              />
            ),
            children: <TabBasicInfo tab={safeTab} onChange={handleBasicChange} />,
          },
          {
            key: 'functions',
            label: (
              <StepSectionHeader
                step={2}
                title="函数绑定"
                summary={
                  tabSummary.functionCount === 0
                    ? '当前尚未绑定函数，先确定主函数'
                    : `已绑定 ${tabSummary.functionCount} 个函数，主函数优先排第一位`
                }
                status={tabSummary.functionCount === 0 ? '当前待办' : '已完成基础绑定'}
                statusColor={tabSummary.functionCount === 0 ? 'orange' : 'green'}
              />
            ),
            children: (
              <TabFunctionManager
                tab={safeTab}
                descriptors={descriptors}
                onDrop={handleDrop}
                onRemoveFunction={handleRemoveFunction}
                onAddFunctions={(functionIds) => {
                  const nextFunctions = [...(safeTab.functions || []), ...functionIds];
                  onChange({ ...safeTab, functions: nextFunctions });
                }}
                onOpenLayoutWizard={(descriptor) =>
                  dispatch({ type: 'openLayoutWizard', payload: descriptor })
                }
              />
            ),
          },
          {
            key: 'layout',
            label: (
              <StepSectionHeader
                step={3}
                title="页面骨架"
                summary={
                  tabSummary.functionCount === 0
                    ? '先绑定函数后再选择布局'
                    : `当前布局 ${tabSummary.layoutType}，优先自动生成`
                }
                status={
                  tabSummary.functionCount === 0
                    ? '等待前置步骤'
                    : tabSummary.columnCount + tabSummary.fieldCount === 0
                    ? '推荐现在处理'
                    : '已有基础骨架'
                }
                statusColor={
                  tabSummary.functionCount === 0
                    ? 'default'
                    : tabSummary.columnCount + tabSummary.fieldCount === 0
                    ? 'orange'
                    : 'green'
                }
              />
            ),
            children: (
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  {tabSummary.functionCount === 0
                    ? '先回到上一步绑定主函数。绑定后，这里会更适合自动生成列表页、表单页或详情页。'
                    : '优先走自动生成或推荐场景，先把页面跑起来，只有标准路径不够时再使用高级布局工具。'}
                </Typography.Text>
                <LayoutTypeSelector
                  layoutType={safeTab.layout.type}
                  functions={safeTab.functions}
                  descriptors={descriptors}
                  recommendedScenario={recommendedScenario}
                  showAdvancedActions={editorMode === 'advanced'}
                  undoStackLength={orchestratorUndoStack.length}
                  redoStackLength={orchestratorRedoStack.length}
                  onLayoutTypeChange={handleLayoutTypeChange}
                  onApplyScenario={handleApplyScenario}
                  onApplyLayout={(layout) => onChange({ ...safeTab, layout })}
                  onAutoLayout={handleAutoLayout}
                  onHealLayout={handleHealLayout}
                  onUndo={handleUndoOrchestration}
                  onRedo={handleRedoOrchestration}
                />
              </Space>
            ),
          },
          {
            key: 'config',
            label: (
              <StepSectionHeader
                step={4}
                title="页面细节"
                summary={
                  tabSummary.columnCount > 0 || tabSummary.fieldCount > 0
                    ? `已配置 ${tabSummary.columnCount + tabSummary.fieldCount} 项，继续细化即可`
                    : '从核心字段、主列或详情分区开始'
                }
                status={
                  tabSummary.columnCount > 0 || tabSummary.fieldCount > 0
                    ? '可继续细化'
                    : '下一步补主字段/主列'
                }
                statusColor={
                  tabSummary.columnCount > 0 || tabSummary.fieldCount > 0 ? 'green' : 'blue'
                }
              />
            ),
            children: (
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  优先完成主函数、主字段或主列配置；导入导出和 JSON
                  已收进工具区，只有标准编辑不够时再展开。
                </Typography.Text>
                <LayoutConfigRenderer
                  layout={safeTab.layout}
                  tab={safeTab}
                  descriptors={descriptors}
                  onTabChange={onChange}
                  onOpenColumnEditor={handleOpenColumnEditor}
                  onOpenFieldEditor={handleOpenFieldEditor}
                />
              </Space>
            ),
          },
        ]}
      />

      {editorMode === 'advanced' ? (
        <Card
          size="small"
          title="高级能力"
          styles={{
            body: {
              background:
                'linear-gradient(180deg, rgba(250,173,20,0.06) 0%, rgba(255,255,255,0.98) 100%)',
            },
          }}
        >
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Alert
              type="warning"
              showIcon
              message="高级模式只在标准编辑流不够时使用"
              description={advancedCapabilitySummary}
            />
            <Space wrap size={[8, 8]}>
              <Button
                type="primary"
                disabled={safeTab.functions.length < 2}
                onClick={() => dispatch({ type: 'openOrchestrator' })}
              >
                打开多函数编排向导
              </Button>
              <Button size="small" icon={<ThunderboltOutlined />} onClick={handleHealLayout}>
                一键补全当前布局
              </Button>
              <Button
                size="small"
                disabled={orchestratorUndoStack.length === 0}
                onClick={handleUndoOrchestration}
              >
                撤销编排
              </Button>
              <Button
                size="small"
                disabled={orchestratorRedoStack.length === 0}
                onClick={handleRedoOrchestration}
              >
                重做编排
              </Button>
            </Space>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              多函数编排适合主从页、查询-处理复合页或复杂对象工作台。单函数列表、表单、详情页优先保持在基础模式完成。
            </Typography.Text>
          </Space>
        </Card>
      ) : null}

      <Modal
        title={`界面向导: ${
          uiState.layoutWizardDescriptor?.displayName?.zh ||
          uiState.layoutWizardDescriptor?.id ||
          ''
        }`}
        open={!!uiState.layoutWizardDescriptor}
        footer={null}
        onCancel={() => {
          autoWizardDismissedRef.current = true;
          dispatch({ type: 'closeLayoutWizard' });
        }}
      >
        {uiState.layoutWizardDescriptor && (
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Alert
              type="info"
              showIcon
              message="先选一个最接近业务意图的页面骨架"
              description="不用一次选对所有细节。先生成一个基础页，后面仍然可以继续改字段、改布局或重新生成。"
            />
            <Space wrap>
              <Button
                onClick={() => handleApplyFunctionLayout(uiState.layoutWizardDescriptor!, 'list')}
              >
                生成为列表页
              </Button>
              <Button
                onClick={() => handleApplyFunctionLayout(uiState.layoutWizardDescriptor!, 'form')}
              >
                生成为表单页
              </Button>
              <Button
                onClick={() => handleApplyFunctionLayout(uiState.layoutWizardDescriptor!, 'detail')}
              >
                生成为详情页
              </Button>
              <Button
                onClick={() =>
                  handleApplyFunctionLayout(uiState.layoutWizardDescriptor!, 'form-detail')
                }
              >
                生成为查询详情页
              </Button>
            </Space>
          </Space>
        )}
      </Modal>

      <OrchestrationWizard
        open={uiState.orchestratorOpen}
        mode={uiState.orchestratorMode}
        applyMode={uiState.orchestratorApplyMode}
        bindings={uiState.orchestratorBindings}
        functions={safeTab.functions}
        descriptors={descriptors}
        orchestrationPlan={orchestrationPlan}
        activeRoles={activeOrchestratorRoles}
        invalidRoles={invalidOrchestratorRoles}
        defaultBindings={defaultBindings}
        displayedAssignments={displayedAssignments}
        riskTips={orchestrationRiskTips}
        diffPreview={layoutDiffPreview}
        onClose={() => dispatch({ type: 'closeOrchestrator' })}
        onApply={handleApplyOrchestration}
        onModeChange={(mode) => dispatch({ type: 'setOrchestratorMode', payload: mode })}
        onApplyModeChange={(mode) => dispatch({ type: 'setOrchestratorApplyMode', payload: mode })}
        onBindingsChange={(bindings) =>
          dispatch({ type: 'setOrchestratorBindings', payload: bindings })
        }
        onResetBindings={() =>
          dispatch({ type: 'setOrchestratorBindings', payload: defaultBindings })
        }
      />

      <ColumnEditorModal
        open={uiState.columnModalOpen}
        editingColumn={uiState.editingColumn}
        onOk={handleColumnSave}
        onCancel={() => dispatch({ type: 'closeColumnEditor' })}
      />

      <FieldEditorModal
        open={uiState.fieldModalOpen}
        editingField={uiState.editingField}
        onOk={handleFieldSave}
        onCancel={() => dispatch({ type: 'closeFieldEditor' })}
      />
    </Space>
  );
}
