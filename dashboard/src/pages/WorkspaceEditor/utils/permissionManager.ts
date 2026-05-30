/**
 * 编辑权限控制
 *
 * 管理配置编辑权限，支持基于角色和对象的访问控制。
 *
 * @module pages/WorkspaceEditor/utils/permissionManager
 */

/** 用户角色 */
export type UserRole = 'admin' | 'developer' | 'viewer' | 'custom';

/** 权限项 */
export type Permission =
  | 'workspace:read' // 读取工作空间
  | 'workspace:edit' // 编辑工作空间
  | 'workspace:publish' // 发布工作空间
  | 'workspace:delete' // 删除工作空间
  | 'workspace:rollback' // 回滚版本
  | 'tab:edit' // 编辑 Tab
  | 'tab:delete' // 删除 Tab
  | 'sensitive:edit' // 编辑敏感配置
  | 'template:use' // 使用模板
  | 'audit:read'; // 查看审计日志

/** 权限规则 */
export interface PermissionRule {
  /** 权限项 */
  permission: Permission;
  /** 允许的角色 */
  roles?: UserRole[];
  /** 允许的用户列表 */
  users?: string[];
  /** 拒绝的用户列表 */
  exceptUsers?: string[];
  /** 条件函数（返回 true 表示允许） */
  condition?: (context: PermissionContext) => boolean;
}

/** 权限上下文 */
export interface PermissionContext {
  /** 对象标识 */
  objectKey?: string;
  /** 当前用户 */
  user?: {
    id: string;
    name: string;
    roles: UserRole[];
    email?: string;
  };
  /** 配置数据 */
  config?: {
    isPublished?: boolean;
    status?: string;
  };
}

/** 访问控制结果 */
export interface AccessControlResult {
  /** 是否允许 */
  allowed: boolean;
  /** 拒绝原因 */
  reason?: string;
  /** 所需权限 */
  requiredPermission?: Permission;
}

/** 默认权限配置 */
const DEFAULT_PERMISSIONS: Record<Permission, PermissionRule> = {
  'workspace:read': {
    roles: ['admin', 'developer', 'viewer', 'custom'],
  },
  'workspace:edit': {
    roles: ['admin', 'developer'],
  },
  'workspace:publish': {
    roles: ['admin'],
  },
  'workspace:delete': {
    roles: ['admin'],
  },
  'workspace:rollback': {
    roles: ['admin', 'developer'],
  },
  'tab:edit': {
    roles: ['admin', 'developer'],
  },
  'tab:delete': {
    roles: ['admin', 'developer'],
  },
  'sensitive:edit': {
    roles: ['admin'],
  },
  'template:use': {
    roles: ['admin', 'developer', 'viewer', 'custom'],
  },
  'audit:read': {
    roles: ['admin', 'developer'],
  },
};

/** 敏感配置列表 */
const SENSITIVE_CONFIGS = new Set([
  'payment', // 支付相关
  'security', // 安全相关
  'auth', // 认证相关
  'admin', // 管理员功能
  'system', // 系统配置
]);

/** 本地存储 key */
const STORAGE_KEY = 'workspace:permissions';

/**
 * 加载自定义权限配置
 */
function loadCustomPermissions(): Record<Permission, PermissionRule[]> {
  try {
    const data = localStorage.getItem(STORAGE_KEY);
    return data ? JSON.parse(data) : {};
  } catch {
    return {};
  }
}

/**
 * 保存自定义权限配置
 */
function saveCustomPermissions(permissions: Record<Permission, PermissionRule[]>) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(permissions));
  } catch (error) {
    console.error('Failed to save permissions:', error);
  }
}

/**
 * 检查用户是否有指定权限
 */
export function hasPermission(
  permission: Permission,
  context: PermissionContext,
): AccessControlResult {
  const result: AccessControlResult = {
    allowed: false,
    requiredPermission: permission,
  };

  // 获取默认权限规则
  const defaultRule = DEFAULT_PERMISSIONS[permission];
  if (!defaultRule) {
    result.reason = '未知权限项';
    return result;
  }

  // 加载自定义权限规则
  const customRules = loadCustomPermissions();
  const rules = [...(customRules[permission] || []), defaultRule];

  // 检查每条规则
  for (const rule of rules) {
    // 检查排除用户
    if (rule.exceptUsers && context.user?.id && rule.exceptUsers.includes(context.user.id)) {
      continue;
    }

    // 检查角色
    if (rule.roles && rule.roles.length > 0) {
      const hasRole = context.user?.roles?.some((r) => rule.roles!.includes(r));
      if (hasRole) {
        // 检查条件
        if (rule.condition && !rule.condition(context)) {
          continue;
        }
        result.allowed = true;
        return result;
      }
    }

    // 检查用户白名单
    if (rule.users && context.user?.id && rule.users.includes(context.user.id)) {
      if (rule.condition && !rule.condition(context)) {
        continue;
      }
      result.allowed = true;
      return result;
    }
  }

  result.reason = '权限不足';
  return result;
}

/**
 * 批量检查权限
 */
export function checkPermissions(
  permissions: Permission[],
  context: PermissionContext,
): Record<Permission, AccessControlResult> {
  const results: Record<Permission, AccessControlResult> = {} as any;
  permissions.forEach((p) => {
    results[p] = hasPermission(p, context);
  });
  return results;
}

/**
 * 检查是否可以编辑敏感配置
 */
export function canEditSensitiveConfig(context: PermissionContext): AccessControlResult {
  // 检查对象是否敏感
  const isSensitive = SENSITIVE_CONFIGS.has(context.objectKey || '');

  if (!isSensitive) {
    return { allowed: true };
  }

  return hasPermission('sensitive:edit', context);
}

/**
 * 检查是否为只读模式
 */
export function isReadOnlyMode(context: PermissionContext): boolean {
  const editResult = hasPermission('workspace:edit', context);
  return !editResult.allowed;
}

/**
 * 添加自定义权限规则
 */
export function addPermissionRule(permission: Permission, rule: PermissionRule): void {
  const customRules = loadCustomPermissions();
  if (!customRules[permission]) {
    customRules[permission] = [];
  }
  customRules[permission].push(rule);
  saveCustomPermissions(customRules);
}

/**
 * 移除自定义权限规则
 */
export function removePermissionRule(permission: Permission, ruleIndex: number): void {
  const customRules = loadCustomPermissions();
  if (customRules[permission]) {
    customRules[permission].splice(ruleIndex, 1);
    if (customRules[permission].length === 0) {
      delete customRules[permission];
    }
    saveCustomPermissions(customRules);
  }
}

/**
 * 获取所有权限规则
 */
export function getAllPermissionRules(): Record<Permission, PermissionRule[]> {
  const customRules = loadCustomPermissions();
  const result: Record<Permission, PermissionRule[]> = {} as any;

  Object.keys(DEFAULT_PERMISSIONS).forEach((permission) => {
    const p = permission as Permission;
    result[p] = [...(customRules[p] || []), DEFAULT_PERMISSIONS[p]];
  });

  return result;
}

/**
 * 获取权限文本说明
 */
export function getPermissionText(permission: Permission): string {
  const texts: Record<Permission, string> = {
    'workspace:read': '读取工作空间',
    'workspace:edit': '编辑工作空间',
    'workspace:publish': '发布工作空间',
    'workspace:delete': '删除工作空间',
    'workspace:rollback': '回滚版本',
    'tab:edit': '编辑标签页',
    'tab:delete': '删除标签页',
    'sensitive:edit': '编辑敏感配置',
    'template:use': '使用模板',
    'audit:read': '查看审计日志',
  };
  return texts[permission] || permission;
}

/**
 * 获取角色文本说明
 */
export function getRoleText(role: UserRole): string {
  const texts: Record<UserRole, string> = {
    admin: '管理员',
    developer: '开发者',
    viewer: '查看者',
    custom: '自定义',
  };
  return texts[role] || role;
}

/**
 * 获取角色颜色
 */
export function getRoleColor(role: UserRole): string {
  const colors: Record<UserRole, string> = {
    admin: 'red',
    developer: 'blue',
    viewer: 'green',
    custom: 'default',
  };
  return colors[role] || 'default';
}

/**
 * 模拟当前用户（实际应从认证系统获取）
 */
export function getCurrentUser(): PermissionContext['user'] | null {
  try {
    const userStr = localStorage.getItem('workspace:current_user');
    return userStr
      ? JSON.parse(userStr)
      : {
          id: 'user_default',
          name: '默认用户',
          roles: ['developer'],
        };
  } catch {
    return {
      id: 'user_default',
      name: '默认用户',
      roles: ['developer'],
    };
  }
}

/**
 * 设置当前用户（用于模拟）
 */
export function setCurrentUser(user: PermissionContext['user']): void {
  try {
    localStorage.setItem('workspace:current_user', JSON.stringify(user));
  } catch (error) {
    console.error('Failed to save current user:', error);
  }
}

/**
 * 检查 Tab 是否可编辑
 */
export function canEditTab(context: PermissionContext): boolean {
  const editResult = hasPermission('tab:edit', context);
  return editResult.allowed;
}

/**
 * 检查 Tab 是否可删除
 */
export function canDeleteTab(context: PermissionContext): boolean {
  const deleteResult = hasPermission('tab:delete', context);
  return deleteResult.allowed;
}

/**
 * 创建只读上下文
 */
export function createReadOnlyContext(objectKey: string): PermissionContext {
  return {
    objectKey,
    user: getCurrentUser(),
  };
}
