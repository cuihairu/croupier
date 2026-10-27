/** C1：游戏运营模板玩家工作台 CRUD fixture 级端到端验收。
 *
 * 不改 PageRenderer 主链路，直接用 gameOpsTemplates 注册表的 player-manage
 * 条目（PageSpec 常量）驱动 ResourcePageRenderer，配上独立 gameOpsDemo
 * 内存 fixture，走真实渲染交互链：
 * 列表查询 → 详情 → 新建 → 编辑 → 删除确认 → 成功刷新。
 *
 * 另含缺能力显式提示断言：玩家模板 CRUD 角色齐备，其余模板缺口由
 * missingTemplateCrudRoles 显式暴露（模板列表与采纳前可查），不以
 * 「树里没有节点」代替可用流程判定。
 *
 * @module components/PageRenderer/templates/gameOpsCrudFixture
 */

import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { App } from 'antd';
import ResourcePageRenderer from '@/components/PageRenderer/ResourcePageRenderer';
import {
  gameOpsTemplateCrudRoles,
  gameOpsTemplates,
  missingTemplateCrudRoles,
} from './gameOpsTemplates';
import { createPlayerManageDemoExecute } from './playerManage';
import { createGameOpsDemoStore } from './gameOpsDemoData';
import type { PageExecuteFn } from '@/types/dashboard';

jest.mock('@/components/PageRenderer', () => ({
  __esModule: true,
  default: () => null,
}));

jest.mock('@/components/SchemaFormRenderer', () => {
  const formMockState = { validate: true, values: {} as Record<string, unknown> };
  const Stub = React.forwardRef<unknown, Record<string, unknown>>((_props, ref) => {
    React.useImperativeHandle(ref, () => ({
      validate: () => formMockState.validate,
      getValues: () => formMockState.values,
    }));
    return <div data-testid="schema-form-stub" />;
  });
  Stub.displayName = 'SchemaFormRendererStub';
  return { __esModule: true, default: Stub, formMockState };
});

jest.mock('@ant-design/pro-components', () => {
  const ProTableStub = ({
    request,
    columns,
    rowKey,
    actionRef,
    toolBarRender,
    headerTitle,
    pagination,
  }: {
    request?: (params: Record<string, unknown>) => Promise<{ data: unknown[]; total: number }>;
    columns?: Array<{
      key?: string;
      dataIndex?: string;
      render?: (dom: unknown, record: Record<string, unknown>) => React.ReactNode;
    }>;
    rowKey?: (record: Record<string, unknown>) => string;
    actionRef?: React.MutableRefObject<{ reload?: () => void } | null>;
    toolBarRender?: () => React.ReactNode[];
    headerTitle?: React.ReactNode;
    pagination?:
      false | { showTotal?: (total: number) => React.ReactNode; defaultPageSize?: number };
  }) => {
    const [rows, setRows] = React.useState<Record<string, unknown>[]>([]);
    const reload = React.useCallback(() => {
      void request?.({}).then((r) => setRows(r.data as Record<string, unknown>[]));
    }, [request]);
    React.useEffect(() => {
      void reload();
    }, [reload]);
    React.useEffect(() => {
      if (actionRef) actionRef.current = { reload };
    }, [actionRef, reload]);
    return (
      <div>
        <div data-testid="protable-title">{headerTitle}</div>
        <div>{toolBarRender?.()}</div>
        <div data-testid="rows-loaded" data-count={String(rows.length)} />
        {pagination ? (
          <div data-testid="pagination-bar">{pagination.showTotal?.(rows.length)}</div>
        ) : null}
        <table>
          <tbody>
            {rows.map((row) => (
              <tr key={rowKey?.(row) ?? 'row'}>
                {columns?.map((column) => (
                  <td key={column.key ?? ''}>
                    {column.render
                      ? column.render(null, row)
                      : String(row[column.dataIndex ?? ''] ?? '')}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  };
  const ProDescriptionsStub = ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="prodescriptions">{children}</div>
  );
  (
    ProDescriptionsStub as unknown as {
      Item: React.FC<{ label?: React.ReactNode; children?: React.ReactNode }>;
    }
  ).Item = ({ label, children }) => (
    <div data-testid="prodescriptions-item">
      <span>{label}</span>
      <span>{children}</span>
    </div>
  );
  const PageContainerStub = ({ children }: { children?: React.ReactNode }) => <div>{children}</div>;
  return {
    ProTable: jest.fn(ProTableStub),
    ProDescriptions: ProDescriptionsStub,
    PageContainer: PageContainerStub,
  };
});

const { formMockState } = jest.requireMock('@/components/SchemaFormRenderer') as {
  formMockState: { validate: boolean; values: Record<string, unknown> };
};

const playerEntry = gameOpsTemplates.find((entry) => entry.key === 'player-manage');

describe('C1 gameOps 玩家模板 CRUD fixture 端到端', () => {
  beforeEach(() => {
    formMockState.validate = true;
    formMockState.values = {};
  });

  it('列表查询 → 详情 → 新建/编辑 → 删除确认 → 成功刷新', async () => {
    expect(playerEntry).toBeDefined();
    const entry = playerEntry!;
    const resourceSpec = entry.pageSpec.resource;
    expect(resourceSpec).toBeDefined();

    const store = createGameOpsDemoStore();
    const onExecute = jest.fn(createPlayerManageDemoExecute(store));
    render(
      <App>
        <ResourcePageRenderer
          spec={resourceSpec!}
          bindings={entry.pageSpec.bindings}
          onExecute={onExecute as unknown as PageExecuteFn}
        />
      </App>,
    );

    // 1. 列表查询：默认分页第一页，fixture 首行可见
    await waitFor(() => expect(screen.getByText('玩家1号')).toBeInTheDocument());
    expect(screen.getByTestId('rows-loaded')).toHaveAttribute('data-count', '10');
    expect(onExecute).toHaveBeenCalledWith('list', { form: {} });

    // 2. 详情：click 查看 → drawer 拉到同一 fixture 行
    fireEvent.click(screen.getAllByRole('button', { name: /查\s*看/ })[0]);
    await waitFor(() => expect(screen.getByText('详情')).toBeInTheDocument());
    await waitFor(() => expect(screen.getAllByText('玩家1号').length).toBeGreaterThanOrEqual(2));
    expect(onExecute.mock.calls.filter(([id]) => id === 'detail')).toHaveLength(1);
    expect(onExecute).toHaveBeenCalledWith('detail', {
      row: expect.objectContaining({ id: '20001' }),
    });

    fireEvent.click(screen.getByRole('button', { name: 'Close' }));

    // 3. 新建：工具栏入口 → 表单提交 → 成功 → 表格刷新出现新行
    fireEvent.click(screen.getByRole('button', { name: /新\s*建/ }));
    await waitFor(() => expect(screen.getByTestId('schema-form-stub')).toBeInTheDocument());
    formMockState.values = { nickname: 'fixture-new', level: 7, vip: 2 };
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(screen.getByText('创建成功')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('create', {
      form: { nickname: 'fixture-new', level: 7, vip: 2 },
    });
    await waitFor(() => expect(screen.getByText('fixture-new')).toBeInTheDocument());

    // 4. 编辑：行操作抽屉表单提交 → 成功 → 表格刷新显示新值
    fireEvent.click(screen.getAllByRole('button', { name: /编\s*辑/ })[0]);
    await waitFor(() => expect(screen.getByTestId('schema-form-stub')).toBeInTheDocument());
    formMockState.values = { nickname: 'fixture-edited', level: 8, vip: 3 };
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(screen.getByText('更新成功')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('update', {
      form: { nickname: 'fixture-edited', level: 8, vip: 3 },
      row: expect.objectContaining({ id: '20027', nickname: 'fixture-new' }),
    });
    await waitFor(() => expect(screen.getByText('fixture-edited')).toBeInTheDocument());

    // 5. 删除：显式确认文案 → 确认 → 成功 → 刷新后新行消失、原首行回显
    expect(resourceSpec?.deleteAction).toMatchObject({ bindingId: 'delete', risk: 'high' });
    fireEvent.click(screen.getAllByRole('button', { name: /删\s*除/ })[0]);
    await waitFor(() => expect(screen.getByText('删除玩家')).toBeInTheDocument());
    expect(screen.getByText('删除后不可恢复，确认删除该玩家？')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /确认删除/ }));
    await waitFor(() => expect(screen.getByText('删除成功')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('delete', {
      row: expect.objectContaining({ id: '20027', nickname: 'fixture-edited' }),
    });
    await waitFor(() => expect(screen.queryByText('fixture-edited')).not.toBeInTheDocument());
    await waitFor(() => expect(screen.getByText('玩家1号')).toBeInTheDocument());

    // 6. 成功刷新：初始 + 新建/编辑/删除三次 reload，列表被真实重查四次
    expect(onExecute.mock.calls.filter(([id]) => id === 'list')).toHaveLength(4);
  }, 30000);

  it('缺 CRUD 角色显式提示：玩家齐备，其余模板缺口可查', () => {
    expect(gameOpsTemplateCrudRoles).toEqual(['create', 'update', 'delete']);
    expect(gameOpsTemplates.map((entry) => missingTemplateCrudRoles(entry))).toEqual([
      [],
      ['delete'],
      ['update', 'delete'],
      ['create', 'update', 'delete'],
      ['update', 'delete'],
    ]);
    for (const entry of gameOpsTemplates) {
      expect(entry.capabilitiesNote['zh-CN'].length).toBeGreaterThan(0);
      expect(entry.capabilitiesNote['en-US'].length).toBeGreaterThan(0);
    }
  });
});
