import {
  createAdmin,
  deleteAdmin,
  getAdmin,
  getAdminGames,
  listAdmins,
  resetAdminPassword,
  updateAdmin,
  updateAdminGames,
  type AdminGame,
  type AdminRecord,
} from './permissions';

// Legacy compatibility DTO used by older user-management pages.
// Canonical source is AdminRecord from croupier/internal/api/admin/dto.go.
export type UserRecord = AdminRecord & {
  display_name: string;
  active: boolean;
};

// Legacy compatibility DTO for user game scopes.
// Canonical source is AdminGamesResponse from croupier/internal/api/admin/dto.go.
export type UserGameEnv = {
  game_ids: string[];
  games: AdminGame[];
};

// Map canonical admin DTO to the legacy user-management page shape.
function toUserRecord(admin: AdminRecord): UserRecord {
  return {
    ...admin,
    display_name: admin.nickname,
    active: admin.status === 1,
  };
}

// Compatibility wrapper around listAdmins for legacy pages expecting { users: ... }.
export async function listUsers(params?: {
  page?: number;
  pageSize?: number;
  search?: string;
  role?: string;
  status?: number;
}) {
  const resp = await listAdmins(params);
  return {
    users: Array.isArray(resp.items) ? resp.items.map(toUserRecord) : [],
    total: resp.total,
    page: resp.page,
    pageSize: resp.pageSize,
  };
}

// Compatibility wrapper around createAdmin.
// Source request/response: croupier/internal/api/admin/dto.go
export async function createUser(body: {
  username: string;
  password: string;
  display_name?: string;
  email?: string;
  phone?: string;
  active?: boolean;
  roles: string[];
}) {
  const created = await createAdmin({
    username: body.username,
    password: body.password,
    nickname: body.display_name || '',
    email: body.email,
    phone: body.phone,
    roles: body.roles,
  });
  if (body.active === false) {
    return toUserRecord(
      await updateAdmin(created.id, {
        status: 0,
      }),
    );
  }
  return toUserRecord(created);
}

// Compatibility wrapper around getAdmin.
export async function getUser(id: number) {
  const admin = await getAdmin(id);
  return toUserRecord(admin);
}

// Compatibility wrapper around updateAdmin.
// Source request/response: croupier/internal/api/admin/dto.go
export async function updateUser(
  id: number,
  body: {
    display_name?: string;
    email?: string;
    phone?: string;
    active?: boolean;
    roles?: string[];
  },
) {
  const updated = await updateAdmin(id, {
    nickname: body.display_name,
    email: body.email,
    phone: body.phone,
    roles: body.roles,
    status: typeof body.active === 'boolean' ? (body.active ? 1 : 0) : undefined,
  });
  return toUserRecord(updated);
}

export async function deleteUser(id: number) {
  return deleteAdmin(id);
}

export async function setUserPassword(id: number, password: string) {
  return resetAdminPassword(id, password);
}

// Compatibility wrapper returning both canonical games and legacy game_ids projection.
// Source response: croupier/internal/api/admin/dto.go AdminGamesResponse
export async function listUserGames(userId: number): Promise<UserGameEnv> {
  const resp = await getAdminGames(userId);
  const games = Array.isArray(resp.games) ? resp.games : [];
  return {
    game_ids: games.map((game) => game.gameId),
    games,
  };
}

// Legacy helper kept for page compatibility. It merges bare IDs into current scopes.
export async function setUserGames(userId: number, gameIds: Array<string | number>) {
  const current = await getAdminGames(userId);
  const currentGames = Array.isArray(current.games) ? current.games : [];
  const currentMap = new Map(currentGames.map((game) => [String(game.gameId), game]));
  const mergedGames = Array.from(new Set(gameIds.map((id) => String(id))))
    .map((id) => currentMap.get(id))
    .filter(Boolean) as AdminGame[];
  return updateAdminGames(userId, mergedGames);
}

// 获取用户游戏环境的兼容函数。
export async function listUserGameEnvs(userId: number, gameId: string | number) {
  const resp = await getAdminGames(userId);
  const target = (resp.games || []).find((game) => String(game.gameId) === String(gameId));
  return { envs: target?.envs || [] };
}

// 设置用户游戏环境的兼容函数。
export async function setUserGameEnvs(userId: number, gameId: string | number, envs: string[]) {
  const resp = await getAdminGames(userId);
  const games = Array.isArray(resp.games) ? resp.games : [];
  const nextGames = [];
  let matched = false;
  for (const game of games) {
    if (String(game.gameId) === String(gameId)) {
      matched = true;
      nextGames.push({ ...game, envs });
    } else {
      nextGames.push(game);
    }
  }
  if (!matched) {
    nextGames.push({ gameId: String(gameId), gameName: String(gameId), envs });
  }
  return updateAdminGames(userId, nextGames);
}
