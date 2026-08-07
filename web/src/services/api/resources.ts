import { request } from '@umijs/max';
import type { OperationSpec, ResourceSpec } from '@/types/dashboard';

const BASE = '/api/v1/resources';

type ResourceListResponse = {
  items?: ResourceSpec[];
};

type ResourceDetailResponse = {
  resource?: ResourceSpec;
};

type ResourceOperationsResponse = {
  items?: OperationSpec[];
};

export async function listResources(params?: {
  category?: string;
  q?: string;
}): Promise<ResourceSpec[]> {
  const response = await request<ResourceListResponse>(BASE, {
    method: 'GET',
    params,
  });
  return Array.isArray(response?.items) ? response.items : [];
}

export async function getResource(resourceKey: string): Promise<ResourceSpec> {
  const response = await request<ResourceDetailResponse>(
    `${BASE}/${encodeURIComponent(resourceKey)}`,
    {
      method: 'GET',
    },
  );
  if (!response?.resource) {
    throw new Error(`resource not found: ${resourceKey}`);
  }
  return response.resource;
}

export async function listResourceOperations(resourceKey: string): Promise<OperationSpec[]> {
  const response = await request<ResourceOperationsResponse>(
    `${BASE}/${encodeURIComponent(resourceKey)}/operations`,
    { method: 'GET' },
  );
  return Array.isArray(response?.items) ? response.items : [];
}
