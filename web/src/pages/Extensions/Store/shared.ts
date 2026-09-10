import type { JSONValue } from '@/types/dashboard';

export type InstallFormValues = {
  releaseVersion: string;
  scopeType: string;
  scopeId: string;
  targetType: string;
  targetId?: string;
  config?: Record<string, JSONValue>;
  configJson?: string;
};

export function buildSchemaDefaults(schema?: Record<string, JSONValue>): Record<string, JSONValue> {
  if (!schema || typeof schema !== 'object') return {};
  const properties = schema?.properties;
  if (!properties || typeof properties !== 'object') return {};
  const defaults: Record<string, JSONValue> = {};
  Object.entries(properties).forEach(([key, raw]) => {
    const prop = (raw || {}) as Record<string, JSONValue>;
    if (Object.prototype.hasOwnProperty.call(prop, 'default')) {
      defaults[key] = prop.default;
    }
  });
  return defaults;
}

export function normalizeConfigBySchema(
  rawConfig: Record<string, JSONValue>,
  schema?: Record<string, JSONValue>,
): Record<string, JSONValue> {
  if (!schema || typeof schema !== 'object') return rawConfig || {};
  const properties = schema?.properties;
  if (!properties || typeof properties !== 'object') return rawConfig || {};

  const out: Record<string, JSONValue> = { ...(rawConfig || {}) };
  Object.entries(properties).forEach(([key, raw]) => {
    const field = (raw || {}) as Record<string, JSONValue>;
    const fieldType = String(field.type || '');
    const value = out[key];
    if (value === undefined || value === null) return;

    if ((fieldType === 'number' || fieldType === 'integer') && typeof value === 'string') {
      const n = Number(value);
      if (!Number.isNaN(n)) {
        out[key] = fieldType === 'integer' ? Math.trunc(n) : n;
      }
      return;
    }
    if (fieldType === 'boolean' && typeof value === 'string') {
      const v = value.trim().toLowerCase();
      if (v === 'true' || v === '1') out[key] = true;
      if (v === 'false' || v === '0') out[key] = false;
      return;
    }
    if ((fieldType === 'array' || fieldType === 'object') && typeof value === 'string') {
      try {
        out[key] = JSON.parse(value);
      } catch {
        // keep raw text, backend validation will reject if invalid
      }
    }
  });
  return out;
}
