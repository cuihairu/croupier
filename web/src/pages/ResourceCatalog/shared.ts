import type {
  CapabilityKind,
  ResourceCatalogItem,
  ResourceSemanticConflicts,
  ResourceSemanticVersions,
  SemanticConflictInfo,
  SemanticSource,
  SemanticsInfo,
  TaskSemanticInfo,
  UpdateResourceSemanticsRequest,
} from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';
import type { AffectedPageInfo } from '@/types/dashboard';

/** 资源目录页共享常量与纯函数（状态/能力/来源映射、语义表单转换与提交载荷压缩）。 */

/** 标签 Map 的展示文案以 textId/textDefault 双字段承载，渲染处经 intl 解析（先例 Functions/History statusConfig） */
export type LabelText = { textId: string; textDefault: string };

/** 纯函数接收 intl 的最小结构（@umijs/max 未导出 IntlShape 类型，先例 ProposalInbox/shared.tsx） */
export type IntlFormatter = {
  formatMessage: (
    descriptor: { id: string; defaultMessage: string },
    values?: Record<string, string | number>,
  ) => string;
};

/** 解析标签 Map 条目为展示文案；未知键（entry 缺失）回退空串，由调用方自行兜底 raw 值 */
export const formatLabelText = (intl: IntlFormatter, entry?: LabelText): string =>
  entry ? intl.formatMessage({ id: entry.textId, defaultMessage: entry.textDefault }) : '';

export const statusColors: Record<ResourceCatalogItem['status'], string> = {
  identified: 'success',
  pending: 'warning',
  conflict: 'error',
  not_executable: 'default',
};

export const statusLabels: Record<ResourceCatalogItem['status'], LabelText> = {
  identified: { textId: 'pages.resourceCatalog.shared.status.identified', textDefault: '已识别' },
  pending: { textId: 'pages.resourceCatalog.shared.status.pending', textDefault: '待确认' },
  conflict: { textId: 'pages.resourceCatalog.shared.status.conflict', textDefault: '冲突' },
  not_executable: {
    textId: 'pages.resourceCatalog.shared.status.notExecutable',
    textDefault: '不可执行',
  },
};

export const capabilityLabels: Record<CapabilityKind, LabelText> = {
  collection_query: {
    textId: 'pages.resourceCatalog.shared.capability.collectionQuery',
    textDefault: '列表查询',
  },
  item_query: {
    textId: 'pages.resourceCatalog.shared.capability.itemQuery',
    textDefault: '详情查询',
  },
  create: { textId: 'pages.resourceCatalog.shared.capability.create', textDefault: '创建' },
  update: { textId: 'pages.resourceCatalog.shared.capability.update', textDefault: '更新' },
  delete: { textId: 'pages.resourceCatalog.shared.capability.delete', textDefault: '删除' },
  action: { textId: 'pages.resourceCatalog.shared.capability.action', textDefault: '动作' },
  task: { textId: 'pages.resourceCatalog.shared.capability.task', textDefault: '任务' },
  report: { textId: 'pages.resourceCatalog.shared.capability.report', textDefault: '报表' },
};

export const semanticSources: SemanticSource[] = [
  'platform_review',
  'sdk_explicit',
  'openapi_rest',
];

export const sourceLabels: Record<SemanticSource, LabelText> = {
  platform_review: {
    textId: 'pages.resourceCatalog.shared.source.platformReview',
    textDefault: '平台确认',
  },
  sdk_explicit: {
    textId: 'pages.resourceCatalog.shared.source.sdkExplicit',
    textDefault: 'SDK 显式',
  },
  openapi_rest: {
    textId: 'pages.resourceCatalog.shared.source.openapiRest',
    textDefault: 'OpenAPI REST',
  },
};

export const sourceColors: Record<SemanticSource, string> = {
  platform_review: 'green',
  sdk_explicit: 'blue',
  openapi_rest: 'gold',
};

export const riskColors: Record<string, string> = {
  danger: 'red',
  high: 'orange',
  warning: 'gold',
  safe: 'green',
};

export const affectedKindLabels: Record<'draft' | 'published' | 'proposal', LabelText> = {
  draft: { textId: 'pages.resourceCatalog.shared.affectedKind.draft', textDefault: '草稿' },
  published: {
    textId: 'pages.resourceCatalog.shared.affectedKind.published',
    textDefault: '已发布',
  },
  proposal: { textId: 'pages.resourceCatalog.shared.affectedKind.proposal', textDefault: '提案' },
};

export const affectedKindColors: Record<'draft' | 'published' | 'proposal', string> = {
  draft: 'blue',
  published: 'green',
  proposal: 'gold',
};

export const emptySemanticMeta: ResourceSemanticConflicts = {
  conflicts: [],
  provenance: [],
};

export const emptySemanticVersions: ResourceSemanticVersions = {
  items: [],
  total: 0,
};

export const displaySemanticValue = (value?: string): string => {
  if (!value) {
    return '-';
  }
  try {
    const parsed = JSON.parse(value);
    return typeof parsed === 'string' ? parsed : JSON.stringify(parsed);
  } catch {
    return value;
  }
};

export const pageTitleText = (page?: AffectedPageInfo): string =>
  localizedText(page?.title, 'zh-CN', '-') || page?.pageKey || '-';

export const bindingFreshnessSummary = (intl: IntlFormatter, page?: AffectedPageInfo): string => {
  if (!page?.bindingFreshness || page.bindingFreshness.length === 0) {
    return intl.formatMessage({
      id: 'pages.resourceCatalog.shared.freshnessNone',
      defaultMessage: '无',
    });
  }
  return page.bindingFreshness.map((item) => item.status).join(', ');
};

export const conflictSources = (conflict: SemanticConflictInfo): SemanticSource[] =>
  semanticSources.filter((source) => conflict.values[source] !== undefined);

export const semanticsToFormValues = (
  semantics?: SemanticsInfo,
): UpdateResourceSemanticsRequest => ({
  identityField: semantics?.identityField,
  identityFieldType: semantics?.identityFieldType || 'string',
  identityPath: semantics?.identityPath,
  collectionQueryId: semantics?.collectionQueryId,
  collectionPath: semantics?.collectionPath,
  pageFieldName: semantics?.pageFieldName,
  pageSizeFieldName: semantics?.pageSizeFieldName,
  itemsFieldName: semantics?.itemsFieldName,
  totalFieldName: semantics?.totalFieldName,
  itemQueryId: semantics?.itemQueryId,
  itemPath: semantics?.itemPath,
  createId: semantics?.createId,
  updateId: semantics?.updateId,
  deleteId: semantics?.deleteId,
  actions: semantics?.actions || [],
  tasks: semantics?.tasks || [],
  reports: semantics?.reports || [],
});

const compactTaskSemantic = (task: TaskSemanticInfo): TaskSemanticInfo | undefined => {
  const startFunctionId = task.start?.functionId?.trim();
  const statusFunctionId = task.status?.function?.functionId?.trim();
  if (!startFunctionId || !statusFunctionId) {
    return undefined;
  }
  const next: TaskSemanticInfo = {
    start: { functionId: startFunctionId },
    taskId: {
      resultPath: task.taskId?.resultPath?.trim() || '',
      valueType: task.taskId?.valueType || 'string',
    },
    status: {
      function: { functionId: statusFunctionId },
      taskIdInput: task.status?.taskIdInput?.trim() || '',
      statePath: task.status?.statePath?.trim() || '',
    },
  };
  if (task.events?.function?.functionId) {
    next.events = {
      function: { functionId: task.events.function.functionId.trim() },
      taskIdInput: task.events.taskIdInput?.trim() || '',
      eventsPath: task.events.eventsPath?.trim() || '',
    };
  }
  if (task.result?.function?.functionId) {
    next.result = {
      function: { functionId: task.result.function.functionId.trim() },
      taskIdInput: task.result.taskIdInput?.trim() || '',
      resultPath: task.result.resultPath?.trim() || '',
    };
  }
  if (task.cancel?.function?.functionId) {
    next.cancel = {
      function: { functionId: task.cancel.function.functionId.trim() },
      taskIdInput: task.cancel.taskIdInput?.trim() || '',
    };
  }
  return next;
};

export const compactSemanticsPayload = (
  values: UpdateResourceSemanticsRequest,
): UpdateResourceSemanticsRequest => {
  const payload: UpdateResourceSemanticsRequest = {};

  const assignString = (key: keyof UpdateResourceSemanticsRequest, value?: string) => {
    const trimmed = value?.trim();
    if (trimmed) {
      Object.assign(payload, { [key]: trimmed });
    }
  };
  const assignNumber = (key: keyof UpdateResourceSemanticsRequest, value?: number) => {
    if (typeof value === 'number' && value > 0) {
      Object.assign(payload, { [key]: value });
    }
  };

  assignString('identityField', values.identityField);
  assignString('identityFieldType', values.identityFieldType);
  assignString('identityPath', values.identityPath);
  assignNumber('collectionQueryId', values.collectionQueryId);
  assignString('collectionPath', values.collectionPath);
  assignString('pageFieldName', values.pageFieldName);
  assignString('pageSizeFieldName', values.pageSizeFieldName);
  assignString('itemsFieldName', values.itemsFieldName);
  assignString('totalFieldName', values.totalFieldName);
  assignNumber('itemQueryId', values.itemQueryId);
  assignString('itemPath', values.itemPath);
  assignNumber('createId', values.createId);
  assignNumber('updateId', values.updateId);
  assignNumber('deleteId', values.deleteId);
  if (values.actions) {
    const actions = values.actions
      .map((action) => {
        const functionId = action.functionId?.trim();
        if (!functionId || !action.subject) {
          return undefined;
        }
        return {
          functionId,
          subject: action.subject,
          identityInput: action.identityInput?.trim(),
        };
      })
      .filter((action): action is NonNullable<typeof action> => Boolean(action));
    Object.assign(payload, { actions });
  }
  if (values.tasks) {
    const tasks = values.tasks
      .map(compactTaskSemantic)
      .filter((task): task is TaskSemanticInfo => Boolean(task));
    Object.assign(payload, { tasks });
  }
  if (values.reports) {
    const reports = values.reports
      .map((report) => ({
        query: { functionId: report.query?.functionId?.trim() || '' },
        datasetPath: report.datasetPath?.trim() || '',
        dimensions: (report.dimensions || [])
          .map((item) => item?.trim())
          .filter((item): item is string => Boolean(item)),
        metrics: (report.metrics || [])
          .map((item) => item?.trim())
          .filter((item): item is string => Boolean(item)),
      }))
      .filter((report) => report.query.functionId);
    Object.assign(payload, { reports });
  }
  assignString('changeReason', values.changeReason);

  return payload;
};
