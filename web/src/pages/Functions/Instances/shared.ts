import type { FunctionDescriptor } from '@/services/api/functions';
import type { FunctionInstance } from '@/services/api';
import { parseInputSchema, type JSONSchemaType } from '@/utils/json';

// 从 descriptor 提取输入 JSON Schema：兼容顶层 inputSchema（字符串/对象）与
// descriptor.input / openapiSpec.requestBody.content[...].schema 嵌套形态。
export function resolveDescriptorSchema(
  descriptor?: FunctionDescriptor | null,
): JSONSchemaType | null {
  if (!descriptor) return null;
  const { inputSchema, schema, params } = descriptor as {
    inputSchema?: unknown;
    schema?: unknown;
    params?: unknown;
  };
  const candidates: unknown[] = [inputSchema, schema, params];
  const input = (descriptor as { input?: unknown }).input;
  if (input && typeof input === 'object') {
    candidates.push(input);
  }
  for (const value of candidates) {
    if (typeof value === 'string') {
      const parsed = parseInputSchema(value);
      if (parsed) return parsed;
    } else if (value && typeof value === 'object' && !Array.isArray(value)) {
      return value as JSONSchemaType;
    }
  }
  return null;
}

export type CoverageData = {
  totalFunctions: number;
  coveredFunctions: number;
  coveragePercentage: number;
  totalInstances: number;
  activeInstances: number;
  inactiveInstances: number;
  functionsByResourcePrefix: Record<string, number>;
  instancesByGame: Record<string, number>;
};

export type InstanceDetail = {
  instance: FunctionInstance;
  functionInfo?: FunctionDescriptor;
  logs?: Array<{
    timestamp: string;
    level: string;
    message: string;
  }>;
};

export type FunctionInstanceRow = FunctionInstance & {
  rowKey: string;
};

export function buildInstanceRowKey(
  instance: Pick<FunctionInstance, 'agentId' | 'serviceId' | 'providerId' | 'functionId' | 'addr'>,
  index: number,
  seen: Map<string, number>,
) {
  const baseKey = [
    instance.agentId,
    instance.serviceId || instance.providerId || '',
    instance.functionId,
    instance.addr,
  ]
    .map((value) => String(value || '').trim())
    .join('|');
  const normalizedBaseKey = baseKey || `__instance__${index}`;
  const duplicateCount = seen.get(normalizedBaseKey) || 0;
  seen.set(normalizedBaseKey, duplicateCount + 1);
  return duplicateCount === 0 ? normalizedBaseKey : `${normalizedBaseKey}#${duplicateCount}`;
}
