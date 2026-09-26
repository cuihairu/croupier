/**
 * 权限树回归（docs/BUGS.md BUG-020）。
 *
 * 修复前 `PermissionsTab` 是一层平铺的 `SimpleList`：只有「已授权」数据、
 * 没有资源/操作分层、没有已授权/未授权的两态区分。且后端
 * `/profile/permissions` 的 `permissions[]` 是 `resource:"role"` + actions 塞
 * 角色名的编造数据（BUG-019），`permissionIDs` 里还混着角色名。
 *
 * 覆盖：
 *   - 纯逻辑：id 拆分（与后端 rbac 同语义）、通配判定、树的分组与两态；
 *   - 渲染：三层层级、绿/灰 + 勾/叉、展开收起、无角色占位、通配提示。
 */
import React from 'react';
import { render, screen, within } from '@testing-library/react';
import { App } from 'antd';
import { fireEvent } from '@testing-library/react';
import PermissionTreeView, { DENIED_COLOR, GRANTED_COLOR } from '../PermissionTreeView';
import {
  buildPermissionTree,
  hasGlobalWildcard,
  roleGrants,
  splitPermissionId,
  type PermissionCatalogEntry,
  type RoleGrant,
} from '../permissionTree';

// 与 configs/permissions.json 形态一致的目录（resource:action）
const CATALOG: PermissionCatalogEntry[] = [
  { id: 'user:read', name: '用户查看', description: '查看用户信息', category: 'user' },
  { id: 'user:write', name: '用户管理', description: '增删改用户', category: 'user' },
  { id: 'pages:read', name: '页面查看', category: 'page' },
  { id: 'pages:write', name: '页面管理', category: 'page' },
  { id: 'log:read', name: '日志查看', category: 'log' },
  { id: 'admin:all', name: '全部管理权限', category: 'admin' },
  { id: '*', name: '通配符权限', category: 'admin' },
];

function renderTree(roles: RoleGrant[], fullAccess = false) {
  return render(
    <App>
      <PermissionTreeView roles={roles} catalog={CATALOG} fullAccess={fullAccess} />
    </App>,
  );
}

describe('splitPermissionId（与后端 rbac 同语义）', () => {
  it('标准 resource:action', () => {
    expect(splitPermissionId('user:read')).toEqual({ resource: 'user', action: 'read' });
  });

  it('大小写与空白归一', () => {
    expect(splitPermissionId('  USER:READ ')).toEqual({ resource: 'user', action: 'read' });
  });

  it('通配 id 一律归到 resource=*', () => {
    expect(splitPermissionId('*')).toEqual({ resource: '*', action: '*' });
    expect(splitPermissionId('admin:all')).toEqual({ resource: '*', action: '*' });
    expect(splitPermissionId('')).toEqual({ resource: '*', action: '*' });
  });

  it('无冒号：整串即资源，不按 . 猜切分', () => {
    expect(splitPermissionId('legacy.flag')).toEqual({ resource: 'legacy.flag', action: '*' });
  });

  it('缺段归一为 *', () => {
    expect(splitPermissionId('user:')).toEqual({ resource: 'user', action: '*' });
    expect(splitPermissionId(':read')).toEqual({ resource: '*', action: 'read' });
  });
});

describe('通配判定', () => {
  it('账号级通配只有 * 与 admin:all', () => {
    expect(hasGlobalWildcard(['user:read', '*'])).toBe(true);
    expect(hasGlobalWildcard(['admin:all'])).toBe(true);
    expect(hasGlobalWildcard(['user:*'])).toBe(false);
    expect(hasGlobalWildcard(['user:all'])).toBe(false);
    expect(hasGlobalWildcard(['user:read'])).toBe(false);
  });

  it('roleGrants 覆盖资源级通配', () => {
    expect(roleGrants(['user:*'], 'user', 'write')).toBe(true);
    expect(roleGrants(['user:all'], 'user', 'write')).toBe(true);
    expect(roleGrants(['user:*'], 'log', 'read')).toBe(false);
  });
});

describe('buildPermissionTree', () => {
  it('三层结构：角色 → 资源 → 操作', () => {
    const tree = buildPermissionTree(
      [{ role: 'viewer', permissionIds: ['user:read', 'pages:read'] }],
      CATALOG,
    );
    expect(tree.roles).toHaveLength(1);
    const role = tree.roles[0];
    expect(role.role).toBe('viewer');
    expect(role.resources.map((r) => r.resource)).toEqual(['log', 'pages', 'user']);
    // 资源下同时含已授权与未授权操作
    const user = role.resources.find((r) => r.resource === 'user')!;
    expect(user.actions.map((a) => [a.action, a.granted])).toEqual([
      ['read', true],
      ['write', false],
    ]);
  });

  it('目录提供未授权项（这是修复前的核心缺口）', () => {
    // 只授予 1 条，但目录有 5 条非通配权限 → 必须全部出现在树上
    const tree = buildPermissionTree([{ role: 'r', permissionIds: ['user:read'] }], CATALOG);
    const all = tree.roles[0].resources.flatMap((r) => r.actions);
    expect(all).toHaveLength(5);
    expect(all.filter((a) => a.granted)).toHaveLength(1);
  });

  it('目录里的通配条目不生成假的 "*" 资源节点', () => {
    const tree = buildPermissionTree([{ role: 'r', permissionIds: [] }], CATALOG);
    expect(tree.roles[0].resources.map((r) => r.resource)).not.toContain('*');
  });

  it('空角色仍然出现在树上并标为无权限', () => {
    const tree = buildPermissionTree([{ role: 'guest', permissionIds: [] }], CATALOG);
    expect(tree.roles).toHaveLength(1);
    expect(tree.roles[0].empty).toBe(true);
    expect(tree.roles[0].grantedCount).toBe(0);
  });

  it('角色级通配：整棵树全绿，且不新增 * 节点', () => {
    const tree = buildPermissionTree([{ role: 'boss', permissionIds: ['*'] }], CATALOG);
    const all = tree.roles[0].resources.flatMap((r) => r.actions);
    expect(all.every((a) => a.granted)).toBe(true);
    expect(tree.roles[0].resources.map((r) => r.resource)).not.toContain('*');
  });

  it('资源级通配 user:* 覆盖该资源全部操作，但不影响别的资源', () => {
    const tree = buildPermissionTree([{ role: 'ru', permissionIds: ['user:*'] }], CATALOG);
    const role = tree.roles[0];
    const user = role.resources.find((r) => r.resource === 'user')!;
    expect(user.actions.every((a) => a.granted)).toBe(true);
    // 通配操作本身也要能被看见
    expect(user.actions.some((a) => a.action === '*' && a.granted)).toBe(true);
    const log = role.resources.find((r) => r.resource === 'log')!;
    expect(log.actions.some((a) => a.granted)).toBe(false);
  });

  it('多角色各自独立判定', () => {
    const tree = buildPermissionTree(
      [
        { role: 'reader', permissionIds: ['user:read'] },
        { role: 'writer', permissionIds: ['user:write'] },
      ],
      CATALOG,
    );
    const byRole = Object.fromEntries(tree.roles.map((r) => [r.role, r]));
    const pick = (role: string, res: string, act: string) =>
      byRole[role].resources.find((r) => r.resource === res)!.actions.find((a) => a.action === act)!;
    expect(pick('reader', 'user', 'read').granted).toBe(true);
    expect(pick('reader', 'user', 'write').granted).toBe(false);
    expect(pick('writer', 'user', 'write').granted).toBe(true);
    expect(pick('writer', 'user', 'read').granted).toBe(false);
  });

  it('目录为空时退化为「仅已授权」，不崩也不伪造未授权项', () => {
    const tree = buildPermissionTree([{ role: 'r', permissionIds: ['user:read'] }], []);
    const all = tree.roles[0].resources.flatMap((r) => r.actions);
    expect(all).toHaveLength(1);
    expect(all[0].granted).toBe(true);
  });

  it('held-but-not-catalog 资源也会出现（目录落后于实际授权）', () => {
    const tree = buildPermissionTree([{ role: 'r', permissionIds: ['legacy:flag'] }], CATALOG);
    const res = tree.roles[0].resources.find((r) => r.resource === 'legacy')!;
    expect(res).toBeDefined();
    expect(res.actions[0].id).toBe('legacy:flag');
    expect(res.actions[0].granted).toBe(true);
  });
});

describe('PermissionTreeView 渲染', () => {
  it('展示角色/资源/操作三层，绿=已授权 灰=未授权，带勾/叉', () => {
    renderTree([{ role: 'viewer', permissionIds: ['user:read'] }]);
    expect(screen.getByTestId('role-viewer')).toBeInTheDocument();
    expect(screen.getByTestId('resource-user')).toBeInTheDocument();

    const granted = screen.getByTestId('action-user:read');
    expect(granted).toHaveAttribute('data-granted', 'true');
    expect(granted).toHaveTextContent('用户查看');
    // 图标用 antd 的 CheckOutlined / CloseOutlined（SVG），按类名断言：
    // 颜色之外必须有第二重区分（色觉障碍），所以图标本身就是契约的一部分
    expect(within(granted).getByTestId('action-icon-user:read').querySelector('.anticon-check')).not.toBeNull();
    expect(granted.querySelector('.anticon-close')).toBeNull();
    // 绿色
    expect(within(granted).getByTestId('action-icon-user:read')).toHaveStyle({ color: GRANTED_COLOR });

    const denied = screen.getByTestId('action-user:write');
    expect(denied).toHaveAttribute('data-granted', 'false');
    expect(denied).toHaveTextContent('用户管理');
    expect(within(denied).getByTestId('action-icon-user:write').querySelector('.anticon-close')).not.toBeNull();
    expect(denied.querySelector('.anticon-check')).toBeNull();
    // 灰色，且与绿色不同
    expect(within(denied).getByTestId('action-icon-user:write')).toHaveStyle({ color: DENIED_COLOR });
    expect(DENIED_COLOR).not.toBe(GRANTED_COLOR);
  });

  it('图例同时给出「已授权 / 未授权」两种态', () => {
    renderTree([{ role: 'viewer', permissionIds: ['user:read'] }]);
    expect(screen.getByTestId('legend-已授权')).toBeInTheDocument();
    expect(screen.getByTestId('legend-未授权')).toBeInTheDocument();
  });

  it('汇总数与图例一致', () => {
    renderTree([{ role: 'viewer', permissionIds: ['user:read', 'pages:read'] }]);
    expect(screen.getByTestId('perm-tree-summary')).toHaveTextContent('已授权 2/5 项操作');
  });

  it('默认展开全部，可一键收起为仅角色层、再展开', () => {
    renderTree([{ role: 'viewer', permissionIds: ['user:read'] }]);
    // 默认展开：操作叶子可见
    expect(screen.getByTestId('action-user:read')).toBeInTheDocument();
    // 点一次 → 只留角色层，操作叶子消失
    fireEvent.click(screen.getByTestId('perm-tree-toggle-all'));
    expect(screen.queryByTestId('action-user:read')).toBeNull();
    // 再点一次 → 全展开
    fireEvent.click(screen.getByTestId('perm-tree-toggle-all'));
    expect(screen.getByTestId('action-user:read')).toBeInTheDocument();
  });

  it('无角色时显示占位而不是空树', () => {
    renderTree([]);
    expect(screen.getByText('当前账号没有任何角色')).toBeInTheDocument();
    expect(screen.queryByTestId('perm-tree')).toBeNull();
  });

  it('持通配权限时给出显式说明（灰色不等于不可用）', () => {
    renderTree([{ role: 'boss', permissionIds: ['*'] }], true);
    expect(screen.getByTestId('perm-tree-full-access')).toBeInTheDocument();
    expect(screen.getByTestId('perm-tree-full-access-hint')).toHaveTextContent('通配权限');
    expect(screen.getByTestId('action-user:read')).toHaveAttribute('data-granted', 'true');
  });

  it('无显式权限的角色被标注出来', () => {
    renderTree([{ role: 'guest', permissionIds: [] }]);
    expect(screen.getByTestId('role-guest-empty')).toBeInTheDocument();
  });
});
