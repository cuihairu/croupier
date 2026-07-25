import { request } from '@umijs/max';
import type {
  Diagnostic,
  JSONValue,
  LocalizedText,
  OperationKind,
  OperationPlacement,
  RiskLevel,
} from '@/types/dashboard';

/**
 * ============================================================================
 * OpenAPI 3.0.3 服务
 * ============================================================================
 */

/**
 * OpenAPI 3.0.3 扩展字段类型
 */
// Canonical frontend projection for OpenAPI operation extensions.
// Source: croupier/internal/api/openapi/dto.go OpenAPISpecResponse.Spec
export type OpenAPIExtensions = {
  'x-category'?: string;
  'x-risk'?: RiskLevel;
  'x-entity'?: string;
  'x-operation'?: string; // business action key, e.g. "ban", "send", "list"
  'x-category-display'?: LocalizedText;
  'x-entity-display'?: LocalizedText;
  'x-operation-display'?: LocalizedText;
  'x-operation-kind'?: OperationKind;
  'x-placement'?: OperationPlacement;
  'x-page-hint'?: string;
};

/**
 * OpenAPI 3.0.3 Operation Object
 */
// Canonical frontend projection for a single function OpenAPI operation.
// Backend returns { spec: operation } from GET /api/v1/functions/:id/openapi.
export type OpenAPIOperation = {
  operationId?: string;
  summary?: string;
  description?: string;
  tags?: string[];
  parameters?: JSONValue[];
  requestBody?: JSONValue;
  responses?: JSONValue;
  extensions?: OpenAPIExtensions;
};

/**
 * OpenAPI 3.0.3 Document
 */
// Source: croupier/internal/api/openapi/dto.go OpenAPIDocumentResponse.Spec / OpenAPISourceCreateRequest.Spec
export type OpenAPIDocument = {
  openapi: string;
  info: {
    title?: string;
    version?: string;
    description?: string;
  };
  paths?: Record<string, JSONValue>;
  components?: JSONValue;
};

// Source: croupier/internal/api/openapi/dto.go OpenAPISpecResponse
export type GetFunctionOpenAPIResponse = {
  spec: OpenAPIOperation;
};

export interface OpenAPISourceSummary {
  sourceId: string;
  gameId?: string;
  env?: string;
  name: string;
  revision: number;
  format: string;
  openapiVersion: string;
  infoTitle?: string;
  infoVersion?: string;
  contentHash: string;
  operationCount: number;
  diagnosticCount: number;
  createdAt: string;
  updatedAt: string;
  diagnostics?: Diagnostic[];
}

export interface OpenAPISourceOperation {
  operationId: string;
  method: string;
  path: string;
  summary?: string;
  description?: string;
  tags?: string[];
  category?: string;
  categoryDisplay?: LocalizedText;
  entity?: string;
  entityDisplay?: LocalizedText;
  operation?: string;
  operationDisplay?: LocalizedText;
  operationKind?: OperationKind;
  placement?: OperationPlacement;
  pageHint?: string;
  risk?: RiskLevel;
  bound: boolean;
  bindingId?: string;
  functionId?: string;
}

export interface OpenAPISourceBinding {
  bindingId: string;
  operationId: string;
  kind: 'provider';
  functionId?: string;
  providerId?: string;
  createdAt: string;
  updatedAt: string;
}

export interface OpenAPISourceDetail extends OpenAPISourceSummary {
  spec?: OpenAPIDocument;
  operations: OpenAPISourceOperation[];
  bindings?: OpenAPISourceBinding[];
}

export type OpenAPISourceListResponse = {
  items: OpenAPISourceSummary[];
};

export type OpenAPISourceGetResponse = {
  source: OpenAPISourceDetail;
};

export type OpenAPISourceDiagnosticsResponse = {
  sourceId: string;
  diagnostics: Diagnostic[];
};

export type OpenAPISourceBindingResponse = {
  binding: OpenAPISourceBinding;
};

export function normalizeFunctionOpenAPIResponse(
  resp: GetFunctionOpenAPIResponse | OpenAPIOperation,
): OpenAPIOperation {
  if (resp && typeof resp === 'object' && 'spec' in resp) {
    return resp.spec || {};
  }
  return resp || {};
}

/**
 * 获取函数的 OpenAPI 规范
 * @param functionId 函数 ID
 * @returns OpenAPI Operation Object
 */
export async function getFunctionOpenAPI(functionId: string) {
  const resp = await request<GetFunctionOpenAPIResponse>(`/api/v1/functions/${functionId}/openapi`);
  return normalizeFunctionOpenAPIResponse(resp);
}

export async function createOpenAPISource(spec: OpenAPIDocument, name?: string) {
  return request<OpenAPISourceGetResponse>('/api/v1/openapi/sources', {
    method: 'POST',
    data: { name, spec },
  });
}

export async function uploadOpenAPISourceFile(file: File, name?: string) {
  const data = new FormData();
  data.append('file', file);
  if (name) {
    data.append('name', name);
  }
  return request<OpenAPISourceGetResponse>('/api/v1/openapi/sources', {
    method: 'POST',
    data,
    requestType: 'form',
  });
}

export async function listOpenAPISources() {
  return request<OpenAPISourceListResponse>('/api/v1/openapi/sources');
}

export async function getOpenAPISource(sourceId: string) {
  return request<OpenAPISourceGetResponse>(`/api/v1/openapi/sources/${encodeURIComponent(sourceId)}`);
}

export async function getOpenAPISourceDiagnostics(sourceId: string) {
  return request<OpenAPISourceDiagnosticsResponse>(
    `/api/v1/openapi/sources/${encodeURIComponent(sourceId)}/diagnostics`,
  );
}

export async function bindOpenAPISourceProvider(
  sourceId: string,
  payload: {
    operationId: string;
    functionId: string;
    bindingId?: string;
    providerId?: string;
  },
) {
  return request<OpenAPISourceBindingResponse>(
    `/api/v1/openapi/sources/${encodeURIComponent(sourceId)}/bindings`,
    {
      method: 'POST',
      data: {
        ...payload,
        kind: 'provider',
      },
    },
  );
}

export async function deleteOpenAPISourceBinding(sourceId: string, bindingId: string) {
  return request<OpenAPISourceBindingResponse>(
    `/api/v1/openapi/sources/${encodeURIComponent(sourceId)}/bindings/${encodeURIComponent(bindingId)}`,
    {
      method: 'DELETE',
    },
  );
}

/**
 * 验证 OpenAPI 规范
 * @param spec OpenAPI Document
 * @returns 验证结果
 */
export async function validateOpenAPISpec(spec: OpenAPIDocument) {
  return request<{
    valid: boolean;
    errors: string[];
    warnings: string[];
  }>('/api/v1/openapi/validate', {
    method: 'POST',
    data: { spec },
  });
}

/**
 * 将函数描述符转换为 OpenAPI 规范
 * @param descriptor 函数描述符
 * @returns OpenAPI Operation Object
 */
export function descriptorToOpenAPI(descriptor: {
  id: string;
  display_name?: LocalizedText;
  description?: string;
  summary?: LocalizedText;
  tags?: string[];
  category?: string;
  risk?: RiskLevel;
  entity?: string;
  operation?: string;
}): OpenAPIOperation {
  const operation: OpenAPIOperation = {
    operationId: descriptor.id,
    summary: descriptor.display_name?.zh || descriptor.display_name?.en || descriptor.id,
    description: descriptor.description || descriptor.summary?.zh || descriptor.summary?.en,
    tags: descriptor.tags || [descriptor.category],
  };

  // 添加扩展字段
  if (descriptor.category) {
    operation.extensions = {
      ...operation.extensions,
      'x-category': descriptor.category,
    };
  }

  if (descriptor.risk) {
    operation.extensions = {
      ...operation.extensions,
      'x-risk': descriptor.risk,
    };
  }

  if (descriptor.entity) {
    operation.extensions = {
      ...operation.extensions,
      'x-entity': descriptor.entity,
    };
  }

  if (descriptor.operation) {
    operation.extensions = {
      ...operation.extensions,
      'x-operation': descriptor.operation,
    };
  }

  return operation;
}

/**
 * 从 OpenAPI 扩展字段提取元数据
 * @param operation OpenAPI Operation
 * @returns 元数据对象
 */
export function extractOpenAPIMetadata(operation: OpenAPIOperation): {
  category?: string;
  risk?: RiskLevel;
  entity?: string;
  operation?: string;
  operationKind?: OperationKind;
  placement?: OperationPlacement;
} {
  return {
    category: operation.extensions?.['x-category'],
    risk: operation.extensions?.['x-risk'],
    entity: operation.extensions?.['x-entity'],
    operation: operation.extensions?.['x-operation'],
    operationKind: operation.extensions?.['x-operation-kind'],
    placement: operation.extensions?.['x-placement'],
  };
}
