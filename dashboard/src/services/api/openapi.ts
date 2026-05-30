import { request } from '@umijs/max';

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
  'x-risk'?: 'safe' | 'warning' | 'danger';
  'x-entity'?: string;
  'x-operation'?: 'create' | 'read' | 'update' | 'delete' | 'custom';
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
  parameters?: any[];
  requestBody?: any;
  responses?: any;
  extensions?: OpenAPIExtensions;
};

/**
 * OpenAPI 3.0.3 Document
 */
// Source: croupier/internal/api/openapi/dto.go OpenAPIDocumentResponse.Spec / OpenAPIImportRequest.Spec
export type OpenAPIDocument = {
  openapi: string;
  info: {
    title?: string;
    version?: string;
    description?: string;
  };
  paths?: Record<string, any>;
  components?: any;
};

// Source: croupier/internal/api/openapi/dto.go OpenAPISpecResponse
export type GetFunctionOpenAPIResponse = {
  spec: OpenAPIOperation;
};

// Source: croupier/internal/api/openapi/dto.go OpenAPIImportResponse
export type OpenAPIImportResponse = {
  imported: number;
  failed: string[];
};

// Source: croupier/internal/api/openapi/dto.go EntityFunctionsResponse
export type EntityFunctionsResponse = {
  items: Array<{
    id: string;
    operation: 'create' | 'read' | 'update' | 'delete' | 'custom';
    name: string;
    summary?: string;
  }>;
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

/**
 * 导入 OpenAPI 规范
 * @param spec OpenAPI 3.0.3 Document
 * @returns 导入结果
 */
export async function importOpenAPISpec(spec: OpenAPIDocument) {
  return request<OpenAPIImportResponse>('/api/v1/functions/_import', {
    method: 'POST',
    data: { spec },
  });
}

/**
 * 获取 Entity 关联的函数列表
 * @param entityId Entity ID
 * @returns Entity 函数列表
 */
export async function getEntityFunctions(entityId: string) {
  return request<EntityFunctionsResponse>(`/api/v1/entities/${entityId}/functions`);
}

/**
 * 验证 OpenAPI 规范
 * @param spec OpenAPI Document
 * @returns 验证结果
 */
export async function validateOpenAPISpec(spec: any) {
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
export function descriptorToOpenAPI(descriptor: any): OpenAPIOperation {
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
  risk?: 'safe' | 'warning' | 'danger';
  entity?: string;
  operationType?: 'create' | 'read' | 'update' | 'delete' | 'custom';
} {
  return {
    category: operation.extensions?.['x-category'],
    risk: operation.extensions?.['x-risk'],
    entity: operation.extensions?.['x-entity'],
    operationType: operation.extensions?.['x-operation'],
  };
}
