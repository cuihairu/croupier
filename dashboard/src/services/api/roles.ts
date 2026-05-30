import {
  createRole as createRoleCanonical,
  deleteRole as deleteRoleCanonical,
  getRole as getRoleCanonical,
  listRoles as listRolesCanonical,
  updateRole as updateRoleCanonical,
  updateRolePermissions,
  type RoleRecord as CanonicalRoleRecord,
} from './permissions';

// Legacy compatibility DTO used by older role-management pages.
// Canonical source is croupier/internal/api/role/dto.go Role.
export type RoleRecord = CanonicalRoleRecord & {
  perms: string[];
};

// Map canonical role DTO to the legacy role-management page shape.
function toLegacyRole(role: CanonicalRoleRecord): RoleRecord {
  return {
    ...role,
    perms: role.permissions || [],
  };
}

// Compatibility wrapper around listRoles for pages expecting { roles: ... }.
// Source response: croupier/internal/api/role/dto.go RolesListResponse
export async function listRoles(params?: {
  page?: number;
  pageSize?: number;
  category?: string;
  search?: string;
}) {
  const resp = await listRolesCanonical(params);
  return {
    roles: Array.isArray(resp.items) ? resp.items.map(toLegacyRole) : [],
    total: resp.total,
    page: resp.page,
    pageSize: resp.pageSize,
  };
}

// Compatibility wrapper around createRole.
// Source request/response: croupier/internal/api/role/dto.go
export async function createRole(body: {
  name: string;
  description?: string;
  category?: string;
  perms?: string[];
}) {
  const created = await createRoleCanonical({
    name: body.name,
    description: body.description,
    category: body.category,
    permissions: body.perms || [],
  });
  return toLegacyRole(created);
}

// Compatibility wrapper around getRole.
export async function getRole(id: number) {
  return toLegacyRole(await getRoleCanonical(id));
}

// Compatibility wrapper around updateRole.
// Source request/response: croupier/internal/api/role/dto.go
export async function updateRole(
  id: number,
  body: {
    name?: string;
    description?: string;
    category?: string;
    perms?: string[];
  },
) {
  const updated = await updateRoleCanonical(id, {
    name: body.name,
    description: body.description,
    category: body.category,
    permissions: body.perms,
  });
  return toLegacyRole(updated);
}

export async function deleteRole(id: number) {
  return deleteRoleCanonical(id);
}

// Compatibility wrapper around updateRolePermissions.
export async function setRolePerms(id: number, permissions: string[]) {
  return updateRolePermissions(id, permissions);
}
