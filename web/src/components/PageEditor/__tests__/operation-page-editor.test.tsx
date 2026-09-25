/** OperationPageEditor 覆盖：表单面板默认展开与状态 Tag、确认配置
 * （有/无 confirm 两分支、标题与描述编辑）、结果视图（字段卡片 dataType
 * 与标题编辑、重排传播、无字段时的成功/错误消息编辑即从无到有创建
 * resultView）、readonly 下拖拽手柄隐藏与表单选择器禁用。 */
import React from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import OperationPageEditor from '../OperationPageEditor';
import type { ConfirmActionSpec, OperationPageSpec, ResultViewSpec } from '@/types/dashboard';

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

/** 面板头部（标题始终渲染，可断言状态 Tag；内容惰性渲染需先点击展开） */
const headerOf = (title: string): HTMLElement => {
  const header = Array.from(document.querySelectorAll('.ant-collapse-header')).find((el) =>
    el.textContent?.includes(title),
  );
  if (!header) throw new Error(`未找到面板头部：${title}`);
  return header as HTMLElement;
};

/** 展开折叠面板（已展开则跳过），调用方用 findBy* 等待内容渲染 */
const openPanel = (title: string): void => {
  const header = headerOf(title);
  if (header.parentElement?.className.includes('ant-collapse-item-active')) return;
  fireEvent.click(header);
};

/** 按 label 文案定位 .ant-form-item（LocalizedTextEditor 不转发 id，
 * getByLabelText 关联不到控件，需作用域内再查 textbox） */
const itemOf = (labelText: string): HTMLElement => {
  const label = screen.getByText(labelText);
  const item = label.closest('.ant-form-item');
  if (!item) throw new Error(`未找到表单项：${labelText}`);
  return item as HTMLElement;
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

const confirmSpec: ConfirmActionSpec = {
  title: { 'zh-CN': '确认删除' },
  description: { 'zh-CN': '删除后不可恢复' },
  confirmText: { 'zh-CN': '删除' },
  bindingId: 'player.delete',
  risk: 'danger',
};

const resultView: ResultViewSpec = {
  fields: [{ key: 'ok', title: { 'zh-CN': '结果' }, dataType: 'boolean' }],
  successMessage: { 'zh-CN': '执行成功' },
};

const opSpec = (overrides: Partial<OperationPageSpec> = {}): OperationPageSpec => ({
  form: { jsonSchema: {} },
  confirm: confirmSpec,
  resultView,
  ...overrides,
});

describe('OperationPageEditor：表单面板', () => {
  it('默认展开表单面板：状态 Tag 为已配置，布局切换透传 form 变更', async () => {
    const onChange = jest.fn();
    render(<OperationPageEditor value={opSpec()} onChange={onChange} />);
    expect(headerOf('表单配置').textContent).toContain('已配置');
    expect(await screen.findByText(/这里只调整展示/)).toBeInTheDocument();
    await pickOption(screen.getAllByRole('combobox')[0], '行内');
    expect(onChange).toHaveBeenLastCalledWith({
      ...opSpec(),
      form: { jsonSchema: {}, layout: 'inline' },
    });
  });
});

describe('OperationPageEditor：确认配置', () => {
  it('已配置：标题与描述编辑透传 confirm 合并', async () => {
    const onChange = jest.fn();
    render(<OperationPageEditor value={opSpec()} onChange={onChange} />);
    expect(headerOf('确认配置').textContent).toContain('已配置');
    openPanel('确认配置');
    const titleInput = await screen.findByDisplayValue('确认删除');
    expect(screen.getByDisplayValue('删除后不可恢复')).toBeInTheDocument();
    fireEvent.change(titleInput, { target: { value: '确认删除道具' } });
    expect(onChange).toHaveBeenLastCalledWith({
      ...opSpec(),
      confirm: { ...confirmSpec, title: { 'zh-CN': '确认删除道具' } },
    });
  });

  it('未配置：渲染只读提示且不出现确认标题编辑项', async () => {
    const value = opSpec({ confirm: undefined });
    render(<OperationPageEditor value={value} onChange={jest.fn()} />);
    expect(headerOf('确认配置').textContent).toContain('未配置');
    openPanel('确认配置');
    expect(
      await screen.findByText(
        '确认要求由已发布 binding 的风险与审批策略决定，不能在页面编辑器中新增或移除。',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('确认标题（多语言）')).not.toBeInTheDocument();
  });
});

describe('OperationPageEditor：结果视图', () => {
  it('已配置字段：头部计数、dataType 换选与标题编辑、重排传播', async () => {
    const onChange = jest.fn();
    render(<OperationPageEditor value={opSpec()} onChange={onChange} />);
    expect(headerOf('结果视图').textContent).toContain('1 字段');
    openPanel('结果视图');
    const card = (await screen.findByText('ok')).closest('.ant-card') as HTMLElement;
    // dataType 下拉在卡片标题；其后是标题 LocalizedTextEditor 的语言选择
    await pickOption(within(card).getAllByRole('combobox')[0], '数字');
    expect(onChange).toHaveBeenLastCalledWith({
      ...opSpec(),
      resultView: {
        ...resultView,
        fields: [{ key: 'ok', title: { 'zh-CN': '结果' }, dataType: 'number' }],
      },
    });

    fireEvent.change(screen.getByDisplayValue('结果'), { target: { value: '执行结果' } });
    expect(onChange).toHaveBeenLastCalledWith({
      ...opSpec(),
      resultView: {
        ...resultView,
        fields: [{ key: 'ok', title: { 'zh-CN': '执行结果' }, dataType: 'boolean' }],
      },
    });

    fireEvent.click(screen.getByTestId('stub-reorder'));
    expect(onChange).toHaveBeenLastCalledWith({
      ...opSpec(),
      resultView: { ...resultView, fields: resultView.fields },
    });
  });

  it('无字段：头部计数为 0，成功消息编辑从无到有创建 resultView', async () => {
    const value = opSpec({ resultView: undefined });
    const onChange = jest.fn();
    render(<OperationPageEditor value={value} onChange={onChange} />);
    expect(headerOf('结果视图').textContent).toContain('0 字段');
    openPanel('结果视图');
    await screen.findByText(/结果字段来自已发布输出映射/);
    expect(screen.queryByTestId('sortable-stub')).not.toBeInTheDocument();
    fireEvent.change(within(itemOf('成功消息（多语言）')).getByRole('textbox'), {
      target: { value: '全部完成' },
    });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      resultView: { successMessage: { 'zh-CN': '全部完成' } },
    });
  });
});

describe('OperationPageEditor：readonly', () => {
  it('readonly：隐藏结果字段拖拽手柄，表单布局选择器禁用', async () => {
    const { container } = render(
      <OperationPageEditor value={opSpec()} onChange={jest.fn()} readonly />,
    );
    openPanel('结果视图');
    await screen.findByText('ok');
    expect(container.querySelector('.anticon-holder')).toBeNull();
    const layoutSelect = screen.getAllByRole('combobox')[0].closest('.ant-select');
    expect(layoutSelect).toHaveClass('ant-select-disabled');
  });
});
