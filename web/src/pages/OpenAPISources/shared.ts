import type { OpenAPIDocument, OpenAPISourceOperation } from '@/services/api/openapi';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { Diagnostic } from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';

export type ApiErrorLike = {
  response?: {
    data?: {
      message?: string;
      details?: Record<string, unknown>;
    };
  };
  message?: string;
};

export function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message;
  const apiError = error as ApiErrorLike;
  if (apiError?.response?.data?.message) return apiError.response.data.message;
  return fallback;
}

function isDiagnostic(value: unknown): value is Diagnostic {
  if (!value || typeof value !== 'object') return false;
  const item = value as Partial<Diagnostic>;
  return typeof item.code === 'string' && typeof item.message === 'string';
}

export function diagnosticsFromError(error: unknown): Diagnostic[] {
  const apiError = error as ApiErrorLike;
  const details = apiError?.response?.data?.details;
  const raw = details?.diagnostics;
  return Array.isArray(raw) ? raw.filter(isDiagnostic) : [];
}

export function diagnosticColor(severity?: Diagnostic['severity']) {
  if (severity === 'error') return 'red';
  if (severity === 'warning') return 'orange';
  return 'blue';
}

export function diagnosticAlertType(
  severity?: Diagnostic['severity'],
): 'error' | 'warning' | 'info' {
  if (severity === 'error') return 'error';
  if (severity === 'warning') return 'warning';
  return 'info';
}

export function riskColor(risk?: string) {
  if (risk === 'danger') return 'red';
  if (risk === 'high') return 'volcano';
  if (risk === 'warning') return 'orange';
  return 'green';
}

export function capabilityColor(capability?: string) {
  if (capability === 'task') return 'purple';
  if (capability === 'report') return 'geekblue';
  if (capability === 'collection_query' || capability === 'item_query') return 'cyan';
  if (capability === 'create' || capability === 'update' || capability === 'delete')
    return 'volcano';
  return 'blue';
}

export function executionColor(execution?: string) {
  if (execution === 'task') return 'purple';
  return 'green';
}

export function formatDate(value?: string): string {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

export function functionLabel(fn: FunctionDescriptor): string {
  const title =
    localizedText(fn.summary, 'zh-CN', '') || localizedText(fn.displayName, 'zh-CN', '') || fn.id;
  return `${title} (${fn.id})`;
}

export function operationLabel(operation: OpenAPISourceOperation): string {
  return operation.summary || operation.operation || operation.operationId;
}

export function proposalInboxPath(proposalKey: string, resourceKey?: string): string {
  const params = new URLSearchParams();
  if (resourceKey) params.set('resourceKey', resourceKey);
  params.set('proposalKey', proposalKey);
  return `/functions/pages?${params.toString()}`;
}

export function parseOpenAPIDocument(text: string): OpenAPIDocument {
  const value: unknown = JSON.parse(text);
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('OpenAPI JSON 必须是对象');
  }
  const record = value as Record<string, unknown>;
  if (typeof record.openapi !== 'string' || record.openapi.trim() === '') {
    throw new Error('OpenAPI JSON 缺少 openapi 字段');
  }
  if (!record.info || typeof record.info !== 'object' || Array.isArray(record.info)) {
    throw new Error('OpenAPI JSON 缺少 info 对象');
  }
  return value as OpenAPIDocument;
}

export type SourceModalMode = 'create' | 'update';
