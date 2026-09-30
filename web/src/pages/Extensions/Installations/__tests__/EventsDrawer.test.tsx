/**
 * 扩展事件抽屉单测（覆盖率巡检：EventsDrawer.tsx 286 行 0% → 收口，
 * Extensions 簇余量顺序首位）。
 *
 * 锁定契约：抽屉标题（displayName 兜底 extensionId）、概览三项（事件总数
 * 副本/级别/关键词 chips）、表格列渲染矩阵（时间 formatUnix 秒/毫秒自适应、
 * payload 空兜底 '-'）、无安装实例守卫（不发请求 + 默认空态文案）、关键词/
 * 级别筛选（trim 载荷、Alert 已生效条件单/组合拼接、筛选态空态文案切换、
 * 清空筛选双态复位 + 按钮禁用门）、切换安装实例重置筛选并按新 id 重拉、
 * request 失败静默翼（success:false 不弹错）、关闭态内容不挂载不拉取。
 *
 * mock 口径：services/api/extensions 的 listExtensionEvents jest.mock；
 * adapter / formatUnix / SummaryOverview / ProTable 走真实实现；@umijs/max
 * 本地 mock（与 index.test 同款）。筛选断言一律锚「最后一次调用」——打开时
 * effect 的 reload 与首挂载请求会被 ProTable 内部 abort 合并，计数不具确定性。
 *
 * 边界（诚实）：无不可达分支——`kw?.trim() || undefined` 的左翼（kw 恒
 * string）与 `installation?.id` 的 undefined 翼已由守卫用例覆盖。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import EventsDrawer from '../EventsDrawer';
import type { ExtensionEventItem, ExtensionInstallationItem } from '@/services/api/extensions';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/extensions', () => ({
  listExtensionEvents: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

import { listExtensionEvents } from '@/services/api/extensions';

const mEvents = listExtensionEvents as jest.MockedFunction<typeof listExtensionEvents>;

const installation: ExtensionInstallationItem = {
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

const ev1: ExtensionEventItem = {
  eventType: 'install.succeeded',
  level: 'info',
  message: '安装成功',
  payload: '{"version":"1.4.0"}',
  createdBy: 'admin',
  createdAt: 1727500000, // 秒级
};
const ev2: ExtensionEventItem = {
  eventType: 'health.failed',
  level: 'error',
  message: '健康检查失败',
  payload: '',
  createdBy: 'system',
  createdAt: 2727500000123, // 毫秒级（>1e12 直用）
};

function renderDrawer(props?: Partial<React.ComponentProps<typeof EventsDrawer>>) {
  return render(<EventsDrawer open installation={installation} onClose={jest.fn()} {...props} />);
}

/** 关键词输入框 */
function keywordInput() {
  return screen.getByPlaceholderText('筛选事件/内容/操作者') as HTMLInputElement;
}

/** 级别下拉：antd6 Select 无稳定 placeholder 形态（span/input 因版本而异），
 * 改锚抽屉内首个 .ant-select——工具栏 Space 在 DOM 序上先于表格分页的
 * size changer，首个即级别筛选 */
function levelSelectRoot() {
  const drawer = document.querySelector('.ant-drawer') as HTMLElement;
  expect(drawer).not.toBeNull();
  return drawer.querySelector('.ant-select') as HTMLElement;
}

/** 在级别下拉里选一个 option（可见 option 行无 role，点 content 冒泡） */
async function pickLevel(label: string) {
  fireEvent.mouseDown(levelSelectRoot());
  const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
  expect(dropdown).not.toBeNull();
  fireEvent.click(
    within(dropdown).getByText(label, { selector: '.ant-select-item-option-content' }),
  );
}

beforeEach(() => {
  jest.clearAllMocks();
  mEvents.mockResolvedValue({ items: [ev1, ev2], total: 2 });
});

describe('扩展事件抽屉 首拉与渲染矩阵', () => {
  it('标题/概览三项/六列矩阵（formatUnix 秒与毫秒、payload 空 -）、request 载荷', async () => {
    renderDrawer();

    expect(await screen.findByText('install.succeeded')).toBeInTheDocument();
    expect(screen.getByText('扩展事件: ChatOps (#7)')).toBeInTheDocument();

    // 概览三项：总数副本在 request 成功后同步 + 未筛选态两 chip
    expect(screen.getByText('事件 2')).toBeInTheDocument();
    expect(screen.getByText('全部级别')).toBeInTheDocument();
    expect(screen.getByText('未设置关键词')).toBeInTheDocument();

    // 列矩阵：时间（与组件同进程 toLocaleString 计算，秒级 ×1000 / 毫秒直用）
    expect(screen.getByText(new Date(1727500000 * 1000).toLocaleString())).toBeInTheDocument();
    expect(screen.getByText(new Date(2727500000123).toLocaleString())).toBeInTheDocument();
    expect(screen.getByText('info')).toBeInTheDocument();
    expect(screen.getByText('error')).toBeInTheDocument();
    expect(screen.getByText('安装成功')).toBeInTheDocument();
    expect(screen.getByText('健康检查失败')).toBeInTheDocument();
    expect(screen.getByText('{"version":"1.4.0"}')).toBeInTheDocument();
    expect(screen.getByText('-')).toBeInTheDocument(); // ev2 payload 空
    expect(screen.getByText('admin')).toBeInTheDocument();
    expect(screen.getByText('system')).toBeInTheDocument();

    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith(
        7,
        expect.objectContaining({ level: undefined, keyword: undefined, page: 1, pageSize: 10 }),
      ),
    );
  });

  it('displayName 空 → 标题兜底 extensionId', async () => {
    mEvents.mockResolvedValue({ items: [], total: 0 });
    renderDrawer({ installation: { ...installation, displayName: '' } });
    expect(await screen.findByText('扩展事件: chatops (#7)')).toBeInTheDocument();
  });

  it('无安装实例守卫：不发请求 + 默认空态文案（非筛选态）', async () => {
    renderDrawer({ installation: null });
    expect(
      await screen.findByText('暂时没有事件数据，后续有安装动作后会显示在这里。'),
    ).toBeInTheDocument();
    await new Promise((r) => setTimeout(r, 30));
    expect(mEvents).not.toHaveBeenCalled();
  });

  it('request 失败静默翼：success:false 不弹错、总数落零', async () => {
    mEvents.mockRejectedValue(new Error('events down'));
    renderDrawer();
    await waitFor(() => expect(mEvents).toHaveBeenCalled());
    expect(
      await screen.findByText('暂时没有事件数据，后续有安装动作后会显示在这里。'),
    ).toBeInTheDocument();
    expect(screen.getByText('事件 0')).toBeInTheDocument();
  });

  it('关闭态：抽屉内容不挂载、不发起请求', async () => {
    renderDrawer({ open: false, installation: null });
    expect(screen.queryByText('事件筛选')).not.toBeInTheDocument();
    await new Promise((r) => setTimeout(r, 20));
    expect(mEvents).not.toHaveBeenCalled();
  });
});

describe('扩展事件抽屉 筛选', () => {
  it('关键词输入 → trim 载荷 + Alert 单条件拼接', async () => {
    renderDrawer();
    await screen.findByText('install.succeeded');

    fireEvent.change(keywordInput(), { target: { value: '  install  ' } });
    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith(7, expect.objectContaining({ keyword: 'install' })),
    );
    expect(screen.getByText('当前正在查看筛选后的事件范围')).toBeInTheDocument();
    expect(screen.getByText('已生效条件：关键词 install')).toBeInTheDocument();
    expect(screen.getByText('关键词 install')).toBeInTheDocument();
  });

  it('级别下拉 → level 载荷 + 筛选态空态文案切换', async () => {
    mEvents.mockResolvedValue({ items: [], total: 0 });
    renderDrawer();
    await screen.findByText('未设置关键词');

    await pickLevel('error');
    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith(7, expect.objectContaining({ level: 'error' })),
    );
    expect(
      await screen.findByText('当前筛选条件下没有匹配事件，请调整筛选后重试。'),
    ).toBeInTheDocument();
    expect(screen.getByText('已生效条件：级别 error')).toBeInTheDocument();
    expect(screen.getByText('级别 error')).toBeInTheDocument();
  });

  it('关键词 + 级别组合：Alert 以 / 拼接；清空筛选双态复位 + 按钮回禁用', async () => {
    renderDrawer();
    await screen.findByText('install.succeeded');

    fireEvent.change(keywordInput(), { target: { value: 'health' } });
    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith(7, expect.objectContaining({ keyword: 'health' })),
    );
    await pickLevel('warn');
    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith(7, expect.objectContaining({ level: 'warn' })),
    );
    expect(screen.getByText('已生效条件：关键词 health / 级别 warn')).toBeInTheDocument();

    // 清空前可用、清空后回禁用
    const clear = screen.getByRole('button', { name: '清空筛选' });
    expect(clear).toBeEnabled();
    fireEvent.click(clear);
    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith(
        7,
        expect.objectContaining({ keyword: undefined, level: undefined }),
      ),
    );
    expect(screen.queryByText('当前正在查看筛选后的事件范围')).not.toBeInTheDocument();
    expect(keywordInput().value).toBe('');
    await waitFor(() => expect(screen.getByRole('button', { name: '清空筛选' })).toBeDisabled());
  });

  it('切换安装实例：筛选重置 + 按新实例 id 重拉', async () => {
    const { rerender } = renderDrawer();
    await screen.findByText('install.succeeded');

    fireEvent.change(keywordInput(), { target: { value: 'stale' } });
    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith(7, expect.objectContaining({ keyword: 'stale' })),
    );

    const other: ExtensionInstallationItem = { ...installation, id: 8, extensionId: 'wiki' };
    rerender(<EventsDrawer open installation={other} onClose={jest.fn()} />);
    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith(
        8,
        expect.objectContaining({ keyword: undefined, level: undefined }),
      ),
    );
    expect(keywordInput().value).toBe('');
    expect(await screen.findByText('扩展事件: ChatOps (#8)')).toBeInTheDocument();
  });
});
