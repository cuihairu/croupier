/**
 * 数据备份页单测（覆盖率补缺轮：Ops/Backups/index.tsx 218 行 0% → 收口，
 * 零测试簇排行现席）。
 *
 * 锁定契约：
 * - 挂载：listOpsBackups()（零参）→ 行渲染矩阵（ID / 类型 / 大小原值 /
 *   状态 Tag 三色 / 时间原值）+ 卡片标题「数据备份」+ 工具栏 刷新/创建备份 +
 *   下载锚 getOpsBackupDownloadUrl(id) href 透传；
 * - 刷新：按钮 → 第 2 次 listOpsBackups；
 * - 创建主链：开窗 → 选类型 → 确定 → createOpsBackup(表单值) → toast
 *   已创建 → setTimeout(load, 500) 真实计时器重拉 → destroyOnHidden 卸载；
 * - required 拦截：不选类型提交 → 不触达 createOpsBackup、弹窗保持；
 *   取消（footer 取消钮）关闭；
 * - target 可选填：redis + 连接串 → 完整载荷（target 键进入提交）；
 * - 创建失败：reject → 静默 catch return false（无本地弹错、无成功
 *   toast、不重拉）弹窗保持——注释言明「全局拦截器已 toast」，测试环境
 *   service 已 mock 故断言全静默为现状锁定；
 * - 删除主链：行内删除 → modal.confirm → 确认 → deleteOpsBackup(id) →
 *   toast 已删除 → 重拉；
 * - 删除失败三翼：Error → e.message toast / 非 Error → intl「操作失败」/
 *   Error('') → `errMsg ||` 右翼 intl「失败」；失败不重拉。
 *
 * mock 口径：services/api/ops 四函数 jest.mock（getOpsBackupDownloadUrl
 * 为同步纯函数，按 id 回构造 href）；@umijs/max 本地 mock——本页
 * intl/FormattedMessage 全带 defaultMessage，取 `defaultMessage ?? id ?? ''`
 * 使文案确定性可见；pro-components spread requireActual（ModalForm footer
 * 校验/destroyOnHidden 依赖本体）+ PageContainer 桩；antd App 真实包裹
 * （页面默认导出**不自包 App**，App.useApp 的 message/modal 必须有宿主）。
 *
 * 坑实证（antd 6.6.0 实测，四条）：
 * - 双字中文 Button 自动插空格（刷 新 / 删 除 / 取 消 / 确 定），
 *   role name 一律宽松正则；
 * - modal.confirm 定位走类名（`.ant-modal-confirm-btns .ant-btn-primary`
 *   ——locale 无关）；标题 `.ant-modal-title` 与 `.ant-modal-confirm-title`
 *   双渲染，断言须 selector 收窄；
 * - Select option 点击配方：sleep≥60ms → mouseDown 落 `.ant-select` 根
 *   （无 .ant-select-selector）→ 点可见 dropdown 内
 *   `.ant-select-item-option-content`（Behavior 套件同款）；
 * - 成功创建的 `setTimeout(load, 500)` 是真实计时器——用例内必须等第 2 次
 *   listOpsBackups 消费掉，否则挂起的 timer 在下个用例触发毒化计数断言
 *   （jest.clearAllMocks 不清计时器）。
 *
 * 现状锁定 / 边界（诚实清单，不造假用例不删防御分支）：
 * 1. `r?.backups || []` 双右翼——listOpsBackups 归一层恒返
 *    `{ backups: response.backups.map(normalizeOpsBackup) }`（map 恒产
 *    数组），resolve undefined/非对象违反返回类型即造假，登记。
 * 2. load 的 try/finally 无 catch——listOpsBackups reject 成 unhandled
 *    rejection（同族页面既有口径），不造假 reject 场景。
 * 3. 状态 Tag 三元链 else 翼（gold）——经 running 状态真实覆盖，无登记项。
 * 4. del 的 `errMsg ||` 右翼经 Error('') 真实触达；onFinish 静默 catch
 *    经真实 reject 触达。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import OpsBackupsPage from '../index';
import {
  createOpsBackup,
  deleteOpsBackup,
  getOpsBackupDownloadUrl,
  listOpsBackups,
  type OpsBackup,
} from '@/services/api/ops';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/ops', () => ({
  listOpsBackups: jest.fn(),
  createOpsBackup: jest.fn(),
  deleteOpsBackup: jest.fn(),
  getOpsBackupDownloadUrl: jest.fn(),
}));

// 本页 intl/FormattedMessage 全带 defaultMessage——回 defaultMessage 使文案可见
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? opts.id ?? '',
  }),
}));

// ModalForm 的 footer 校验/destroyOnHidden 依赖真实实现，仅 PageContainer 桩
jest.mock('@ant-design/pro-components', () => ({
  ...jest.requireActual('@ant-design/pro-components'),
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

const mList = listOpsBackups as jest.MockedFunction<typeof listOpsBackups>;
const mCreate = createOpsBackup as jest.MockedFunction<typeof createOpsBackup>;
const mDel = deleteOpsBackup as jest.MockedFunction<typeof deleteOpsBackup>;
const mUrl = getOpsBackupDownloadUrl as jest.MockedFunction<typeof getOpsBackupDownloadUrl>;

// 行 1 done（green）+ size；行 2 failed（red）；行 3 running（else 翼 → gold）+ 缺 size
const ROWS: OpsBackup[] = [
  {
    id: 'bk-001',
    type: 'postgres',
    status: 'done',
    size: 1048576,
    createdAt: '2026-09-30T00:00:00Z',
  },
  { id: 'bk-002', type: 'redis', status: 'failed', size: 2048, createdAt: '2026-09-29T12:00:00Z' },
  { id: 'bk-003', type: 'packs', status: 'running', createdAt: '2026-09-28T08:00:00Z' },
];

function sleep(ms: number) {
  return new Promise((r) => setTimeout(r, ms));
}

function renderPage() {
  return render(
    <App>
      <OpsBackupsPage />
    </App>,
  );
}

/** ModalForm 主提交按钮（双字中文自动插空格 → 正则收窄） */
function footerOk() {
  const footer = document.querySelector('.ant-modal-footer') as HTMLElement;
  return within(footer).getByRole('button', { name: /确\s*定/ });
}

async function waitList() {
  await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
}

/** 表体行（文本锚 id → tr） */
function rowOf(id: string): HTMLElement {
  return screen.getByText(id).closest('tr') as HTMLElement;
}

/** 打开单选下拉并点可见 option（rc-select 关闭动画，重开须留时间隙） */
async function pickOption(selectRoot: HTMLElement, label: string) {
  await sleep(60);
  fireEvent.mouseDown(selectRoot);
  const item = await waitFor(() => {
    const els = Array.from(
      document.querySelectorAll(
        '.ant-select-dropdown:not(.ant-dropdown-hidden) .ant-select-item-option-content',
      ),
    ).filter((el) => el.textContent === label);
    expect(els.length).toBeGreaterThan(0);
    return els[els.length - 1] as HTMLElement;
  });
  fireEvent.click(item);
}

/** 开创建弹窗并等挂载 */
async function openCreate() {
  fireEvent.click(screen.getByRole('button', { name: /创建备份/ }));
  // 弹窗标题与工具栏按钮同文本——锚 .ant-modal-title 收窄
  await waitFor(() => {
    expect(document.querySelector('.ant-modal-title')).toHaveTextContent('创建备份');
  });
}

beforeEach(() => {
  jest.clearAllMocks();
  mList.mockResolvedValue({ backups: ROWS });
  mCreate.mockResolvedValue(undefined);
  mDel.mockResolvedValue(undefined);
  mUrl.mockImplementation((id: string) => `/dl/${id}`);
});

describe('数据备份 挂载与列表', () => {
  it('首拉无参 + 列渲染矩阵（类型/大小/状态三色 Tag/时间）+ 工具栏 + 下载链接', async () => {
    renderPage();
    await waitList();
    expect(mList).toHaveBeenCalledWith(); // 零参调用
    await waitFor(() => expect(screen.getByText('bk-001')).toBeInTheDocument());

    expect(screen.getByText('数据备份')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /刷\s*新/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /创建备份/ })).toBeInTheDocument();

    // 列头（thead 收窄——「操作」与行内按钮同文本）
    const thead = document.querySelector('.ant-table-thead') as HTMLElement;
    for (const h of ['ID', '类型', '大小', '状态', '时间', '操作']) {
      expect(within(thead).getByText(h)).toBeInTheDocument();
    }

    // 行值：类型 / size 原值 / createdAt 原值（无格式化列）
    expect(screen.getByText('postgres')).toBeInTheDocument();
    expect(screen.getByText('1048576')).toBeInTheDocument();
    expect(screen.getByText('2026-09-30T00:00:00Z')).toBeInTheDocument();
    // 缺 size 行不渲染该格值
    expect(within(rowOf('bk-003')).queryByText('2026-09-30T00:00:00Z')).not.toBeInTheDocument();

    // 状态 Tag 三色矩阵：done→green / failed→red / else(running)→gold
    expect(within(rowOf('bk-001')).getByText('done').className).toContain('ant-tag-green');
    expect(within(rowOf('bk-002')).getByText('failed').className).toContain('ant-tag-red');
    expect(within(rowOf('bk-003')).getByText('running').className).toContain('ant-tag-gold');

    // 下载锚：URL 构造按行 id 透传 + href 落 DOM
    expect(mUrl).toHaveBeenCalledWith('bk-001');
    const link = within(rowOf('bk-001')).getByText('下载').closest('a') as HTMLAnchorElement;
    expect(link).toHaveAttribute('href', '/dl/bk-001');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noreferrer');
  });

  it('刷新按钮 → listOpsBackups 第 2 次调用', async () => {
    renderPage();
    await waitList();

    fireEvent.click(screen.getByRole('button', { name: /刷\s*新/ }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });
});

describe('数据备份 创建弹窗', () => {
  it('创建主链：选类型提交 → 载荷 + 已创建 + 500ms 重拉 + destroyOnHidden 卸载', async () => {
    renderPage();
    await waitList();
    await openCreate();

    await pickOption(document.querySelector('.ant-modal .ant-select') as HTMLElement, 'postgres');
    fireEvent.click(footerOk());

    await waitFor(() => expect(mCreate).toHaveBeenCalledTimes(1));
    // target 未填：键缺省或 undefined 二择在 toEqual 语义下等价
    expect(mCreate).toHaveBeenCalledWith({ kind: 'postgres' });
    expect(await screen.findByText('已创建')).toBeInTheDocument();

    // destroyOnHidden：成功关闭即整体卸载（.ant-modal 消失）
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
    // setTimeout(load, 500) 真实计时器 → 第 2 次拉取（用例内消费，防跨用例泄漏）
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('required 拦截：不选类型提交 → 不触达服务 + 弹窗保持；取消关闭', async () => {
    renderPage();
    await waitList();
    await openCreate();

    fireEvent.click(footerOk());
    // rules=[{required:true}] 无 message → antd 默认文案（locale 相关），断言结构
    await waitFor(() =>
      expect(document.querySelector('.ant-modal .ant-form-item-explain-error')).not.toBeNull(),
    );
    expect(mCreate).not.toHaveBeenCalled();
    expect(document.querySelector('.ant-modal')).not.toBeNull();

    const cancel = within(document.querySelector('.ant-modal-footer') as HTMLElement).getByRole(
      'button',
      { name: /取\s*消/ },
    );
    fireEvent.click(cancel);
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
    expect(mCreate).not.toHaveBeenCalled();
  });

  it('target 可选填：redis + 连接串 → 完整载荷（target 键进提交）', async () => {
    renderPage();
    await waitList();
    await openCreate();

    await pickOption(document.querySelector('.ant-modal .ant-select') as HTMLElement, 'redis');
    fireEvent.change(screen.getByPlaceholderText(/可选/), {
      target: { value: 'redis://host:6379/0' },
    });
    fireEvent.click(footerOk());

    await waitFor(() => expect(mCreate).toHaveBeenCalledTimes(1));
    expect(mCreate).toHaveBeenCalledWith({ kind: 'redis', target: 'redis://host:6379/0' });
    expect(await screen.findByText('已创建')).toBeInTheDocument();
    // 消费成功链的 500ms 重拉计时器（防跨用例泄漏毒化计数）
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('创建失败：reject → 静默 return false（无成功 toast、不重拉）弹窗保持', async () => {
    renderPage();
    await waitList();
    await openCreate();
    await pickOption(document.querySelector('.ant-modal .ant-select') as HTMLElement, 'postgres');

    mCreate.mockRejectedValueOnce(new Error('denied'));
    fireEvent.click(footerOk());

    await waitFor(() => expect(mCreate).toHaveBeenCalledTimes(1));
    expect(document.querySelector('.ant-modal')).not.toBeNull();
    expect(screen.queryByText('已创建')).not.toBeInTheDocument();
    // 静默 catch（无本地弹错）+ 不重拉
    expect(screen.queryByText('denied')).not.toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });
});

describe('数据备份 删除', () => {
  async function confirmDelete(id: string) {
    fireEvent.click(within(rowOf(id)).getByRole('button', { name: /删\s*除/ }));
    // confirm 标题双渲染——selector 收窄到 .ant-modal-confirm-title
    await screen.findByText('删除备份', { selector: '.ant-modal-confirm-title' });
    const ok = document.querySelector(
      '.ant-modal-confirm-btns .ant-btn-primary',
    ) as HTMLButtonElement;
    expect(ok).not.toBeNull();
    fireEvent.click(ok);
  }

  it('删除主链：确认 → deleteOpsBackup(id) + 已删除 + 重拉', async () => {
    renderPage();
    await waitList();

    await confirmDelete('bk-002');
    await waitFor(() => expect(mDel).toHaveBeenCalledWith('bk-002'));
    expect(await screen.findByText('已删除')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('删除失败 Error → e.message toast + 不重拉', async () => {
    renderPage();
    await waitList();

    mDel.mockRejectedValueOnce(new Error('boom'));
    await confirmDelete('bk-001');

    expect(await screen.findByText('boom')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });

  it('删除失败非 Error（字符串）→ intl「操作失败」兜底', async () => {
    renderPage();
    await waitList();

    mDel.mockRejectedValueOnce('raw fail' as never);
    await confirmDelete('bk-001');

    expect(await screen.findByText('操作失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });

  it('删除失败 Error("") → errMsg 空串落 `||` 右翼「失败」', async () => {
    renderPage();
    await waitList();

    mDel.mockRejectedValueOnce(new Error(''));
    await confirmDelete('bk-001');

    expect(await screen.findByText('失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });
});
