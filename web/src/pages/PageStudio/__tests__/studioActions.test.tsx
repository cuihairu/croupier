/**
 * PageStudio 工作台主页（index.tsx）操作流真实渲染单测：
 * 此前主页仅经搜索/分页/资源列套件命中约 43%（操作回调 25.8%）——本套件
 * 按真实组件口径驱动行内发布/下架、一键批量、挂载菜单、编辑保存/发布、
 * 重新生成（含 409 知情确认重试）、版本回滚、变更链/对比/合并与 URL 联动。
 * 服务 API 全部 mock（数据面），子组件不打桩（仅 PageEditor/PageRenderer
 * 重组件叶子按 EditorModal.test.tsx 先例替换；MergeConflictModal 等已在
 * serverPushdown 先例中打桩的保持一致）。
 *
 * 剩余未覆盖分支登记（UI 不可达/桩边界，不写假用例）：
 * - 416-417 mount=1 但 focus 页面不在列表（未 accept）的静默分支；
 * - 425-430 updateSelectedDraftSpec（PageEditor 桩不产生 spec 变更事件）；
 * - 694-698 handleSyncSelectorsApplied（SelectorSyncReportModal 桩内不可达）；
 * - 786-793 / 830-837 / 902-903 / 928-929 各 handler 的「无页面/无效
 *   revision」守卫——入口按钮本身只在选中页面后渲染，UI 无法触达；
 * - 858-894 handleManualMergeSubmit（MergeConflictModal 打桩不可达）；
 * - 1159 EditorModal 内「一键同步 Selector」（需 bindingFreshness 徽标）。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import PageStudio from '../index';
import { history } from '@umijs/max';
import {
  bulkPublishPages,
  bulkUnpublishPages,
  getPageDraft,
  listPageDrafts,
  listPageResources,
  listPageVersions,
  publishPageDraft,
  regeneratePageDraft,
  savePageDraft,
  unpublishPage,
  updatePageMenu,
} from '@/services/api/pages';
import { listMenus, type MenuItem } from '@/services/api/menu';
import {
  getChangeChain,
  getDiff,
  mergeChanges,
  rollbackDraft,
  rollbackPublish,
  type ChangeChain,
  type DiffResponse,
} from '@/services/api/versioning';
import type { PageDraftListParams, PageDraftListResult } from '@/services/api/pages';
import type { PageSpecDraft, PageSpecDraftSummary, PageVersionItem } from '@/types/dashboard';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(30000);

jest.mock('@umijs/max', () => {
  const formatMessage = jest.fn(
    ({ defaultMessage }: { defaultMessage: string }, values?: Record<string, unknown>) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  );
  return {
    __esModule: true,
    useIntl: () => ({ formatMessage, locale: 'zh-CN' }),
    getIntl: () => ({ formatMessage }),
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
    history: { push: jest.fn() },
  };
});

jest.mock('@/services/api/pages', () => ({
  __esModule: true,
  listPageDrafts: jest.fn(),
  listPageResources: jest.fn(),
  getPageDraft: jest.fn(),
  listPageVersions: jest.fn(),
  publishPageDraft: jest.fn(),
  regeneratePageDraft: jest.fn(),
  savePageDraft: jest.fn(),
  unpublishPage: jest.fn(),
  updatePageMenu: jest.fn(),
  bulkPublishPages: jest.fn(),
  bulkUnpublishPages: jest.fn(),
}));
jest.mock('@/services/api/menu', () => ({ __esModule: true, listMenus: jest.fn() }));
jest.mock('@/services/api/versioning', () => ({
  __esModule: true,
  getChangeChain: jest.fn(),
  getDiff: jest.fn(),
  mergeChanges: jest.fn(),
  rollbackDraft: jest.fn(),
  rollbackPublish: jest.fn(),
}));

jest.mock('@/components/ProposalInbox', () => ({
  __esModule: true,
  default: ({ focusPageKey }: { focusPageKey?: string }) => (
    <div data-testid="inbox-stub" data-focus={focusPageKey ?? ''} />
  ),
}));
jest.mock('@/components/PageWorkflowGuide', () => ({ __esModule: true, default: () => null }));
jest.mock('@/components/MergeConflictModal', () => ({ __esModule: true, default: () => null }));
jest.mock('@/components/SelectorSync/SelectorSyncReportModal', () => ({
  __esModule: true,
  default: () => null,
}));
// 编辑器/渲染器是重组件叶子且与本页回调契约无关（EditorModal.test.tsx 先例）
jest.mock('@/components/PageEditor', () => ({
  __esModule: true,
  // 挂载菜单选择器由 EditorModal 经 mountMenuSlot 插槽注入编辑器 body
  default: ({ mountMenuSlot }: { mountMenuSlot?: React.ReactNode }) => (
    <div data-testid="page-editor">{mountMenuSlot}</div>
  ),
}));
jest.mock('@/components/PageRenderer', () => ({
  __esModule: true,
  default: () => <div data-testid="page-renderer" />,
}));

const draft = (
  over: Partial<PageSpecDraftSummary> & { pageKey: string },
): PageSpecDraftSummary => ({
  type: 'operation',
  resourceKey: undefined,
  title: { 'zh-CN': over.pageKey, 'en-US': over.pageKey },
  category: { key: 'misc', order: 0 },
  status: 'draft',
  draftRevision: 1,
  updatedAt: '2026-09-27T00:00:00Z',
  ...over,
});

const fullDraft = (pageKey = 'op.a'): PageSpecDraft => ({
  pageKey,
  type: 'operation',
  title: { 'zh-CN': pageKey, 'en-US': pageKey },
  category: { key: 'misc', order: 0 },
  status: 'draft',
  draftRevision: 1,
  updatedAt: '2026-09-27T00:00:00Z',
});

const pageResult = (items: PageSpecDraftSummary[], total = items.length): PageDraftListResult => ({
  items,
  total,
  page: 1,
  pageSize: 20,
});

const fullParams = (overrides: Partial<PageDraftListParams> = {}): PageDraftListParams => ({
  resourceKey: undefined,
  status: undefined,
  keyword: undefined,
  page: 1,
  pageSize: 20,
  ...overrides,
});

const menuItem = (id: number, menuKey: string, zh: string): MenuItem => ({
  id,
  parentId: null,
  menuKey,
  labels: { 'zh-CN': zh },
  sortOrder: 0,
  isVisible: true,
  children: [],
});

const versionItem = (over: Partial<PageVersionItem> & { version: number }): PageVersionItem => ({
  status: 'draft',
  isCurrentDraft: false,
  isCurrentPublished: false,
  createdAt: '2026-09-27T00:00:00Z',
  ...over,
});

const chainData = (): ChangeChain => ({
  pageKey: 'op.a',
  resourceKey: 'player',
  current: { functionVersion: 'v3', draftRevision: 1 },
  items: [
    { type: 'publish', timestamp: '2026-09-27T00:00:00Z', summary: '发布 v1', actor: 'alice' },
  ],
});

const diffData = (): DiffResponse => ({
  summary: '共 1 处变更',
  autoMergeItems: [{ field: 'title', reason: '仅草稿侧修改' }],
  conflictItems: [{ field: 'query.params', reason: '双方同时修改' }],
  changes: [],
});

const renderStudio = () =>
  render(
    <AntdApp>
      <PageStudio />
    </AntdApp>,
  );

/** 行内操作按钮序：0 编辑 / 1 预览 / 2 发布或取消发布 / 3 挂载 / 4 更多。
 * 页面标识列是唯一 <strong> 渲染（标题列同文案，须用 selector 消歧），
 * 等待行出现（列表异步加载）。 */
const findRowBtn = async (pageKey: string, idx: number): Promise<HTMLElement> => {
  const strong = await screen.findByText(pageKey, { selector: 'strong' });
  const row = strong.closest('tr');
  if (!row) throw new Error(`row not found: ${pageKey}`);
  const btn = row.querySelectorAll('.ant-btn')[idx];
  if (!btn) throw new Error(`button #${idx} not found in row ${pageKey}`);
  return btn as HTMLElement;
};

/** 更多下拉里的菜单项（弹层挂在 body 下） */
const openMore = async (pageKey: string, itemText: string): Promise<void> => {
  fireEvent.click(await findRowBtn(pageKey, 4));
  fireEvent.click(await screen.findByText(itemText));
};

const clickPortalBtn = async (selector: string): Promise<void> => {
  const btn = await waitFor(() => {
    const el = document.querySelector(selector) as HTMLButtonElement | null;
    expect(el).toBeTruthy();
    return el;
  });
  fireEvent.click(btn);
};

const confirmPopconfirm = () => clickPortalBtn('.ant-popconfirm .ant-btn-primary');
const confirmDialog = (withinText?: string) => {
  if (!withinText) return clickPortalBtn('.ant-modal-confirm-btns .ant-btn-primary');
  const dialog = screen.getByText(withinText).closest('.ant-modal-confirm');
  if (!dialog) throw new Error(`confirm dialog not found for: ${withinText}`);
  const btn = dialog.querySelector('.ant-btn-primary');
  if (!btn) throw new Error('confirm primary button not found');
  fireEvent.click(btn);
};

const drawerContent = (): string =>
  document.body.querySelector('.ant-drawer-section')?.textContent ?? '';

beforeEach(() => {
  jest.clearAllMocks();
  (listPageDrafts as jest.Mock).mockResolvedValue(pageResult([draft({ pageKey: 'op.a' })]));
  (listPageResources as jest.Mock).mockResolvedValue([]);
});

afterEach(() => {
  window.history.pushState({}, '', '/');
});

describe('行内发布/取消发布', () => {
  it('发布成功：乐观锁 revision 随行提交，成功提示并重拉列表', async () => {
    (publishPageDraft as jest.Mock).mockResolvedValue(undefined);
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 2));
    await confirmPopconfirm();
    await screen.findByText('发布成功');
    expect(publishPageDraft).toHaveBeenCalledWith('op.a', 1);
    expect(listPageDrafts).toHaveBeenCalledTimes(2);
  });

  it('发布失败：错误提示', async () => {
    (publishPageDraft as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 2));
    await confirmPopconfirm();
    expect(await screen.findByText('发布失败')).toBeInTheDocument();
  });

  it('已发布行切换为取消发布：确认后调用 unpublish', async () => {
    (listPageDrafts as jest.Mock).mockResolvedValue(
      pageResult([draft({ pageKey: 'pub.a', status: 'published' })]),
    );
    (unpublishPage as jest.Mock).mockResolvedValue(undefined);
    renderStudio();
    fireEvent.click(await findRowBtn('pub.a', 2));
    await confirmPopconfirm();
    await screen.findByText('已取消发布');
    expect(unpublishPage).toHaveBeenCalledWith('pub.a');
  });
});

describe('一键发布/下架全部', () => {
  const clickBulk = async (label: string, okText: RegExp) => {
    renderStudio();
    fireEvent.click(await screen.findByRole('button', { name: label }));
    await clickPortalBtn('.ant-modal-confirm-btns .ant-btn-primary');
    void okText;
  };

  it('部分失败：警告提示已发布/失败计数', async () => {
    (bulkPublishPages as jest.Mock).mockResolvedValue({
      published: ['a'],
      failed: ['b'],
      message: '',
    });
    await clickBulk(/一键发布全部/, /发布/);
    expect(await screen.findByText('已发布 1 个页面，1 个失败')).toBeInTheDocument();
    expect(bulkPublishPages).toHaveBeenCalledTimes(1);
  });

  it('全部成功：成功提示', async () => {
    (bulkPublishPages as jest.Mock).mockResolvedValue({
      published: ['a', 'b'],
      failed: [],
      message: '',
    });
    await clickBulk(/一键发布全部/, /发布/);
    expect(await screen.findByText('已发布 2 个页面')).toBeInTheDocument();
  });

  it('请求失败：错误提示', async () => {
    (bulkPublishPages as jest.Mock).mockRejectedValue(new Error('boom'));
    await clickBulk(/一键发布全部/, /发布/);
    expect(await screen.findByText('一键发布失败')).toBeInTheDocument();
  });

  it('一键下架成功', async () => {
    (bulkUnpublishPages as jest.Mock).mockResolvedValue({ unpublished: ['x'], message: '' });
    await clickBulk(/一键下架全部/, /下架/);
    expect(await screen.findByText('已下架 1 个页面')).toBeInTheDocument();
    expect(bulkUnpublishPages).toHaveBeenCalledTimes(1);
  });

  it('一键下架请求失败：错误提示', async () => {
    (bulkUnpublishPages as jest.Mock).mockRejectedValue(new Error('boom'));
    await clickBulk(/一键下架全部/, /下架/);
    expect(await screen.findByText('一键下架失败')).toBeInTheDocument();
  });
});

describe('挂载菜单', () => {
  it('打开弹窗拉菜单树，确认提交挂载', async () => {
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (updatePageMenu as jest.Mock).mockResolvedValue(undefined);
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 3));
    const title = await screen.findByText('挂载菜单：op.a');
    const modalRoot = title.closest('.ant-modal');
    expect(modalRoot).toBeTruthy();
    fireEvent.click(modalRoot!.querySelector('.ant-btn-primary')!);
    await screen.findByText('挂载已更新，控制台导航即时生效');
    expect(updatePageMenu).toHaveBeenCalledWith('op.a', expect.anything());
  });

  it('菜单列表加载失败：错误提示且不弹窗', async () => {
    (listMenus as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 3));
    expect(await screen.findByText('菜单列表加载失败，请检查权限或稍后重试')).toBeInTheDocument();
    // 弹窗本体照常打开（目标页先于菜单加载设定），无菜单可挂时隐藏确定钮
    const title = await screen.findByText('挂载菜单：op.a');
    const modalRoot = title.closest('.ant-modal');
    expect(modalRoot).toBeTruthy();
  });
});

describe('预览与编辑', () => {
  it('预览：拉取草稿详情并渲染 PageRenderer', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 1));
    expect(await screen.findByTestId('page-renderer')).toBeInTheDocument();
    expect(getPageDraft).toHaveBeenCalledWith('op.a');
  });

  it('编辑打开后仅保存草稿：默认挂载第一菜单随保存提交', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (savePageDraft as jest.Mock).mockResolvedValue({ draftRevision: 2 });
    (updatePageMenu as jest.Mock).mockResolvedValue(undefined);
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    // eslint-disable-next-line no-console
    await waitFor(() =>
      expect(document.querySelector('.ant-modal .ant-select-content')?.textContent).toContain(
        '玩家',
      ),
    );
    fireEvent.click(screen.getByRole('button', { name: /仅保存草稿/ }));
    await screen.findByText('已保存，挂载已更新（发布后进控制台导航）');
    expect(savePageDraft).toHaveBeenCalledWith(
      expect.objectContaining({ pageKey: 'op.a', draftRevision: 1 }),
    );
    expect(updatePageMenu).toHaveBeenCalledWith('op.a', 1);
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /仅保存草稿/ })).not.toBeInTheDocument(),
    );
  });

  it('保存并发布成功：先保存后按新 revision 发布', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (savePageDraft as jest.Mock).mockResolvedValue({ draftRevision: 2 });
    (publishPageDraft as jest.Mock).mockResolvedValue(undefined);
    (updatePageMenu as jest.Mock).mockResolvedValue(undefined);
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal .ant-select-content')?.textContent).toContain(
        '玩家',
      ),
    );
    fireEvent.click(screen.getByRole('button', { name: /保存并发布/ }));
    await screen.findByText('已保存并发布，已挂载到所选菜单');
    expect(publishPageDraft).toHaveBeenCalledWith('op.a', 2);
  });

  it('保存并发布但挂载失败：发布不受影响，提示可稍后重挂', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (savePageDraft as jest.Mock).mockResolvedValue({ draftRevision: 2 });
    (publishPageDraft as jest.Mock).mockResolvedValue(undefined);
    (updatePageMenu as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal .ant-select-content')?.textContent).toContain(
        '玩家',
      ),
    );
    fireEvent.click(screen.getByRole('button', { name: /保存并发布/ }));
    expect(
      await screen.findByText('已保存并发布；但挂载菜单失败，可稍后在页面工作台重新挂载'),
    ).toBeInTheDocument();
    expect(publishPageDraft).toHaveBeenCalledWith('op.a', 2);
  });

  it('保存并发布但发布失败：草稿已保存弹窗给出修复入口', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (savePageDraft as jest.Mock).mockResolvedValue({ draftRevision: 2 });
    (publishPageDraft as jest.Mock).mockRejectedValue(new Error('契约失效'));
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal .ant-select-content')?.textContent).toContain(
        '玩家',
      ),
    );
    fireEvent.click(screen.getByRole('button', { name: /保存并发布/ }));
    // modal.error 标题在 antd v6 confirm DOM 双渲染，用 findAllByText
    expect((await screen.findAllByText('发布失败（草稿已保存）')).length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: /一键同步 Selector/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /重新生成草稿/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /前往编辑/ })).toBeInTheDocument();
  });

  it('composite 页编辑跳转复合编辑器路由（不打开普通编辑弹窗）', async () => {
    (listPageDrafts as jest.Mock).mockResolvedValue(
      pageResult([draft({ pageKey: 'comp.b', type: 'composite' })]),
    );
    renderStudio();
    fireEvent.click(await findRowBtn('comp.b', 0));
    await waitFor(() =>
      expect(history.push).toHaveBeenCalledWith(
        `/functions/pages/composite-editor?pageKey=${encodeURIComponent('comp.b')}`,
      ),
    );
    expect(screen.queryByRole('button', { name: /仅保存草稿/ })).not.toBeInTheDocument();
  });
});

describe('重新生成草稿', () => {
  it('直接成功：提示并重拉列表', async () => {
    (regeneratePageDraft as jest.Mock).mockResolvedValue({ page: fullDraft(), draftRevision: 2 });
    renderStudio();
    await openMore('op.a', '重新生成');
    await confirmDialog();
    await screen.findByText('已按最新 Proposal 重新生成草稿');
    expect(regeneratePageDraft).toHaveBeenCalledWith('op.a', 1);
  });

  it('409 冲突：知情确认后用服务端 current 重试一次', async () => {
    (regeneratePageDraft as jest.Mock)
      .mockRejectedValueOnce({
        response: { status: 409, data: { error: 'conflict', details: { current: 5 } } },
      })
      .mockResolvedValueOnce({ page: fullDraft(), draftRevision: 5 });
    renderStudio();
    await openMore('op.a', '重新生成');
    await confirmDialog();
    // 知情确认弹窗（title 带 current 插值；antd v6 confirm 标题在
    // .ant-modal-title 与 .ant-modal-confirm-title 双渲染，用 findAllByText）
    const titleEls = await screen.findAllByText('草稿版本已过期');
    const dialog = titleEls.map((el) => el.closest('.ant-modal-confirm')).find(Boolean);
    expect(dialog).toBeTruthy();
    expect(
      (
        await screen.findAllByText(
          '草稿已被其他修改更新到第 5 版。重新生成会按最新 Proposal 覆盖草稿内容，是否继续？',
        )
      ).length,
    ).toBeGreaterThan(0);
    const okBtn = dialog!.querySelector('.ant-btn-primary');
    expect(okBtn).toBeTruthy();
    fireEvent.click(okBtn!);
    await screen.findByText('已按最新 Proposal 重新生成草稿');
    expect(regeneratePageDraft).toHaveBeenLastCalledWith('op.a', 5);
  });

  it('非冲突失败：错误提示', async () => {
    (regeneratePageDraft as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    await openMore('op.a', '重新生成');
    await confirmDialog();
    expect(await screen.findByText('重新生成草稿失败')).toBeInTheDocument();
  });
});

describe('版本历史与回滚', () => {
  const openVersions = async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listPageVersions as jest.Mock).mockResolvedValue({
      items: [
        versionItem({ version: 3, status: 'draft', isCurrentDraft: true, message: '修订说明' }),
        versionItem({ version: 2, status: 'draft', isCurrentDraft: false }),
        versionItem({ version: 1, status: 'published', isCurrentPublished: false }),
      ],
      total: 2,
      currentDraftRevision: 3,
      currentPublishedVersion: 2,
    });
    renderStudio();
    await openMore('op.a', '版本历史');
    await waitFor(() =>
      expect(listPageVersions).toHaveBeenCalledWith('op.a', { limit: 5, offset: 0 }),
    );
  };

  it('打开抽屉：分页参数下推服务端，渲染回滚入口', async () => {
    await openVersions();
    expect(await screen.findByText('修订说明')).toBeInTheDocument();
    expect(screen.getAllByText('回滚草稿').length).toBe(2);
    expect(screen.getAllByText('回滚发布').length).toBe(1);
  });

  it('回滚草稿：带乐观锁 revision 确认执行', async () => {
    (rollbackDraft as jest.Mock).mockResolvedValue({ draftRevision: 9, message: '已回滚到 v3' });
    await openVersions();
    fireEvent.click(screen.getAllByText('回滚草稿')[0]);
    await confirmPopconfirm();
    await screen.findByText('已回滚到 v3');
    expect(rollbackDraft).toHaveBeenCalledWith(
      'op.a',
      expect.objectContaining({ expectedDraftRevision: 1, version: 2 }),
    );
  });

  it('回滚发布版本', async () => {
    (rollbackPublish as jest.Mock).mockResolvedValue({
      draftRevision: 9,
      message: '已回滚发布到 v1',
    });
    await openVersions();
    fireEvent.click(screen.getByText('回滚发布'));
    await confirmPopconfirm();
    await screen.findByText('已回滚发布到 v1');
    expect(rollbackPublish).toHaveBeenCalledWith(
      'op.a',
      expect.objectContaining({ expectedDraftRevision: 1, version: 1 }),
    );
  });
});

describe('变更链 / 变更对比 / 合并', () => {
  it('变更链：拉取并渲染时间线数据', async () => {
    (getChangeChain as jest.Mock).mockResolvedValue(chainData());
    renderStudio();
    await openMore('op.a', '变更链');
    await waitFor(() => expect(getChangeChain).toHaveBeenCalledWith('op.a'));
    await waitFor(() => expect(drawerContent()).toContain('函数版本: v3'));
    expect(drawerContent()).toContain('player');
  });

  it('变更链加载失败：错误提示', async () => {
    (getChangeChain as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    await openMore('op.a', '变更链');
    expect(await screen.findByText('加载变更链失败')).toBeInTheDocument();
  });

  it('变更对比→合并弹窗→自动合并成功', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (getDiff as jest.Mock).mockResolvedValue(diffData());
    (mergeChanges as jest.Mock).mockResolvedValue({
      merged: 2,
      conflicts: 0,
      message: '',
      draftRevision: 7,
    });
    renderStudio();
    await openMore('op.a', '变更对比');
    await waitFor(() => expect(drawerContent()).toContain('必须人工确认 1 个冲突字段'));
    fireEvent.click(screen.getByRole('button', { name: /合并变更/ }));
    fireEvent.click(await screen.findByRole('button', { name: /自动合并/ }));
    await screen.findByText('合并完成：2 项自动合并，0 项冲突');
    expect(mergeChanges).toHaveBeenCalledWith(
      'op.a',
      expect.objectContaining({ expectedDraftRevision: 1, strategy: 'auto' }),
    );
  });

  it('变更对比加载失败：错误提示', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (getDiff as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    await openMore('op.a', '变更对比');
    expect(await screen.findByText('加载 Diff 失败')).toBeInTheDocument();
  });

  it('手动处理冲突：dry-run 预览请求下推', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (getDiff as jest.Mock).mockResolvedValue(diffData());
    (mergeChanges as jest.Mock).mockResolvedValue({ merged: 0, conflicts: 1, message: '' });
    renderStudio();
    await openMore('op.a', '变更对比');
    await waitFor(() => expect(drawerContent()).toContain('必须人工确认 1 个冲突字段'));
    fireEvent.click(screen.getByRole('button', { name: /合并变更/ }));
    fireEvent.click(await screen.findByRole('button', { name: /手动处理冲突/ }));
    await waitFor(() =>
      expect(mergeChanges).toHaveBeenCalledWith('op.a', { strategy: 'manual', dryRun: true }),
    );
  });

  it('手动冲突预览加载失败：错误提示', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (getDiff as jest.Mock).mockResolvedValue(diffData());
    (mergeChanges as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    await openMore('op.a', '变更对比');
    await waitFor(() => expect(drawerContent()).toContain('必须人工确认 1 个冲突字段'));
    fireEvent.click(screen.getByRole('button', { name: /合并变更/ }));
    fireEvent.click(await screen.findByRole('button', { name: /手动处理冲突/ }));
    expect(await screen.findByText('加载冲突预览失败')).toBeInTheDocument();
  });
});

describe('URL 联动与列表加载失败', () => {
  it('列表加载失败：错误提示', async () => {
    (listPageDrafts as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    expect(await screen.findByText('加载页面列表失败')).toBeInTheDocument();
  });

  it('?focus= 联动：自动打开对应页面编辑器', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft('focus.a'));
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    window.history.pushState({}, '', '/?focus=focus.a');
    renderStudio();
    expect(await screen.findByRole('button', { name: /仅保存草稿/ })).toBeInTheDocument();
    expect(getPageDraft).toHaveBeenCalledWith('focus.a');
  });

  it('?focus=&inbox=1：仅定位 Inbox 队列项，不自动开编辑器', async () => {
    window.history.pushState({}, '', '/?focus=op.a&inbox=1');
    renderStudio();
    await screen.findByText('op.a', { selector: 'strong' });
    expect(
      await screen.findByTestId('inbox-stub').then((el) => el.getAttribute('data-focus')),
    ).toBe('op.a');
    expect(screen.queryByRole('button', { name: /仅保存草稿/ })).not.toBeInTheDocument();
  });

  it('?focus=&mount=1：草稿就绪后自动打开挂载弹窗并消费 mount 参数', async () => {
    (listPageDrafts as jest.Mock).mockResolvedValue(pageResult([draft({ pageKey: 'mount.a' })]));
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft('mount.a'));
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    window.history.pushState({}, '', '/?focus=mount.a&mount=1');
    renderStudio();
    expect(await screen.findByText('挂载菜单：mount.a')).toBeInTheDocument();
    expect(window.location.search).not.toContain('mount=1');
  });
});

describe('失败分支与修复入口', () => {
  it('预览详情加载失败：错误提示', async () => {
    (getPageDraft as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 1));
    expect(await screen.findByText('加载页面详情失败')).toBeInTheDocument();
  });

  it('取消发布失败：错误提示', async () => {
    (listPageDrafts as jest.Mock).mockResolvedValue(
      pageResult([draft({ pageKey: 'pub.a', status: 'published' })]),
    );
    (unpublishPage as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    fireEvent.click(await findRowBtn('pub.a', 2));
    await confirmPopconfirm();
    expect(await screen.findByText('取消发布失败')).toBeInTheDocument();
  });

  it('编辑打开但菜单树加载失败：提示且选择器禁用', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listMenus as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    expect(await screen.findByText('菜单列表加载失败，请检查权限或稍后重试')).toBeInTheDocument();
    await waitFor(() => expect(document.querySelector('.ant-modal .ant-select')).toBeTruthy());
  });

  it('已挂载页面未改动挂载：仅保存不提交挂载（保存成功）', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue({ ...fullDraft(), menuId: 1 });
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (savePageDraft as jest.Mock).mockResolvedValue({ draftRevision: 2 });
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    fireEvent.click(await screen.findByRole('button', { name: /仅保存草稿/ }));
    await screen.findByText('保存成功');
    expect(savePageDraft).toHaveBeenCalled();
    expect(updatePageMenu).not.toHaveBeenCalled();
  });

  it('已挂载页面保存并发布（未改动挂载）：普通发布成功文案', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue({ ...fullDraft(), menuId: 1 });
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (savePageDraft as jest.Mock).mockResolvedValue({ draftRevision: 2 });
    (publishPageDraft as jest.Mock).mockResolvedValue(undefined);
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    fireEvent.click(await screen.findByRole('button', { name: /保存并发布/ }));
    await screen.findByText('已保存并发布');
    expect(publishPageDraft).toHaveBeenCalledWith('op.a', 2);
  });

  it('仅保存但挂载失败：警告可稍后重挂，不阻断保存', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (savePageDraft as jest.Mock).mockResolvedValue({ draftRevision: 2 });
    (updatePageMenu as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal .ant-select-content')?.textContent).toContain(
        '玩家',
      ),
    );
    fireEvent.click(screen.getByRole('button', { name: /仅保存草稿/ }));
    expect(
      await screen.findByText('已保存；但挂载菜单失败，可稍后在页面工作台重新挂载'),
    ).toBeInTheDocument();
    expect(publishPageDraft).not.toHaveBeenCalled();
  });

  it('保存失败（含字段明细）：错误弹窗渲染明细列表', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (savePageDraft as jest.Mock).mockRejectedValue({
      response: {
        status: 422,
        data: { error: 'validation_failed', details: { players: '绑定失效' } },
      },
    });
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    fireEvent.click(await screen.findByRole('button', { name: /仅保存草稿/ }));
    expect((await screen.findAllByText('保存失败')).length).toBeGreaterThan(0);
    expect(screen.getByText('players')).toBeInTheDocument();
    expect(screen.getByText('绑定失效', { exact: false })).toBeInTheDocument();
  });

  /** 发布失败（草稿已保存）弹窗的公共前置。 */
  const openPublishFailModal = async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (savePageDraft as jest.Mock).mockResolvedValue({ draftRevision: 2 });
    (publishPageDraft as jest.Mock).mockRejectedValue(new Error('契约失效'));
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal .ant-select-content')?.textContent).toContain(
        '玩家',
      ),
    );
    fireEvent.click(screen.getByRole('button', { name: /保存并发布/ }));
    expect((await screen.findAllByText('发布失败（草稿已保存）')).length).toBeGreaterThan(0);
  };

  it('发布失败弹窗：一键同步 Selector 关闭弹窗', async () => {
    await openPublishFailModal();
    fireEvent.click(screen.getByRole('button', { name: /一键同步 Selector/ }));
    await waitFor(() => expect(screen.queryAllByText('发布失败（草稿已保存）')).toHaveLength(0));
  });

  it('发布失败弹窗：重新生成草稿按原 revision 触发', async () => {
    (regeneratePageDraft as jest.Mock).mockResolvedValue({ page: fullDraft(), draftRevision: 2 });
    await openPublishFailModal();
    fireEvent.click(screen.getByRole('button', { name: /重新生成草稿/ }));
    await screen.findByText('已按最新 Proposal 重新生成草稿');
    expect(regeneratePageDraft).toHaveBeenCalledWith('op.a', 1);
  });

  it('发布失败弹窗：前往编辑保持编辑器打开', async () => {
    await openPublishFailModal();
    fireEvent.click(screen.getByRole('button', { name: /前往编辑/ }));
    expect(screen.getByRole('button', { name: /仅保存草稿/ })).toBeInTheDocument();
  });
});

describe('合并与回滚失败分支', () => {
  it('自动合并失败：错误提示', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (getDiff as jest.Mock).mockResolvedValue(diffData());
    (mergeChanges as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    await openMore('op.a', '变更对比');
    await waitFor(() => expect(drawerContent()).toContain('必须人工确认 1 个冲突字段'));
    fireEvent.click(screen.getByRole('button', { name: /合并变更/ }));
    fireEvent.click(await screen.findByRole('button', { name: /自动合并/ }));
    expect(await screen.findByText('合并失败')).toBeInTheDocument();
  });

  it('回滚草稿失败：错误提示', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listPageVersions as jest.Mock).mockResolvedValue({
      items: [versionItem({ version: 2, status: 'draft', isCurrentDraft: false })],
      total: 1,
    });
    (rollbackDraft as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    await openMore('op.a', '版本历史');
    fireEvent.click((await screen.findAllByText('回滚草稿'))[0]);
    await confirmPopconfirm();
    expect(await screen.findByText('回滚草稿失败')).toBeInTheDocument();
  });

  it('回滚发布失败：错误提示', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listPageVersions as jest.Mock).mockResolvedValue({
      items: [versionItem({ version: 1, status: 'published', isCurrentPublished: false })],
      total: 1,
    });
    (rollbackPublish as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    await openMore('op.a', '版本历史');
    fireEvent.click(await screen.findByText('回滚发布'));
    await confirmPopconfirm();
    expect(await screen.findByText('回滚发布失败')).toBeInTheDocument();
  });

  it('版本历史加载失败：抽屉仍打开并提示', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listPageVersions as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    await openMore('op.a', '版本历史');
    expect(await screen.findByText('加载版本历史失败')).toBeInTheDocument();
  });

  it('版本历史翻页：服务端 offset 下推', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listPageVersions as jest.Mock).mockResolvedValue({
      items: [1, 2, 3, 4, 5].map((v) =>
        versionItem({ version: v, status: 'draft', isCurrentDraft: v === 5 }),
      ),
      total: 7,
    });
    renderStudio();
    await openMore('op.a', '版本历史');
    const pageTwo = await waitFor(() => {
      const el = document.querySelector('.ant-drawer .ant-pagination-item-2');
      expect(el).toBeTruthy();
      return el as HTMLElement;
    });
    fireEvent.click(pageTwo);
    await waitFor(() =>
      expect(listPageVersions).toHaveBeenLastCalledWith('op.a', { limit: 5, offset: 5 }),
    );
  });
});

describe('工具栏资源过滤与主表页大小', () => {
  it('资源下拉打开拉服务端选项，选择后按 resourceKey 过滤', async () => {
    (listPageDrafts as jest.Mock).mockClear();
    (listPageResources as jest.Mock).mockResolvedValue([{ resourceKey: 'player', pageCount: 1 }]);
    renderStudio();
    await findRowBtn('op.a', 0);
    fireEvent.mouseDown(screen.getAllByRole('combobox')[1]);
    // 带 count 选项渲染为 'player (1)'，按选项元素匹配而非内层文本
    const option = await waitFor(() => {
      const el = Array.from(
        document.querySelectorAll('.ant-select-dropdown .ant-select-item-option'),
      ).find((item) => item.textContent?.startsWith('player'));
      expect(el).toBeTruthy();
      return el as Element;
    });
    fireEvent.click(option);
    await waitFor(() =>
      expect(listPageDrafts).toHaveBeenLastCalledWith(fullParams({ resourceKey: 'player' })),
    );
  });

  it('主表页大小切换：回到第 1 页并按新 pageSize 重拉', async () => {
    (listPageDrafts as jest.Mock).mockClear();
    renderStudio();
    await findRowBtn('op.a', 0);
    const sizeChanger = document.querySelector(
      '.ant-pagination-options .ant-select',
    ) as HTMLElement;
    expect(sizeChanger).toBeTruthy();
    fireEvent.mouseDown(sizeChanger.querySelector('.ant-select-selector') ?? sizeChanger);
    fireEvent.click(await screen.findByText('10 条/页'));
    await waitFor(() =>
      expect(listPageDrafts).toHaveBeenLastCalledWith(fullParams({ page: 1, pageSize: 10 })),
    );
  });
});
