/**
 * PageStudio 工作台主页（index.tsx）回调收口单测（round-6 覆盖率巡检）：
 * 补 studioActions.test.tsx 未触达的真实 UI 面——抽屉/弹窗关闭回调、
 * 挂载提交失败、mount=1 失效链接静默、版本列表空字段兜底、合并无效
 * revision 守卫、编辑器解除挂载两态文案、无草稿保存守卫、绑定过期
 * 一键同步入口、批量操作空结果兜底、状态过滤清空与 draftRevision=0
 * 兜底；手动合并改为真实渲染 MergeConflictModal（该组件此前全库无
 * 任何测试，桩边界解除后触达 handleManualMergeSubmit 全链）。
 * 服务 API 全部 mock（数据面），子组件不打桩（与 studioActions 相同，
 * 仅 PageEditor/PageRenderer 重组件叶子与 SelectorSyncReportModal——
 * 其已有独立套件——按既有口径替换）。
 *
 * 剩余未覆盖分支登记（UI 不可达/桩边界，不写假用例）：
 * - 425-430 updateSelectedDraftSpec（PageEditor 桩不产生 spec 变更事件）；
 * - 694-698 / 1157-1159 handleSyncSelectorsApplied（SelectorSyncReportModal
 *   桩边界，组件自身已有独立套件覆盖）；
 * - 830-837 / 902-903 / 928-929 / 1175 各 handler 的「无页面」守卫——
 *   入口按钮只在选中页面后渲染，UI 无法触达。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import PageStudio from '../index';
import {
  bulkPublishPages,
  bulkUnpublishPages,
  getPageDraft,
  listPageDrafts,
  listPageResources,
  listPageVersions,
  publishPageDraft,
  savePageDraft,
  updatePageMenu,
} from '@/services/api/pages';
import { listMenus, type MenuItem } from '@/services/api/menu';
import {
  getChangeChain,
  getDiff,
  mergeChanges,
  type ChangeChain,
  type DiffResponse,
  type MergeResponse,
} from '@/services/api/versioning';
import type { PageDraftListParams, PageDraftListResult } from '@/services/api/pages';
import type { PageSpecDraft, PageSpecDraftSummary, PageVersionItem } from '@/types/dashboard';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(45000);

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
// MergeConflictModal 本套件刻意不打桩：真实渲染以触达主页手动合并提交链
jest.mock('@/components/SelectorSync/SelectorSyncReportModal', () => ({
  __esModule: true,
  default: () => null,
}));
// 编辑器/渲染器是重组件叶子且与本页回调契约无关（EditorModal.test.tsx 先例）
jest.mock('@/components/PageEditor', () => ({
  __esModule: true,
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

/** dry-run 冲突预览（真实 MergeConflictModal 渲染输入） */
const conflictPreview = (): MergeResponse => ({
  merged: 0,
  conflicts: 1,
  message: '检测到 1 处冲突',
  autoMergeItems: [{ field: 'title', reason: '仅草稿侧修改' }],
  conflictItems: [
    {
      field: 'query.params',
      reason: '双方同时修改',
      draftValue: { a: 1 },
      latestValue: { a: 2 },
    },
  ],
});

const renderStudio = () =>
  render(
    <AntdApp>
      <PageStudio />
    </AntdApp>,
  );

/** 行内操作按钮序：0 编辑 / 1 预览 / 2 发布或取消发布 / 3 挂载 / 4 更多。 */
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

const drawerContent = (): string =>
  document.body.querySelector('.ant-drawer-section')?.textContent ?? '';

const drawerClosed = async (): Promise<void> => {
  await waitFor(() => expect(document.querySelector('.ant-drawer-open')).toBeNull());
};

/** 按 placeholder 定位 antd v6 Select 根节点（placeholder 是 span 不是 input 属性） */
const selectByPlaceholder = (text: string): HTMLElement => {
  const ph = Array.from(document.querySelectorAll('.ant-select-placeholder')).find(
    (el) => el.textContent === text,
  );
  if (!ph) throw new Error(`select with placeholder not found: ${text}`);
  return ph.closest('.ant-select') as HTMLElement;
};

/** 打开下拉并点选指定文案的选项 */
const pickOption = async (select: HTMLElement, label: string): Promise<void> => {
  fireEvent.mouseDown(select.querySelector('input.ant-select-input')!);
  const option = await waitFor(() => {
    const el = Array.from(
      document.querySelectorAll(
        '.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option',
      ),
    ).find((o) => o.textContent === label);
    expect(el).toBeTruthy();
    return el as HTMLElement;
  });
  fireEvent.click(option);
};

const modalRootOf = (titleText: string): HTMLElement => {
  const title = screen.getByText(titleText);
  const root = title.closest('.ant-modal');
  if (!root) throw new Error(`modal not found for: ${titleText}`);
  return root as HTMLElement;
};

beforeEach(() => {
  jest.clearAllMocks();
  (listPageDrafts as jest.Mock).mockResolvedValue(pageResult([draft({ pageKey: 'op.a' })]));
  (listPageResources as jest.Mock).mockResolvedValue([]);
});

afterEach(() => {
  window.history.pushState({}, '', '/');
});

describe('抽屉/弹窗关闭回调', () => {
  it('预览抽屉：右上角关闭触达 onClose', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 1));
    expect(await screen.findByTestId('page-renderer')).toBeInTheDocument();
    fireEvent.click(document.querySelector('.ant-drawer-close')!);
    await drawerClosed();
  });

  it('编辑弹窗：底部「取消」触达 onClose', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    fireEvent.click(await screen.findByRole('button', { name: '取消' }));
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /仅保存草稿/ })).not.toBeInTheDocument(),
    );
  });

  it('版本历史抽屉：关闭触达 onClose', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listPageVersions as jest.Mock).mockResolvedValue({
      items: [versionItem({ version: 1, status: 'draft', message: '唯一修订备注' })],
      total: 1,
      currentDraftRevision: 1,
      currentPublishedVersion: 0,
    });
    renderStudio();
    await openMore('op.a', '版本历史');
    await screen.findByText('唯一修订备注');
    fireEvent.click(document.querySelector('.ant-drawer-close')!);
    await drawerClosed();
  });

  it('变更链抽屉：关闭触达 onClose', async () => {
    (getChangeChain as jest.Mock).mockResolvedValue(chainData());
    renderStudio();
    await openMore('op.a', '变更链');
    await waitFor(() => expect(drawerContent()).toContain('函数版本: v3'));
    fireEvent.click(document.querySelector('.ant-drawer-close')!);
    await drawerClosed();
  });

  it('变更对比抽屉：关闭触达 onClose', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (getDiff as jest.Mock).mockResolvedValue(diffData());
    renderStudio();
    await openMore('op.a', '变更对比');
    await waitFor(() => expect(drawerContent()).toContain('必须人工确认 1 个冲突字段'));
    fireEvent.click(document.querySelector('.ant-drawer-close')!);
    await drawerClosed();
  });

  it('合并弹窗：底部「取消」触达 onCancel', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (getDiff as jest.Mock).mockResolvedValue(diffData());
    renderStudio();
    await openMore('op.a', '变更对比');
    await waitFor(() => expect(drawerContent()).toContain('必须人工确认 1 个冲突字段'));
    fireEvent.click(screen.getByRole('button', { name: /合并变更/ }));
    fireEvent.click(await screen.findByRole('button', { name: /自动合并/ }));
    fireEvent.click(screen.getByRole('button', { name: '取消' }));
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /自动合并/ })).not.toBeInTheDocument(),
    );
  });

  it('挂载弹窗：取消关闭且不提交挂载', async () => {
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 3));
    const modalRoot = modalRootOf('挂载菜单：op.a');
    fireEvent.click(modalRoot.querySelector('.ant-modal-footer .ant-btn')!);
    await waitFor(() => expect(screen.queryByText('挂载菜单：op.a')).not.toBeInTheDocument());
    expect(updatePageMenu).not.toHaveBeenCalled();
  });
});

describe('挂载菜单失败与失效链接', () => {
  it('挂载提交失败：错误提示且弹窗保留', async () => {
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (updatePageMenu as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 3));
    // 先 await 弹窗标题（间隔让 listMenus 解析挂载菜单树，empty 翻转为 false）
    await screen.findByText('挂载菜单：op.a');
    const modalRoot = modalRootOf('挂载菜单：op.a');
    fireEvent.click(modalRoot.querySelector('.ant-btn-primary')!);
    expect(await screen.findByText('更新挂载失败')).toBeInTheDocument();
    expect(screen.getByText('挂载菜单：op.a')).toBeInTheDocument();
  });

  it('?focus=&mount=1 但页面不在列表：静默不弹窗且不消费参数', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft('ghost.a'));
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    window.history.pushState({}, '', '/?focus=ghost.a&mount=1');
    renderStudio();
    await screen.findByText('op.a', { selector: 'strong' });
    // 草稿就绪后仍无 ghost.a（未 accept）：不打开挂载弹窗、不清理 mount 参数
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /仅保存草稿/ })).toBeInTheDocument(),
    );
    expect(screen.queryByText('挂载菜单：ghost.a')).not.toBeInTheDocument();
    expect(window.location.search).toContain('mount=1');
  });
});

describe('版本历史空字段兜底', () => {
  it('items/total 缺省：空表渲染不崩溃，无回滚入口', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listPageVersions as jest.Mock).mockResolvedValue({});
    renderStudio();
    await openMore('op.a', '版本历史');
    await waitFor(() =>
      expect(listPageVersions).toHaveBeenCalledWith('op.a', { limit: 5, offset: 0 }),
    );
    expect(document.querySelector('.ant-drawer-open')).not.toBeNull();
    expect(screen.queryByText('回滚草稿')).not.toBeInTheDocument();
    expect(screen.queryByText('回滚发布')).not.toBeInTheDocument();
  });
});

describe('合并守卫与手动合并全链', () => {
  it('详情与 Diff 均失败后合并：无效 revision 守卫拦截', async () => {
    (getPageDraft as jest.Mock).mockRejectedValue(new Error('boom'));
    (getDiff as jest.Mock).mockRejectedValue(new Error('boom'));
    renderStudio();
    await openMore('op.a', '变更对比');
    expect(await screen.findByText('加载 Diff 失败')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /合并变更/ }));
    fireEvent.click(await screen.findByRole('button', { name: /自动合并/ }));
    expect(await screen.findByText('页面草稿版本无效，请刷新后重试')).toBeInTheDocument();
    expect(mergeChanges).not.toHaveBeenCalled();
  });

  it('手动合并（真实 MergeConflictModal）：选最新值+原因提交', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (getDiff as jest.Mock).mockResolvedValue(diffData());
    (mergeChanges as jest.Mock)
      .mockResolvedValueOnce(conflictPreview())
      .mockResolvedValueOnce({ merged: 1, conflicts: 0, message: '', draftRevision: 7 });
    renderStudio();
    await openMore('op.a', '变更对比');
    await waitFor(() => expect(drawerContent()).toContain('必须人工确认 1 个冲突字段'));
    fireEvent.click(screen.getByRole('button', { name: /合并变更/ }));
    fireEvent.click(await screen.findByRole('button', { name: /手动处理冲突/ }));
    // 真实弹窗渲染：自动合并卡 + 冲突卡（draft/latest 双值）。
    // 身后的 DiffDrawer 也渲染 query.params 文案，断言限定在弹窗根内。
    expect(await screen.findByText('手动解决冲突')).toBeInTheDocument();
    expect(await screen.findByText('将自动合并 1 个展示字段')).toBeInTheDocument();
    const conflictRoot = modalRootOf('手动解决冲突');
    expect(conflictRoot.textContent).toContain('检测到 1 处冲突');
    expect(conflictRoot.textContent).toContain('query.params');
    expect(conflictRoot.textContent).toContain('"a": 1');
    expect(conflictRoot.textContent).toContain('"a": 2');
    // 未选择解决方式时确定钮禁用
    const okBtn = () =>
      Array.from(document.querySelectorAll('.ant-modal-footer .ant-btn')).find((b) =>
        b.textContent?.includes('应用合并结果'),
      ) as HTMLButtonElement;
    expect(okBtn().disabled).toBe(true);
    fireEvent.click(screen.getByText('接受最新 Proposal 值'));
    await waitFor(() => expect(okBtn().disabled).toBe(false));
    fireEvent.change(screen.getByPlaceholderText('可选：记录本次人工合并原因'), {
      target: { value: '  fix conflict  ' },
    });
    fireEvent.click(okBtn());
    await screen.findByText('合并完成：草稿已更新到版本 7');
    expect(mergeChanges).toHaveBeenLastCalledWith(
      'op.a',
      expect.objectContaining({
        strategy: 'manual',
        conflicts: [{ path: 'query.params', acceptNew: true }],
        reason: 'fix conflict',
      }),
    );
    await waitFor(() => expect(screen.queryByText('手动解决冲突')).not.toBeInTheDocument());
  }, 90000);

  it('手动合并提交失败：错误提示且弹窗保持打开（可改后重试）', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (getDiff as jest.Mock).mockResolvedValue(diffData());
    // 首调=dry-run 冲突预览成功；二调=manual 提交被拒（并发修改等）→ catch 分支
    (mergeChanges as jest.Mock)
      .mockResolvedValueOnce(conflictPreview())
      .mockRejectedValueOnce(new Error('revision conflict'));
    renderStudio();
    await openMore('op.a', '变更对比');
    await waitFor(() => expect(drawerContent()).toContain('必须人工确认 1 个冲突字段'));
    fireEvent.click(screen.getByRole('button', { name: /合并变更/ }));
    fireEvent.click(await screen.findByRole('button', { name: /手动处理冲突/ }));
    expect(await screen.findByText('手动解决冲突')).toBeInTheDocument();
    fireEvent.click(screen.getByText('接受最新 Proposal 值'));
    const okBtn = () =>
      Array.from(document.querySelectorAll('.ant-modal-footer .ant-btn')).find((b) =>
        b.textContent?.includes('应用合并结果'),
      ) as HTMLButtonElement;
    await waitFor(() => expect(okBtn().disabled).toBe(false));
    fireEvent.click(okBtn());
    expect(await screen.findByText('手动合并失败')).toBeInTheDocument();
    // 失败路径不关弹窗（可修改后重试），且 dry-run+提交恰好两次调用
    expect(screen.getByText('手动解决冲突')).toBeInTheDocument();
    expect(mergeChanges).toHaveBeenCalledTimes(2);
  }, 90000);

  it('自定义 JSON 非法：错误提示且不提交', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (getDiff as jest.Mock).mockResolvedValue(diffData());
    (mergeChanges as jest.Mock).mockResolvedValueOnce(conflictPreview());
    renderStudio();
    await openMore('op.a', '变更对比');
    await waitFor(() => expect(drawerContent()).toContain('必须人工确认 1 个冲突字段'));
    fireEvent.click(screen.getByRole('button', { name: /合并变更/ }));
    fireEvent.click(await screen.findByRole('button', { name: /手动处理冲突/ }));
    expect(await screen.findByText('手动解决冲突')).toBeInTheDocument();
    fireEvent.click(screen.getByText('自定义 JSON'));
    const jsonArea = await screen.findByPlaceholderText('请输入合法 JSON');
    expect(jsonArea).toHaveValue('{\n  "a": 1\n}');
    fireEvent.change(jsonArea, { target: { value: 'not-json' } });
    const okBtn = Array.from(document.querySelectorAll('.ant-modal-footer .ant-btn')).find((b) =>
      b.textContent?.includes('应用合并结果'),
    ) as HTMLButtonElement;
    fireEvent.click(okBtn);
    expect(await screen.findByText('字段 query.params 的自定义 JSON 非法')).toBeInTheDocument();
    expect(mergeChanges).toHaveBeenCalledTimes(1);
  }, 90000);

  it('手动冲突弹窗：关闭触达 onCancel', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (getDiff as jest.Mock).mockResolvedValue(diffData());
    (mergeChanges as jest.Mock).mockResolvedValueOnce(conflictPreview());
    renderStudio();
    await openMore('op.a', '变更对比');
    await waitFor(() => expect(drawerContent()).toContain('必须人工确认 1 个冲突字段'));
    fireEvent.click(screen.getByRole('button', { name: /合并变更/ }));
    fireEvent.click(await screen.findByRole('button', { name: /手动处理冲突/ }));
    expect(await screen.findByText('手动解决冲突')).toBeInTheDocument();
    // 身后隐藏的 MergeModal 仍在 DOM（destroyOnHidden 未开），关闭钮须限定本弹窗
    fireEvent.click(modalRootOf('手动解决冲突').querySelector('.ant-modal-close')!);
    await waitFor(() => expect(screen.queryByText('手动解决冲突')).not.toBeInTheDocument());
  }, 90000);
});

describe('编辑器保存边界', () => {
  it('详情加载失败仍开编辑器：保存无草稿可提交（静默守卫）', async () => {
    (getPageDraft as jest.Mock).mockRejectedValue(new Error('boom'));
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    expect(await screen.findByText('加载页面详情失败')).toBeInTheDocument();
    fireEvent.click(await screen.findByRole('button', { name: /仅保存草稿/ }));
    expect(savePageDraft).not.toHaveBeenCalled();
  });

  it('绑定过期：Alert 与一键同步 Selector 入口渲染并可点击', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue({
      ...fullDraft(),
      bindingFreshness: [
        {
          bindingId: 'b1',
          status: 'input_schema_stale',
          diagnostic: { code: 'input_schema_stale', severity: 'error', message: '契约漂移' },
        },
      ],
    });
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    expect(
      await screen.findByText('页面绑定与函数契约不一致（发布会校验失败）'),
    ).toBeInTheDocument();
    expect(screen.getByText('b1')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /一键同步 Selector/ }));
    expect(screen.getByRole('button', { name: /一键同步 Selector/ })).toBeInTheDocument();
  });

  const openEditorForUnmount = async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (savePageDraft as jest.Mock).mockResolvedValue({ draftRevision: 2 });
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal .ant-select-content')?.textContent).toContain(
        '玩家',
      ),
    );
    // 清空挂载 TreeSelect（allowClear）→ menuId=null 提交解除挂载
    const treeSelect = document.querySelector('.ant-modal .ant-select') as HTMLElement;
    fireEvent.mouseEnter(treeSelect);
    fireEvent.click(treeSelect.querySelector('.ant-select-clear')!);
  };

  it('解除挂载后仅保存草稿：提交 menuId=null 并提示解除文案', async () => {
    (updatePageMenu as jest.Mock).mockResolvedValue(undefined);
    await openEditorForUnmount();
    fireEvent.click(await screen.findByRole('button', { name: /仅保存草稿/ }));
    expect(await screen.findByText('已保存，已解除菜单挂载')).toBeInTheDocument();
    expect(updatePageMenu).toHaveBeenCalledWith('op.a', null);
  });

  it('解除挂载后保存并发布：发布按新 revision 执行', async () => {
    (updatePageMenu as jest.Mock).mockResolvedValue(undefined);
    (publishPageDraft as jest.Mock).mockResolvedValue(undefined);
    await openEditorForUnmount();
    fireEvent.click(await screen.findByRole('button', { name: /保存并发布/ }));
    expect(await screen.findByText('已保存并发布，已解除菜单挂载')).toBeInTheDocument();
    expect(publishPageDraft).toHaveBeenCalledWith('op.a', 2);
  });

  it('draftRevision=0 兜底：保存按 0 提交乐观锁', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue({ ...fullDraft(), draftRevision: 0 });
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (savePageDraft as jest.Mock).mockResolvedValue({ draftRevision: 1 });
    (updatePageMenu as jest.Mock).mockResolvedValue(undefined);
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    fireEvent.click(await screen.findByRole('button', { name: /仅保存草稿/ }));
    await screen.findByText('已保存，挂载已更新（发布后进控制台导航）');
    expect(savePageDraft).toHaveBeenCalledWith(
      expect.objectContaining({ pageKey: 'op.a', draftRevision: 0 }),
    );
  });

  it('保存失败明细无字段：只渲染原因不渲染字段 code', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
    (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
    (savePageDraft as jest.Mock).mockRejectedValue({
      response: {
        status: 422,
        data: {
          error: 'validation_failed',
          details: [{ field: '', message: '整体校验失败' }],
        },
      },
    });
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    fireEvent.click(await screen.findByRole('button', { name: /仅保存草稿/ }));
    expect((await screen.findAllByText('保存失败')).length).toBeGreaterThan(0);
    expect(screen.getByText('整体校验失败')).toBeInTheDocument();
  });
});

describe('批量操作空结果兜底', () => {
  it('一键发布返回缺省：按 0 计数提示成功', async () => {
    (bulkPublishPages as jest.Mock).mockResolvedValue({ message: '' });
    renderStudio();
    fireEvent.click(await screen.findByRole('button', { name: /一键发布全部/ }));
    await clickPortalBtn('.ant-modal-confirm-btns .ant-btn-primary');
    expect(await screen.findByText('已发布 0 个页面')).toBeInTheDocument();
  });

  it('一键下架返回缺省：按 0 计数提示成功', async () => {
    (bulkUnpublishPages as jest.Mock).mockResolvedValue({ message: '' });
    renderStudio();
    fireEvent.click(await screen.findByRole('button', { name: /一键下架全部/ }));
    await clickPortalBtn('.ant-modal-confirm-btns .ant-btn-primary');
    expect(await screen.findByText('已下架 0 个页面')).toBeInTheDocument();
  });
});

describe('过滤与列表参数', () => {
  it('状态过滤选择后清空：清空臂回到 undefined 下推', async () => {
    renderStudio();
    await screen.findByText('op.a', { selector: 'strong' });
    const statusSelect = selectByPlaceholder('按状态过滤');
    await pickOption(statusSelect, '草稿');
    await waitFor(() =>
      expect(listPageDrafts).toHaveBeenLastCalledWith(fullParams({ status: 'draft' })),
    );
    fireEvent.mouseEnter(statusSelect);
    fireEvent.click(statusSelect.querySelector('.ant-select-clear')!);
    await waitFor(() =>
      expect(listPageDrafts).toHaveBeenLastCalledWith(fullParams({ status: undefined })),
    );
  });
});
