import { request } from '@umijs/max';

export type EntityDefinition = {
  id: string;
  name?: string;
  description?: string;
  schema?: any; // JSON Schema
  uiSchema?: any; // UI Schema for form rendering
  operations?: string[]; // Available operations like 'create', 'read', 'update', 'delete'
  created_at?: string;
  updated_at?: string;
};

export type EntityValidationResult = {
  valid: boolean;
  errors?: string[];
  warnings?: string[];
};

export type EntityPreviewResult = {
  preview_html?: string;
  preview_data?: any;
};

// List all entity definitions
export async function listEntities(params?: { game_id?: string; env?: string }) {
  const res = await request<{ entities?: EntityDefinition[] } | EntityDefinition[]>('/api/v1/entities', { params });
  if (Array.isArray(res)) return res;
  if (res && Array.isArray((res as any).entities)) return (res as any).entities;
  return [];
}

// Get specific entity definition
export async function getEntity(id: string, params?: { game_id?: string; env?: string }) {
  return request<EntityDefinition>(`/api/v1/entities/${encodeURIComponent(id)}`, { params });
}

// Create new entity definition
export async function createEntity(entity: Omit<EntityDefinition, 'id' | 'created_at' | 'updated_at'>, params?: { game_id?: string; env?: string }) {
  return request<EntityDefinition>('/api/v1/entities', {
    method: 'POST',
    data: entity,
    params,
  });
}

// Update existing entity definition
export async function updateEntity(id: string, entity: Partial<EntityDefinition>, params?: { game_id?: string; env?: string }) {
  return request<EntityDefinition>(`/api/v1/entities/${encodeURIComponent(id)}`, {
    method: 'PUT',
    data: entity,
    params,
  });
}

// Delete entity definition
export async function deleteEntity(id: string, params?: { game_id?: string; env?: string }) {
  return request<void>(`/api/v1/entities/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    params,
  });
}

// Validate entity definition - 注意：后端可能是 POST /api/v1/entities/validate 或 /api/v1/schemas/validate
export async function validateEntity(entity: Partial<EntityDefinition>, params?: { game_id?: string; env?: string }) {
  return request<EntityValidationResult>('/api/v1/entities/validate', {
    method: 'POST',
    data: entity,
    params,
  });
}

// Preview entity definition UI
export async function previewEntity(id: string, params?: { game_id?: string; env?: string }) {
  return request<EntityPreviewResult>(`/api/v1/entities/${encodeURIComponent(id)}/preview`, { params });
}