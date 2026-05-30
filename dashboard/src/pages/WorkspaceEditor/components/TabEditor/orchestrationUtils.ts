import type { TabConfig, ColumnConfig, FieldConfig } from '@/types/workspace';
import type { FunctionDescriptor } from '@/services/api/functions';
import {
  schemaToColumns,
  schemaToDetailSections,
  schemaToFields,
} from '../../utils/schemaToLayout';

type OrchestratorMode = 'list' | 'form-detail' | 'split' | 'dashboard';
type OrchestratorRole = 'list' | 'detail' | 'submit' | 'query' | 'data';
export type OrchestratorBindings = Record<OrchestratorRole, string>;

export function cloneLayout(layout: any): any {
  try {
    return JSON.parse(JSON.stringify(layout ?? {}));
  } catch {
    return layout;
  }
}

export function createDefaultLayout(
  type: string,
  primaryFunctionId: string,
  primaryDescriptor?: FunctionDescriptor,
): any {
  const autoColumns = primaryDescriptor ? schemaToColumns(primaryDescriptor) : [];
  const autoFields = primaryDescriptor ? schemaToFields(primaryDescriptor) : [];
  const autoSections = primaryDescriptor ? schemaToDetailSections(primaryDescriptor) : [];
  const fallbackColumns = [
    { key: 'id', title: 'ID' },
    { key: 'name', title: '名称' },
    { key: 'status', title: '状态' },
    { key: 'updatedAt', title: '更新时间', render: 'datetime' },
  ];
  const fallbackFields = [
    { key: 'name', label: '名称', type: 'input', required: true },
    { key: 'status', label: '状态', type: 'select' },
  ];
  const fallbackSections = [
    {
      title: '基础信息',
      column: 2,
      fields: [
        { key: 'id', label: 'ID' },
        { key: 'name', label: '名称' },
        { key: 'status', label: '状态' },
      ],
    },
  ];
  const defaultColumns = autoColumns.length > 0 ? autoColumns : fallbackColumns;
  const defaultFields = autoFields.length > 0 ? autoFields : fallbackFields;
  const defaultSections = autoSections.length > 0 ? autoSections : fallbackSections;

  switch (type) {
    case 'form-detail':
      return {
        type: 'form-detail',
        queryFunction: primaryFunctionId || '',
        queryFields: defaultFields.slice(0, 4),
        detailSections: defaultSections,
      };
    case 'list':
      return {
        type: 'list',
        listFunction: primaryFunctionId || '',
        columns: defaultColumns,
      };
    case 'form':
      return {
        type: 'form',
        submitFunction: primaryFunctionId || '',
        fields: defaultFields,
      };
    case 'detail':
      return {
        type: 'detail',
        detailFunction: primaryFunctionId || '',
        sections: defaultSections,
      };
    case 'kanban':
      return {
        type: 'kanban',
        dataFunction: primaryFunctionId || '',
        columns: [
          { id: 'todo', title: '待处理', color: '#1677ff' },
          { id: 'processing', title: '处理中', color: '#faad14' },
          { id: 'done', title: '已完成', color: '#52c41a' },
        ],
      };
    case 'timeline':
      return {
        type: 'timeline',
        dataFunction: primaryFunctionId || '',
        showFilter: true,
        reverse: true,
      };
    case 'split':
      return {
        type: 'split',
        direction: 'horizontal',
        panels: [
          {
            key: 'left',
            title: '列表',
            span: 12,
            component: {
              type: 'list',
              config: {
                listFunction: primaryFunctionId || '',
                columns: defaultColumns,
              },
            },
          },
          {
            key: 'right',
            title: '详情',
            span: 12,
            component: {
              type: 'detail',
              config: {
                detailFunction: primaryFunctionId || '',
                sections: defaultSections,
              },
            },
          },
        ],
      };
    case 'wizard':
      return {
        type: 'wizard',
        steps: [
          {
            key: 'step1',
            title: '步骤1',
            component: {
              type: 'form',
              config: {
                submitFunction: primaryFunctionId || '',
                fields: defaultFields.slice(0, 3),
              },
            },
          },
        ],
      };
    case 'dashboard':
      return {
        type: 'dashboard',
        stats: [
          { key: 'total', title: '总数', value: 0 },
          { key: 'active', title: '活跃', value: 0 },
        ],
        panels: [
          {
            key: 'list',
            title: '列表',
            span: 12,
            component: {
              type: 'list',
              config: {
                listFunction: primaryFunctionId || '',
                columns: defaultColumns,
              },
            },
          },
        ],
      };
    case 'grid':
      return {
        type: 'grid',
        columns: 2,
        items: [],
      };
    case 'custom':
      return {
        type: 'custom',
        component: '',
        props: {},
      };
    default:
      return { type: 'form', submitFunction: primaryFunctionId || '', fields: defaultFields };
  }
}

export function getRolesForOrchestratorMode(mode: OrchestratorMode): OrchestratorRole[] {
  switch (mode) {
    case 'list':
      return ['list'];
    case 'form-detail':
      return ['query', 'detail', 'submit'];
    case 'split':
      return ['list', 'detail'];
    case 'dashboard':
      return ['list', 'detail', 'data'];
    default:
      return ['list', 'detail', 'submit', 'query', 'data'];
  }
}

function pickFunctionByRole(
  descriptors: FunctionDescriptor[],
  role: OrchestratorRole,
): FunctionDescriptor | null {
  if (!descriptors.length) return null;
  const roleKeywords: Record<string, string[]> = {
    list: ['list', 'search', 'page', 'inventory', 'mail'],
    detail: ['detail', 'profile', 'info', 'get'],
    submit: ['create', 'update', 'submit', 'save', 'ban', 'mute', 'send'],
    query: ['query', 'search', 'find', 'get'],
    data: ['stats', 'analytics', 'timeline', 'kanban', 'report', 'data'],
  };
  const kws = roleKeywords[role] || [];
  let best: FunctionDescriptor = descriptors[0];
  let bestScore = -1;
  descriptors.forEach((d) => {
    const text = `${d.id || ''} ${d.operation || ''} ${(d.tags || []).join(' ')} ${
      d.summary?.zh || ''
    } ${d.summary?.en || ''}`.toLowerCase();
    let score = 0;
    kws.forEach((kw) => {
      if (text.includes(kw)) score += 2;
    });
    if (score > bestScore) {
      best = d;
      bestScore = score;
    }
  });
  return best;
}

export function buildDefaultOrchestratorBindings(
  functionIds: string[],
  descriptors: FunctionDescriptor[],
): OrchestratorBindings {
  const descs = functionIds
    .map((id) => descriptors.find((d) => d.id === id))
    .filter(Boolean) as FunctionDescriptor[];
  const first = descs[0]?.id || functionIds[0] || '';
  const pick = (role: OrchestratorRole) => pickFunctionByRole(descs, role)?.id || first;
  return {
    list: pick('list'),
    detail: pick('detail'),
    submit: pick('submit'),
    query: pick('query'),
    data: pick('data'),
  };
}

export function buildOrchestrationLayout(
  mode: OrchestratorMode,
  functionIds: string[],
  descriptors: FunctionDescriptor[],
  manualBindings?: OrchestratorBindings | null,
): { layout: any; assignments: string[] } | null {
  if (!Array.isArray(functionIds) || functionIds.length === 0) return null;
  const descs = functionIds
    .map((id) => descriptors.find((d) => d.id === id))
    .filter(Boolean) as FunctionDescriptor[];
  if (descs.length === 0) return null;

  const defaults = buildDefaultOrchestratorBindings(functionIds, descriptors);
  const binding = manualBindings || defaults;
  const listFn = binding.list || defaults.list;
  const detailFn = binding.detail || defaults.detail;
  const submitFn = binding.submit || defaults.submit;
  const queryFn = binding.query || defaults.query;
  const dataFn = binding.data || defaults.data;

  const findDesc = (id: string) => descs.find((d) => d.id === id);
  const listDesc = findDesc(listFn);
  const detailDesc = findDesc(detailFn);
  const submitDesc = findDesc(submitFn);
  const queryDesc = findDesc(queryFn);

  const listColumns = listDesc ? schemaToColumns(listDesc) : [];
  const detailSections = detailDesc ? schemaToDetailSections(detailDesc) : [];
  const submitFields = submitDesc ? schemaToFields(submitDesc) : [];
  const queryFields = queryDesc ? schemaToFields(queryDesc).slice(0, 4) : [];

  const assignments = [
    `list -> ${listFn}`,
    `detail -> ${detailFn}`,
    `submit -> ${submitFn}`,
    `query -> ${queryFn}`,
    `data -> ${dataFn}`,
  ];

  if (mode === 'list') {
    return {
      layout: {
        type: 'list',
        listFunction: listFn,
        columns: listColumns.length > 0 ? listColumns : [{ key: 'id', title: 'ID' }],
      },
      assignments,
    };
  }
  if (mode === 'split') {
    return {
      layout: {
        type: 'split',
        direction: 'horizontal',
        panels: [
          {
            key: 'left',
            title: '列表',
            span: 12,
            component: {
              type: 'list',
              config: {
                listFunction: listFn,
                columns: listColumns.length > 0 ? listColumns : [{ key: 'id', title: 'ID' }],
              },
            },
          },
          {
            key: 'right',
            title: '详情',
            span: 12,
            component: {
              type: 'detail',
              config: {
                detailFunction: detailFn,
                sections:
                  detailSections.length > 0
                    ? detailSections
                    : [{ title: '基础信息', fields: [{ key: 'id', label: 'ID' }] }],
              },
            },
          },
        ],
      },
      assignments,
    };
  }
  if (mode === 'dashboard') {
    return {
      layout: {
        type: 'dashboard',
        stats: [
          { key: 'online', title: '在线人数', value: 0 },
          { key: 'dau', title: 'DAU', value: 0 },
        ],
        panels: [
          {
            key: 'list',
            title: '列表',
            span: 12,
            component: {
              type: 'list',
              config: {
                listFunction: listFn,
                columns: listColumns.length > 0 ? listColumns : [{ key: 'id', title: 'ID' }],
              },
            },
          },
          {
            key: 'detail',
            title: '详情',
            span: 12,
            component: {
              type: 'detail',
              config: {
                detailFunction: detailFn,
                sections:
                  detailSections.length > 0
                    ? detailSections
                    : [{ title: '基础信息', fields: [{ key: 'id', label: 'ID' }] }],
              },
            },
          },
        ],
      },
      assignments,
    };
  }

  return {
    layout: {
      type: 'form-detail',
      queryFunction: queryFn,
      queryFields:
        queryFields.length > 0 ? queryFields : [{ key: 'keyword', label: '关键字', type: 'input' }],
      detailSections:
        detailSections.length > 0
          ? detailSections
          : [{ title: '基础信息', fields: [{ key: 'id', label: 'ID' }] }],
      actions: submitFn
        ? [
            {
              key: 'submit',
              label: '提交',
              type: 'modal',
              function: submitFn,
              fields: submitFields.slice(0, 6),
            },
          ]
        : [],
    },
    assignments,
  };
}

export function mergeLayoutByMissing(current: any, planned: any): any {
  if (current === undefined || current === null || current === '') return planned;
  if (planned === undefined || planned === null) return current;

  if (Array.isArray(planned)) {
    if (!Array.isArray(current) || current.length === 0) return planned;
    return current;
  }

  if (typeof planned !== 'object') {
    return current;
  }

  const base = typeof current === 'object' && !Array.isArray(current) ? current : {};
  const result: Record<string, any> = { ...base };
  Object.keys(planned).forEach((key) => {
    result[key] = mergeLayoutByMissing(base[key], planned[key]);
  });
  return result;
}

export function buildLayoutDiffPreview(current: any, next: any): string[] {
  const entries: Array<{ line: string; priority: number }> = [];
  collectLayoutDiff(current, next, 'layout', entries, 0, 3);
  return entries
    .sort((a, b) => b.priority - a.priority)
    .map((x) => x.line)
    .slice(0, 16);
}

function collectLayoutDiff(
  current: any,
  next: any,
  path: string,
  entries: Array<{ line: string; priority: number }>,
  depth: number,
  maxDepth: number,
) {
  if (depth > maxDepth || entries.length >= 32) return;
  if (JSON.stringify(current) === JSON.stringify(next)) return;

  if (Array.isArray(next)) {
    const currentLen = Array.isArray(current) ? current.length : 0;
    const nextLen = next.length;
    if (currentLen !== nextLen) {
      pushDiffEntry(entries, `${path}: ${currentLen} -> ${nextLen}`, path);
    } else {
      pushDiffEntry(entries, `${path}: 内容已更新`, path);
    }
    return;
  }

  if (typeof next !== 'object' || next === null) {
    pushDiffEntry(
      entries,
      `${path}: ${formatDiffValue(current)} -> ${formatDiffValue(next)}`,
      path,
    );
    return;
  }

  const keys = new Set<string>([
    ...Object.keys(current && typeof current === 'object' ? current : {}),
    ...Object.keys(next),
  ]);
  keys.forEach((key) => {
    const c = current?.[key];
    const n = next?.[key];
    if (JSON.stringify(c) === JSON.stringify(n)) return;
    const nextPath = `${path}.${key}`;
    collectLayoutDiff(c, n, nextPath, entries, depth + 1, maxDepth);
  });
}

function pushDiffEntry(
  entries: Array<{ line: string; priority: number }>,
  line: string,
  path: string,
) {
  const p = getDiffPriority(path);
  const normalized = p >= 100 ? `关键变更: ${line}` : line;
  entries.push({ line: normalized, priority: p });
}

function getDiffPriority(path: string): number {
  const keyPath = path.toLowerCase();
  if (
    keyPath.includes('listfunction') ||
    keyPath.includes('queryfunction') ||
    keyPath.includes('submitfunction') ||
    keyPath.includes('detailfunction') ||
    keyPath.includes('datafunction')
  ) {
    return 120;
  }
  if (keyPath.endsWith('.type') || keyPath.includes('.type')) return 90;
  if (
    keyPath.includes('.columns') ||
    keyPath.includes('.fields') ||
    keyPath.includes('.sections')
  ) {
    return 70;
  }
  return 50;
}

function formatDiffValue(value: any): string {
  if (value === undefined || value === null || value === '') return '∅';
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (Array.isArray(value)) return `Array(${value.length})`;
  if (typeof value === 'object') return 'Object';
  return String(value);
}

export function buildOrchestrationRiskTips(
  applyMode: 'overwrite' | 'merge',
  currentLayout: any,
  nextLayout: any,
  diffLines: string[],
): string[] {
  if (!nextLayout) return [];
  const tips: string[] = [];
  const currentType = String(currentLayout?.type || '-');
  const nextType = String(nextLayout?.type || '-');

  if (applyMode === 'overwrite') {
    tips.push('覆盖模式会重置当前布局下的字段、列、分区与子组件配置。');
  } else {
    tips.push('补空模式不会覆盖已存在值，但会补齐缺失结构。');
  }

  if (currentType !== nextType) {
    tips.push(`布局类型将变化: ${currentType} -> ${nextType}。`);
  }

  const keyDiffCount = diffLines.filter((line) => line.startsWith('关键变更:')).length;
  if (keyDiffCount > 0) {
    tips.push(`检测到 ${keyDiffCount} 项关键函数绑定变更，请确认函数角色映射。`);
  }

  if (diffLines.length >= 12) {
    tips.push('本次变更项较多，建议先保存当前版本再应用。');
  }
  return tips.slice(0, 4);
}

export function assessOrchestrationRiskLevel(
  applyMode: 'overwrite' | 'merge',
  currentLayout: any,
  nextLayout: any,
  diffLines: string[],
): 'low' | 'medium' | 'high' {
  if (!nextLayout) return 'low';
  const currentType = String(currentLayout?.type || '-');
  const nextType = String(nextLayout?.type || '-');
  const keyDiffCount = diffLines.filter((line) => line.startsWith('关键变更:')).length;

  if (
    applyMode === 'overwrite' &&
    (currentType !== nextType || keyDiffCount >= 2 || diffLines.length >= 10)
  ) {
    return 'high';
  }
  if (
    applyMode === 'overwrite' ||
    currentType !== nextType ||
    keyDiffCount > 0 ||
    diffLines.length >= 6
  ) {
    return 'medium';
  }
  return 'low';
}
