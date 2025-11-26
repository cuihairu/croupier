import { request } from '@umijs/max';
import { createEventSource } from '../core/http';

export type FunctionDescriptor = {
  id: string;
  version?: string;
  category?: string;
  params?: any;
  auth?: Record<string, any>;
  // Optional outputs schema for UI rendering (views/layout); present in generated descriptors
  outputs?: any;
};

export async function listDescriptors() {
  return request<FunctionDescriptor[]>('/api/v1/functions/descriptors');
}

export async function invokeFunction(
  function_id: string,
  payload: any,
  opts?: { route?: 'lb' | 'broadcast' | 'targeted' | 'hash'; target_service_id?: string; hash_key?: string },
) {
  const data: any = { function_id, payload };
  if (opts?.route) data.route = opts.route;
  if (opts?.target_service_id) data.target_service_id = opts.target_service_id;
  if (opts?.hash_key) data.hash_key = opts.hash_key;
  return request<any>(`/api/v1/functions/${function_id}/invoke`, { method: 'POST', data });
}

export async function startJob(
  function_id: string,
  payload: any,
  opts?: { route?: 'lb' | 'broadcast' | 'targeted' | 'hash'; target_service_id?: string; hash_key?: string },
) {
  const data: any = { function_id, payload };
  if (opts?.route) data.route = opts.route;
  if (opts?.target_service_id) data.target_service_id = opts.target_service_id;
  if (opts?.hash_key) data.hash_key = opts.hash_key;
  return request<{ job_id: string }>(`/api/v1/functions/${function_id}/invoke`, { method: 'POST', data });
}

export async function cancelJob(job_id: string) {
  return request<void>('/api/v1/jobs/cancel', { method: 'POST', data: { id: job_id } });
}

export async function fetchJobResult(id: string) {
  return request<{ state: string; payload?: any; error?: string }>(`/api/v1/jobs/${id}/result`, { method: 'GET' });
}

export async function listFunctionInstances(params: { game_id?: string; function_id: string }) {
  return request<{ instances: { agent_id: string; service_id: string; addr: string; version: string }[] }>(
    `/api/v1/functions/${params.function_id}/instances`,
    { params },
  );
}

export async function fetchFunctionUiSchema(functionId: string) {
  return request<{ uiSchema?: any; uischema?: any }>(`/api/v1/functions/${functionId}/ui`, { method: 'GET' });
}

export function openJobEventSource(jobId: string) {
  return createEventSource(`/api/v1/jobs/${jobId}/stream`);
}
