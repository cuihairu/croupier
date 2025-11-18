import { request } from '@umijs/max';

export async function uploadAsset(file: File) {
  const form = new FormData();
  form.append('file', file);
  return request<{ Key: string; URL: string }>('/api/upload', {
    method: 'POST',
    data: form,
  });
}

