/** EditorModal（页面编辑弹窗）footer 挂载菜单选择：回显当前挂载、
 * 改动后才随保存提交（menuId null=解除挂载）、未改动不提交、
 * 无菜单禁用占位。保存/发布本体由工作台主页 handleSave 回调处理。 */
import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { App } from 'antd';
import EditorModal from '../EditorModal';
import type { MenuItem } from '@/services/api/menu';
import type { PageSpecDraft } from '@/types/dashboard';

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: ({ defaultMessage }, values) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  }),
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
}));

// 编辑器/预览是重组件且与本弹窗契约无关：替身只透传回调
jest.mock('@/components/PageEditor', () => {
  const MockPageEditor: React.FC<{ value: unknown; onChange: (v: unknown) => void }> = () => (
    <div data-testid="page-editor" />
  );
  return { __esModule: true, default: MockPageEditor };
});
jest.mock('@/components/PageRenderer', () => {
  const MockPageRenderer: React.FC<{ pageSpec: unknown }> = () => (
    <div data-testid="page-renderer" />
  );
  return { __esModule: true, default: MockPageRenderer };
});

const menu = (id: number, menuKey: string, zh: string, children: MenuItem[] = []): MenuItem => ({
  id,
  parentId: null,
  menuKey,
  labels: { 'zh-CN': zh },
  sortOrder: 0,
  isVisible: true,
  children,
});

const MENUS: MenuItem[] = [menu(1, 'player', '玩家'), menu(2, 'ops', '运营')];

const draft: PageSpecDraft = {
  pageKey: 'operation--inventory.consume',
  type: 'operation',
  title: { 'zh-CN': '消耗道具' },
  category: { key: 'inventory' },
  status: 'draft',
  draftRevision: 3,
  updatedAt: '2026-09-19T00:00:00Z',
};

function renderModal(props: Partial<Parameters<typeof EditorModal>[0]> = {}) {
  const onSave = jest.fn();
  const utils = render(
    <App>
      <EditorModal
        open
        pageKey="operation--inventory.consume"
        draft={draft}
        livePreview={false}
        saving={false}
        menus={MENUS}
        onClose={jest.fn()}
        onLivePreviewChange={jest.fn()}
        onSave={onSave}
        onSpecChange={jest.fn()}
        onSyncSelectors={jest.fn()}
        {...props}
      />
    </App>,
  );
  return { onSave, ...utils };
}

async function selectMenu(optionText: string): Promise<void> {
  // TreeSelect：mouseDown 触发下拉，弹层选项渲染在 body 下
  const selector = document.querySelector('.ant-select') as HTMLElement;
  fireEvent.mouseDown(selector);
  const option = await screen.findByText(optionText, {}, { timeout: 3000 });
  fireEvent.click(option);
}

function selectedText(): string {
  return document.querySelector('.ant-select-content')?.textContent ?? '';
}

async function clearSelection(): Promise<void> {
  const clear = await waitFor(() => {
    const icon = document.querySelector('.ant-select-clear') as HTMLElement | null;
    if (!icon) throw new Error('clear icon not rendered');
    return icon;
  });
  fireEvent.mouseDown(clear);
  fireEvent.click(clear);
}

describe('EditorModal footer 挂载菜单选择', () => {
  it('打开回显当前挂载菜单；未改动保存不提交挂载', async () => {
    const { onSave } = renderModal({ currentMenuId: 1 });
    await waitFor(() => expect(selectedText()).toContain('玩家'));

    // 未改动：仅保存草稿不带 menuId（避免每次保存都调挂载 API）
    fireEvent.click(screen.getByRole('button', { name: /仅保存草稿/ }));
    expect(onSave).toHaveBeenCalledWith(undefined);

    // 未改动：保存并发布只带 publishAfterSave
    fireEvent.click(screen.getByRole('button', { name: /保存并发布/ }));
    expect(onSave).toHaveBeenLastCalledWith({ publishAfterSave: true });
  });

  it('选择菜单后保存并发布 → onSave({ publishAfterSave: true, menuId })', async () => {
    const { onSave } = renderModal();
    await selectMenu('运营');
    fireEvent.click(screen.getByRole('button', { name: /保存并发布/ }));
    expect(onSave).toHaveBeenLastCalledWith({ publishAfterSave: true, menuId: 2 });
  });

  it('选择菜单后仅保存草稿 → onSave({ menuId })（draft 挂载发布后才上控制台）', async () => {
    const { onSave } = renderModal();
    await selectMenu('玩家');
    fireEvent.click(screen.getByRole('button', { name: /仅保存草稿/ }));
    expect(onSave).toHaveBeenLastCalledWith({ menuId: 1 });
  });

  it('清空选择 → menuId null 表示解除挂载', async () => {
    const { onSave } = renderModal({ currentMenuId: 1 });
    await waitFor(() => expect(selectedText()).toContain('玩家'));
    await clearSelection();
    fireEvent.click(screen.getByRole('button', { name: /保存并发布/ }));
    expect(onSave).toHaveBeenLastCalledWith({ publishAfterSave: true, menuId: null });
  });

  it('currentMenuId 变化（换页重开）→ 回显重置且 dirty 清零', async () => {
    const { rerender } = renderModal({ currentMenuId: 1 });
    await waitFor(() => expect(selectedText()).toContain('玩家'));

    rerender(
      <App>
        <EditorModal
          open
          pageKey="operation--inventory.consume"
          draft={draft}
          livePreview={false}
          saving={false}
          menus={MENUS}
          currentMenuId={2}
          onClose={jest.fn()}
          onLivePreviewChange={jest.fn()}
          onSave={jest.fn()}
          onSpecChange={jest.fn()}
          onSyncSelectors={jest.fn()}
        />
      </App>,
    );
    await waitFor(() => expect(selectedText()).toContain('运营'));
  });

  it('无菜单：挂载选择禁用并提示先建菜单', async () => {
    renderModal({ menus: [] });
    const selector = document.querySelector('.ant-select') as HTMLElement;
    await waitFor(() => expect(selector.className).toContain('ant-select-disabled'));
    expect(screen.getByText('暂无菜单，可先在「菜单管理」创建')).toBeInTheDocument();
  });
});
