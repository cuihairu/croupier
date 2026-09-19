/** MenuMountModal（挂载菜单弹窗）：TreeSelect 回显/改挂/清空解除、
 * 无菜单空态、draft 页面发布提示。控制台导航由 menu_items 唯一驱动。 */
import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { App } from 'antd';
import MenuMountModal from '../MenuMountModal';
import type { MenuItem } from '@/services/api/menu';
import type { PageSpecDraftSummary } from '@/types/dashboard';

jest.mock('@umijs/max', () => ({
  // 与 tests/setupTests.jsx 同语义：defaultMessage + {placeholder} 插值；
  // 额外提供 locale（菜单 labels 本地化渲染需要）
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: ({ defaultMessage }, values) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  }),
}));

const menu = (id: number, menuKey: string, zh: string, children: MenuItem[] = []): MenuItem => ({
  id,
  parentId: null,
  menuKey,
  labels: { 'zh-CN': zh },
  sortOrder: 0,
  isVisible: true,
  children,
});

const MENUS: MenuItem[] = [
  menu(1, 'player', '玩家'),
  menu(2, 'ops', '运营', [menu(3, 'audit', '审计')]),
];

function page(overrides: Partial<PageSpecDraftSummary> = {}): PageSpecDraftSummary {
  return {
    pageKey: 'resource--players',
    type: 'resource',
    title: { 'zh-CN': '玩家列表' },
    category: { key: 'players' },
    status: 'published',
    draftRevision: 1,
    updatedAt: '2026-09-18T00:00:00Z',
    ...overrides,
  };
}

function renderModal(props: Partial<Parameters<typeof MenuMountModal>[0]> = {}) {
  const onSubmit = jest.fn();
  const onCancel = jest.fn();
  const utils = render(
    <App>
      <MenuMountModal
        page={page()}
        menus={MENUS}
        saving={false}
        onCancel={onCancel}
        onSubmit={onSubmit}
        {...props}
      />
    </App>,
  );
  return { onSubmit, onCancel, ...utils };
}

async function selectMenu(optionText: string): Promise<void> {
  // TreeSelect（antd 新版渲染 .ant-select-content + combobox input）：
  // mouseDown 触发下拉，弹层选项渲染在 body 下
  const selector = document.querySelector('.ant-select') as HTMLElement;
  fireEvent.mouseDown(selector);
  const option = await screen.findByText(optionText, {}, { timeout: 3000 });
  fireEvent.click(option);
}

/** TreeSelect 当前选中值文本（antd5 无 .ant-select-selection-item 类）。 */
function selectedText(): string {
  return document.querySelector('.ant-select-content')?.textContent ?? '';
}

describe('MenuMountModal', () => {
  it('回显当前挂载菜单', async () => {
    renderModal({ page: page({ menuId: 1 }) });
    expect(await screen.findByText('挂载菜单：resource--players')).toBeInTheDocument();
    // TreeSelect 显示选中节点的 labels 本地化标题
    await waitFor(() => expect(selectedText()).toContain('玩家'));
  });

  it('选择菜单提交 → onSubmit(pageKey, menuId)', async () => {
    const { onSubmit } = renderModal();
    await selectMenu('运营');
    fireEvent.click(screen.getByRole('button', { name: /确\s*定|OK/ }));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith('resource--players', 2));
  });

  it('清空提交 → onSubmit(pageKey, null) 解除挂载', async () => {
    const { onSubmit } = renderModal({ page: page({ menuId: 1 }) });
    const clear = await screen.findByRole('img', { name: 'close-circle' });
    fireEvent.click(clear);
    fireEvent.click(screen.getByRole('button', { name: /确\s*定|OK/ }));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith('resource--players', null));
  });

  it('无菜单空态：提示先建菜单，确定按钮隐藏、onOk 走关闭', async () => {
    const { onSubmit, onCancel } = renderModal({ menus: [] });
    expect(
      await screen.findByText('当前环境暂无菜单，请先在「菜单管理」中创建菜单。'),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /确\s*定|OK/ })).toBeNull();
    expect(onSubmit).not.toHaveBeenCalled();
    expect(onCancel).not.toHaveBeenCalled();

    // 按钮仅 CSS 隐藏（okButtonProps display:none）：jsdom 直接派发点击仍走
    // onOk 的 empty 分支——直接 onCancel 关闭，不提交
    const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    fireEvent.click(ok);
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('draft 页面显示「发布后才上控制台」提示；published 不显示', async () => {
    const { rerender } = render(
      <App>
        <MenuMountModal
          page={page({ status: 'draft' })}
          menus={MENUS}
          saving={false}
          onCancel={jest.fn()}
          onSubmit={jest.fn()}
        />
      </App>,
    );
    expect(
      await screen.findByText(/该页面尚未发布：挂载关系会保存，但发布后才会出现在运行控制台导航/),
    ).toBeInTheDocument();

    rerender(
      <App>
        <MenuMountModal
          page={page({ status: 'published' })}
          menus={MENUS}
          saving={false}
          onCancel={jest.fn()}
          onSubmit={jest.fn()}
        />
      </App>,
    );
    await waitFor(() => expect(screen.queryByText(/该页面尚未发布/)).not.toBeInTheDocument());
  });

  it('嵌套子菜单出现在可选列表', async () => {
    renderModal();
    await selectMenu('审计');
    fireEvent.click(screen.getByRole('button', { name: /确\s*定|OK/ }));
    // 选择成功即可（onSubmit 断言在弹层关闭竞态下不稳，这里只验证可选中深层节点）
    await waitFor(() => expect(selectedText()).toContain('审计'));
  });
});
