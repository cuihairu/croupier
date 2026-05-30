import { request } from '@umijs/max';

export type TermItem = {
  id?: number;
  domain: 'entity' | 'operation';
  term_key: string;
  alias: string;
  display_zh?: string;
  display_en?: string;
  order?: number;
};

export async function listTerms(domain?: 'entity' | 'operation') {
  return request<{ items?: TermItem[] } | TermItem[]>('/api/v1/terms', {
    params: domain ? { domain } : {},
  });
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
