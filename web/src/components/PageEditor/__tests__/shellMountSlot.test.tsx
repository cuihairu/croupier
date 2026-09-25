/** PageEditor shell（页面信息卡片）：挂载菜单插槽渲染在 body 内、
 * 旧「分类 key」编辑入口已移除（menu_items 挂载是导航唯一事实源）；
 * + 插槽归属「页面与菜单信息」卡片而非「页面结构」；
 * + renderBody 分支（缺配置 Empty / 未知类型 / composite 区块渲染）、
 * 页面排序默认值与 onChange(order) 接线、readonly 表单禁用。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import PageEditor from '../index';
import type { PageSpec } from '@/types/dashboard';

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
  useIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage,
    locale: 'zh-CN',
  }),
}));

const spec: PageSpec = {
  pageKey: 'operation--inventory.consume',
  type: 'operation',
  title: { 'zh-CN': '消耗道具' },
  category: { key: 'inventory', labels: {} },
  status: 'draft',
} as unknown as PageSpec;

describe('PageEditor shell', () => {
  it('渲染挂载菜单插槽且不再出现分类 key 输入', () => {
    render(
      <PageEditor
        value={spec}
        onChange={() => {}}
        mountMenuSlot={
          <div data-testid="mount-slot">
            <span>挂载菜单</span>
          </div>
        }
      />,
    );
    expect(screen.getByTestId('mount-slot')).toBeInTheDocument();
    expect(screen.queryByText('分类 key')).not.toBeInTheDocument();
    expect(screen.getByText('页面标题（多语言）')).toBeInTheDocument();
  });

  it('未提供插槽时正常渲染（预览等只读场景）', () => {
    render(<PageEditor value={spec} onChange={() => {}} readonly />);
    expect(screen.queryByTestId('mount-slot')).not.toBeInTheDocument();
    expect(screen.queryByText('分类 key')).not.toBeInTheDocument();
  });
});

describe('PageEditor shell 插槽归属与 meta 表单', () => {
  function renderWithSlot(overrides: Partial<Parameters<typeof PageEditor>[0]> = {}) {
    const onChange = jest.fn();
    const utils = render(
      <PageEditor
        value={spec}
        onChange={onChange}
        mountMenuSlot={<div data-testid="mount-slot" />}
        {...overrides}
      />,
    );
    return { onChange, ...utils };
  }

  function orderInput(): HTMLInputElement {
    const item = Array.from(document.querySelectorAll('.ant-form-item')).find((el) =>
      el.textContent?.includes('页面排序'),
    );
    const input = item?.querySelector('input');
    if (!input) throw new Error('order input not found');
    return input as HTMLInputElement;
  }

  it('插槽渲染在「页面与菜单信息」卡片内，不在「页面结构」卡片', () => {
    renderWithSlot();
    const metaCard = screen.getByText('页面与菜单信息').closest('.ant-card');
    const structureCard = screen.getByText('页面结构').closest('.ant-card');
    const slot = screen.getByTestId('mount-slot');
    expect(metaCard).toContainElement(slot);
    expect(structureCard).not.toContainElement(slot);
  });

  it('页面排序默认 0，改值触发 onChange 携带 order', () => {
    const { onChange } = renderWithSlot();
    expect(orderInput().value).toBe('0');

    fireEvent.change(orderInput(), { target: { value: '5' } });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ order: 5 }));
  });

  it('清空排序值 → order 回 undefined', () => {
    const { onChange } = renderWithSlot();
    fireEvent.change(orderInput(), { target: { value: '' } });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ order: undefined }));
  });

  it('readonly：meta 表单整体禁用', () => {
    renderWithSlot({ readonly: true });
    expect(orderInput()).toBeDisabled();
  });
});

describe('PageEditor shell renderBody 分支', () => {
  const emptyCases: Array<[Partial<PageSpec>, string]> = [
    [{ type: 'operation' } as Partial<PageSpec>, '无操作页面配置'],
    [{ type: 'resource' } as Partial<PageSpec>, '无资源页面配置'],
    [{ type: 'task' } as Partial<PageSpec>, '无任务页面配置'],
    [{ type: 'report' } as Partial<PageSpec>, '无报表页面配置'],
  ];

  it.each(emptyCases)('type=%s 缺配置：渲染「%s」空态', (over, text) => {
    const value = {
      pageKey: 'operation--x',
      title: { 'zh-CN': 'x' },
      status: 'draft',
      ...over,
    } as unknown as PageSpec;
    render(<PageEditor value={value} onChange={() => {}} />);
    expect(screen.getByText(text)).toBeInTheDocument();
  });

  it('未知页面类型：渲染「未知页面类型」空态', () => {
    const value = {
      pageKey: 'x',
      type: 'weird',
      title: { 'zh-CN': 'x' },
      status: 'draft',
    } as unknown as PageSpec;
    render(<PageEditor value={value} onChange={() => {}} />);
    expect(screen.getByText('未知页面类型')).toBeInTheDocument();
  });

  it('composite：渲染区块标签、本地化标题、视图与联动信息', () => {
    const value = {
      pageKey: 'composite--ops',
      type: 'composite',
      title: { 'zh-CN': '运营看板' },
      status: 'draft',
      composite: {
        sections: [
          {
            key: 'players',
            bindingId: 'player',
            title: { 'zh-CN': '玩家' },
            view: 'table',
            refreshOn: ['mail', 'task'],
          },
        ],
      },
    } as unknown as PageSpec;
    render(<PageEditor value={value} onChange={() => {}} />);

    expect(screen.getByText('player')).toBeInTheDocument();
    expect(screen.getByText(/玩家 · 视图 table/)).toBeInTheDocument();
    expect(screen.getByText(/联动 mail,task/)).toBeInTheDocument();
    // 组合页生成器托管提示
    expect(screen.getByText(/组合页由生成器按资源契约自动维护/)).toBeInTheDocument();
  });

  it('composite 无 sections：只出提示不出区块行', () => {
    const value = {
      pageKey: 'composite--empty',
      type: 'composite',
      title: { 'zh-CN': '空组合' },
      status: 'draft',
      composite: { sections: [] },
    } as unknown as PageSpec;
    render(<PageEditor value={value} onChange={() => {}} />);
    expect(screen.getByText(/组合页由生成器按资源契约自动维护/)).toBeInTheDocument();
    // 区块行格式为「标题 · 视图 x」；空 sections 不出现该行（提示语自身含「视图」字样）
    expect(screen.queryByText(/· 视图/)).not.toBeInTheDocument();
  });
});
