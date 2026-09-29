/**
 * 扩展安装页单测（覆盖率巡检：index.tsx 431 行 0% → 行覆盖收口）。
 *
 * 锁定契约：概览五项统计（items 派生：启用/禁用/健康/作用域去重）、列表 7 列
 * 渲染矩阵（displayName 兜底 extensionId、#id/installationKey、启用/禁用 Tag、
 * 健康三色兜底 '-'、scope/target 兜底 '-'、updatedAt falsy 兜底）、筛选栏
 * （扩展 ID 输入/状态下拉 → request 载荷、筛选生效 Alert + chips、清空筛选）、
 * 刷新、withReload 成功/失败两翼（extractErrorMessage 兜底）、启停回调按
 * row.enabled 分派 enable/disable、重建绑定、卸载 confirm 流（成功/依赖阻塞
 * blockers warning/普通错误透传 mapExtensionError）、三个 overlay 受控开合
 * 与 onUpgraded/onSaved → reload 链、canExtensionsManage=false 菜单禁用。
 *
 * mock 口径：services/api/extensions 五个函数 jest.mock；adapters/mapper/
 * extractErrorMessage/公共组件（SummaryOverview 等）走真实实现；三个 overlay
 * （EventsDrawer/UpgradeModal/InstallationDetailDrawer）mock 成桩组件——聚焦
 * 页面契约（open/row/onClose/onUpgraded/onSaved），详情抽屉本体已有独立套件，
 * 事件抽屉与升级弹窗留独立簇（登记于交付说明）。@umijs/max 本地 mock。
 *
 * 边界（诚实）：无不可达分支——页面全部行为经 UI 可达。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import ExtensionsInstallationsPage from '../index';
import type { ExtensionInstallationItem } from '@/services/api/extensions';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/extensions', () => ({
  listExtensionInstallations: jest.fn(),
  enableExtension: jest.fn(),
  disableExtension: jest.fn(),
  reconcileExtension: jest.fn(),
  uninstallExtension: jest.fn(),
}));

jest.mock('../EventsDrawer', () => ({
  __esModule: true,
  default: (props: { open: boolean; installation: { id: number } | null; onClose: () => void }) =>
    props.open ? (
      <div data-testid="events-drawer" data-row={props.installation?.id ?? ''}>
        <button onClick={props.onClose}>__events-close</button>
      </div>
    ) : null,
}));

jest.mock('../UpgradeModal', () => ({
  __esModule: true,
  default: (props: {
    open: boolean;
    row: { id: number } | null;
    onClose: () => void;
    onUpgraded: () => Promise<void>;
  }) =>
    props.open ? (
      <div data-testid="upgrade-modal" data-row={props.row?.id ?? ''}>
        <button onClick={props.onClose}>__upgrade-close</button>
        <button onClick={() => void props.onUpgraded()}>__upgrade-done</button>
      </div>
    ) : null,
}));

jest.mock('../InstallationDetailDrawer', () => ({
  __esModule: true,
  default: (props: {
    open: boolean;
    row: { id: number } | null;
    onClose: () => void;
    onSaved: () => Promise<void>;
  }) =>
    props.open ? (
      <div data-testid="detail-drawer" data-row={props.row?.id ?? ''}>
        <button onClick={props.onClose}>__detail-close</button>
        <button onClick={() => void props.onSaved()}>__detail-saved</button>
      </div>
    ) : null,
}));

// mock* 前缀变量：babel-jest hoist 白名单，允许 mock 工厂延迟绑定
const mockCanManage = jest.fn(() => true);

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
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, unknown>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
      }
      return text;
    },
  }),
  useAccess: () => ({ canExtensionsManage: mockCanManage() }),
  PageContainer: ({
    children,
    title,
    subTitle,
  }: {
    children?: React.ReactNode;
    title?: React.ReactNode;
    subTitle?: React.ReactNode;
  }) => (
    <div>
      <h1>
        {title}
        {subTitle ? <span>{subTitle}</span> : null}
      </h1>
      {children}
    </div>
  ),
}));

import {
  disableExtension,
  enableExtension,
  listExtensionInstallations,
  reconcileExtension,
  uninstallExtension,
} from '@/services/api/extensions';

const mList = listExtensionInstallations as jest.MockedFunction<typeof listExtensionInstallations>;
const mEnable = enableExtension as jest.MockedFunction<typeof enableExtension>;
const mDisable = disableExtension as jest.MockedFunction<typeof disableExtension>;
const mReconcile = reconcileExtension as jest.MockedFunction<typeof reconcileExtension>;
const mUninstall = uninstallExtension as jest.MockedFunction<typeof uninstallExtension>;

const row1: ExtensionInstallationItem = {
  id: 7,
  installationKey: 'chatops-demo',
  extensionId: 'chatops',
  displayName: 'ChatOps',
  releaseVersion: '1.4.0',
  scopeType: 'game',
  scopeId: 'demo',
  targetType: 'agent',
  targetId: 'agent-1',
  status: 'running',
  desiredState: 'active',
  enabled: true,
  healthStatus: 'healthy',
  lastError: '',
  updatedAt: 1727500000,
};

// 覆盖兜底翼：enabled=false、healthStatus/targetId/updatedAt 空值
const row2: ExtensionInstallationItem = {
  id: 8,
  installationKey: 'wiki-prod',
  extensionId: 'wiki',
  displayName: 'Wiki',
  releaseVersion: '0.9.1',
  scopeType: 'system',
  scopeId: 'global',
  targetType: 'agent_group',
  targetId: '',
  status: 'disabled',
  desiredState: 'inactive',
  enabled: false,
  healthStatus: '',
  lastError: 'boom',
  updatedAt: 0,
};

function renderPage() {
  return render(
    <App>
      <ExtensionsInstallationsPage />
    </App>,
  );
}

/** 等首拉落定（ProTable 20ms 防抖：先等 times(1) 再驱动筛选，避免合并计数） */
async function waitFirstLoad() {
  expect(await screen.findByText('ChatOps')).toBeInTheDocument();
  await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
}

/** 打开某行的「更多」下拉菜单（菜单渲染在 body portal） */
async function openMoreMenu(rowTitle: string) {
  const row = screen.getByText(rowTitle).closest('tr') as HTMLElement;
  fireEvent.click(within(row).getByRole('button', { name: /更多/ }));
  return await screen.findByRole('menu');
}

/** 打开卸载确认框并点危险色确定按钮（antd6 confirm 标题双渲染，selector 收窄） */
async function confirmUninstall() {
  expect(
    await screen.findByText('确认卸载扩展', { selector: '.ant-modal-confirm-title' }),
  ).toBeInTheDocument();
  const ok = document.querySelector(
    '.ant-modal-confirm-btns .ant-btn-dangerous',
  ) as HTMLButtonElement;
  expect(ok).not.toBeNull();
  fireEvent.click(ok);
}

beforeEach(() => {
  jest.clearAllMocks();
  mockCanManage.mockReturnValue(true);
  mList.mockResolvedValue({ items: [row1, row2], total: 2 });
  mEnable.mockResolvedValue({ id: 8 } as never);
  mDisable.mockResolvedValue({ id: 7 } as never);
  mReconcile.mockResolvedValue({ id: 7 } as never);
  mUninstall.mockResolvedValue({ id: 7 } as never);
});

describe('扩展安装页 列表与概览', () => {
  it('概览五项统计 + 7 列渲染矩阵（启用/禁用 Tag、健康兜底 -、scope/target、updatedAt falsy 兜底）', async () => {
    renderPage();
    await waitFirstLoad();

    // 概览五项（items 派生：2 行中 1 启用 1 禁用 1 健康、scope 去重 2）
    expect(screen.getByText('总数 2')).toBeInTheDocument();
    expect(screen.getByText('启用 1')).toBeInTheDocument();
    expect(screen.getByText('禁用 1')).toBeInTheDocument();
    expect(screen.getByText('健康 1')).toBeInTheDocument();
    expect(screen.getByText('作用域 2')).toBeInTheDocument();

    // 列矩阵 row1
    expect(screen.getByText('ChatOps')).toBeInTheDocument();
    expect(screen.getByText('#7 / chatops-demo')).toBeInTheDocument();
    expect(screen.getByText('1.4.0')).toBeInTheDocument();
    expect(screen.getByText('running')).toBeInTheDocument();
    expect(screen.getByText('healthy')).toBeInTheDocument();
    expect(screen.getByText('game:demo')).toBeInTheDocument();
    expect(screen.getByText('agent:agent-1')).toBeInTheDocument();

    // 列矩阵 row2：健康空→'-'、targetId 空→'agent_group:-'、updatedAt 0→'-'
    expect(screen.getByText('Wiki')).toBeInTheDocument();
    expect(screen.getByText('#8 / wiki-prod')).toBeInTheDocument();
    expect(screen.getByText('0.9.1')).toBeInTheDocument();
    expect(screen.getByText('disabled')).toBeInTheDocument();
    expect(screen.getByText('agent_group:-')).toBeInTheDocument();
    expect(screen.getByText('system:global')).toBeInTheDocument();

    // 结果计数 + 页头
    expect(screen.getByText('当前结果 2 个安装实例')).toBeInTheDocument();
    expect(screen.getByText('扩展安装')).toBeInTheDocument();
  });

  it('request 失败静默翼：catch 返回 success:false，概览落零值、无 crash', async () => {
    mList.mockRejectedValue(new Error('list down'));
    renderPage();

    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
    // 静默失败：无数据行、概览全零
    expect(await screen.findByText('总数 0')).toBeInTheDocument();
    expect(screen.getByText('启用 0')).toBeInTheDocument();
    expect(screen.getByText('作用域 0')).toBeInTheDocument();
    expect(screen.queryByText('ChatOps')).not.toBeInTheDocument();
  });

  it('刷新按钮：点击触发 reload（第二次 request）', async () => {
    renderPage();
    await waitFirstLoad();

    fireEvent.click(screen.getByRole('button', { name: '刷新' }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });
});

describe('扩展安装页 筛选', () => {
  it('扩展 ID 输入 + 状态选择 → request 载荷更新 + Alert 生效条件 + 清空筛选恢复', async () => {
    renderPage();
    await waitFirstLoad();

    // 无筛选态：无清空按钮、无 Alert
    expect(screen.queryByRole('button', { name: '清空筛选' })).not.toBeInTheDocument();
    expect(screen.queryByText('当前正在查看筛选后的安装实例')).not.toBeInTheDocument();

    // 输入扩展 ID（草稿态直接进提交态：params 变化触发第二次拉取）
    fireEvent.change(screen.getByPlaceholderText('扩展 ID'), { target: { value: 'chat' } });
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(
        expect.objectContaining({ extensionId: 'chat', status: undefined, page: 1 }),
      ),
    );

    // 状态下拉选择 running（页面唯一 Select；antd6 无 .ant-select-selector，
    // mouseDown 直接落 .ant-select 根；option 收窄到 dropdown portal 避开行内状态 Tag）
    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
    expect(dropdown).not.toBeNull();
    // dropdown 内 a11y listbox 与可见 option 双份 DOM：onSelect 绑在可见 option 行，
    // 点可见 content（冒泡到 .ant-select-item-option 的 onClick）
    fireEvent.click(
      within(dropdown).getByText('running', { selector: '.ant-select-item-option-content' }),
    );
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(
        expect.objectContaining({ extensionId: 'chat', status: 'running' }),
      ),
    );

    // 筛选生效 Alert + chips
    expect(screen.getByText('当前正在查看筛选后的安装实例')).toBeInTheDocument();
    expect(screen.getByText('已生效条件：扩展 chat / 状态 running')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '清空筛选' })).toBeInTheDocument();

    // 清空筛选：双态复位 + 回第 1 页
    fireEvent.click(screen.getByRole('button', { name: '清空筛选' }));
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(
        expect.objectContaining({ extensionId: undefined, status: undefined }),
      ),
    );
    expect(screen.queryByText('当前正在查看筛选后的安装实例')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '清空筛选' })).not.toBeInTheDocument();
  });
});

describe('扩展安装页 行动作（withReload 与卸载）', () => {
  it('启用行 → 菜单「禁用当前安装」→ disableExtension + 「已禁用扩展」+ reload', async () => {
    renderPage();
    await waitFirstLoad();

    const menu = await openMoreMenu('ChatOps');
    fireEvent.click(within(menu).getByRole('menuitem', { name: '禁用当前安装' }));

    await waitFor(() => expect(mDisable).toHaveBeenCalledWith(7));
    expect(await screen.findByText('已禁用扩展')).toBeInTheDocument();
    expect(mEnable).not.toHaveBeenCalled();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('禁用行 → 菜单「启用当前安装」→ enableExtension + 「已启用扩展」+ reload', async () => {
    renderPage();
    await waitFirstLoad();

    const menu = await openMoreMenu('Wiki');
    fireEvent.click(within(menu).getByRole('menuitem', { name: '启用当前安装' }));

    await waitFor(() => expect(mEnable).toHaveBeenCalledWith(8));
    expect(await screen.findByText('已启用扩展')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('菜单「重建当前绑定」→ reconcileExtension + 「已触发重建绑定」', async () => {
    renderPage();
    await waitFirstLoad();

    const menu = await openMoreMenu('ChatOps');
    fireEvent.click(within(menu).getByRole('menuitem', { name: '重建当前绑定' }));

    await waitFor(() => expect(mReconcile).toHaveBeenCalledWith(7));
    expect(await screen.findByText('已触发重建绑定')).toBeInTheDocument();
  });

  it('withReload 失败两翼：reject Error 透出 message；reject undefined 落「操作失败」兜底', async () => {
    renderPage();
    await waitFirstLoad();

    // 翼 1：Error 对象 → extractErrorMessage 取 message
    mReconcile.mockRejectedValue(new Error('reconcile down'));
    let menu = await openMoreMenu('ChatOps');
    fireEvent.click(within(menu).getByRole('menuitem', { name: '重建当前绑定' }));
    expect(await screen.findByText('reconcile down')).toBeInTheDocument();

    // 翼 2：非 Error → fallback 文案
    mReconcile.mockRejectedValue(undefined);
    menu = await openMoreMenu('ChatOps');
    fireEvent.click(within(menu).getByRole('menuitem', { name: '重建当前绑定' }));
    expect(await screen.findByText('操作失败')).toBeInTheDocument();
  });

  it('卸载成功链：confirm 内容含实例号 → 危险确定 → uninstallExtension + 「已卸载扩展」+ reload', async () => {
    renderPage();
    await waitFirstLoad();

    const menu = await openMoreMenu('ChatOps');
    fireEvent.click(within(menu).getByRole('menuitem', { name: '卸载当前安装' }));
    expect(
      await screen.findByText('安装实例 #7 将被卸载，是否继续？', {
        selector: '.ant-modal-confirm-content',
      }),
    ).toBeInTheDocument();

    await confirmUninstall();
    await waitFor(() => expect(mUninstall).toHaveBeenCalledWith(7));
    expect(await screen.findByText('已卸载扩展')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('卸载依赖阻塞：dependency_blocked + blockers → modal.warning「无法卸载：存在依赖」+ orange Tags', async () => {
    mUninstall.mockRejectedValue({
      response: {
        data: {
          details: { code: 'dependency_blocked', blockers: ['chat-bridge', 'audit-log'] },
          message: 'blocked',
        },
      },
    });
    renderPage();
    await waitFirstLoad();

    const menu = await openMoreMenu('ChatOps');
    fireEvent.click(within(menu).getByRole('menuitem', { name: '卸载当前安装' }));
    await confirmUninstall();

    await waitFor(() => expect(mUninstall).toHaveBeenCalledWith(7));
    expect(
      await screen.findByText('无法卸载：存在依赖', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    expect(
      screen.getByText('以下扩展仍依赖当前扩展，请先处理它们：', {
        selector: '.ant-modal-confirm-content span',
      }),
    ).toBeInTheDocument();
    expect(screen.getByText('chat-bridge')).toBeInTheDocument();
    expect(screen.getByText('audit-log')).toBeInTheDocument();
    // blockers 非空：不走 message.error 分支
    expect(screen.queryByText('blocked')).not.toBeInTheDocument();
  });

  it('卸载普通错误：forbidden → message.error 透传 mapExtensionError 的 message', async () => {
    mUninstall.mockRejectedValue({
      response: { data: { details: { code: 'forbidden' } } },
    });
    renderPage();
    await waitFirstLoad();

    const menu = await openMoreMenu('ChatOps');
    fireEvent.click(within(menu).getByRole('menuitem', { name: '卸载当前安装' }));
    await confirmUninstall();

    expect(
      await screen.findByText('You do not have permission for this operation.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('无法卸载：存在依赖')).not.toBeInTheDocument();
  });

  it('卸载依赖阻塞但 blockers 空 → 不弹 warning、走 message.error 分支', async () => {
    mUninstall.mockRejectedValue({
      response: { data: { details: { code: 'dependency_blocked', blockers: [] } } },
    });
    renderPage();
    await waitFirstLoad();

    const menu = await openMoreMenu('ChatOps');
    fireEvent.click(within(menu).getByRole('menuitem', { name: '卸载当前安装' }));
    await confirmUninstall();

    expect(
      await screen.findByText('Uninstall blocked because other extensions still depend on it.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('无法卸载：存在依赖')).not.toBeInTheDocument();
  });

  it('卸载非 HTTP 错误（无 response.data.details）→ unknown 兜底 message', async () => {
    mUninstall.mockRejectedValue(new Error('plain'));
    renderPage();
    await waitFirstLoad();

    const menu = await openMoreMenu('ChatOps');
    fireEvent.click(within(menu).getByRole('menuitem', { name: '卸载当前安装' }));
    await confirmUninstall();

    expect(
      await screen.findByText('Please retry or contact an administrator.'),
    ).toBeInTheDocument();
  });

  it('columns 空值兜底：displayName 空→extensionId、status 空→-、healthStatus error→红 Tag', async () => {
    mList.mockResolvedValue({
      total: 1,
      items: [{ ...row1, id: 9, displayName: '', status: '', healthStatus: 'error' }],
    });
    renderPage();

    expect(await screen.findByText('#9 / chatops-demo')).toBeInTheDocument();
    expect(screen.getByText('chatops', { selector: 'strong' })).toBeInTheDocument();
    expect(screen.getByText('-')).toBeInTheDocument();
    expect(screen.getByText('error')).toBeInTheDocument();
    expect(screen.getByText('总数 1')).toBeInTheDocument();
  });
});

describe('扩展安装页 overlay 受控组合', () => {
  it('详情/事件/升级三桩打开携正确 row；events onClose 关闭；upgrade onUpgraded → reload', async () => {
    renderPage();
    await waitFirstLoad();

    // 查看详情 → detail 桩 open（两行都有同名按钮，scope 到 row1 行）
    const rowChat = screen.getByText('ChatOps').closest('tr') as HTMLElement;
    fireEvent.click(within(rowChat).getByRole('button', { name: '查看详情' }));
    const detail = await screen.findByTestId('detail-drawer');
    expect(detail).toHaveAttribute('data-row', '7');
    // onSaved → reload
    fireEvent.click(within(detail).getByRole('button', { name: '__detail-saved' }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    // onClose → 桩卸载
    fireEvent.click(within(detail).getByRole('button', { name: '__detail-close' }));
    await waitFor(() => expect(screen.queryByTestId('detail-drawer')).not.toBeInTheDocument());

    // 查看事件 → events 桩 open + onClose 关闭
    fireEvent.click(within(rowChat).getByRole('button', { name: '查看事件' }));
    const events = await screen.findByTestId('events-drawer');
    expect(events).toHaveAttribute('data-row', '7');
    fireEvent.click(within(events).getByRole('button', { name: '__events-close' }));
    await waitFor(() => expect(screen.queryByTestId('events-drawer')).not.toBeInTheDocument());

    // 更多菜单「升级当前安装」→ upgrade 桩 open；onUpgraded → reload
    const menu = await openMoreMenu('ChatOps');
    fireEvent.click(within(menu).getByRole('menuitem', { name: '升级当前安装' }));
    const upgrade = await screen.findByTestId('upgrade-modal');
    expect(upgrade).toHaveAttribute('data-row', '7');
    fireEvent.click(within(upgrade).getByRole('button', { name: '__upgrade-done' }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(3));
    fireEvent.click(within(upgrade).getByRole('button', { name: '__upgrade-close' }));
    await waitFor(() => expect(screen.queryByTestId('upgrade-modal')).not.toBeInTheDocument());
  });

  it('canExtensionsManage=false：更多菜单全部动作禁用，点击不触达任何写服务', async () => {
    mockCanManage.mockReturnValue(false);
    renderPage();
    await waitFirstLoad();

    const menu = await openMoreMenu('ChatOps');
    // antd menuitem 是 li：禁用形态是 aria-disabled（jest-dom toBeDisabled 不识别）
    expect(within(menu).getByRole('menuitem', { name: '禁用当前安装' })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
    expect(within(menu).getByRole('menuitem', { name: '升级当前安装' })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
    expect(within(menu).getByRole('menuitem', { name: '重建当前绑定' })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
    expect(within(menu).getByRole('menuitem', { name: '卸载当前安装' })).toHaveAttribute(
      'aria-disabled',
      'true',
    );

    fireEvent.click(within(menu).getByRole('menuitem', { name: '卸载当前安装' }));
    await new Promise((r) => setTimeout(r, 30));
    expect(mUninstall).not.toHaveBeenCalled();
    expect(screen.queryByText('确认卸载扩展')).not.toBeInTheDocument();
  });
});
