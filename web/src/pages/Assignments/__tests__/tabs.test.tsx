/** Assignments 三 Tab 展示收口（覆盖率批次：RouteTab 27.5% / CategoryTab 39.13% /
 * ListTab 45.12%——三文件此前零单测）：
 * 1. RouteTab：数据行 + 归属说明 Alert（title/description）渲染；空数据不崩；
 * 2. CategoryTab：资源分组行渲染；空数据不崩；
 * 3. ListTab：分组卡片 + 计数 Tag + 顶部操作条渲染；renderResourceActions
 *    按组调用（操作条 + 卡片 extra）；行选择 onChange 上抛；空分组仅留操作条。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { App } from 'antd';
import type { ProColumns } from '@ant-design/pro-components';
import RouteTab from '../RouteTab';
import CategoryTab from '../CategoryTab';
import ListTab from '../ListTab';
import type { AssignmentGroup, AssignmentItem } from '../types';

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
  getIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

const item = (overrides: Partial<AssignmentItem> = {}): AssignmentItem => {
  // id 与名称刻意不同值：两列（ID/名称）同值会导致 getByText 命中多 cell。
  const { id = 'fn.a', name, ...rest } = overrides;
  return {
    id,
    name: name ?? `${id} 名称`,
    version: '1.0.0',
    resource: 'res.a',
    status: 'active',
    ...rest,
  };
};

const group = (resource: string, ids: string[]): AssignmentGroup => ({
  resource,
  items: ids.map((id) => item({ id, resource })),
  activeCount: ids.length,
});

const itemColumns: ProColumns<AssignmentItem>[] = [
  { title: 'ID', dataIndex: 'id' },
  { title: '名称', dataIndex: 'name' },
];

const groupColumns: ProColumns<AssignmentGroup>[] = [{ title: '资源', dataIndex: 'resource' }];

const renderWithApp = (ui: React.ReactElement) => render(<App>{ui}</App>);

describe('RouteTab', () => {
  it('渲染数据行与归属说明 Alert', async () => {
    renderWithApp(<RouteTab data={[item(), item({ id: 'fn.b' })]} columns={itemColumns} />);
    expect(await screen.findByText('函数能力归属说明')).toBeInTheDocument();
    expect(
      screen.getByText(/这里只展示已分配函数的 resource\/operation 能力归属/),
    ).toBeInTheDocument();
    expect(await screen.findByText('fn.b')).toBeInTheDocument();
    expect(screen.getByText('fn.b 名称')).toBeInTheDocument();
  });

  it('空数据：仅提示条、无崩溃', async () => {
    renderWithApp(<RouteTab data={[]} columns={itemColumns} />);
    expect(await screen.findByText('函数能力归属说明')).toBeInTheDocument();
    expect(screen.queryByText('fn.a')).not.toBeInTheDocument();
  });
});

describe('CategoryTab', () => {
  it('按资源渲染分组行', async () => {
    renderWithApp(
      <CategoryTab
        data={[group('res.a', ['fn.a']), group('res.b', ['fn.b'])]}
        columns={groupColumns}
      />,
    );
    expect(await screen.findByText('res.a')).toBeInTheDocument();
    expect(screen.getByText('res.b')).toBeInTheDocument();
  });

  it('空数据不崩溃', async () => {
    const { container } = renderWithApp(<CategoryTab data={[]} columns={groupColumns} />);
    expect(container.querySelector('.ant-table')).toBeInTheDocument();
  });
});

describe('ListTab', () => {
  const renderList = (props?: Partial<React.ComponentProps<typeof ListTab>>) => {
    const onSelectionChange = jest.fn();
    const renderResourceActions = jest.fn((resource: string) => (
      <span>{`actions-${resource}`}</span>
    ));
    renderWithApp(
      <ListTab
        groupedAssignments={[group('res.a', ['fn.a', 'fn.b']), group('res.b', ['fn.c'])]}
        selected={[]}
        columns={itemColumns}
        toolbarActions={[
          <button key="add" type="button">
            新增分配
          </button>,
        ]}
        renderResourceActions={renderResourceActions}
        onSelectionChange={onSelectionChange}
        {...props}
      />,
    );
    return { onSelectionChange, renderResourceActions };
  };

  it('分组卡片 + 计数 Tag + 顶部操作条渲染', async () => {
    renderList();
    expect(await screen.findByText('新增分配')).toBeInTheDocument();
    expect(await screen.findByText('2 个函数')).toBeInTheDocument();
    expect(screen.getByText('1 个函数')).toBeInTheDocument();
    expect(screen.getByText('2 已启用')).toBeInTheDocument();
    expect(await screen.findByText('fn.c')).toBeInTheDocument();
  });

  it('renderResourceActions 按组调用（操作条 + 卡片 extra）', async () => {
    const { renderResourceActions } = renderList();
    await screen.findByText('fn.a');
    const calls = renderResourceActions.mock.calls.filter(
      ([resource, size]) => resource === 'res.a' && size === 'small',
    );
    expect(calls.length).toBeGreaterThanOrEqual(2);
    expect(renderResourceActions).toHaveBeenCalledWith('res.b', 'small');
  });

  it('行选择勾选上抛 onSelectionChange', async () => {
    const { onSelectionChange } = renderList();
    await screen.findByText('fn.a');
    const checkboxes = screen.getAllByRole('checkbox');
    // 首个为全选框，取首个数据行勾选框
    fireEvent.click(checkboxes[1]);
    // ProTable rowSelection onChange 透传 antd 三参（keys/rows/info），只断言首参
    expect(onSelectionChange.mock.calls[0][0]).toEqual(['fn.a']);
  });

  it('目录总开关禁用的函数：勾选框 disabled', async () => {
    renderList({
      groupedAssignments: [
        {
          resource: 'res.a',
          items: [item({ id: 'fn.a' }), item({ id: 'fn.b', directoryDisabled: true })],
          activeCount: 1,
        },
      ],
    });
    await screen.findByText('fn.a');
    const checkboxes = screen.getAllByRole('checkbox');
    // [0]=全选框 [1]=fn.a [2]=fn.b（目录禁用）
    expect(checkboxes[1]).not.toBeDisabled();
    expect(checkboxes[2]).toBeDisabled();
  });

  it('空分组：仅留操作条、无卡片', () => {
    const { container } = renderWithApp(
      <ListTab
        groupedAssignments={[]}
        selected={[]}
        columns={itemColumns}
        toolbarActions={[
          <button key="add" type="button">
            新增分配
          </button>,
        ]}
        renderResourceActions={() => null}
        onSelectionChange={jest.fn()}
      />,
    );
    expect(screen.getByText('新增分配')).toBeInTheDocument();
    expect(container.querySelector('.ant-card')).not.toBeInTheDocument();
  });
});
