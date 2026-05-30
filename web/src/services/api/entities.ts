import { request } from '@umijs/max';

// Source: croupier/internal/api/entity/dto.go EntitiesListRequest and existing scoped dashboard calls.
export type EntityScopeParams = {
  gameId?: string;
  env?: string;
  page?: number;
  pageSize?: number;
  type?: string;
};

// Source: croupier/internal/api/entity/dto.go EntityItem / EntityDetailResponse / EntityCreateResponse / EntityUpdateResponse.
export type EntityRecord = {
  id: string;
  type: string;
  data: unknown;
  providerId?: string;
  status: number;
  createdAt: string;
  updatedAt: string;
};

// Source: croupier/internal/api/entity/dto.go EntitiesListResponse.
export type EntityRecordsListResponse = {
  items: EntityRecord[];
  total: number;
  page: number;
  size: number;
};

// Source: croupier/internal/api/entity/dto.go EntityCreateRequest / EntityUpdateRequest / EntityValidateRequest.
export type EntityMutationRequest = {
  type: string;
  data: unknown;
};

// Source: croupier/internal/api/entity/dto.go EntityValidateResponse.
export type EntityValidationResult = {
  valid: boolean;
  errors?: string[];
  warnings?: string[];
};

// Source: croupier/internal/api/entity/dto.go EntityPreviewResponse.
export type EntityPreviewResult = {
  data: unknown;
  previewHtml?: string;
  previewData?: unknown;
};

// Source: croupier/internal/api/openapi/dto.go EntityFunctionsResponse.Item.
export type EntityFunction = {
  id: string;
  operation?: 'create' | 'read' | 'update' | 'delete' | 'custom' | string;
  name?: string;
  summary?: string;
};

// Compatibility projection for current dashboard entity-definition pages.
// This is not the backend canonical DTO. New code should prefer EntityRecord.
export type EntityDefinition = {
  id: string;
  type?: string;
  name?: string;
  description?: string;
  schema?: any;
  uiSchema?: any;
  operations?: string[];
  data?: unknown;
  providerId?: string;
  status?: number;
  createdAt?: string;
  updatedAt?: string;
};

type RawEntityRecord = {
  id?: string | number;
  type?: string;
  data?: unknown;
  providerId?: string;
  provider_id?: string;
  status?: number;
  createdAt?: string;
  created_at?: string;
  updatedAt?: string;
  updated_at?: string;
};

type RawEntityRecordsListResponse = {
  items?: RawEntityRecord[];
  total?: number;
  page?: number;
  size?: number;
};

type RawEntityPreviewResponse = {
  data?: unknown;
};

type RawEntityFunctionsResponse = {
  items?: EntityFunction[];
};

function normalizeEntityRecord(raw: RawEntityRecord): EntityRecord {
  return {
    id: String(raw.id ?? ''),
    type: raw.type ?? '',
    data: raw.data,
    providerId: raw.providerId ?? raw.provider_id,
    status: raw.status ?? 0,
    createdAt: raw.createdAt ?? raw.created_at ?? '',
    updatedAt: raw.updatedAt ?? raw.updated_at ?? '',
  };
}

function normalizeEntityRecordsListResponse(
  raw?: RawEntityRecordsListResponse,
): EntityRecordsListResponse {
  return {
    items: (raw?.items || []).map(normalizeEntityRecord),
    total: raw?.total || 0,
    page: raw?.page || 1,
    size: raw?.size || 20,
  };
}

function normalizeEntityPreviewResult(raw?: RawEntityPreviewResponse): EntityPreviewResult {
  return {
    data: raw?.data,
    // Compatibility fallback for legacy preview modal that expects HTML.
    previewHtml:
      raw?.data === undefined
        ? undefined
        : `<pre>${escapeHtml(JSON.stringify(raw.data, null, 2))}</pre>`,
    previewData: raw?.data,
  };
}

function projectEntityDefinition(record: EntityRecord): EntityDefinition {
  const payload = isRecord(record.data) ? record.data : {};
  const schema = isRecord(payload.schema) ? payload.schema : payload.schema;
  const uiSchema = isRecord(payload.uiSchema) ? payload.uiSchema : payload.uiSchema;
  const operations = Array.isArray(payload.operations)
    ? payload.operations.map((item) => String(item))
    : undefined;
  return {
    id: record.id,
    type: record.type,
    name: asString(payload.name) || record.type || record.id,
    description: asString(payload.description),
    schema,
    uiSchema,
    operations,
    data: record.data,
    providerId: record.providerId,
    status: record.status,
    createdAt: record.createdAt,
    updatedAt: record.updatedAt,
  };
}

function buildEntityMutationRequest(entity: Partial<EntityDefinition>): EntityMutationRequest {
  const type = `${entity.type || entity.name || entity.id || ''}`.trim();
  return {
    type,
    data: {
      name: entity.name,
      description: entity.description,
      schema: entity.schema,
      uiSchema: entity.uiSchema,
      operations: entity.operations || [],
    },
  };
}

function buildEntityScopeParams(params?: EntityScopeParams) {
  return {
    game_id: params?.gameId,
    env: params?.env,
    page: params?.page,
    pageSize: params?.pageSize,
    type: params?.type,
  };
}

function isRecord(value: unknown): value is Record<string, any> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function asString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value : undefined;
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

export async function listEntityRecords(params?: EntityScopeParams) {
  const response = await request<RawEntityRecordsListResponse>('/api/v1/entities', {
    params: buildEntityScopeParams(params),
  });
  return normalizeEntityRecordsListResponse(response);
}

export async function getEntityRecord(id: string, params?: EntityScopeParams) {
  const response = await request<RawEntityRecord>(`/api/v1/entities/${encodeURIComponent(id)}`, {
    params: buildEntityScopeParams(params),
  });
  return normalizeEntityRecord(response);
}

// Compatibility wrappers for current entity-definition pages.
export async function listEntities(params?: EntityScopeParams) {
  const response = await listEntityRecords(params);
  return response.items.map(projectEntityDefinition);
}

export async function getEntity(id: string, params?: EntityScopeParams) {
  const response = await getEntityRecord(id, params);
  return projectEntityDefinition(response);
}

export async function createEntity(entity: Partial<EntityDefinition>, params?: EntityScopeParams) {
  const response = await request<RawEntityRecord>('/api/v1/entities', {
    method: 'POST',
    data: buildEntityMutationRequest(entity),
    params: buildEntityScopeParams(params),
  });
  return projectEntityDefinition(normalizeEntityRecord(response));
}

export async function updateEntity(
  id: string,
  entity: Partial<EntityDefinition>,
  params?: EntityScopeParams,
) {
  const response = await request<RawEntityRecord>(`/api/v1/entities/${encodeURIComponent(id)}`, {
    method: 'PUT',
    data: buildEntityMutationRequest(entity),
    params: buildEntityScopeParams(params),
  });
  return projectEntityDefinition(normalizeEntityRecord(response));
}

export async function deleteEntity(id: string, params?: EntityScopeParams) {
  return request<void>(`/api/v1/entities/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    params: buildEntityScopeParams(params),
  });
}

export async function validateEntity(
  entity: Partial<EntityDefinition>,
  params?: EntityScopeParams,
) {
  const response = await request<EntityValidationResult>('/api/v1/entities/validate', {
    method: 'POST',
    data: buildEntityMutationRequest(entity),
    params: buildEntityScopeParams(params),
  });
  return {
    valid: !!response.valid,
    errors: response.errors || [],
    warnings: response.warnings || [],
  };
}

export async function previewEntity(id: string, params?: EntityScopeParams) {
  const response = await request<RawEntityPreviewResponse>(
    `/api/v1/entities/${encodeURIComponent(id)}/preview`,
    {
      params: buildEntityScopeParams(params),
    },
  );
  return normalizeEntityPreviewResult(response);
}

// Entity-related function discovery is owned by the OpenAPI layer on the backend.
export async function listEntityFunctions(id: string, params?: EntityScopeParams) {
  const response = await request<RawEntityFunctionsResponse>(
    `/api/v1/entities/${encodeURIComponent(id)}/functions`,
    {
      params: buildEntityScopeParams(params),
    },
  );
  return Array.isArray(response?.items) ? response.items : [];
}
