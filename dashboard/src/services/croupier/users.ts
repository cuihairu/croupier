// 用户管理功能已统一到 permissions.ts
export type { AdminRecord as UserRecord, AdminGame as UserGameEnv } from './permissions';

export {
  listAdmins as listUsers,
  createAdmin as createUser,
  getAdmin as getUser,
  updateAdmin as updateUser,
  deleteAdmin as deleteUser,
  resetAdminPassword as setUserPassword,
  getAdminGames as listUserGames,
  updateAdminGames as setUserGames
} from './permissions';

// 获取用户游戏环境的兼容函数（已弃用，使用 listUserGames）
export async function listUserGameEnvs(userId: number, gameId: string) {
  // 这个需要根据实际 API 实现，暂时返回空数组
  return Promise.resolve<{ envs: string[] }>({ envs: [] });
}

// 设置用户游戏环境的兼容函数（已弃用，使用 setUserGames）
export async function setUserGameEnvs(userId: number, gameId: string, envs: string[]) {
  // 这个需要根据实际 API 实现，暂时返回成功
  return Promise.resolve<void>();
}
