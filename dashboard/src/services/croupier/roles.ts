// 角色管理功能已统一到 permissions.ts
export type { RoleRecord } from './permissions';

export {
  listRoles,
  createRole,
  getRole,
  updateRole,
  deleteRole
} from './permissions';
