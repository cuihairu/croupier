import { request } from '@umijs/max';

type StorageObjectsPayload = {
  objects?: Array<{
    key?: string;
    size?: number;
    last_modified?: string;
  }>;
  prefixes?: string[];
  is_truncated?: boolean;
  next_marker?: string;
};

type LegacyEnvelope<T> = {
  code?: number;
  message?: string;
  data?: T;
};

type UploadAssetOptions = {
  path?: string;
};

function getFileExtension(name: string): string {
  const trimmed = (name || '').trim();
  const index = trimmed.lastIndexOf('.');
  if (index <= 0 || index === trimmed.length - 1) {
    return '';
  }
  return trimmed.slice(index).toLowerCase();
}

export function buildAvatarObjectKey(file: File): string {
  const extension = getFileExtension(file.name) || '.bin';
  const randomPart =
    typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID()
      : `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
  return `avatars/${Date.now()}-${randomPart}${extension}`;
}

function uploadObjectMultipart(file: File, path?: string): Promise<{ path?: string }> {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    form.append('file', file);
    if (path) {
      form.append('path', path);
    }

    const xhr = new XMLHttpRequest();
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';

    xhr.open('POST', '/api/v1/storage/objects');
    if (token) {
      xhr.setRequestHeader('Authorization', `Bearer ${token}`);
    }

    xhr.onload = () => {
      let payload: any = null;
      try {
        payload = xhr.responseText ? JSON.parse(xhr.responseText) : null;
      } catch (error) {
        reject(new Error('上传响应解析失败'));
        return;
      }

      if (xhr.status >= 200 && xhr.status < 300) {
        const data =
          payload && typeof payload === 'object' && 'data' in payload && payload.data
            ? payload.data
            : payload;
        resolve(data || {});
        return;
      }

      reject(new Error(payload?.message || '上传失败'));
    };

    xhr.onerror = () => reject(new Error('上传失败'));
    xhr.send(form);
  });
}

export async function uploadAsset(file: File, options?: UploadAssetOptions) {
  const requestedPath = options?.path?.trim() || file.name;
  const uploadPayload = await uploadObjectMultipart(file, requestedPath);
  const key = uploadPayload?.path || requestedPath;
  const signed = await getSignedUrl(key);
  return { Key: key, URL: signed.url };
}

export async function listObjects(params: {
  prefix?: string;
  marker?: string;
  limit?: number;
  delimiter?: string;
}) {
  const resp = await request<StorageObjectsPayload | LegacyEnvelope<StorageObjectsPayload>>(
    '/api/v1/storage/objects',
    { params },
  );
  const payload =
    resp && typeof resp === 'object' && 'data' in resp && resp.data ? resp.data : resp;
  return {
    objects: Array.isArray(payload?.objects) ? payload.objects : [],
    prefixes: Array.isArray(payload?.prefixes) ? payload.prefixes : [],
    is_truncated: Boolean(payload?.is_truncated),
    next_marker: payload?.next_marker || '',
  };
}

export async function uploadObject(file: File) {
  return uploadObjectMultipart(file, file.name);
}

export async function deleteObject(key: string) {
  return request('/api/v1/storage/objects', {
    method: 'DELETE',
    params: { path: key },
  });
}

export async function batchDeleteObjects(keys: string[]) {
  return request('/api/v1/storage/objects/batch-delete', {
    method: 'POST',
    data: { paths: keys },
  });
}

export async function createDirectory(prefix: string) {
  return request('/api/v1/storage/directories', {
    method: 'POST',
    data: { prefix },
  });
}

export async function getSignedUrl(path: string) {
  const resp = await request<{ url?: string } | LegacyEnvelope<{ url?: string }>>(
    '/api/v1/storage/signed-url',
    {
      params: { path },
    },
  );
  const payload =
    resp && typeof resp === 'object' && 'data' in resp && resp.data ? resp.data : resp;
  return { url: payload?.url || '' };
}
