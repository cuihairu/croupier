import { request } from '@umijs/max';
import type {
  ApprovalPolicy,
  CapabilityKind,
  Diagnostic,
  FunctionExecution,
  JSONValue,
  LocalizedText,
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
  'x-resource'?: string;
  'x-capability'?: CapabilityKind;
  'x-execution'?: FunctionExecution;
  'x-risk'?: RiskLevel;
  'x-operation'?: string; // business action key, e.g. "ban", "send", "list"
  'x-enabled'?: boolean;
  'x-permission'?: string;
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
  resource?: string;
  operation?: string;
  capability?: CapabilityKind;
  execution?: FunctionExecution;
  approval: ApprovalPolicy;
  risk?: RiskLevel;
  enabled?: boolean;
  permission?: string;
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

export interface OpenAPIBindingProposal {
  proposalKey: string;
  pageKey: string;
  pageType: string;
  resourceKey?: string;
  quality: string;
  status: string;
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
  proposal?: OpenAPIBindingProposal;
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

export async function updateOpenAPISource(sourceId: string, spec: OpenAPIDocument, name?: string) {
  return request<OpenAPISourceGetResponse>(
    `/api/v1/openapi/sources/${encodeURIComponent(sourceId)}`,
    {
      method: 'PUT',
      data: { name, spec },
    },
  );
}

export async function listOpenAPISources() {
  return request<OpenAPISourceListResponse>('/api/v1/openapi/sources');
}

export async function getOpenAPISource(sourceId: string) {
  return request<OpenAPISourceGetResponse>(
    `/api/v1/openapi/sources/${encodeURIComponent(sourceId)}`,
  );
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
  summary?: LocalizedText;
  description?: string;
  tags?: string[];
  resource?: string;
  capability?: CapabilityKind;
  execution?: FunctionExecution;
  risk?: RiskLevel;
  operation?: string;
  enabled?: boolean;
  permission?: string;
}): OpenAPIOperation {
  const operation: OpenAPIOperation = {
    operationId: descriptor.id,
    summary: descriptor.summary?.zh || descriptor.summary?.en || descriptor.id,
    description: descriptor.description || descriptor.summary?.zh || descriptor.summary?.en,
    tags: descriptor.tags || (descriptor.resource ? [descriptor.resource] : undefined),
  };

  if (descriptor.resource) {
    operation.extensions = {
      ...operation.extensions,
      'x-resource': descriptor.resource,
    };
  }

  if (descriptor.capability) {
    operation.extensions = {
      ...operation.extensions,
      'x-capability': descriptor.capability,
    };
  }

  if (descriptor.execution) {
    operation.extensions = {
      ...operation.extensions,
      'x-execution': descriptor.execution,
    };
  }

  if (descriptor.risk) {
    operation.extensions = {
      ...operation.extensions,
      'x-risk': descriptor.risk,
    };
  }

  if (descriptor.operation) {
    operation.extensions = {
      ...operation.extensions,
      'x-operation': descriptor.operation,
    };
  }

  if (typeof descriptor.enabled === 'boolean') {
    operation.extensions = {
      ...operation.extensions,
      'x-enabled': descriptor.enabled,
    };
  }

  if (descriptor.permission) {
    operation.extensions = {
      ...operation.extensions,
      'x-permission': descriptor.permission,
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
  resource?: string;
  capability?: CapabilityKind;
  execution?: FunctionExecution;
  risk?: RiskLevel;
  operation?: string;
  enabled?: boolean;
  permission?: string;
} {
  return {
    resource: operation.extensions?.['x-resource'],
    capability: operation.extensions?.['x-capability'],
    execution: operation.extensions?.['x-execution'],
    risk: operation.extensions?.['x-risk'],
    operation: operation.extensions?.['x-operation'],
    enabled: operation.extensions?.['x-enabled'],
    permission: operation.extensions?.['x-permission'],
  };
}
