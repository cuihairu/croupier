/** ResourcePageEditor 残余分支：updateForm 已配置时的面板头部「更新」Tag
 * 与更新表单编辑器（既有用例只覆盖 updateForm 缺省的「未配置」回退）。 */
import React from 'react';
import { fireEvent, render, screen, within, configure } from '@testing-library/react';
import ResourcePageEditor from '../ResourcePageEditor';
import type { ResourcePageSpec } from '@/types/dashboard';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

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
        ...props.items.map((item, index) =>
          R.createElement(R.Fragment, { key: index }, props.children(item, index, {})),
        ),
      ),
  };
});

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

/** 更新表单编辑区（标题与其后的编辑器容器） */
const updateFormScope = (): HTMLElement => {
  const title = screen.getByText('更新表单');
  const wrap = title.parentElement;
  if (!wrap) throw new Error('未找到更新表单容器');
  return wrap;
};

const specWithUpdateForm = (): ResourcePageSpec => ({
  listView: { columns: [], rowActions: [], batchActions: [], toolbarActions: [] },
  createForm: { jsonSchema: {}, fields: [{ key: 'title' }] },
  updateForm: { jsonSchema: {}, fields: [{ key: 'title' }] },
  deleteAction: { title: { 'zh-CN': '删除' } },
});

describe('ResourcePageEditor 残余分支', () => {
  it('updateForm 已配置 → 头部 Tag 显示「创建 更新」且更新表单区不再回退未配置', async () => {
    render(<ResourcePageEditor value={specWithUpdateForm()} onChange={jest.fn()} />);
    expect(headerOf('表单配置').textContent).toContain('创建');
    expect(headerOf('表单配置').textContent).toContain('更新');

    openPanel('表单配置');
    expect(await screen.findByText('更新表单')).toBeInTheDocument();
    expect(updateFormScope()).toBeInTheDocument();
    expect(within(updateFormScope()).queryByText('未配置')).not.toBeInTheDocument();
    expect(within(updateFormScope()).getAllByRole('combobox').length).toBeGreaterThanOrEqual(1);
  });

  it('更新表单编辑器可编辑 → onChange 回写 updateForm', async () => {
    const value = specWithUpdateForm();
    const onChange = jest.fn();
    render(<ResourcePageEditor value={value} onChange={onChange} />);
    openPanel('表单配置');
    expect(await screen.findByText('更新表单')).toBeInTheDocument();

    await pickOption(within(updateFormScope()).getAllByRole('combobox')[0], '行内');
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      updateForm: { jsonSchema: {}, fields: [{ key: 'title' }], layout: 'inline' },
    });
  });
});
