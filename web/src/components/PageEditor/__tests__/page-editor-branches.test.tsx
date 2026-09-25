/** PageEditor（index.tsx）分支补充：四类页面有配置时渲染子编辑器并把
 * 子 onChange 包装为 {...value, resource/operation/task/report} 透传、
 * meta 表单的 title/order/icon onChange 包装、composite 缺 composite
 * 字段的 `|| []` 兜底、section 无 title 回退 key、无 refreshOn 无联动
 * 后缀。缺配置/未知类型/composite 常规分支由 shellMountSlot.test 覆盖。 */
import React from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import PageEditor from '../index';
import type { PageSpec } from '@/types/dashboard';

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

// SortableList 替身（listView 列表重排容器）：条目 + 回调按钮
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

const shellSpec = (overrides: Record<string, unknown> = {}): PageSpec =>
  ({
    pageKey: 'resource--players',
    type: 'resource',
    title: { 'zh-CN': '玩家管理' },
    category: { key: 'player' },
    bindings: [],
    ...overrides,
  }) as unknown as PageSpec;

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

/** 按 label 文案定位 .ant-form-item（LocalizedTextEditor 不转发 id） */
const itemOf = (labelText: string): HTMLElement => {
  const label = screen.getByText(labelText);
  const item = label.closest('.ant-form-item');
  if (!item) throw new Error(`未找到表单项：${labelText}`);
  return item as HTMLElement;
};

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

describe('PageEditor：四类页面有配置分支（onChange 包装）', () => {
  it('resource：渲染资源编辑器，子变更包装 {...value, resource}', async () => {
    const value = shellSpec({ type: 'resource', resource: {} });
    const onChange = jest.fn();
    render(<PageEditor value={value} onChange={onChange} />);
    expect(
      await screen.findByText('导航配置（标题、分类）在页面级别设置，不在此编辑器中配置。'),
    ).toBeInTheDocument();
    openPanel('列表视图');
    fireEvent.click(await screen.findByRole('button', { name: /添加列/ }));
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      resource: {
        listView: {
          columns: [
            { key: 'column_1', title: { 'zh-CN': '新列' }, dataType: 'string', visible: true },
          ],
        },
      },
    });
  });

  it('operation：渲染操作编辑器，布局切换包装 {...value, operation}', async () => {
    const value = shellSpec({
      type: 'operation',
      operation: { form: { jsonSchema: {}, fields: [] } },
    });
    const onChange = jest.fn();
    render(<PageEditor value={value} onChange={onChange} />);
    const structureCard = screen.getByText('页面结构').closest('.ant-card') as HTMLElement;
    // 结构卡片内首个下拉 = FPE 布局（meta 卡片的 IconPicker 不在作用域内）
    const layoutSelect = await within(structureCard).findByRole('combobox');
    await pickOption(layoutSelect, '横向');
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      operation: { form: { jsonSchema: {}, fields: [], layout: 'horizontal' } },
    });
  });

  it('task：渲染任务编辑器，任务视图开关包装 {...value, task}', async () => {
    const taskView = {
      taskIdStateKey: 'jobId',
      statusBindingId: 'b-status',
      statusStatePath: '/status',
    };
    const value = shellSpec({
      type: 'task',
      task: { form: { jsonSchema: {}, fields: [] }, taskView },
    });
    const onChange = jest.fn();
    render(<PageEditor value={value} onChange={onChange} />);
    await screen.findByText('显示时间线');
    const label = screen.getByText('显示时间线');
    fireEvent.click(within(label.closest('.ant-form-item') as HTMLElement).getByRole('switch'));
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      task: {
        form: { jsonSchema: {}, fields: [] },
        taskView: { ...taskView, showTimeline: true },
      },
    });
  });

  it('report：渲染报表编辑器，添加图表包装 {...value, report}', async () => {
    const report = {
      queryForm: { jsonSchema: {} },
      dataset: { dimensions: [], metrics: [] },
    };
    const value = shellSpec({ type: 'report', report });
    const onChange = jest.fn();
    render(<PageEditor value={value} onChange={onChange} />);
    openPanel('图表');
    fireEvent.click(await screen.findByRole('button', { name: /添加图表/ }));
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      report: {
        ...report,
        charts: [{ type: 'line', title: { 'zh-CN': '新图表' } }],
      },
    });
  });
});

describe('PageEditor：meta 表单 onChange 包装', () => {
  it('页面标题编辑 → {...value, title} 合并', () => {
    const value = shellSpec();
    const onChange = jest.fn();
    render(<PageEditor value={value} onChange={onChange} />);
    fireEvent.change(within(itemOf('页面标题（多语言）')).getByDisplayValue('玩家管理'), {
      target: { value: '玩家中心' },
    });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      title: { 'zh-CN': '玩家中心' },
    });
  });

  it('order 存在时输入框显示现值（value.order ?? 0 左支）', () => {
    const value = shellSpec({ order: 3 });
    render(<PageEditor value={value} onChange={jest.fn()} />);
    const input = itemOf('页面排序').querySelector('input');
    expect(input).not.toBeNull();
    expect((input as HTMLInputElement).value).toBe('3');
  });

  it('图标选择 → {...value, icon}', async () => {
    const value = shellSpec({ icon: 'BellOutlined' });
    const onChange = jest.fn();
    render(<PageEditor value={value} onChange={onChange} />);
    const iconSelect = within(itemOf('图标')).getByRole('combobox');
    await pickOption(iconSelect, 'AppstoreOutlined');
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      icon: 'AppstoreOutlined',
    });
  });
});

describe('PageEditor：composite 兜底分支', () => {
  it('缺 composite 字段：sections 走 || [] 右支，仅渲染托管提示', () => {
    const value = shellSpec({ type: 'composite' });
    render(<PageEditor value={value} onChange={jest.fn()} />);
    expect(screen.getByText(/组合页由生成器按资源契约自动维护/)).toBeInTheDocument();
    expect(screen.queryByText(/· 视图/)).not.toBeInTheDocument();
  });

  it('section 无 title 回退 key，无 refreshOn 不带联动后缀', () => {
    const value = shellSpec({
      type: 'composite',
      composite: { sections: [{ key: 'sec-a', bindingId: 'bind.a', view: 'form' }] },
    });
    render(<PageEditor value={value} onChange={jest.fn()} />);
    expect(screen.getByText(/sec-a · 视图 form/)).toBeInTheDocument();
    expect(screen.queryByText(/联动/)).not.toBeInTheDocument();
  });
});
