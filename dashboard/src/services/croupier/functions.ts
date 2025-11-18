import { request } from '@umijs/max';

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
  return request<FunctionDescriptor[]>('/api/descriptors');
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
  return request<any>('/api/invoke', { method: 'POST', data });
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
  return request<{ job_id: string }>('/api/start_job', { method: 'POST', data });
}

export async function cancelJob(job_id: string) {
  return request<void>('/api/cancel_job', { method: 'POST', data: { job_id } });
}

export async function fetchJobResult(id: string) {
  return request<{ state: string; payload?: any; error?: string }>('/api/job_result', { params: { id } });
}

export async function listFunctionInstances(params: { game_id?: string; function_id: string }) {
  return request<{ instances: { agent_id: string; service_id: string; addr: string; version: string }[] }>(
    '/api/function_instances',
    { params },
  );
}
