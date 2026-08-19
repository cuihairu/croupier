import { request } from '@umijs/max';
import type { JSONValue, LocalizedText } from '@/types/dashboard';
import { normalizeLocalizedText } from './functions-enhanced';

export type PendingFunctionRow = {
  functionId: string;
  displayName?: LocalizedText;
  summary?: LocalizedText;
  suggestedPermissions?: { verbs?: string[]; scopes?: string[] };
};

type RawPendingFunctionRow = {
  functionId?: string;
  displayName?: LocalizedText | string | Record<string, string>;
  summary?: LocalizedText | string | Record<string, string>;
  suggestedPermissions?: { verbs?: string[]; scopes?: string[] };
};

function normalizePendingFunctionRow(raw: RawPendingFunctionRow): PendingFunctionRow {
  return {
    functionId: raw.functionId || '',
    displayName: normalizeLocalizedText(raw.displayName),
    summary: normalizeLocalizedText(raw.summary),
    suggestedPermissions: raw.suggestedPermissions,
  };
}

export async function listPendingFunctions() {
  const res = await request<{ pending?: RawPendingFunctionRow[] }>('/api/v1/functions/pending', {
    method: 'GET',
  });
  return (res?.pending || []).map(normalizePendingFunctionRow);
}

export async function publishPendingFunction(functionId: string) {
  return request<void>(`/api/v1/functions/${encodeURIComponent(functionId)}/publish`, {
    method: 'POST',
  });
}

export async function getAdminFunctionPermissions(functionId: string) {
  const res = await request<{ permissions?: Record<string, JSONValue> }>(
    `/api/v1/functions/${encodeURIComponent(functionId)}/permissions`,
    { method: 'GET' },
  );
  return res?.permissions || {};
}

export async function setAdminFunctionPermissions(
  functionId: string,
  permissions: Record<string, JSONValue>,
) {
  return request<void>(`/api/v1/functions/${encodeURIComponent(functionId)}/permissions`, {
    method: 'PUT',
    data: permissions,
  });
}
