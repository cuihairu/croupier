/**
 * GamesTab 权限区回归（docs/BUGS.md BUG-018）。
 *
 * 修复前后端把 `ProfileGame.Permissions` 硬编码成 `[]string{}`，对所有用户
 * 所有游戏恒为空——admin 的「游戏访问权限」因此永远是一块空白，既看不出是
 * 「全部权限」也看不出是「没有权限」。现在：
 *   - accessLevel=full → 绿色「全部权限」标签；
 *   - accessLevel=none → 灰色「无显式权限」+ 具体说明（不留白）；
 *   - 有真实权限 → 逐条标签。
 */
import React from 'react';
import { render, screen, within } from '@testing-library/react';
import GamesTab from '../GamesTab';
import type { ProfileGame } from '@/services/api/me';

function renderTab(games: ProfileGame[]) {
  return render(<GamesTab games={games} loading={false} />);
}

const base = { gameId: 'demo', gameName: 'Demo', envs: ['production'] };

describe('GamesTab 权限区', () => {
  it('accessLevel=full：显示绿色「全部权限」而不是空白', () => {
    renderTab([
      { ...base, permissions: ['*'], accessLevel: 'full', permissionScope: 'role' },
    ]);
    expect(screen.getByTestId('game-access-full-demo')).toHaveTextContent('全部权限');
    expect(screen.getByTestId('game-perm-demo-*')).toHaveTextContent('全部');
    // 关键：不得再出现空白的权限区
    expect(within(screen.getByTestId('game-permissions-demo')).queryByText('无显式权限')).toBeNull();
  });

  it('accessLevel=none：明确说明「无显式权限」并给出原因，不得留白', () => {
    renderTab([{ ...base, permissions: [], accessLevel: 'none', permissionScope: 'role' }]);
    expect(screen.getByTestId('game-access-none-demo')).toBeInTheDocument();
    expect(screen.getByTestId('game-permissions-demo')).toHaveTextContent(
      '实际可访问范围由管理员分配的游戏/环境决定',
    );
  });

  it('旧的空 permissions 且无 accessLevel：按 none 处理而不是空白', () => {
    // 后端未升级时的兼容路径：不能因为缺字段就渲染出空区域
    renderTab([{ ...base, permissions: [] }]);
    expect(screen.getByTestId('game-access-none-demo')).toBeInTheDocument();
  });

  it('scoped：逐条列出真实权限 id', () => {
    renderTab([
      {
        ...base,
        permissions: ['user:read', 'pages:write'],
        accessLevel: 'scoped',
        permissionScope: 'role',
      },
    ]);
    expect(screen.getByTestId('game-perm-demo-user:read')).toHaveTextContent('user:read');
    expect(screen.getByTestId('game-perm-demo-pages:write')).toHaveTextContent('pages:write');
    expect(screen.queryByTestId('game-access-full-demo')).toBeNull();
    expect(screen.queryByTestId('game-access-none-demo')).toBeNull();
  });

  it('permissionScope=role 时说明授权维度，避免误解为可按游戏单独授权', () => {
    renderTab([{ ...base, permissions: ['*'], accessLevel: 'full', permissionScope: 'role' }]);
    expect(screen.getByTestId('game-permissions-demo')).toHaveTextContent('权限按角色授予');
  });
});
