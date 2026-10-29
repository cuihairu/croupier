/** banManagement 模板覆盖：PageSpec 结构 + demo 执行器主要分支。 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import BanManagementTemplate, {
  createBanManagementDemoExecute,
  banManagementPageSpec,
} from './banManagement';
import type { BindingExecutionContext, JSONValue } from '@/types/dashboard';

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ title, children }: { title?: React.ReactNode; children?: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {children}
    </div>
  ),
}));
jest.mock('@/components/PageRenderer', () => ({
  __esModule: true,
  default: () => <div data-testid="page-renderer" />,
}));

const run = (
  execute: ReturnType<typeof createBanManagementDemoExecute>,
  bindingId: string,
  context: BindingExecutionContext = {},
) => execute(bindingId, context) as Promise<{ kind: string; data: JSONValue }>;

describe('banManagement PageSpec', () => {
  it('归属 ban 分类，行/批量操作与创建绑定齐备', () => {
    expect(banManagementPageSpec.category.key).toBe('ban');
    if (banManagementPageSpec.type !== 'resource') return;
    const actions = [
      ...(banManagementPageSpec.resource.listView?.rowActions ?? []),
      ...(banManagementPageSpec.resource.listView?.batchActions ?? []),
    ];
    expect(actions.length).toBeGreaterThanOrEqual(3);
    for (const action of actions) {
      expect(banManagementPageSpec.bindings.some((b) => b.id === action.bindingId)).toBe(true);
    }
    expect(banManagementPageSpec.bindings.some((b) => b.id === 'create')).toBe(true);
  });
});

describe('banManagement demo 执行器', () => {
  it('list：分页/昵称/状态/原因筛选', async () => {
    const execute = createBanManagementDemoExecute();
    const page1 = await run(execute, 'list', { form: { current: 2, pageSize: 5 } });
    expect(page1.data).toMatchObject({ total: 12 });
    // D5 身份列前置查询：记录 ID 与玩家 ID 均精确命中一条
    const byId = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, id: 'ban-30001' },
    });
    expect(byId.data).toMatchObject({ total: 1 });
    const byPlayerId = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, playerId: '20004' },
    });
    expect(byPlayerId.data).toMatchObject({ total: 1 });
    const byStatus = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, status: 'lifted' },
    });
    expect(byStatus.data).toMatchObject({ total: 2 });
    const byReason = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, reason: 'cheating' },
    });
    expect(byReason.data).toMatchObject({ total: 3 });
    const byServer = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, serverId: 's2' },
    });
    expect(byServer.data).toMatchObject({ total: 8 });
    const serverRows = (byServer.data as { items: Array<{ serverId: string }> }).items;
    for (const row of serverRows) {
      expect(row.serverId).toBe('s2');
    }
  });

  it('create：缺玩家 ID/非正时长报错，合法创建置顶', async () => {
    const execute = createBanManagementDemoExecute();
    await expect(
      run(execute, 'create', { form: { playerId: '', durationHours: 24 } }),
    ).rejects.toThrow('player_id_required');
    await expect(
      run(execute, 'create', { form: { playerId: '200001', durationHours: 0 } }),
    ).rejects.toThrow('duration_must_be_positive');
    const created = await run(execute, 'create', {
      form: { playerId: '200001', reason: 'fraud', durationHours: 48, serverId: 's2' },
    });
    const list = await run(execute, 'list', { form: { current: 1, pageSize: 1 } });
    expect((list.data as { items: Array<{ id: string }> }).items[0].id).toBe(
      (created.data as { id: string }).id,
    );
  });

  it('unban：非生效中报错，生效中解封成功', async () => {
    const execute = createBanManagementDemoExecute();
    const list = await run(execute, 'list', {
      form: { current: 1, pageSize: 20, status: 'lifted' },
    });
    const liftedId = (list.data as { items: Array<{ id: string }> }).items[0].id;
    await expect(run(execute, 'action.unban', { row: { id: liftedId } })).rejects.toThrow(
      'ban_not_active',
    );
    const active = await run(execute, 'list', {
      form: { current: 1, pageSize: 1, status: 'active' },
    });
    const activeId = (active.data as { items: Array<{ id: string }> }).items[0].id;
    const result = await run(execute, 'action.unban', {
      row: { id: activeId },
      form: { reason: '申诉通过' },
    });
    expect(result.data).toMatchObject({ success: true });
    await expect(run(execute, 'action.unban', { row: { id: 'nope' } })).rejects.toThrow(
      'ban_not_found',
    );
  });

  it('extend：非正时长报错，成功后到期时间后移', async () => {
    const execute = createBanManagementDemoExecute();
    const active = await run(execute, 'list', {
      form: { current: 1, pageSize: 1, status: 'active' },
    });
    const target = (active.data as { items: Array<{ id: string; expiresAt: string }> }).items[0];
    await expect(
      run(execute, 'action.extend', { row: { id: target.id }, form: { extraHours: 0 } }),
    ).rejects.toThrow('duration_must_be_positive');
    await run(execute, 'action.extend', { row: { id: target.id }, form: { extraHours: 24 } });
    const detail = await run(execute, 'detail', { row: { id: target.id } });
    expect(new Date((detail.data as { expiresAt: string }).expiresAt).getTime()).toBeGreaterThan(
      new Date(target.expiresAt).getTime(),
    );
  });

  it('batchUnban：仅生效中记录被解封，全不生效报错', async () => {
    const execute = createBanManagementDemoExecute();
    const list = await run(execute, 'list', { form: { current: 1, pageSize: 20 } });
    const rows = (list.data as { items: Array<{ id: string; status: string }> }).items;
    const liftedRows = rows.filter((row) => row.status === 'lifted');
    await expect(run(execute, 'action.batchUnban', { selection: liftedRows })).rejects.toThrow(
      'no_active_ban_selected',
    );
    const activeRows = rows.filter((row) => row.status === 'active').slice(0, 2);
    const result = await run(execute, 'action.batchUnban', { selection: activeRows });
    expect(result.data).toMatchObject({ lifted: 2 });
  });

  it('未知 binding 走默认成功分支', async () => {
    const execute = createBanManagementDemoExecute();
    const result = await run(execute, 'unknown');
    expect(result.data).toMatchObject({ success: true });
  });
});

describe('BanManagementTemplate 组件', () => {
  it('挂载渲染标题与 PageRenderer', () => {
    render(<BanManagementTemplate />);
    expect(screen.getByRole('heading', { name: '封禁管理' })).toBeInTheDocument();
    expect(screen.getByTestId('page-renderer')).toBeInTheDocument();
  });
});
