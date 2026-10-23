/** activityConfig 模板覆盖：PageSpec 结构 + demo 执行器主要分支。 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import ActivityConfigTemplate, {
  createActivityConfigDemoExecute,
  activityConfigPageSpec,
} from './activityConfig';
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
  execute: ReturnType<typeof createActivityConfigDemoExecute>,
  bindingId: string,
  context: BindingExecutionContext = {},
) => execute(bindingId, context) as Promise<{ kind: string; data: JSONValue }>;

describe('activityConfig PageSpec', () => {
  it('归属 operation 分类，行操作（发布/上线/下线/回滚）绑定齐备', () => {
    expect(activityConfigPageSpec.category.key).toBe('operation');
    if (activityConfigPageSpec.type !== 'resource') return;
    const rowKeys = (activityConfigPageSpec.resource.listView?.rowActions ?? []).map(
      (action) => action.key,
    );
    expect(rowKeys).toEqual(['publish', 'activate', 'end', 'rollback']);
    for (const action of activityConfigPageSpec.resource.listView?.rowActions ?? []) {
      expect(activityConfigPageSpec.bindings.some((b) => b.id === action.bindingId)).toBe(true);
    }
  });
});

describe('activityConfig demo 执行器', () => {
  it('list：分页/名称/状态/类型筛选', async () => {
    const execute = createActivityConfigDemoExecute();
    const page1 = await run(execute, 'list', { form: { current: 1, pageSize: 5 } });
    expect(page1.data).toMatchObject({ total: 8 });
    const active = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, status: 'active' },
    });
    expect(active.data).toMatchObject({ total: 2 });
    const empty = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, name: '不存在' },
    });
    expect(empty.data).toMatchObject({ items: [], total: 0 });
  });

  it('detail：版本历史倒序摘要，未命中返回 null', async () => {
    const execute = createActivityConfigDemoExecute();
    const detail = await run(execute, 'detail', { row: { id: 'act-4002' } });
    const history = (detail.data as { versionHistory: string }).versionHistory;
    expect(history).toContain('v2');
    expect(history.indexOf('v2')).toBeLessThan(history.indexOf('v1'));
    const miss = await run(execute, 'detail', { row: { id: 'nope' } });
    expect(miss.data).toBeNull();
  });

  it('create：缺名称报错，合法创建为已配置零版本', async () => {
    const execute = createActivityConfigDemoExecute();
    await expect(run(execute, 'create', { form: { name: ' ' } })).rejects.toThrow('name_required');
    const created = await run(execute, 'create', {
      form: { name: '新春活动', type: 'holiday_event', timezone: 'UTC' },
    });
    const detail = await run(execute, 'detail', {
      row: { id: (created.data as { id: string }).id },
    });
    expect(detail.data).toMatchObject({ status: 'configured', currentVersion: 0 });
  });

  it('状态机：未发布不可上线、上线后可下线、发布递增版本', async () => {
    const execute = createActivityConfigDemoExecute();
    const list = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, status: 'configured' },
    });
    const target = (list.data as { items: Array<{ id: string }> }).items[0];
    await expect(run(execute, 'action.activate', { row: target })).rejects.toThrow(
      'activate_requires_published',
    );
    const published = await run(execute, 'action.publish', {
      row: target,
      form: { note: '首发' },
    });
    const firstVersion = (published.data as { version: number }).version;
    expect(firstVersion).toBeGreaterThanOrEqual(1);
    const before = await run(execute, 'detail', { row: target });
    const versionBefore = (before.data as { currentVersion: number }).currentVersion;
    expect(versionBefore).toBe(firstVersion);
    const again = await run(execute, 'action.publish', { row: target, form: { note: '调值' } });
    expect(again.data).toMatchObject({ version: versionBefore + 1 });
    await run(execute, 'action.activate', { row: target });
    const afterActivate = await run(execute, 'detail', { row: target });
    expect(afterActivate.data).toMatchObject({ status: 'active' });
    await run(execute, 'action.end', { row: target });
    const afterEnd = await run(execute, 'detail', { row: target });
    expect(afterEnd.data).toMatchObject({ status: 'ended' });
    await expect(run(execute, 'action.end', { row: target })).rejects.toThrow(
      'end_requires_active',
    );
  });

  it('rollback：目标版本不存在报错，存在则置为已回滚并切版本', async () => {
    const execute = createActivityConfigDemoExecute();
    const list = await run(execute, 'list', { form: { current: 1, pageSize: 1 } });
    const target = (list.data as { items: Array<{ id: string }> }).items[0];
    await expect(
      run(execute, 'action.rollback', { row: target, form: { targetVersion: 99 } }),
    ).rejects.toThrow('target_version_not_found');
    const result = await run(execute, 'action.rollback', {
      row: target,
      form: { targetVersion: 1 },
    });
    expect(result.data).toMatchObject({ version: 1 });
    const detail = await run(execute, 'detail', { row: target });
    expect(detail.data).toMatchObject({ status: 'rolled_back', currentVersion: 1 });
    await expect(
      run(execute, 'action.rollback', { row: { id: 'nope' }, form: {} }),
    ).rejects.toThrow('activity_not_found');
  });

  it('未知 binding 走默认成功分支', async () => {
    const execute = createActivityConfigDemoExecute();
    const result = await run(execute, 'unknown');
    expect(result.data).toMatchObject({ success: true });
  });
});

describe('ActivityConfigTemplate 组件', () => {
  it('挂载渲染标题与 PageRenderer', () => {
    render(<ActivityConfigTemplate />);
    expect(screen.getByRole('heading', { name: '活动配置' })).toBeInTheDocument();
    expect(screen.getByTestId('page-renderer')).toBeInTheDocument();
  });
});
