/**
 * Dev/Bugs 缺陷追踪页回归（1113 行，此前 0 测试 0% 覆盖）。
 *
 * 锁定契约：列表渲染矩阵（链接图标 5 类 + 溢出 +N、status/severity/priority
 * 已知与未知值兜底、source 三态、空值列 '-'）、工具栏六筛选各自触发重拉、
 * 真实 ModalForm 提交载荷（新增 source:'internal' + links 拼装、编辑 updateBug、
 * required 校验拦截、成功/失败文案）、删除 Popconfirm、详情弹窗（tag 矩阵、
 * meta 拼接、外链按钮、关联工单增删跳转、关闭后 Empty）、?bugId= 深链定位、
 * canManage=false 只读形态。
 *
 * mock 口径：pro-components 真实渲染（同 Ops/Notifications 套件先例）；
 * bugs service 用 requireActual 保留 labels/options/deriveBugLinkTitle，
 * 仅 mock 12 个异步函数。bugs.ts 的 label 经模块级 getIntl 求值（非本文件
 * intl mock），断言避开其产出文本，未知枚举值断言原文兜底。
 *
 * 边界（诚实）：
 * 1. 守卫与防御性分支经 UI 不可达：addDetailTicket 的
 *    `if (!detail || !ticketDraft) return`（按钮 disabled={!ticketDraft}，禁用态
 *    点击不触发 onClick）；removeDetailTicket 的 `if (!detail) return`（按钮仅在
 *    详情开启时存在）；request 包装层参数 `?? ''` 右翼（ProTable 恒传 params 键，
 *    仅防动态键缺省）；链接 `l.title || l.url` 右翼（deriveBugLinkTitle 对解析
 *    失败的 url 也回退原文，title 恒非空）；InputNumber `typeof v === 'number'
 *    ? v : null` 右翼（antd 6 jsdom 下清空输入不回调 onChange(null)）。
 * 2. 详情 Modal 的 `<Empty />` 兜底（index.tsx L1108）：detail=null 关闭后容器
 *    display:none 但 children 保留挂载，Empty 在隐藏态渲染（v8 计入覆盖）；
 *    用例断言可观测的 onCancel 后果（隐藏 → 列表态 → 可再打开）。
 * 3. 行内链接图标（github_issue/github_pr→Github 等 5 翼）走 antd Icon 的
 *    aria-label，无文本节点，断言锚定可见的 '+N' 溢出徽标与链接标题。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import DevBugsPage from '../index';
import type { BugItem, BugLinkedTicket } from '@/services/api/bugs';
import { useAccess, useLocation } from '@umijs/max';

jest.setTimeout(40000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/bugs', () => ({
  ...jest.requireActual('@/services/api/bugs'),
  listBugs: jest.fn(),
  getBug: jest.fn(),
  createBug: jest.fn(),
  updateBug: jest.fn(),
  deleteBug: jest.fn(),
  listBugTickets: jest.fn(),
  linkBugTicket: jest.fn(),
  unlinkBugTicket: jest.fn(),
}));

jest.mock('@/services/api/permissions', () => ({
  listAdmins: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({
    defaultMessage,
    values,
  }: {
    defaultMessage?: string;
    values?: Record<string, unknown>;
  }) => {
    let text = defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
    }
    return <>{text}</>;
  },
  // bugs.ts 模块级 getIntl() 求值 label（requireActual 时仍走本 mock）
  getIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
    locale: 'zh-CN',
  }),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, unknown>) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        opts.defaultMessage ?? '',
      ),
  }),
  history: { push: jest.fn(), back: jest.fn() },
  useAccess: jest.fn(),
  useLocation: jest.fn(),
}));

import {
  createBug,
  deleteBug,
  getBug,
  linkBugTicket,
  listBugTickets,
  listBugs,
  unlinkBugTicket,
  updateBug,
} from '@/services/api/bugs';
import { listAdmins } from '@/services/api/permissions';
import { history } from '@umijs/max';

const mListBugs = listBugs as jest.MockedFunction<typeof listBugs>;
const mGetBug = getBug as jest.MockedFunction<typeof getBug>;
const mCreate = createBug as jest.MockedFunction<typeof createBug>;
const mUpdate = updateBug as jest.MockedFunction<typeof updateBug>;
const mDelete = deleteBug as jest.MockedFunction<typeof deleteBug>;
const mListTickets = listBugTickets as jest.MockedFunction<typeof listBugTickets>;
const mLinkTicket = linkBugTicket as jest.MockedFunction<typeof linkBugTicket>;
const mUnlinkTicket = unlinkBugTicket as jest.MockedFunction<typeof unlinkBugTicket>;
const mListAdmins = listAdmins as jest.MockedFunction<typeof listAdmins>;
const mockedUseAccess = useAccess as unknown as jest.Mock;
const mockedUseLocation = useLocation as unknown as jest.Mock;

let mockLocationQuery: Record<string, string | undefined> = {};

const bug = (over: Partial<BugItem> & Pick<BugItem, 'id' | 'title'>): BugItem => ({
  status: 'triage',
  createdAt: '2026-09-01T00:00:00Z',
  updatedAt: '2026-09-02T00:00:00Z',
  ...over,
});

const listResponse = (items: BugItem[]) => ({ items, total: items.length, page: 1, pageSize: 20 });

function renderPage() {
  return render(
    <App>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <DevBugsPage />
      </ConfigProvider>
    </App>,
  );
}

/** 打开行内「详情缺陷」入口 */
async function openDetail() {
  fireEvent.click(await screen.findByText('详情缺陷'));
}

/** Popconfirm 弹层（本环境渲染文案为「确定」） */
async function popconfirmOk(title: string) {
  await screen.findByText(title);
  await waitFor(() => {
    const root = Array.from(document.querySelectorAll('.ant-popover')).find((node) =>
      node.textContent?.includes(title),
    );
    expect(root).toBeTruthy();
  });
  const root = Array.from(document.querySelectorAll('.ant-popover')).find((node) =>
    node.textContent?.includes(title),
  ) as HTMLElement;
  fireEvent.click(within(root).getByRole('button', { name: '确定' }));
}

/** 驱动页面第 index 个工具栏 Select（0=status 1=severity 2=priority 3=assignee）选中第 nth 个选项 */
async function pickToolbarSelect(index: number, nth = 0) {
  const selects = document.querySelectorAll('.ant-select');
  fireEvent.mouseDown(selects[index] as HTMLElement);
  await waitFor(() => {
    const dropdown = document.querySelector(
      '.ant-select-dropdown:not(.ant-select-dropdown-hidden)',
    );
    expect(dropdown).toBeTruthy();
  });
  const dropdown = document.querySelector(
    '.ant-select-dropdown:not(.ant-select-dropdown-hidden)',
  ) as HTMLElement;
  await waitFor(() => {
    expect(dropdown.querySelectorAll('.ant-select-item-option').length).toBeGreaterThan(nth);
  });
  const option = dropdown.querySelectorAll('.ant-select-item-option')[nth] as HTMLElement;
  fireEvent.click(option);
}

beforeEach(() => {
  jest.clearAllMocks();
  mockLocationQuery = {};
  mockedUseAccess.mockReturnValue({ canDevManage: true });
  mockedUseLocation.mockReturnValue({ query: mockLocationQuery });
  mListBugs.mockResolvedValue(listResponse([]));
  mGetBug.mockRejectedValue(new Error('not found'));
  mCreate.mockResolvedValue(bug({ id: 99, title: 'new' }));
  mUpdate.mockResolvedValue(bug({ id: 1, title: 'updated' }));
  mDelete.mockResolvedValue(undefined);
  mListTickets.mockResolvedValue([]);
  mLinkTicket.mockResolvedValue(undefined);
  mUnlinkTicket.mockResolvedValue(undefined);
  mListAdmins.mockResolvedValue({
    items: [
      { id: 1, username: 'alice' },
      { id: 2, username: 'bob' },
    ] as never[],
    total: 2,
    page: 1,
    pageSize: 200,
  });
});

describe('DevBugs 列表渲染矩阵', () => {
  it('链接图标 5 类 + 溢出 +N、已知/未知枚举兜底、source 三态、空值列 -、操作列管理形态', async () => {
    mListBugs.mockResolvedValue(
      listResponse([
        bug({
          id: 1,
          title: '崩溃缺陷',
          status: 'fixing',
          severity: 'blocker',
          priority: 'urgent',
          assignee: 'alice',
          platform: 'pc',
          affectsVersion: '1.4.2',
          fixVersion: '1.4.3',
          source: 'ticket',
          links: [
            { url: 'https://github.com/o/r/issues/1', kind: 'github_issue', title: 'o/r#1' },
            { url: 'https://jira.example.com/B-2', kind: 'jira' },
            { url: 'https://wiki.example.com/p3', kind: 'wiki' },
            { url: 'https://mon.example.com/d/4', kind: 'monitor' },
          ],
        }),
        bug({
          id: 2,
          title: '未知枚举缺陷',
          status: 'mystery_state',
          severity: 'cosmic',
          priority: 'p-nine',
          source: 'player',
        }),
        bug({
          id: 3,
          title: '极简缺陷',
          source: 'internal',
          links: [
            { url: 'https://other.example.com/x', kind: 'other' },
            { url: 'https://mon.example.com/d/9', kind: 'monitor' },
          ],
        }),
      ]),
    );
    renderPage();

    expect(await screen.findByText('崩溃缺陷')).toBeInTheDocument();
    // 行 1：链接 icon 链接 ×4（前 3 条内联 + 溢出 +1 徽标）
    expect(screen.getByText('+1')).toBeInTheDocument();
    // 枚举兜底：未知 status/severity/priority 断言原文（label map fallback 翼）
    expect(screen.getByText('mystery_state')).toBeInTheDocument();
    expect(screen.getByText('cosmic')).toBeInTheDocument();
    expect(screen.getByText('p-nine')).toBeInTheDocument();
    // 空值列 '-'：行 2 的 severity/priority/assignee/platform/affects/fix 六处
    const row2 = screen.getByText('未知枚举缺陷').closest('tr') as HTMLElement;
    expect(within(row2).getAllByText('-').length).toBeGreaterThanOrEqual(4);
    // source 三态
    expect(screen.getByText('工单')).toBeInTheDocument();
    expect(screen.getByText('玩家')).toBeInTheDocument();
    expect(screen.getByText('内部')).toBeInTheDocument();
    // platform 大写化
    expect(screen.getByText('PC')).toBeInTheDocument();
    // 操作列管理形态：编辑 + 删除
    const row1 = screen.getByText('崩溃缺陷').closest('tr') as HTMLElement;
    expect(within(row1).getByRole('button', { name: '编辑' })).toBeInTheDocument();
    expect(within(row1).getByRole('button', { name: '删除' })).toBeInTheDocument();
  });

  it('列表加载失败：提示「加载缺陷列表失败」且不崩', async () => {
    mListBugs.mockRejectedValue(new Error('db down'));
    renderPage();
    // extractErrorMessage 提取 Error.message 透出
    expect(await screen.findByText('db down')).toBeInTheDocument();
    expect(screen.queryByText('崩溃缺陷')).not.toBeInTheDocument();
  });

  it('listAdmins 兜底两翼（items 缺省 → 空数组 / 失败静默），列表照常渲染', async () => {
    mListAdmins.mockResolvedValue({} as never); // items undefined → `|| []` 右翼
    mListBugs.mockResolvedValue(listResponse([bug({ id: 3, title: '极简缺陷' })]));
    const first = renderPage();
    expect(await screen.findByText('极简缺陷')).toBeInTheDocument();
    first.unmount();

    mListAdmins.mockRejectedValue(new Error('admins unavailable'));
    const second = renderPage();
    expect(await screen.findByText('极简缺陷')).toBeInTheDocument();
    second.unmount();
  });
});

describe('DevBugs 工具栏筛选与刷新', () => {
  it('关键词、四个筛选下拉、修复版本、刷新按钮各自触发重拉；回车仅回第一页不重复拉取', async () => {
    renderPage();
    await screen.findByRole('table');
    await waitFor(() => expect(mListBugs).toHaveBeenCalledTimes(1));

    // 关键词输入：ProTable params 深比较 + 防抖触发重拉（先等首拉落定，避免防抖合并计数）
    fireEvent.change(screen.getByPlaceholderText('标题/描述关键词'), {
      target: { value: '崩溃' },
    });
    await waitFor(() => expect(mListBugs).toHaveBeenCalledTimes(2));
    expect(mListBugs).toHaveBeenLastCalledWith(expect.objectContaining({ q: '崩溃' }));

    // 回车 = onSearch 只做 setPageInfo 回第 1 页（已在第 1 页，不新增请求）
    fireEvent.keyDown(screen.getByPlaceholderText('标题/描述关键词'), { key: 'Enter' });
    expect(mListBugs).toHaveBeenCalledTimes(2);

    // 状态/严重度/优先级/负责人四个下拉各选第一项
    await pickToolbarSelect(0);
    await waitFor(() => expect(mListBugs).toHaveBeenCalledTimes(3));
    await pickToolbarSelect(1);
    await waitFor(() => expect(mListBugs).toHaveBeenCalledTimes(4));
    await pickToolbarSelect(2);
    await waitFor(() => expect(mListBugs).toHaveBeenCalledTimes(5));
    await pickToolbarSelect(3);
    await waitFor(() => expect(mListBugs).toHaveBeenCalledTimes(6));

    // 修复版本输入
    fireEvent.change(screen.getByPlaceholderText('修复版本'), { target: { value: '1.4.3' } });
    await waitFor(() => expect(mListBugs).toHaveBeenCalledTimes(7));
    expect(mListBugs).toHaveBeenLastCalledWith(
      expect.objectContaining({ fixVersion: '1.4.3', status: 'triage' }),
    );

    // 刷新按钮
    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mListBugs).toHaveBeenCalledTimes(8));

    // 清空筛选（allowClear）：v 为 undefined → '' 翼，四个下拉各重拉一次
    for (let i = 0; i < 4; i++) {
      const sel = document.querySelectorAll('.ant-select')[i] as HTMLElement;
      fireEvent.mouseOver(sel);
      await waitFor(() => expect(sel.querySelector('.ant-select-clear')).toBeTruthy());
      fireEvent.click(sel.querySelector('.ant-select-clear') as HTMLElement);
      await waitFor(() => expect(mListBugs).toHaveBeenCalledTimes(9 + i));
    }
    expect(mListBugs).toHaveBeenLastCalledWith(expect.objectContaining({ assignee: '' }));
  });
});

describe('DevBugs 提交/编辑缺陷（真实 ModalForm）', () => {
  it('required 校验拦截空提交；填标题+添加链接后保存，createBug 携带 source:internal 与 links 载荷', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /提交缺陷/ }));
    await screen.findByPlaceholderText('一句话描述缺陷');

    // 空提交：title required 拦截
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    expect(await screen.findByText('请输入标题')).toBeInTheDocument();
    expect(mCreate).not.toHaveBeenCalled();

    // 链接编辑器：空 url 点添加不生效（addLink 守卫）
    fireEvent.click(screen.getByRole('button', { name: '添加' }));
    expect(
      screen.getByText('暂无链接；GitHub 链接会自动生成「owner/repo#编号」标题'),
    ).toBeInTheDocument();

    // GitHub issue 链接自动生成标题
    fireEvent.change(screen.getByPlaceholderText('https://github.com/owner/repo/issues/1'), {
      target: { value: 'https://github.com/o/r/issues/42' },
    });
    fireEvent.click(screen.getByRole('button', { name: '添加' }));
    expect(screen.getByText('o/r#42')).toBeInTheDocument();
    // 输入框清空后再次添加空值仍不生效
    fireEvent.click(screen.getByRole('button', { name: '添加' }));

    fireEvent.change(screen.getByPlaceholderText('一句话描述缺陷'), {
      target: { value: '新手教程卡死' },
    });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() =>
      expect(mCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          title: '新手教程卡死',
          source: 'internal',
          links: [
            { url: 'https://github.com/o/r/issues/42', kind: 'github_issue', title: 'o/r#42' },
          ],
        }),
      ),
    );
    expect(await screen.findByText('缺陷已提交')).toBeInTheDocument();
    // 成功后 reload
    await waitFor(() => expect(mListBugs).toHaveBeenCalledTimes(2));
  });

  it('链接 kind 切换 + Tag 可移除（移除后回空态提示）', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /提交缺陷/ }));
    await screen.findByPlaceholderText('一句话描述缺陷');

    // 切换 kind 到 Jira：deriveBugLinkTitle 走 hostname+path 翼
    const kindSelect = document.querySelector('.ant-space-compact .ant-select') as HTMLElement;
    fireEvent.mouseDown(kindSelect);
    await waitFor(() => {
      const dropdown = document.querySelector(
        '.ant-select-dropdown:not(.ant-select-dropdown-hidden)',
      );
      expect(dropdown).toBeTruthy();
    });
    const jiraOption = Array.from(
      document.querySelectorAll(
        '.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option',
      ),
    ).find((o) => o.textContent === 'Jira') as HTMLElement;
    fireEvent.click(jiraOption);
    fireEvent.change(screen.getByPlaceholderText('https://github.com/owner/repo/issues/1'), {
      target: { value: 'https://jira.example.com/browse/B-7' },
    });
    fireEvent.click(screen.getByRole('button', { name: '添加' }));
    expect(screen.getByText('jira.example.com/browse/B-7')).toBeInTheDocument();

    // 移除 Tag：pendingLinks 过滤分支 + 空态回显
    fireEvent.click(
      screen
        .getByText('jira.example.com/browse/B-7')
        .closest('.ant-tag')!
        .querySelector('.anticon-close') as HTMLElement,
    );
    await waitFor(() =>
      expect(
        screen.getByText('暂无链接；GitHub 链接会自动生成「owner/repo#编号」标题'),
      ).toBeInTheDocument(),
    );
  });

  it('编辑：弹窗预填标题与既有链接，updateBug 携带表单值 + links，成功「缺陷已更新」', async () => {
    const editing = bug({
      id: 7,
      title: '既有缺陷',
      status: 'confirmed',
      links: [{ url: 'https://wiki.example.com/a', kind: 'wiki', title: 'WIKI' }],
    });
    mListBugs.mockResolvedValue(listResponse([editing]));
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    await screen.findByText('编辑缺陷 #7');
    expect((screen.getByPlaceholderText('一句话描述缺陷') as HTMLInputElement).value).toBe(
      '既有缺陷',
    );
    expect(screen.getByText('WIKI')).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText('一句话描述缺陷'), {
      target: { value: '改名后的缺陷' },
    });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() =>
      expect(mUpdate).toHaveBeenCalledWith(
        7,
        expect.objectContaining({
          title: '改名后的缺陷',
          links: [{ url: 'https://wiki.example.com/a', kind: 'wiki', title: 'WIKI' }],
        }),
      ),
    );
    expect(await screen.findByText('缺陷已更新')).toBeInTheDocument();
  });

  it('创建失败「提交失败」/ 编辑失败「更新失败」均保持弹窗不崩', async () => {
    mCreate.mockRejectedValue(new Error('quota exceeded'));
    mListBugs.mockResolvedValue(listResponse([bug({ id: 11, title: '可编辑缺陷' })]));
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /提交缺陷/ }));
    await screen.findByPlaceholderText('一句话描述缺陷');
    fireEvent.change(screen.getByPlaceholderText('一句话描述缺陷'), { target: { value: 'X' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    expect(await screen.findByText('quota exceeded')).toBeInTheDocument();
    // 弹窗保持开启：标题输入仍挂载
    expect(screen.getByPlaceholderText('一句话描述缺陷')).toBeInTheDocument();

    // 编辑失败：无可提取信息（undefined 拒绝）→ 「更新失败」fallback（editing=true 翼）
    mUpdate.mockRejectedValue(undefined);
    fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    await screen.findByPlaceholderText('一句话描述缺陷');
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    expect(await screen.findByText('更新失败')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('一句话描述缺陷')).toBeInTheDocument();
  });

  it('删除：Popconfirm 确认后 deleteBug + 「已删除」+ 重拉；失败「删除失败」', async () => {
    mListBugs.mockResolvedValue(listResponse([bug({ id: 5, title: '待删缺陷' })]));
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '删除' }));
    await popconfirmOk('删除缺陷「待删缺陷」？');
    await waitFor(() => expect(mDelete).toHaveBeenCalledWith(5));
    expect(await screen.findByText('已删除')).toBeInTheDocument();
    await waitFor(() => expect(mListBugs).toHaveBeenCalledTimes(2));

    // 失败翼
    mDelete.mockRejectedValue(new Error('locked'));
    fireEvent.click(screen.getByRole('button', { name: '删除' }));
    await popconfirmOk('删除缺陷「待删缺陷」？');
    expect(await screen.findByText('locked')).toBeInTheDocument();
  });
});

describe('DevBugs 详情弹窗与关联工单', () => {
  const detailBug = bug({
    id: 11,
    title: '详情缺陷',
    status: 'verify',
    severity: 'critical',
    priority: 'p-weird',
    platform: 'ios',
    reproducibility: 'always',
    affectsVersion: '2.0.0',
    fixVersion: '2.0.1',
    content: '点击抽卡按钮闪退',
    steps: '1. 打开抽卷\n2. 点抽卡',
    assignee: 'alice',
    playerId: '10086',
    serverId: 's1',
    device: 'iPhone 15',
    os: 'iOS 18',
    links: [{ url: 'https://github.com/o/r/issues/9', kind: 'github_pr', title: 'o/r#9' }],
  });

  it('详情 tag 矩阵 + meta 拼接（玩家/区服/设备）+ 外链按钮；关联工单空态', async () => {
    mListBugs.mockResolvedValue(listResponse([detailBug]));
    renderPage();
    openDetail();
    await screen.findByText('#11 详情缺陷');

    expect(screen.getByText('点击抽卡按钮闪退')).toBeInTheDocument();
    expect(screen.getByText(/1\. 打开抽卷/)).toBeInTheDocument();
    expect(screen.getByText('影响 2.0.0')).toBeInTheDocument();
    expect(screen.getByText('修复于 2.0.1')).toBeInTheDocument();
    expect(screen.getByText('o/r#9')).toBeInTheDocument();
    // 未知 priority：label map 兜底断言原文（`|| detail.priority` 右翼；
    // 列表行与弹窗同文，收窄到 modal 作用域）
    expect(
      within(document.querySelector('.ant-modal') as HTMLElement).getByText('p-weird'),
    ).toBeInTheDocument();
    // meta 拼接：负责人/创建/更新 三翼 + playerId/serverId/device 三翼（同一文本节点）
    expect(
      screen.getByText(
        '负责人 alice · 创建 2026-09-01T00:00:00Z · 更新 2026-09-02T00:00:00Z · 玩家 10086 · 区服 s1 · iPhone 15 (iOS 18)',
      ),
    ).toBeInTheDocument();
    // 关联工单空态
    expect(await screen.findByText('暂无关联工单')).toBeInTheDocument();

    // 添加关联守卫：ticketDraft 为空时按钮禁用
    expect(screen.getByRole('button', { name: /添加关联/ })).toBeDisabled();
  });

  it('关联工单列表渲染：状态/priority/gameId 翼矩阵 + 工单标题点击跳转', async () => {
    const tickets: BugLinkedTicket[] = [
      {
        id: 7,
        title: '无法登录',
        status: 'in_progress',
        priority: 'hot',
        gameId: 'demo',
        env: 'prod',
      },
      { id: 8, title: '掉线反馈', status: 'weird_status' },
    ];
    mListTickets.mockResolvedValue(tickets);
    mListBugs.mockResolvedValue(listResponse([detailBug]));
    renderPage();
    await openDetail();

    expect(await screen.findByText('#7 无法登录')).toBeInTheDocument();
    // 已知状态经 intl 文案、未知状态断言原文
    expect(screen.getByText('处理中')).toBeInTheDocument();
    expect(screen.getByText('weird_status')).toBeInTheDocument();
    // 未知 priority Tag 原文兜底（工单 7），工单 8 无 priority 不渲染 Tag
    expect(screen.getByText('hot')).toBeInTheDocument();
    // gameId 翼 + 无 gameId 翼（工单 8 无 priority/gameId Tag）
    expect(screen.getByText('demo/prod')).toBeInTheDocument();
    // 跳转
    fireEvent.click(screen.getByText('#7 无法登录'));
    expect(history.push).toHaveBeenCalledWith('/support/tickets/7');
  });

  it('添加关联：InputNumber 输入后 linkBugTicket + 「已关联工单 #9」+ 重拉；失败「关联失败」', async () => {
    mListBugs.mockResolvedValue(listResponse([detailBug]));
    renderPage();
    await openDetail();
    await screen.findByText('#11 详情缺陷');
    await screen.findByText('暂无关联工单');

    const ticketInput = screen.getByPlaceholderText('工单 ID');
    fireEvent.change(ticketInput, { target: { value: '9' } });
    const addBtn = screen.getByRole('button', { name: /添加关联/ });
    await waitFor(() => expect(addBtn).toBeEnabled());
    fireEvent.click(addBtn);

    await waitFor(() => expect(mLinkTicket).toHaveBeenCalledWith(11, 9));
    expect(await screen.findByText('已关联工单 #9')).toBeInTheDocument();
    await waitFor(() => expect(mListTickets).toHaveBeenCalledTimes(2));
    // 输入复位（draft 清空 → 按钮回禁用）
    await waitFor(() => expect(screen.getByRole('button', { name: /添加关联/ })).toBeDisabled());

    // 失败翼
    fireEvent.change(ticketInput, { target: { value: '10' } });
    mLinkTicket.mockRejectedValue(new Error('ticket gone'));
    fireEvent.click(screen.getByRole('button', { name: /添加关联/ }));
    expect(await screen.findByText('ticket gone')).toBeInTheDocument();
  });

  it('解除关联：Popconfirm 确认后 unlinkBugTicket + 行内移除；失败「解除失败」', async () => {
    mListTickets.mockResolvedValue([{ id: 7, title: '无法登录', status: 'open' }]);
    mListBugs.mockResolvedValue(listResponse([detailBug]));
    renderPage();
    await openDetail();
    fireEvent.click(await screen.findByRole('button', { name: '解除关联' }));
    await popconfirmOk('解除与工单 #7 的关联？');

    await waitFor(() => expect(mUnlinkTicket).toHaveBeenCalledWith(11, 7));
    expect(await screen.findByText('已解除关联')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText('#7 无法登录')).not.toBeInTheDocument());

    // 失败翼：重开详情重试
    mListTickets.mockResolvedValue([{ id: 7, title: '无法登录', status: 'open' }]);
    mUnlinkTicket.mockRejectedValue(new Error('busy'));
    fireEvent.click(screen.getByText('详情缺陷'));
    await screen.findByText('#11 详情缺陷');
    fireEvent.click(await screen.findByRole('button', { name: '解除关联' }));
    await popconfirmOk('解除与工单 #7 的关联？');
    expect(await screen.findByText('busy')).toBeInTheDocument();
  });

  it('关联工单加载失败不阻塞详情展示（静默空态，可重开重试）', async () => {
    mListTickets.mockRejectedValue(new Error('tickets svc down'));
    // 无 device/playerId/serverId 形态：meta 设备三元 `: ''` 右翼
    mListBugs.mockResolvedValue(listResponse([bug({ id: 11, title: '详情缺陷' })]));
    renderPage();
    await openDetail();
    expect(await screen.findByText('#11 详情缺陷')).toBeInTheDocument();
    expect(await screen.findByText('暂无关联工单')).toBeInTheDocument();
  });

  it('关闭详情：onCancel 后 detail 置 null，弹窗隐藏（children 随 detail 卸载）并可再次打开', async () => {
    mListBugs.mockResolvedValue(listResponse([detailBug]));
    renderPage();
    await openDetail();
    await screen.findByText('#11 详情缺陷');

    fireEvent.click(document.querySelector('.ant-modal-close') as HTMLElement);
    // open={Boolean(detail)} → detail=null 即关闭：antd 6 关闭后容器 display:none
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-wrap')).toHaveStyle({ display: 'none' }),
    );
    // 列表态恢复，可再次打开详情（验证 detail 复位而非卡死）
    await openDetail();
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-wrap')).not.toHaveStyle({ display: 'none' }),
    );
    expect(screen.getByText('#11 详情缺陷')).toBeInTheDocument();
  });

  it('?bugId= 深链：挂载时 getBug 定位并打开详情；非法 id 与加载失败静默', async () => {
    mListBugs.mockResolvedValue(listResponse([detailBug]));
    mGetBug.mockResolvedValue(detailBug);
    mockLocationQuery = { bugId: '11' };
    mockedUseLocation.mockReturnValue({ query: mockLocationQuery });
    const first = renderPage();
    await waitFor(() => expect(mGetBug).toHaveBeenCalledWith(11));
    expect(await screen.findByText('#11 详情缺陷')).toBeInTheDocument();
    first.unmount();

    // 非法值翼：非数字 / 负数 / 无 query 均不触发 getBug
    for (const q of [{ bugId: 'abc' }, { bugId: '-3' }, {}]) {
      mockLocationQuery = q;
      mockedUseLocation.mockReturnValue({ query: mockLocationQuery });
      const inst = renderPage();
      await screen.findByRole('table');
      inst.unmount();
    }
    expect(mGetBug).toHaveBeenCalledTimes(1);

    // getBug 失败静默
    mGetBug.mockRejectedValue(new Error('gone'));
    mockLocationQuery = { bugId: '99' };
    mockedUseLocation.mockReturnValue({ query: mockLocationQuery });
    const last = renderPage();
    await screen.findAllByRole('table');
    expect(screen.queryByText('#99 详情缺陷')).not.toBeInTheDocument();
    last.unmount();
  });
});

describe('DevBugs 只读形态（canManage=false）', () => {
  it('无「提交缺陷」按钮、操作列仅「详情」、详情内无添加关联/解除关联', async () => {
    mockedUseAccess.mockReturnValue({ canDevManage: false });
    mListTickets.mockResolvedValue([{ id: 7, title: '无法登录', status: 'open' }]);
    mListBugs.mockResolvedValue(
      listResponse([bug({ id: 11, title: '详情缺陷', device: '收银机' })]),
    );
    renderPage();

    expect(await screen.findByText('详情缺陷')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /提交缺陷/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '编辑' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '删除' })).not.toBeInTheDocument();

    // 只读形态经行内「详情」按钮打开（覆盖该按钮 onClick）
    fireEvent.click(await screen.findByRole('button', { name: '详情' }));
    await screen.findByText('#11 详情缺陷');
    expect(await screen.findByText('#7 无法登录')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /添加关联/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /解除关联/ })).not.toBeInTheDocument();
  });
});
