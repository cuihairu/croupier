import { request } from '@umijs/max';

export type RoleRecord = {
  id: number;
  name: string;
  description?: string;
  perms?: string[];
};

export async function listRoles() {
  return request<{ roles: RoleRecord[] }>('/api/roles');
}

export async function createRole(body: { name: string; description?: string; perms?: string[] }) {
  return request<{ id: number }>('/api/roles', { method: 'POST', data: body });
}

export async function updateRole(id: number, body: { name?: string; description?: string }) {
  return request<void>(`/api/roles/${id}`, { method: 'PUT', data: body });
}

export async function deleteRole(id: number) {
  return request<void>(`/api/roles/${id}`, { method: 'DELETE' });
}

export async function setRolePerms(id: number, perms: string[]) {
  return request<void>(`/api/roles/${id}/perms`, { method: 'PUT', data: { perms } });
}

