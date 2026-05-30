import type { FormilySchemaDoc, FormilySchemaVersion } from '@/components/formily/schema/types';
import { fetchFunctionUiSchema, saveFunctionUiSchema } from '@/services/api/functions';
import { convertLegacySchemaToFormily } from './convertLegacySchemaToFormily';

const VERSION: FormilySchemaVersion = 'formily:1';
const DRAFT_TTL_MS = 7 * 24 * 60 * 60 * 1000;

function draftKey(functionId: string) {
  return `function:ui-schema:draft:${functionId}`;
}

function nowISO() {
  return new Date().toISOString();
}

export type DraftStatus = 'active' | 'expired';

export type LocalDraftDoc = FormilySchemaDoc & {
  baseUpdatedAt?: string;
  status: 'draft';
  draftStatus?: DraftStatus;
};

export type UnifiedFormilySchemaState = {
  functionId: string;
  version: FormilySchemaVersion;
  schema: any;
  source: 'draft' | 'published' | 'empty';
  updatedAt?: string;
  publishedUpdatedAt?: string;
  draftUpdatedAt?: string;
  draftConflict: boolean;
  hasDraft: boolean;
};

export type FunctionUiSchemaDocument = {
  schema?: any;
  layout?: any;
  components?: any;
  custom?: boolean;
  hasDefault?: boolean;
  uiSource?: 'custom_metadata' | 'config_file_override' | 'openapi_x_ui' | 'none' | string;
  uiSourceDetail?: string;
  updated_at?: string;
};

function isExpiredISO(iso?: string): boolean {
  if (!iso) return false;
  const ts = Date.parse(iso);
  if (Number.isNaN(ts)) return false;
  return Date.now() - ts > DRAFT_TTL_MS;
}

function normalizeToFormily(schema: any): any {
  return convertLegacySchemaToFormily(schema);
}

export async function fetchFormilySchema(functionId: string): Promise<FormilySchemaDoc | null> {
  const res = await fetchFunctionUISchemaDocument(functionId);
  if (!res?.schema) return null;
  const normalized = normalizeToFormily(res.schema);
  return {
    functionId,
    version: VERSION,
    schema: normalized,
    updatedAt: res.updated_at || undefined,
    status: 'published',
  } as FormilySchemaDoc;
}

export async function fetchFunctionUISchemaDocument(
  functionId: string,
): Promise<FunctionUiSchemaDocument> {
  return fetchFunctionUiSchema(functionId);
}

export async function fetchUnifiedFormilySchemaState(
  functionId: string,
): Promise<UnifiedFormilySchemaState> {
  const published = await fetchFormilySchema(functionId);
  const draft = loadDraft(functionId, { evictExpired: true });
  const draftConflict = Boolean(
    draft &&
      published?.updatedAt &&
      draft.baseUpdatedAt &&
      published.updatedAt !== draft.baseUpdatedAt,
  );
  if (draft) {
    return {
      functionId,
      version: VERSION,
      schema: draft.schema,
      source: 'draft',
      updatedAt: draft.updatedAt,
      publishedUpdatedAt: published?.updatedAt,
      draftUpdatedAt: draft.updatedAt,
      draftConflict,
      hasDraft: true,
    };
  }
  if (published) {
    return {
      functionId,
      version: VERSION,
      schema: published.schema,
      source: 'published',
      updatedAt: published.updatedAt,
      publishedUpdatedAt: published.updatedAt,
      draftUpdatedAt: undefined,
      draftConflict: false,
      hasDraft: false,
    };
  }
  return {
    functionId,
    version: VERSION,
    schema: {},
    source: 'empty',
    updatedAt: undefined,
    publishedUpdatedAt: undefined,
    draftUpdatedAt: undefined,
    draftConflict: false,
    hasDraft: false,
  };
}

export async function saveFormilySchema(functionId: string, schema: any): Promise<void> {
  await saveFunctionUiSchema(functionId, { schema });
}

export function loadDraft(
  functionId: string,
  options?: { evictExpired?: boolean },
): LocalDraftDoc | null {
  if (typeof window === 'undefined') return null;
  const raw = localStorage.getItem(draftKey(functionId));
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw);
    const doc: LocalDraftDoc = {
      functionId,
      version: VERSION,
      schema: parsed?.schema || {},
      updatedAt: parsed?.updatedAt || undefined,
      baseUpdatedAt: parsed?.baseUpdatedAt || undefined,
      status: 'draft',
      draftStatus: isExpiredISO(parsed?.updatedAt) ? 'expired' : 'active',
    };
    if (doc.draftStatus === 'expired' && options?.evictExpired !== false) {
      clearDraft(functionId);
      return null;
    }
    return doc;
  } catch {
    return null;
  }
}

export function saveDraft(functionId: string, schema: any, options?: { baseUpdatedAt?: string }) {
  if (typeof window === 'undefined') return;
  const doc: LocalDraftDoc = {
    functionId,
    version: VERSION,
    schema: normalizeToFormily(schema),
    updatedAt: nowISO(),
    baseUpdatedAt: options?.baseUpdatedAt,
    status: 'draft',
    draftStatus: 'active',
  };
  localStorage.setItem(draftKey(functionId), JSON.stringify(doc));
}

export function hasDraftConflict(functionId: string, publishedUpdatedAt?: string): boolean {
  if (!publishedUpdatedAt) return false;
  const draft = loadDraft(functionId, { evictExpired: true });
  if (!draft?.baseUpdatedAt) return false;
  return draft.baseUpdatedAt !== publishedUpdatedAt;
}

export function clearDraft(functionId: string) {
  if (typeof window === 'undefined') return;
  localStorage.removeItem(draftKey(functionId));
}
