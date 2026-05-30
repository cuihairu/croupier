import { request } from '@umijs/max';

type LocalizedText = {
  zh?: string;
  en?: string;
};

export type PendingFunctionRow = {
  functionId: string;
  displayName?: LocalizedText;
  summary?: LocalizedText;
  suggestedPermissions?: { verbs?: string[]; scopes?: string[] };
};

type RawPendingFunctionRow = {
  function_id?: string;
  functionId?: string;
  display_name?: LocalizedText;
  displayName?: LocalizedText;
  summary?: LocalizedText;
  suggested_permissions?: { verbs?: string[]; scopes?: string[] };
  suggestedPermissions?: { verbs?: string[]; scopes?: string[] };
};

function normalizePendingFunctionRow(raw: RawPendingFunctionRow): PendingFunctionRow {
  return {
    functionId: raw.function_id || raw.functionId || '',
    displayName: raw.display_name || raw.displayName,
    summary: raw.summary,
    suggestedPermissions: raw.suggested_permissions || raw.suggestedPermissions,
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
  const res = await request<{ permissions?: any }>(
    `/api/v1/functions/${encodeURIComponent(functionId)}/permissions`,
    { method: 'GET' },
  );
  return res?.permissions || {};
}

export async function setAdminFunctionPermissions(functionId: string, permissions: any) {
  return request<void>(`/api/v1/functions/${encodeURIComponent(functionId)}/permissions`, {
    method: 'PUT',
    data: permissions,
  });
}
