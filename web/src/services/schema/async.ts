import { request } from '@umijs/max';

export async function fetchOptions(url: string, params?: Record<string, any>) {
  const res = await request(url, { params });
  if (Array.isArray(res)) return res;
  if (res?.options) return res.options;
  return res;
}
