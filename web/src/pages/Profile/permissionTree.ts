/**
 * 权限树的数据层：把「角色 → 资源 → 操作」组装成可渲染的树。
 *
 * 与渲染组件分开是为了让判定逻辑可以纯函数测试——「某项到底算不算已授权」
 * 是这个页面唯一容易出错的判断（BUG-020）。
 */

export type PermissionCatalogEntry = {
  /** 权限 id，形如 `resource:action` */
  id: string;
  name: string;
  description?: string;
  category?: string;
};

export type RoleGrant = {
  role: string;
  permissionIds: string[];
};

export type ActionNode = {
  key: string;
  /** 权限 id（`resource:action`）。资源节点为 `*` 通配时会是 `resource:*` */
  id: string;
  action: string;
  name: string;
  description?: string;
  /** 该角色是否被授予了这一项 */
  granted: boolean;
  /**
   * 授予来源：非空表示这项权限并不在当前角色的授权里，而是被别的角色
   * 或账号级通配（`*` / `admin:all`）覆盖。用于在灰项上说明「为什么也是绿的」。
   */
  grantedBy?: string;
};

export type ResourceNode = {
  key: string;
  resource: string;
  name: string;
  actions: ActionNode[];
  /** 该资源下已授权项数 / 总项数 */
  grantedCount: number;
};

export type RoleNode = {
  key: string;
  role: string;
  resources: ResourceNode[];
  grantedCount: number;
  actionCount: number;
  /** 该角色没有任何显式权限 */
  empty: boolean;
};

export type PermissionTree = {
  roles: RoleNode[];
  /** 全量已授权项数（跨角色去重后的并集口径） */
  totalGranted: number;
  /** 全量条目数（跨角色计，同一权限被多角色授予则重复计入） */
  totalActions: number;
};

/** 通配 id：等价于全部权限。 */
const GLOBAL_WILDCARDS = new Set(['*', 'admin:all']);

/** 把权限 id 拆成资源与操作；与后端 rbac.SplitLogicalPermission 同语义。 */
export function splitPermissionId(
  id: string,
): { resource: string; action: string } {
  const normalized = (id || '').trim().toLowerCase();
  if (normalized === '' || GLOBAL_WILDCARDS.has(normalized)) {
    return { resource: '*', action: '*' };
  }
  const idx = normalized.indexOf(':');
  if (idx < 0) {
    // 无冒号：整串即资源，操作通配（与后端一致，不按 '.' 猜切分）
    return { resource: normalized, action: '*' };
  }
  const rawResource = normalized.slice(0, idx).trim();
  const rawAction = normalized.slice(idx + 1).trim();
  return {
    resource: rawResource === '' ? '*' : rawResource,
    action: rawAction === '' || rawAction === 'all' ? '*' : rawAction,
  };
}

/** 账号级通配：`*` 与 `admin:all` 覆盖一切。 */
export function hasGlobalWildcard(permissionIds: readonly string[]): boolean {
  return permissionIds.some((id) => GLOBAL_WILDCARDS.has((id || '').trim().toLowerCase()));
}

/** 某角色是否持有 `resource:action`。`resource:all` 等价于该资源全部操作。 */
export function roleGrants(
  rolePermissionIds: readonly string[],
  resource: string,
  action: string,
): boolean {
  const ids = new Set((rolePermissionIds || []).map((id) => (id || '').trim().toLowerCase()));
  if (ids.has('*') || ids.has('admin:all')) return true;
  if (ids.has(`${resource}:*`) || ids.has(`${resource}:all`)) return true;
  return ids.has(`${resource}:${action}`);
}

function titleCase(s: string): string {
  return s
    .split(/[_\s]+/)
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ');
}

/**
 * 构建角色 → 资源 → 操作 的树。
 *
 * 关键设计：**资源轴来自权限目录**（catalog），而不是来自「已授权 id」。
 * 只用已授权 id 的话，灰掉的「未授权」操作根本没有数据来源，页面就只能列出
 * 已有的东西——看上去像「全部都有权限」，正是 BUG-020 要消灭的假象。
 * 目录缺失（catalog 为空）时退化为「只用已授权 id」，仍不会崩。
 */
export function buildPermissionTree(
  roles: readonly RoleGrant[],
  catalog: readonly PermissionCatalogEntry[],
  fallbackMessage: (id: string) => string = (id) => id,
): PermissionTree {
  // 资源 → 操作 → 元信息。目录优先（带 name/description）
  const resourceActions = new Map<string, Map<string, PermissionCatalogEntry>>();
  for (const entry of catalog || []) {
    const id = (entry.id || '').trim();
    if (id === '' || GLOBAL_WILDCARDS.has(id.toLowerCase())) continue;
    const { resource, action } = splitPermissionId(id);
    if (resource === '*') continue;
    let actions = resourceActions.get(resource);
    if (!actions) {
      actions = new Map();
      resourceActions.set(resource, actions);
    }
    actions.set(action, { ...entry, id });
  }

  // 账号级并集：用于「别的角色也授过」的说明与全量口径
  const union = new Set<string>();
  for (const r of roles || []) {
    for (const id of r.permissionIds || []) {
      const t = (id || '').trim().toLowerCase();
      if (t !== '') union.add(t);
    }
  }
  const unionWildcard = hasGlobalWildcard([...union]);

  const nodes: RoleNode[] = [];
  for (const grant of roles || []) {
    const roleIds = (grant.permissionIds || []).map((id) =>
      (id || '').trim().toLowerCase(),
    );
    const roleHasWildcard = hasGlobalWildcard(roleIds);

    // 该角色可见的资源 = 目录里的全部资源 + 该角色 id 里出现但目录没有的资源
    const resourceNames = new Set<string>(resourceActions.keys());
    for (const id of roleIds) {
      const { resource } = splitPermissionId(id);
      if (resource !== '*') resourceNames.add(resource);
    }

    const resources: ResourceNode[] = [];
    let roleGranted = 0;
    let roleActions = 0;
    for (const resource of [...resourceNames].sort()) {
      const catalogActions = resourceActions.get(resource);
      // 资源下的操作 = 目录里的 + 该角色持有的（目录缺失时只剩后者）
      const actionNames = new Set<string>(catalogActions ? catalogActions.keys() : []);
      for (const id of roleIds) {
        const split = splitPermissionId(id);
        if (split.resource === resource && split.action !== '*') {
          actionNames.add(split.action);
        }
      }
      // 持有 resource:* 时补一条通配操作，否则「该资源全部可操作」会看不见
      if (roleHasWildcard || roleIds.includes(`${resource}:*`) || roleIds.includes(`${resource}:all`)) {
        actionNames.add('*');
      }
      if (actionNames.size === 0) continue;

      const actions: ActionNode[] = [...actionNames].sort().map((action) => {
        const id =
          action === '*' ? `${resource}:*` : `${resource}:${action}`;
        const meta = catalogActions?.get(action);
        const granted = roleHasWildcard || roleGrants(roleIds, resource, action);
        return {
          key: `${resource}:${action}`,
          id,
          action,
          name: meta?.name || fallbackMessage(id),
          description: meta?.description,
          granted,
          // 没被当前角色授予、但并集里有 → 说明是被别的角色/通配覆盖
          grantedBy: !granted && unionWildcard ? 'wildcard' : undefined,
        };
      });

      const grantedCount = actions.filter((a) => a.granted).length;
      roleGranted += grantedCount;
      roleActions += actions.length;
      resources.push({
        key: `resource:${resource}`,
        resource,
        name: titleCase(resource),
        actions,
        grantedCount,
      });
    }

    nodes.push({
      key: `role:${grant.role}`,
      role: grant.role,
      resources,
      grantedCount: roleGranted,
      actionCount: roleActions,
      empty: roleGranted === 0,
    });
  }

  return {
    roles: nodes,
    totalGranted: nodes.reduce((sum, n) => sum + n.grantedCount, 0),
    totalActions: nodes.reduce((sum, n) => sum + n.actionCount, 0),
  };
}
