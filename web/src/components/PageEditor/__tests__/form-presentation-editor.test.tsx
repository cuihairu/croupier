/** FormPresentationEditor 覆盖：Schema 空字段兜底与提示、布局下拉切换、
 * 字段组件换选/可见/禁用开关（含非目标字段保位）、多语言标签与占位编辑
 * 合并、SortableList 重排传播、readonly 下拖拽手柄隐藏与布局选择器禁用
 * （Form disabled 上下文）。 */
import React from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import FormPresentationEditor from '../FormPresentationEditor';
import type { FormFieldSpec, FormPresentationSpec } from '@/types/dashboard';

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

// SortableList 替身：渲染条目 + 「重排」按钮（点击即倒序回调），
// 覆盖编辑器 onReorder → onChange 传播分支（真实实现另有专测）
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

/** 打开 Select 下拉并点击匹配文案的选项（限定当前可见下拉，避免残留干扰） */
const pickOption = async (combobox: HTMLElement, label: string): Promise<void> => {
  fireEvent.mouseDown(combobox);
  const option = await waitForOption(label);
  fireEvent.click(option);
};

const waitForOption = async (label: string): Promise<HTMLElement> => {
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
  return found;
};

const fieldA: FormFieldSpec = {
  key: 'name',
  widget: 'Input',
  label: { 'zh-CN': '角色名', 'en-US': 'Role name' },
  placeholder: { 'zh-CN': '请输入角色名' },
  visible: true,
};
const fieldB: FormFieldSpec = {
  key: 'age',
  widget: undefined,
  visible: false,
  disabled: true,
};

const twoFields = (): FormFieldSpec[] => [fieldA, fieldB];

const cardOf = (key: string): HTMLElement => {
  const el = screen.getByText(key).closest('.ant-card');
  if (!el) throw new Error(`未找到字段卡片：${key}`);
  return el as HTMLElement;
};

describe('FormPresentationEditor：空字段与布局', () => {
  it('fields 缺省：显示兜底 Tag、不渲染字段列表，顶部提示常驻', async () => {
    render(<FormPresentationEditor value={{ jsonSchema: {} }} onChange={jest.fn()} />);
    expect(await screen.findByText('Schema 未生成可配置字段')).toBeInTheDocument();
    expect(screen.queryByTestId('sortable-stub')).not.toBeInTheDocument();
    expect(screen.getByText(/这里只调整展示/)).toBeInTheDocument();
  });

  it('布局下拉：未配置时回退选中纵向，换选横向触发布局变更', async () => {
    const onChange =
      jest.fn<(value: { jsonSchema: Record<string, unknown>; layout?: string }) => void>();
    render(<FormPresentationEditor value={{ jsonSchema: {} }} onChange={onChange} />);
    // layout 缺省 → value.layout || 'vertical'
    expect(await screen.findByText('纵向')).toBeInTheDocument();
    await pickOption(screen.getByRole('combobox'), '横向');
    expect(onChange).toHaveBeenLastCalledWith({ jsonSchema: {}, layout: 'horizontal' });
  });
});

describe('FormPresentationEditor：字段编辑', () => {
  it('字段卡片：key、组件选择（widget 缺省回退占位）、可见/禁用开关状态与拖拽手柄', async () => {
    const { container } = render(
      <FormPresentationEditor
        value={{ jsonSchema: {}, fields: twoFields() }}
        onChange={jest.fn()}
      />,
    );
    expect(await screen.findByText('name')).toBeInTheDocument();

    const cardA = cardOf('name');
    // 标题内组件选择 + 标签/占位两个 LocalizedTextEditor 的语言选择
    expect(within(cardA).getAllByRole('combobox')).toHaveLength(3);
    expect(cardA.querySelector('.anticon-holder')).toBeTruthy();
    const switchesA = within(cardA).getAllByRole('switch');
    expect(switchesA[0]).toBeChecked(); // visible: true
    expect(switchesA[1]).not.toBeChecked(); // disabled 缺省

    const cardB = cardOf('age');
    // widget 缺省 → 组件下拉显示占位「组件」
    expect(within(cardB).getByText('组件')).toBeInTheDocument();
    const switchesB = within(cardB).getAllByRole('switch');
    expect(switchesB[0]).not.toBeChecked(); // visible: false
    expect(switchesB[1]).toBeChecked(); // disabled: true
    expect(container).toBeInTheDocument();
  });

  it('组件换选：onChange 只更新目标字段，其余字段保位', async () => {
    const onChange = jest.fn();
    render(
      <FormPresentationEditor
        value={{ jsonSchema: {}, fields: twoFields() }}
        onChange={onChange}
      />,
    );
    await screen.findByText('name');
    const widgetSelect = within(cardOf('name')).getAllByRole('combobox')[0];
    await pickOption(widgetSelect, '多行文本');
    expect(onChange).toHaveBeenLastCalledWith({
      jsonSchema: {},
      fields: [{ ...fieldA, widget: 'TextArea' }, fieldB],
    });
  });

  it('可见与禁用开关切换：onChange 携带布尔更新', async () => {
    const onChange = jest.fn();
    render(
      <FormPresentationEditor
        value={{ jsonSchema: {}, fields: twoFields() }}
        onChange={onChange}
      />,
    );
    await screen.findByText('name');
    const switches = within(cardOf('name')).getAllByRole('switch');
    fireEvent.click(switches[0]); // 可见：true → false
    expect(onChange).toHaveBeenLastCalledWith({
      jsonSchema: {},
      fields: [{ ...fieldA, visible: false }, fieldB],
    });
    fireEvent.click(switches[1]); // 禁用：undefined → true
    expect(onChange).toHaveBeenLastCalledWith({
      jsonSchema: {},
      fields: [{ ...fieldA, disabled: true }, fieldB],
    });
  });

  it('标签与占位编辑：合并多语言并保留另一语言', async () => {
    const onChange = jest.fn();
    render(
      <FormPresentationEditor
        value={{ jsonSchema: {}, fields: twoFields() }}
        onChange={onChange}
      />,
    );
    await screen.findByText('name');
    fireEvent.change(screen.getByDisplayValue('角色名'), { target: { value: '角色名称' } });
    expect(onChange).toHaveBeenLastCalledWith({
      jsonSchema: {},
      fields: [{ ...fieldA, label: { 'zh-CN': '角色名称', 'en-US': 'Role name' } }, fieldB],
    });
    fireEvent.change(screen.getByDisplayValue('请输入角色名'), { target: { value: '填写角色名' } });
    expect(onChange).toHaveBeenLastCalledWith({
      jsonSchema: {},
      fields: [{ ...fieldA, placeholder: { 'zh-CN': '填写角色名' } }, fieldB],
    });
  });

  it('重排：onReorder 回调倒序数组透传给 onChange', async () => {
    const onChange = jest.fn();
    render(
      <FormPresentationEditor
        value={{ jsonSchema: {}, fields: twoFields() }}
        onChange={onChange}
      />,
    );
    await screen.findByText('name');
    fireEvent.click(screen.getByTestId('stub-reorder'));
    expect(onChange).toHaveBeenLastCalledWith({
      jsonSchema: {},
      fields: [fieldB, fieldA],
    });
  });
});

describe('FormPresentationEditor：表单分组（SUI-029）', () => {
  const withGroups = (): FormPresentationSpec => ({
    jsonSchema: {},
    fields: twoFields(),
    groups: [
      { key: 'basic', title: { 'zh-CN': '基本信息' }, fields: ['name'] },
      { key: 'advanced', fields: ['age'], collapsible: true, collapsed: true },
    ],
  });

  it('分组回读：key/标题/字段归属/折叠行为原样呈现', async () => {
    render(<FormPresentationEditor value={withGroups()} onChange={jest.fn()} />);
    expect(await screen.findByDisplayValue('基本信息')).toBeInTheDocument();
    expect(screen.getByDisplayValue('basic')).toBeInTheDocument();
    expect(screen.getByDisplayValue('advanced')).toBeInTheDocument();
    // 两组的字段 Select 分别回读各组的 fields
    const selects = screen.getAllByTestId(/^group-fields-/);
    expect(selects).toHaveLength(2);
    expect(selects[0].textContent).toContain('name');
    expect(selects[1].textContent).toContain('age');
    // 可折叠开关开、默认折叠开；第一组未配置折叠
    expect(screen.getByTestId('group-collapsible-0')).not.toBeChecked();
    expect(screen.getByTestId('group-collapsible-1')).toBeChecked();
    expect(screen.getByTestId('group-collapsed-1')).toBeChecked();
  });

  it('新增分组：key 唯一顺延，空分组写入 groups', async () => {
    const onChange = jest.fn();
    render(<FormPresentationEditor value={withGroups()} onChange={onChange} />);
    await screen.findByDisplayValue('基本信息');
    fireEvent.click(screen.getByTestId('group-add'));
    expect(onChange).toHaveBeenLastCalledWith({
      ...withGroups(),
      groups: [...withGroups().groups || [], { key: 'group-3', fields: [] }],
    });
  });

  it('重命名/折叠行为/删除：只改目标分组，顺序保留', async () => {
    const onChange = jest.fn();
    const value = withGroups();
    render(<FormPresentationEditor value={value} onChange={onChange} />);
    await screen.findByDisplayValue('基本信息');
    fireEvent.change(screen.getByTestId('group-key-input-0'), { target: { value: 'contact' } });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      groups: [{ ...value.groups?.[0], key: 'contact' }, value.groups?.[1]],
    });
    fireEvent.click(screen.getByTestId('group-collapsible-0'));
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      groups: [{ ...value.groups?.[0], collapsible: true }, value.groups?.[1]],
    });
    fireEvent.click(screen.getByTestId('group-delete-0'));
    expect(onChange).toHaveBeenLastCalledWith({ ...value, groups: [value.groups?.[1]] });
  });

  it('分组重排：onReorder 透传新的 groups 顺序', async () => {
    const onChange = jest.fn();
    render(<FormPresentationEditor value={withGroups()} onChange={onChange} />);
    await screen.findByDisplayValue('基本信息');
    // 编辑器内两个 SortableList（字段、分组），第二个是分组列表
    const reorderButtons = screen.getAllByTestId('stub-reorder');
    expect(reorderButtons).toHaveLength(2);
    fireEvent.click(reorderButtons[1]);
    expect(onChange).toHaveBeenLastCalledWith({
      ...withGroups(),
      groups: [withGroups().groups?.[1], withGroups().groups?.[0]],
    });
  });

  it('readonly：分组新增/删除手柄隐藏、分组字段选择器禁用', async () => {
    const { container } = render(
      <FormPresentationEditor value={withGroups()} onChange={jest.fn()} readonly />,
    );
    await screen.findByDisplayValue('基本信息');
    expect(screen.queryByTestId('group-add')).not.toBeInTheDocument();
    expect(screen.queryByTestId('group-delete-0')).not.toBeInTheDocument();
    expect(container.querySelector('.anticon-holder')).toBeNull();
  });
});

describe('FormPresentationEditor：readonly', () => {
  it('readonly：隐藏拖拽手柄，布局选择器走 Form disabled 上下文禁用', async () => {
    const { container } = render(
      <FormPresentationEditor
        value={{ jsonSchema: {}, fields: twoFields() }}
        onChange={jest.fn()}
        readonly
      />,
    );
    await screen.findByText('name');
    expect(container.querySelector('.anticon-holder')).toBeNull();
    // 布局下拉在最外层 <Form disabled> 内；卡片标题内的组件选择在 Form 外
    const layoutSelect = screen.getAllByRole('combobox')[0].closest('.ant-select');
    expect(layoutSelect).toHaveClass('ant-select-disabled');
  });
});
