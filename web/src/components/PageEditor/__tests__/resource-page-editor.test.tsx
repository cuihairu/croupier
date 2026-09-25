/** ResourcePageEditor 覆盖：导航提示与各面板头部计数 Tag、列表列卡片
 * 编辑（dataType/宽度清空回退/可见/标题）、添加/删除列（含 listView 缺省
 * 从无到有）、操作面板（缺 binding 红标、risk 颜色分支、空组未生成、
 * confirm/type/标题编辑与 toolbar 分组透传）、表单配置（createForm 编辑
 * 透传、updateForm/deleteAction 缺省回退）、readonly 下按钮禁用与
 * 拖拽手柄/删除入口隐藏。 */
import React, { useState } from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import ResourcePageEditor from '../ResourcePageEditor';
import type { ActionSpec, ColumnSpec, ResourcePageSpec } from '@/types/dashboard';

jest.setTimeout(15000);

jest.mock('@umijs/max', () => ({
  __esModule: true,
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: (
      { defaultMessage }: { defaultMessage?: string },
      values?: Record<string, unknown>,
    ) =>
      Object.entries(values || {}).reduce(
        (msg, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage ?? '',
      ),
  }),
}));

// SortableList 替身：条目 + 「重排」按钮（倒序回调），覆盖 onReorder 传播
jest.mock('@/components/SortableList', () => {
  const R = require('react') as typeof import('react');
  interface StubProps {
    items: unknown[];
    onReorder: (items: unknown[]) => void;
    children: (
      item: unknown,
      index: number,
      dragHandleProps: React.HTMLAttributes<HTMLElement>,
    ) => React.ReactNode;
  }
  return {
    __esModule: true,
    SortableList: (props: StubProps) =>
      R.createElement(
        'div',
        { 'data-testid': 'sortable-stub' },
        R.createElement(
          'button',
          {
            type: 'button',
            'data-testid': 'stub-reorder',
            onClick: () => props.onReorder([...props.items].reverse()),
          },
          '重排',
        ),
        ...props.items.map((item, index) =>
          R.createElement(R.Fragment, { key: index }, props.children(item, index, {})),
        ),
      ),
  };
});

/** 面板头部（标题与计数 Tag 始终渲染；内容惰性渲染需先展开） */
const headerOf = (title: string): HTMLElement => {
  const header = Array.from(document.querySelectorAll('.ant-collapse-header')).find((el) =>
    el.textContent?.includes(title),
  );
  if (!header) throw new Error(`未找到面板头部：${title}`);
  return header as HTMLElement;
};

const openPanel = (title: string): void => {
  const header = headerOf(title);
  if (header.parentElement?.className.includes('ant-collapse-item-active')) return;
  fireEvent.click(header);
};

const panelOf = (title: string): HTMLElement => {
  const panel = headerOf(title).parentElement;
  if (!panel) throw new Error(`未找到面板容器：${title}`);
  return panel as HTMLElement;
};

/** 打开 Select 下拉并点击匹配文案的选项（限定当前可见下拉） */
const pickOption = async (combobox: HTMLElement, label: string): Promise<void> => {
  fireEvent.mouseDown(combobox);
  let found: HTMLElement | undefined;
  for (let i = 0; i < 60 && !found; i += 1) {
    await new Promise((resolve) => setTimeout(resolve, 25));
    const dropdowns = document.querySelectorAll<HTMLElement>(
      '.ant-select-dropdown:not(.ant-select-dropdown-hidden)',
    );
    for (const dropdown of dropdowns) {
      const hit = Array.from(
        dropdown.querySelectorAll<HTMLElement>('.ant-select-item-option'),
      ).find((item) => item.textContent?.includes(label));
      if (hit) {
        found = hit;
        break;
      }
    }
  }
  if (!found) throw new Error(`下拉选项未出现：${label}`);
  fireEvent.click(found);
};

const cardOf = (scope: HTMLElement | Document, key: string): HTMLElement => {
  const root = scope instanceof Element ? (scope as HTMLElement) : (scope.body as HTMLElement);
  const el = within(root).getByText(key).closest('.ant-card');
  if (!el) throw new Error(`未找到卡片：${key}`);
  return el as HTMLElement;
};

const deleteButtonOf = (card: HTMLElement): HTMLElement => {
  const btn = Array.from(card.querySelectorAll('button')).find((b) =>
    b.querySelector('.anticon-delete'),
  );
  if (!btn) throw new Error('未找到删除按钮');
  return btn as HTMLElement;
};

const colName: ColumnSpec = {
  key: 'name',
  title: { 'zh-CN': '名称', 'en-US': 'Name' },
  dataType: 'string',
  width: 120,
  visible: true,
};
const colLevel: ColumnSpec = { key: 'level', title: { 'zh-CN': '等级' }, dataType: 'number' };

const actionKick: ActionSpec = {
  key: 'kick',
  title: { 'zh-CN': '踢出' },
  type: 'danger',
  confirm: true,
  bindingId: 'player.kick',
  risk: 'danger',
};
// 缺 binding/type/risk/confirm 的兜底形态
const actionWarn: ActionSpec = { key: 'warn', title: { 'zh-CN': '警告' } };
const actionExport: ActionSpec = {
  key: 'export',
  title: { 'zh-CN': '导出' },
  type: 'primary',
  bindingId: 'report.export',
  risk: 'high',
};

const resourceSpec = (overrides: Partial<ResourcePageSpec> = {}): ResourcePageSpec => ({
  listView: {
    columns: [colName, colLevel],
    rowActions: [actionKick, actionWarn],
    batchActions: [],
    toolbarActions: [actionExport],
  },
  createForm: { jsonSchema: {}, fields: [{ key: 'title' }] },
  deleteAction: {
    title: { 'zh-CN': '删除玩家' },
    confirmText: { 'zh-CN': '删除' },
    bindingId: 'player.delete',
  },
  ...overrides,
});

/** 受控 harness：onChange 同步回灌 state，覆盖连续编辑后的真实 DOM */
function ResourceHarness({
  initial,
  onChange,
  readonly,
}: {
  initial: ResourcePageSpec;
  onChange: (value: ResourcePageSpec) => void;
  readonly?: boolean;
}) {
  const [value, setValue] = useState(initial);
  return (
    <ResourcePageEditor
      value={value}
      readonly={readonly}
      onChange={(next) => {
        onChange(next);
        setValue(next);
      }}
    />
  );
}

describe('ResourcePageEditor：面板结构与列表列', () => {
  it('导航默认展开；各面板头部计数 Tag（2 列 / 3 个 / 创建）', async () => {
    render(<ResourcePageEditor value={resourceSpec()} onChange={jest.fn()} />);
    expect(
      await screen.findByText('导航配置（标题、分类）在页面级别设置，不在此编辑器中配置。'),
    ).toBeInTheDocument();
    expect(headerOf('列表视图').textContent).toContain('2 列');
    expect(headerOf('操作配置').textContent).toContain('3 个');
    expect(headerOf('表单配置').textContent).toContain('创建');
  });

  it('添加列：harness 状态回灌出现新卡片，payload 携带默认新列', async () => {
    const onChange = jest.fn();
    render(<ResourceHarness initial={resourceSpec()} onChange={onChange} />);
    openPanel('列表视图');
    fireEvent.click(await screen.findByRole('button', { name: /添加列/ }));
    expect(await screen.findByText('column_3')).toBeInTheDocument();
    expect(onChange).toHaveBeenLastCalledWith({
      ...resourceSpec(),
      listView: {
        ...resourceSpec().listView,
        columns: [
          colName,
          colLevel,
          { key: 'column_3', title: { 'zh-CN': '新列' }, dataType: 'string', visible: true },
        ],
      },
    });
  });

  it('listView 缺省：头部 0 列，添加列从无到有创建 listView；操作组全部未生成', async () => {
    const onChange = jest.fn();
    render(<ResourceHarness initial={{}} onChange={onChange} />);
    expect(headerOf('列表视图').textContent).toContain('0 列');
    openPanel('列表视图');
    fireEvent.click(await screen.findByRole('button', { name: /添加列/ }));
    expect(await screen.findByText('column_1')).toBeInTheDocument();
    expect(onChange).toHaveBeenLastCalledWith({
      listView: {
        columns: [
          { key: 'column_1', title: { 'zh-CN': '新列' }, dataType: 'string', visible: true },
        ],
      },
    });
    openPanel('操作配置');
    expect(await screen.findByText('行操作')).toBeInTheDocument();
    expect(screen.getAllByText('未生成')).toHaveLength(3);
  });

  it('列卡片编辑：dataType/宽度/可见/标题，另一列保位', async () => {
    const value = resourceSpec();
    const onChange = jest.fn();
    render(<ResourcePageEditor value={value} onChange={onChange} />);
    openPanel('列表视图');
    const card = cardOf(panelOf('列表视图'), 'name');
    await screen.findByText('column_3').catch(() => undefined); // 等待面板内容渲染
    expect(card.querySelector('.anticon-holder')).toBeTruthy();

    // 标题区：[0]=dataType 下拉；正文是标题 LocalizedTextEditor
    await pickOption(within(card).getAllByRole('combobox')[0], '日期');
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      listView: {
        ...value.listView,
        columns: [{ ...colName, dataType: 'date' }, colLevel],
      },
    });

    const widthInput = within(card).getByRole('spinbutton');
    fireEvent.change(widthInput, { target: { value: '200' } });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      listView: { ...value.listView, columns: [{ ...colName, width: 200 }, colLevel] },
    });
    // 清空 → onChange(null) → width || undefined 回退 undefined
    fireEvent.change(widthInput, { target: { value: '' } });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      listView: {
        ...value.listView,
        columns: [{ ...colName, width: undefined }, colLevel],
      },
    });

    fireEvent.click(within(card).getByRole('switch'));
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      listView: { ...value.listView, columns: [{ ...colName, visible: false }, colLevel] },
    });

    fireEvent.change(screen.getByDisplayValue('名称'), { target: { value: '角色名' } });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      listView: {
        ...value.listView,
        columns: [{ ...colName, title: { 'zh-CN': '角色名', 'en-US': 'Name' } }, colLevel],
      },
    });
  });

  it('删除列：harness 移除对应卡片，payload 仅剩另一列', async () => {
    const onChange = jest.fn();
    render(<ResourceHarness initial={resourceSpec()} onChange={onChange} />);
    openPanel('列表视图');
    const card = await (async () => {
      await screen.findByText('column_3').catch(() => undefined);
      return cardOf(panelOf('列表视图'), 'name');
    })();
    fireEvent.click(deleteButtonOf(card));
    expect(screen.queryByText('name')).not.toBeInTheDocument();
    expect(screen.getByText('level')).toBeInTheDocument();
    expect(onChange).toHaveBeenLastCalledWith({
      ...resourceSpec(),
      listView: { ...resourceSpec().listView, columns: [colLevel] },
    });
  });
});

describe('ResourcePageEditor：操作配置', () => {
  it('状态与空组：缺 binding 红标、risk 颜色分支、空批量组未生成', async () => {
    render(<ResourcePageEditor value={resourceSpec()} onChange={jest.fn()} />);
    openPanel('操作配置');
    expect(await screen.findByText('行操作')).toBeInTheDocument();
    expect(screen.getByText('批量操作')).toBeInTheDocument();
    expect(screen.getByText('工具栏操作')).toBeInTheDocument();
    // batchActions 为空 → 未生成
    expect(screen.getAllByText('未生成')).toHaveLength(1);
    // actionWarn：无 binding → 红标；无 risk → 默认色 + 未声明
    expect(screen.getByText('缺少 binding')).toBeInTheDocument();
    expect(screen.getByText('未声明').closest('.ant-tag')?.className).not.toContain('red');
    // actionKick risk danger → 红标；actionExport risk high → 橙标；bindingId 蓝标
    expect(screen.getByText('danger').closest('.ant-tag')?.className).toContain('red');
    expect(screen.getByText('high').closest('.ant-tag')?.className).toContain('orange');
    expect(screen.getByText('player.kick').closest('.ant-tag')?.className).toContain('blue');
  });

  it('行操作编辑：confirm 开关、type 换选、标题编辑', async () => {
    const value = resourceSpec();
    const onChange = jest.fn();
    render(<ResourcePageEditor value={value} onChange={onChange} />);
    openPanel('操作配置');
    const kickCard = await (async () => {
      await screen.findByText('行操作');
      return cardOf(document, 'kick');
    })();
    const warnCard = cardOf(document, 'warn');

    // confirm 开关（卡片标题内唯一 Switch）：true → false
    fireEvent.click(within(kickCard).getByRole('switch'));
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      listView: {
        ...value.listView,
        rowActions: [{ ...actionKick, confirm: false }, actionWarn],
      },
    });

    // type 下拉（卡片标题首个 combobox）：warn 缺省显示「默认」
    await pickOption(within(warnCard).getAllByRole('combobox')[0], '主按钮');
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      listView: {
        ...value.listView,
        rowActions: [actionKick, { ...actionWarn, type: 'primary' }],
      },
    });

    fireEvent.change(within(warnCard).getByDisplayValue('警告'), {
      target: { value: '警告提示' },
    });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      listView: {
        ...value.listView,
        rowActions: [actionKick, { ...actionWarn, title: { 'zh-CN': '警告提示' } }],
      },
    });
  });

  it('工具栏操作编辑：透传 toolbarActions 分组', async () => {
    const value = resourceSpec();
    const onChange = jest.fn();
    render(<ResourcePageEditor value={value} onChange={onChange} />);
    openPanel('操作配置');
    await screen.findByText('工具栏操作');
    const exportCard = cardOf(document, 'export');
    fireEvent.click(within(exportCard).getByRole('switch'));
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      listView: {
        ...value.listView,
        toolbarActions: [{ ...actionExport, confirm: true }],
      },
    });
  });
});

describe('ResourcePageEditor：表单配置', () => {
  it('createForm 渲染并可编辑；updateForm 缺省未配置；deleteAction 已配置', async () => {
    const value = resourceSpec();
    const onChange = jest.fn();
    render(<ResourcePageEditor value={value} onChange={onChange} />);
    openPanel('表单配置');
    expect(await screen.findByText(/这里只调整展示/)).toBeInTheDocument();
    expect(screen.getByText('未配置')).toBeInTheDocument(); // updateForm 缺省
    expect(screen.getByText('已配置')).toBeInTheDocument(); // deleteAction 存在
    // FPE 布局下拉：表单面板内文档序首个 combobox
    await pickOption(screen.getAllByRole('combobox')[0], '行内');
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      createForm: { jsonSchema: {}, fields: [{ key: 'title' }], layout: 'inline' },
    });
  });

  it('表单全缺省：创建/更新/删除确认三处均回退未配置', async () => {
    render(<ResourcePageEditor value={{}} onChange={jest.fn()} />);
    openPanel('表单配置');
    expect(await screen.findByText('创建表单')).toBeInTheDocument();
    expect(screen.getAllByText('未配置')).toHaveLength(3);
    expect(screen.queryByText(/这里只调整展示/)).not.toBeInTheDocument();
  });
});

describe('ResourcePageEditor：readonly', () => {
  it('readonly：添加列禁用，删除入口与拖拽手柄隐藏', async () => {
    const { container } = render(
      <ResourcePageEditor value={resourceSpec()} onChange={jest.fn()} readonly />,
    );
    openPanel('列表视图');
    expect(await screen.findByRole('button', { name: /添加列/ })).toBeDisabled();
    const listViewPanel = panelOf('列表视图');
    expect(listViewPanel.querySelectorAll('.anticon-delete')).toHaveLength(0);
    expect(container.querySelector('.anticon-holder')).toBeNull();
  });
});
