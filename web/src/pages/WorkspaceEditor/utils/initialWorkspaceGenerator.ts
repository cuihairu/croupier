import type { FunctionDescriptor } from '@/services/api/functions';
import type { WorkspaceConfig, TabConfig, FieldConfig, ColumnConfig } from '@/types/workspace';

type ActionKind =
  | 'list'
  | 'query'
  | 'detail'
  | 'create'
  | 'update'
  | 'delete'
  | 'form-action'
  | 'secondary-action'
  | 'custom';
type ObjectMatchConfidence = 'entity' | 'prefix' | 'none';

type FunctionBuckets = {
  list?: FunctionDescriptor;
  query?: FunctionDescriptor;
  detail?: FunctionDescriptor;
  create?: FunctionDescriptor;
  update?: FunctionDescriptor;
  delete?: FunctionDescriptor;
  formAction?: FunctionDescriptor;
  customs: FunctionDescriptor[];
};

export type InitialWorkspaceSuggestion = {
  functionId: string;
  attachTo?: string;
  reason: 'dangerous-action' | 'secondary-action';
};

export type InitialWorkspaceGeneratorResult = {
  config: WorkspaceConfig;
  matchedFunctions: FunctionDescriptor[];
  suggestions: InitialWorkspaceSuggestion[];
  confidence: ObjectMatchConfidence[];
};

function normalizeText(value?: string | null) {
  return String(value || '').trim().toLowerCase();
}

function safeParseSchema(raw: any): any | null {
  if (!raw) return null;
  if (typeof raw === 'object') return raw;
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw);
    } catch {
      return null;
    }
  }
  return null;
}

function matchObjectKey(
  descriptor: FunctionDescriptor,
  objectKey: string,
): ObjectMatchConfidence {
  const normalizedObjectKey = normalizeText(objectKey);
  const entity = normalizeText(descriptor.entity);
  const prefix = normalizeText(descriptor.id).split('.')[0] || '';

  if (entity && entity === normalizedObjectKey) return 'entity';
  if (!entity && prefix === normalizedObjectKey) return 'prefix';
  return 'none';
}

function belongsToObject(descriptor: FunctionDescriptor, objectKey: string) {
  return matchObjectKey(descriptor, objectKey) !== 'none';
}

function detectActionKind(descriptor: FunctionDescriptor): ActionKind {
  const op = normalizeText(descriptor.operation);
  const id = normalizeText(descriptor.id);

  if (['list'].includes(op)) return 'list';
  if (['query', 'search'].includes(op)) return 'query';
  if (['get', 'detail', 'read'].includes(op)) return 'detail';
  if (['create', 'add'].includes(op)) return 'create';
  if (['update', 'edit'].includes(op)) return 'update';
  if (['delete', 'remove'].includes(op)) return 'delete';

  if (id.endsWith('.list')) return 'list';
  if (id.endsWith('.query') || id.endsWith('.search')) return 'query';
  if (id.endsWith('.get') || id.endsWith('.detail') || id.endsWith('.read')) return 'detail';
  if (id.endsWith('.create') || id.endsWith('.add')) return 'create';
  if (id.endsWith('.update') || id.endsWith('.edit')) return 'update';
  if (id.endsWith('.delete') || id.endsWith('.remove') || id.endsWith('.reset')) return 'delete';

  if (id.endsWith('.send') || id.endsWith('.grant') || id.endsWith('.upsert')) {
    return 'form-action';
  }

  if (id.endsWith('.consume') || id.endsWith('.claim')) {
    return 'secondary-action';
  }

  return 'custom';
}

function scoreDescriptor(d: FunctionDescriptor) {
  let score = 0;
  if (d.operation) score += 30;
  if (d.displayName?.zh || d.displayName?.en) score += 20;
  if (d.summary?.zh || d.summary?.en) score += 10;
  if (d.entity) score += 20;
  return score;
}

function pickBetter(
  current: FunctionDescriptor | undefined,
  candidate: FunctionDescriptor,
): FunctionDescriptor {
  if (!current) return candidate;
  const a = scoreDescriptor(current);
  const b = scoreDescriptor(candidate);
  if (b > a) return candidate;
  if (b < a) return current;
  return candidate.id.localeCompare(current.id) < 0 ? candidate : current;
}

function bucketFunctions(
  objectKey: string,
  allDescriptors: FunctionDescriptor[],
): {
  buckets: FunctionBuckets;
  matched: FunctionDescriptor[];
  confidence: ObjectMatchConfidence[];
} {
  const matched = allDescriptors.filter((d) => belongsToObject(d, objectKey));
  const confidence = matched.map((d) => matchObjectKey(d, objectKey));

  const buckets: FunctionBuckets = { customs: [] };

  for (const d of matched) {
    const kind = detectActionKind(d);
    switch (kind) {
      case 'list':
        buckets.list = pickBetter(buckets.list, d);
        break;
      case 'query':
        buckets.query = pickBetter(buckets.query, d);
        break;
      case 'detail':
        buckets.detail = pickBetter(buckets.detail, d);
        break;
      case 'create':
        buckets.create = pickBetter(buckets.create, d);
        break;
      case 'update':
        buckets.update = pickBetter(buckets.update, d);
        break;
      case 'delete':
        buckets.delete = pickBetter(buckets.delete, d);
        break;
      case 'form-action':
        buckets.formAction = pickBetter(buckets.formAction, d);
        break;
      case 'secondary-action':
        buckets.customs.push(d);
        break;
      default:
        buckets.customs.push(d);
        break;
    }
  }

  return { buckets, matched, confidence };
}

function resolveObjectLabel(objectKey: string, descriptors: FunctionDescriptor[]): string {
  for (const d of descriptors) {
    const entity = normalizeText(d.entity);
    if (entity !== normalizeText(objectKey)) continue;

    const label =
      d.entityDisplay?.zh ||
      d.entityDisplay?.en ||
      d.displayName?.zh ||
      d.displayName?.en;

    if (label) return String(label).trim();
  }
  return objectKey;
}

function inferFormFields(descriptor: FunctionDescriptor): FieldConfig[] {
  const schema = safeParseSchema(descriptor.inputSchema) || safeParseSchema(descriptor.params);
  const props = schema?.properties || {};
  const required = Array.isArray(schema?.required) ? schema.required : [];

  return Object.entries(props)
    .slice(0, 12)
    .map(([key, value]: [string, any]) => ({
      key,
      label: value?.title || value?.description || key,
      type: inferFieldType(key, value),
      required: required.includes(key),
      placeholder: `请输入${value?.title || key}`,
      options: value?.enum ? value.enum.map((v: any) => ({ label: String(v), value: v })) : undefined,
    })) as FieldConfig[];
}

function inferFieldType(key: string, prop: any): FieldConfig['type'] {
  const type = prop?.type;
  const lk = key.toLowerCase();
  if (type === 'integer' || type === 'number') return 'number';
  if (type === 'boolean') return 'switch';
  if (prop?.enum) return 'select';
  if (lk.includes('date') || lk.includes('_at') || lk.endsWith('at')) return 'datetime';
  if (lk.includes('desc') || lk.includes('remark') || lk.includes('note')) return 'textarea';
  return 'input';
}

function inferRender(key: string, prop: any): ColumnConfig['render'] {
  const format = prop?.format;
  const lk = key.toLowerCase();
  if (format === 'date-time' || lk.endsWith('_at') || lk.endsWith('at') || lk.includes('time')) return 'datetime';
  if (format === 'date') return 'date';
  if (lk.includes('status') || lk.includes('state')) return 'status';
  if (lk.includes('amount') || lk.includes('price') || lk.includes('gold') || lk.includes('money')) return 'money';
  if (lk.includes('url') || lk.includes('link') || lk.includes('href')) return 'link';
  if (lk.includes('avatar') || lk.includes('image') || lk.includes('icon')) return 'image';
  if (lk.includes('tag') || lk.includes('label')) return 'tag';
  return 'text';
}

function inferListColumns(descriptor: FunctionDescriptor): ColumnConfig[] {
  const schema = safeParseSchema(descriptor.outputSchema) || safeParseSchema(descriptor.outputs);
  const itemProps =
    schema?.properties?.items?.items?.properties ||
    schema?.properties?.data?.items?.properties ||
    schema?.properties?.list?.items?.properties ||
    null;

  if (!itemProps) return [];

  return Object.entries(itemProps)
    .slice(0, 8)
    .map(([key, value]: [string, any]) => ({
      key,
      title: value?.title || value?.description || key,
      width: key === 'id' || key.endsWith('_id') ? 120 : undefined,
      render: inferRender(key, value),
      fixed: key === 'id' ? ('left' as const) : undefined,
      sortable: value?.type === 'integer' || value?.type === 'number',
    })) as ColumnConfig[];
}

function inferDetailSections(descriptor: FunctionDescriptor) {
  const schema = safeParseSchema(descriptor.outputSchema) || safeParseSchema(descriptor.outputs);
  const props = schema?.properties || {};
  const fields = Object.entries(props)
    .slice(0, 12)
    .map(([key, value]: [string, any]) => ({
      key,
      label: value?.title || value?.description || key,
      render: inferRender(key, value),
    }));

  return [{ title: '详情信息', fields, column: 2 }];
}

function buildListTab(objectKey: string, label: string, fn: FunctionDescriptor): TabConfig {
  return {
    key: `${objectKey}_list`,
    title: `${label}列表`,
    defaultActive: true,
    functions: [fn.id],
    layout: {
      type: 'list',
      listFunction: fn.id,
      columns: inferListColumns(fn),
      pagination: true,
    },
  } as TabConfig;
}

function buildFormDetailTab(
  objectKey: string,
  label: string,
  queryFn: FunctionDescriptor,
  detailFn: FunctionDescriptor,
): TabConfig {
  return {
    key: `${objectKey}_query_detail`,
    title: `${label}查询详情`,
    defaultActive: true,
    functions: [queryFn.id, detailFn.id],
    layout: {
      type: 'form-detail',
      queryFunction: queryFn.id,
      queryFields: inferFormFields(queryFn).slice(0, 6),
      detailSections: inferDetailSections(detailFn),
      actions: [],
    },
  } as TabConfig;
}

function buildDetailTab(objectKey: string, label: string, fn: FunctionDescriptor): TabConfig {
  return {
    key: `${objectKey}_detail`,
    title: `${label}详情`,
    functions: [fn.id],
    layout: {
      type: 'detail',
      detailFunction: fn.id,
      sections: inferDetailSections(fn),
      actions: [],
    },
  } as TabConfig;
}

function buildFormTab(
  objectKey: string,
  label: string,
  primary: FunctionDescriptor,
  secondary?: FunctionDescriptor,
): TabConfig {
  const functions = secondary ? [primary.id, secondary.id] : [primary.id];
  return {
    key: `${objectKey}_form`,
    title: secondary ? `${label}新建/编辑` : `${label}编辑`,
    functions,
    layout: {
      type: 'form',
      submitFunction: primary.id,
      fields: inferFormFields(primary),
      submitText: detectActionKind(primary) === 'create' ? '创建' : '保存',
      showReset: true,
      secondarySubmitFunction: secondary?.id,
    },
  } as TabConfig;
}

function resolveActionLabel(fn: FunctionDescriptor): string {
  const id = normalizeText(fn.id);
  if (id.endsWith('.send')) return '发送';
  if (id.endsWith('.grant')) return '发放';
  if (id.endsWith('.upsert')) return '更新';
  if (id.endsWith('.create')) return '新建';
  if (id.endsWith('.update')) return '编辑';
  return '操作';
}

function resolveSubmitText(fn: FunctionDescriptor): string {
  const id = normalizeText(fn.id);
  if (id.endsWith('.send')) return '发送';
  if (id.endsWith('.grant')) return '发放';
  if (id.endsWith('.upsert')) return '更新';
  if (id.endsWith('.create')) return '创建';
  return '保存';
}

function buildFormActionTab(objectKey: string, label: string, fn: FunctionDescriptor): TabConfig {
  return {
    key: `${objectKey}_action`,
    title: `${label}${resolveActionLabel(fn)}`,
    functions: [fn.id],
    layout: {
      type: 'form',
      submitFunction: fn.id,
      fields: inferFormFields(fn),
      submitText: resolveSubmitText(fn),
      showReset: true,
    },
  } as TabConfig;
}

function buildFallbackCustomTab(objectKey: string, label: string, fn: FunctionDescriptor): TabConfig {
  return {
    key: `${objectKey}_action`,
    title: `${label}操作`,
    defaultActive: true,
    functions: [fn.id],
    layout: {
      type: 'form',
      submitFunction: fn.id,
      fields: inferFormFields(fn),
      submitText: '提交',
      showReset: true,
    },
  } as TabConfig;
}

function buildSecondarySuggestions(objectKey: string, buckets: FunctionBuckets): InitialWorkspaceSuggestion[] {
  const actions: InitialWorkspaceSuggestion[] = [];

  if (buckets.delete) {
    actions.push({
      functionId: buckets.delete.id,
      attachTo: `${objectKey}_detail`,
      reason: 'dangerous-action',
    });
  }

  for (const fn of buckets.customs) {
    const id = normalizeText(fn.id);
    if (id.endsWith('.consume') || id.endsWith('.claim')) {
      actions.push({
        functionId: fn.id,
        attachTo: `${objectKey}_detail`,
        reason: 'secondary-action',
      });
    }
  }

  return actions;
}

export function generateInitialWorkspaceConfig(
  objectKey: string,
  allDescriptors: FunctionDescriptor[],
): InitialWorkspaceGeneratorResult {
  const { buckets, matched, confidence } = bucketFunctions(objectKey, allDescriptors);
  const label = resolveObjectLabel(objectKey, matched);
  const tabs: TabConfig[] = [];

  if (buckets.list) {
    tabs.push(buildListTab(objectKey, label, buckets.list));
  } else if (buckets.query && buckets.detail) {
    tabs.push(buildFormDetailTab(objectKey, label, buckets.query, buckets.detail));
  }

  if (buckets.detail && !buckets.query) {
    tabs.push(buildDetailTab(objectKey, label, buckets.detail));
  }

  if (buckets.create || buckets.update) {
    const primary = buckets.create || buckets.update!;
    const secondary =
      buckets.create && buckets.update
        ? primary.id === buckets.create.id
          ? buckets.update
          : buckets.create
        : undefined;
    tabs.push(buildFormTab(objectKey, label, primary, secondary));
  } else if (buckets.formAction) {
    tabs.push(buildFormActionTab(objectKey, label, buckets.formAction));
  }

  if (tabs.length === 0 && buckets.customs.length > 0) {
    tabs.push(buildFallbackCustomTab(objectKey, label, buckets.customs[0]));
  }

  if (tabs.length > 0) {
    tabs[0].defaultActive = true;
  }

  const suggestions = buildSecondarySuggestions(objectKey, buckets);

  return {
    config: {
      objectKey,
      title: label,
      layout: {
        type: 'tabs',
        tabs,
      },
      menuOrder: 0,
      status: 'draft',
      meta: {
        generatedBy: 'auto-skeleton-v1',
        generatedAt: new Date().toISOString(),
        generatedFromFunctions: matched.map((d) => d.id),
        matchConfidence: confidence,
        suggestions,
      },
    } as WorkspaceConfig,
    matchedFunctions: matched,
    suggestions,
    confidence,
  };
}
