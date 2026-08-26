import { request } from '@umijs/max';
import type { LocalizedText } from '@/types/dashboard';
import { normalizeLocalizedText } from '@/services/api/functions-enhanced';

export type TermItem = {
  id?: number;
  domain: 'resource' | 'operation';
  termKey: string;
  alias: string;
  /** 本地化显示文本，key 为 BCP47 locale（"zh-CN"/"en-US"） */
  display?: LocalizedText;
  order?: number;
};

type RawTermItem = Omit<TermItem, 'display'> & {
  display?: LocalizedText | Record<string, string> | string;
};

function normalizeTerm(raw: RawTermItem): TermItem {
  return { ...raw, display: normalizeLocalizedText(raw.display) };
}

export async function listTerms(domain?: TermItem['domain']) {
  const res = await request<{ items?: RawTermItem[] } | RawTermItem[]>('/api/v1/terms', {
    params: domain ? { domain } : {},
  });
  const items = Array.isArray(res) ? res : res?.items || [];
  return items.map(normalizeTerm);
}

export async function upsertTerm(payload: TermItem) {
  return request<{ ok: boolean }>('/api/v1/terms', {
    method: 'PUT',
    data: payload,
  });
}

export async function deleteTerm(domain: string, alias: string) {
  return request<{ ok: boolean }>('/api/v1/terms', {
    method: 'DELETE',
    params: { domain, alias },
  });
}
