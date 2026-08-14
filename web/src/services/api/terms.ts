import { request } from '@umijs/max';

export type TermItem = {
  id?: number;
  domain: 'resource' | 'operation';
  termKey: string;
  alias: string;
  displayZh?: string;
  displayEn?: string;
  order?: number;
};

export async function listTerms(domain?: TermItem['domain']) {
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
