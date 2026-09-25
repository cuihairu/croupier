/** EditorModal（页面编辑弹窗）footer 挂载菜单选择：回显当前挂载、
 * 改动后才随保存提交（menuId null=解除挂载）、未改动不提交、
 * 无菜单禁用占位。保存/发布本体由工作台主页 handleSave 回调处理。 */
import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { App } from 'antd';
import EditorModal from '../EditorModal';
import type { MenuItem } from '@/services/api/menu';
import type { BindingFreshnessDiagnostic, PageSpecDraft } from '@/types/dashboard';

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

// 编辑器/预览是重组件且与本弹窗契约无关：替身只透传回调与挂载插槽
// （mountMenuSlot 真实渲染，选择器已移入编辑器 body——2026-09 footer 不可见修复）
jest.mock('@/components/PageEditor', () => {
  const MockPageEditor: React.FC<{
    value: unknown;
    onChange: (v: unknown) => void;
    mountMenuSlot?: React.ReactNode;
  }> = ({ mountMenuSlot }) => <div data-testid="page-editor">{mountMenuSlot}</div>;
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
  const onClose = jest.fn();
  const onLivePreviewChange = jest.fn();
  const onSyncSelectors = jest.fn();
  const utils = render(
    <App>
      <EditorModal
        open
        pageKey="operation--inventory.consume"
        draft={draft}
        livePreview={false}
        saving={false}
        menus={MENUS}
        onClose={onClose}
        onLivePreviewChange={onLivePreviewChange}
        onSave={onSave}
        onSpecChange={jest.fn()}
        onSyncSelectors={onSyncSelectors}
        {...props}
      />
    </App>,
  );
  return { onSave, onClose, onLivePreviewChange, onSyncSelectors, ...utils };
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

describe('EditorModal 挂载菜单选择（body 内「页面信息」卡片）', () => {
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

  it('未挂载打开：默认选中第一个菜单，直接保存即挂载（无需手选）', async () => {
    const { onSave } = renderModal();
    // 默认值 = 第一个菜单（进入即有值，不再是空占位）
    await waitFor(() => expect(selectedText()).toContain('玩家'));
    fireEvent.click(screen.getByRole('button', { name: /仅保存草稿/ }));
    expect(onSave).toHaveBeenLastCalledWith({ menuId: 1 });
  });

  it('未挂载默认后清空 → menuId null 保持不挂载', async () => {
    const { onSave } = renderModal();
    await waitFor(() => expect(selectedText()).toContain('玩家'));
    await clearSelection();
    fireEvent.click(screen.getByRole('button', { name: /保存并发布/ }));
    expect(onSave).toHaveBeenLastCalledWith({ publishAfterSave: true, menuId: null });
  });

  it('选择菜单后保存并发布 → onSave({ publishAfterSave: true, menuId })', async () => {
    const { onSave } = renderModal();
    await selectMenu('运营');
    fireEvent.click(screen.getByRole('button', { name: /保存并发布/ }));
    expect(onSave).toHaveBeenLastCalledWith({ publishAfterSave: true, menuId: 2 });
  });

  it('选择菜单后仅保存草稿 → onSave({ menuId })（draft 挂载发布后才上控制台）', async () => {
    const { onSave } = renderModal();
    // 未挂载默认选中第一个菜单（玩家），改选运营验证改动路径
    await selectMenu('运营');
    fireEvent.click(screen.getByRole('button', { name: /仅保存草稿/ }));
    expect(onSave).toHaveBeenLastCalledWith({ menuId: 2 });
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

describe('EditorModal 弹窗基础行为', () => {
  it('draft 为空：渲染「请选择页面」空态，不渲染编辑器', () => {
    renderModal({ draft: null });
    expect(screen.getByText('请选择页面')).toBeInTheDocument();
    expect(screen.queryByTestId('page-editor')).not.toBeInTheDocument();
  });

  it('pageKey 缺省：标题回退占位 -', () => {
    renderModal({ pageKey: '' });
    expect(screen.getByText('-')).toBeInTheDocument();
    expect(screen.queryByText('operation--inventory.consume')).not.toBeInTheDocument();
  });

  it('取消按钮触发 onClose', () => {
    const { onClose } = renderModal();
    fireEvent.click(screen.getByRole('button', { name: /取消/ }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('saving 时保存/发布按钮进入加载态', () => {
    renderModal({ saving: true });
    const draftBtn = screen.getByRole('button', { name: /仅保存草稿/ });
    const publishBtn = screen.getByRole('button', { name: /保存并发布/ });
    expect(draftBtn.className).toContain('ant-btn-loading');
    expect(publishBtn.className).toContain('ant-btn-loading');
  });
});

describe('EditorModal 绑定过期告警', () => {
  const stale = (
    bindingId: string,
    functionId?: string,
    message?: string,
  ): BindingFreshnessDiagnostic => ({
    bindingId,
    functionId,
    status: 'function_version_stale',
    diagnostic: {
      code: 'function_version_stale',
      severity: 'warning',
      message: message ?? '',
      ...(functionId ? { functionId } : {}),
    },
  });

  it('无 bindingFreshness：不渲染告警与同步按钮', () => {
    renderModal();
    expect(
      screen.queryByText('页面绑定与函数契约不一致（发布会校验失败）'),
    ).not.toBeInTheDocument();
  });

  it('有告警：逐条渲染 functionId（缺省回退 bindingId）与诊断消息', () => {
    renderModal({
      draft: {
        ...draft,
        bindingFreshness: [stale('b-1', 'fn.a', '契约已变更'), stale('b-2', undefined, '')],
      },
    });

    expect(screen.getByText('页面绑定与函数契约不一致（发布会校验失败）')).toBeInTheDocument();
    expect(screen.getByText('fn.a')).toBeInTheDocument();
    expect(screen.getByText('：契约已变更')).toBeInTheDocument();
    // 无 functionId → 回退 bindingId；无 message → 不追加冒号
    expect(screen.getByText('b-2')).toBeInTheDocument();
    expect(screen.queryByText('：')).not.toBeInTheDocument();
  });

  it('超过 8 条：截断列表并展示「…以及另外 N 条」', () => {
    const items = Array.from({ length: 10 }, (_, i) => stale(`b-${i}`, `fn.${i}`));
    renderModal({ draft: { ...draft, bindingFreshness: items } });

    expect(screen.getByText('fn.7')).toBeInTheDocument();
    expect(screen.queryByText('fn.8')).not.toBeInTheDocument();
    expect(screen.getByText('…以及另外 2 条')).toBeInTheDocument();
  });

  it('一键同步 Selector 按钮触发 onSyncSelectors', () => {
    const { onSyncSelectors } = renderModal({
      draft: { ...draft, bindingFreshness: [stale('b-1', 'fn.a', 'msg')] },
    });
    fireEvent.click(screen.getByRole('button', { name: '一键同步 Selector' }));
    expect(onSyncSelectors).toHaveBeenCalledTimes(1);
  });
});

describe('EditorModal 实时预览开关', () => {
  it('livePreview 关：无预览卡片；点击开关触发回调', () => {
    const { onLivePreviewChange } = renderModal({ livePreview: false });
    expect(screen.queryByText('实时预览')).not.toBeInTheDocument();
    expect(screen.queryByTestId('page-renderer')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('switch'));
    // antd Switch onChange = (checked, event)
    expect(onLivePreviewChange).toHaveBeenNthCalledWith(1, true, expect.anything());
  });

  it('livePreview 开：渲染预览卡片与提示，关闭后消失', () => {
    const { rerender, onLivePreviewChange } = renderModal({ livePreview: true });
    expect(screen.getByText('实时预览')).toBeInTheDocument();
    expect(screen.getByTestId('page-renderer')).toBeInTheDocument();
    expect(screen.getByText('预览不执行函数；发布后请在运行控制台执行')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('switch'));
    expect(onLivePreviewChange).toHaveBeenNthCalledWith(1, false, expect.anything());

    rerender(
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
          onSave={jest.fn()}
          onSpecChange={jest.fn()}
          onSyncSelectors={jest.fn()}
        />
      </App>,
    );
    expect(screen.queryByText('实时预览')).not.toBeInTheDocument();
  });
});
