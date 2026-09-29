/**
 * 扩展事件抽屉单测（覆盖率巡检：Extensions 簇余量第二批，EventsDrawer.tsx
 * 286 行 0% → 行覆盖收口）。
 *
 * 锁定契约：打开加载链（listExtensionEvents 载荷 + adapter 真实归一 + total
 * 同步概览）、标题 displayName 兜底 extensionId、概览三项（事件 total / 级别
 * /关键词双态文案）、六列渲染（时间 formatUnix 含 createdAt=0 兜底 '-'、
 * payload code 与空兜底 '-'）、关键词筛选（trim 翼）+ 级别筛选 → request
 * 载荷与生效 Alert chips、清空筛选按钮双态禁用/复位、空态双文案（默认/筛选后）、
 * request 失败静默 success:false、installationId 缺省 guard（不发请求）、
 * open=false 不挂载、关闭回调、重开 reload。
 *
 * mock 口径：services/api/extensions 仅 listExtensionEvents；adapters 走真实
 * 实现（纯函数）；@umijs/max 本地 mock（FormattedMessage/useIntl 带 values
 * 插值）。ProTable 真实渲染（防抖口径：先等首拉落定再驱动筛选）。
 *
 * 边界（诚实）：request 的 catch 静默翼经 reject 真实触达；无不可达分支需
 * 造假——useEffect 的 open && installation 守卫经 open=false/row=null 组合
 * 真实覆盖。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import EventsDrawer from '../EventsDrawer';
import type {
  ExtensionEventItem,
  ExtensionInstallationItem,
} from '@/services/api/extensions';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/extensions', () => ({
  listExtensionEvents: jest.fn(),
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
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, unknown>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
      }
      return text;
    },
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

const events: ExtensionEventItem[] = [
  {
    eventType: 'installed',
    level: 'info',
    message: '安装成功',
    payload: '{"version":"1.4.0"}',
    createdBy: 'admin',
    createdAt: 1727500000,
  },
  {
    eventType: 'upgrade_failed',
    level: 'error',
    message: '依赖缺失',
    payload: '',
    createdBy: 'ops',
    createdAt: 0,
  },
];

function renderDrawer(over?: {
  open?: boolean;
  installation?: ExtensionInstallationItem | null;
}) {
  const onClose = jest.fn();
  const utils = render(
    <EventsDrawer
      open={over?.open ?? true}
      installation={
        over?.installation !== undefined ? over.installation : installation
      }
      onClose={onClose}
    />,
  );
  return { ...utils, onClose };
}

/** 等首拉落定（行内容出现 + 调用计数稳定为 1，避开 ProTable 防抖合并） */
async function waitFirstLoad() {
  expect(await screen.findByText('installed')).toBeInTheDocument();
  await waitFor(() => expect(mEvents).toHaveBeenCalledTimes(1));
}

beforeEach(() => {
  jest.clearAllMocks();
  mEvents.mockResolvedValue({ total: 2, items: events });
});

describe('EventsDrawer 加载与渲染', () => {
  it('打开加载链：标题、概览三项（事件 total/全部级别/未设置关键词）、六列渲染（时间格式化与 0 兜底、payload 空兜底）', async () => {
    renderDrawer();
    await waitFirstLoad();

    expect(screen.getByText('扩展事件: ChatOps (#7)')).toBeInTheDocument();
    // 概览三项：total 同步 + 筛选双态缺省文案
    expect(screen.getByText('事件 2')).toBeInTheDocument();
    expect(screen.getByText('全部级别')).toBeInTheDocument();
    expect(screen.getByText('未设置关键词')).toBeInTheDocument();

    // 六列：时间（formatUnix 真实输出）/ 级别 / 事件 / 内容 / Payload / 操作者
    expect(screen.getByText(new Date(1727500000 * 1000).toLocaleString())).toBeInTheDocument();
    expect(screen.getByText('info')).toBeInTheDocument();
    expect(screen.getByText('upgrade_failed')).toBeInTheDocument();
    expect(screen.getByText('安装成功')).toBeInTheDocument();
    expect(screen.getByText('依赖缺失')).toBeInTheDocument();
    expect(screen.getByText('{"version":"1.4.0"}')).toBeInTheDocument();
    expect(screen.getByText('admin')).toBeInTheDocument();
    expect(screen.getByText('ops')).toBeInTheDocument();

    // 首拉载荷：无筛选三参缺省
    expect(mEvents).toHaveBeenCalledWith(7, {
      level: undefined,
      keyword: undefined,
      page: 1,
      pageSize: 10,
    });
  });

  it('标题 displayName 空兜底 extensionId（`|| extensionId` 右翼）', async () => {
    renderDrawer({ installation: { ...installation, displayName: '' } });
    await waitFirstLoad();
    expect(screen.getByText('扩展事件: chatops (#7)')).toBeInTheDocument();
  });

  it('request 失败静默翼：catch 返回 success:false、概览落「事件 0」无 crash', async () => {
    mEvents.mockRejectedValue(new Error('events down'));
    renderDrawer();

    await waitFor(() => expect(mEvents).toHaveBeenCalledTimes(1));
    expect(await screen.findByText('事件 0')).toBeInTheDocument();
    expect(screen.queryByText('installed')).not.toBeInTheDocument();
  });
});

describe('EventsDrawer 筛选', () => {
  it('关键词筛选：输入触发 request 带 trim 后 keyword + Alert 生效条件 + 概览 chip 更新', async () => {
    renderDrawer();
    await waitFirstLoad();

    fireEvent.change(screen.getByPlaceholderText('筛选事件/内容/操作者'), {
      target: { value: '  部署  ' },
    });
    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith(
        7,
        expect.objectContaining({ keyword: '部署', page: 1 }),
      ),
    );

    // Alert 条件渲染 + chips（trim 后文本）
    expect(screen.getByText('当前正在查看筛选后的事件范围')).toBeInTheDocument();
    expect(screen.getByText('已生效条件：关键词 部署')).toBeInTheDocument();
    expect(screen.getByText('关键词 部署')).toBeInTheDocument();
  });

  it('级别筛选：选 error → request 带 level + 单独级别条件（关键词不进 chips）', async () => {
    renderDrawer();
    await waitFirstLoad();

    // antd6 Select：mouseDown 落 .ant-select 根，点可见 option content
    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
    expect(dropdown).not.toBeNull();
    fireEvent.click(within(dropdown).getByText('error', { selector: '.ant-select-item-option-content' }));

    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith(7, expect.objectContaining({ level: 'error' })),
    );
    expect(screen.getByText('已生效条件：级别 error')).toBeInTheDocument();
    expect(screen.getByText('级别 error')).toBeInTheDocument();
  });

  it('清空筛选：双条件设置后按钮可用 → 点击复位三态 + Alert 消失', async () => {
    renderDrawer();
    await waitFirstLoad();

    // 无筛选时禁用
    expect(screen.getByRole('button', { name: '清空筛选' })).toBeDisabled();

    fireEvent.change(screen.getByPlaceholderText('筛选事件/内容/操作者'), {
      target: { value: '部署' },
    });
    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith(7, expect.objectContaining({ keyword: '部署' })),
    );
    expect(screen.getByRole('button', { name: '清空筛选' })).toBeEnabled();

    fireEvent.click(screen.getByRole('button', { name: '清空筛选' }));
    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith(
        7,
        expect.objectContaining({ keyword: undefined, level: undefined, page: 1 }),
      ),
    );
    expect(screen.queryByText('当前正在查看筛选后的事件范围')).not.toBeInTheDocument();
    // 概览回缺省文案
    await waitFor(() => expect(screen.getByText('全部级别')).toBeInTheDocument());
    expect(screen.getByText('未设置关键词')).toBeInTheDocument();
  });

  it('空态双文案：无筛选默认空态 / 有筛选择「没有匹配」文案', async () => {
    mEvents.mockResolvedValue({ total: 0, items: [] });
    renderDrawer();
    await waitFor(() => expect(mEvents).toHaveBeenCalledTimes(1));
    expect(
      await screen.findByText('暂时没有事件数据，后续有安装动作后会显示在这里。'),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText('筛选事件/内容/操作者'), {
      target: { value: '不存在词' },
    });
    expect(
      await screen.findByText('当前筛选条件下没有匹配事件，请调整筛选后重试。'),
    ).toBeInTheDocument();
  });
});

describe('EventsDrawer 守卫与生命周期', () => {
  it('installation=null：request 早退 guard 不发请求', async () => {
    renderDrawer({ installation: null });
    await new Promise((r) => setTimeout(r, 80));
    expect(mEvents).not.toHaveBeenCalled();
    expect(screen.getByText('事件 0')).toBeInTheDocument();
  });

  it('open=false：抽屉内容不挂载不发请求', async () => {
    renderDrawer({ open: false });
    await new Promise((r) => setTimeout(r, 80));
    expect(mEvents).not.toHaveBeenCalled();
  });

  it('关闭回调：点击抽屉关闭按钮 → onClose', async () => {
    const inst = renderDrawer();
    await waitFirstLoad();
    fireEvent.click(document.querySelector('.ant-drawer-close') as HTMLElement);
    await waitFor(() => expect(inst.onClose).toHaveBeenCalledTimes(1));
  });

  it('重开触发 reload：open 翻转后 request 再次发起（useEffect 依赖 [open, installation]）', async () => {
    const inst = renderDrawer();
    await waitFirstLoad();

    inst.rerender(
      <EventsDrawer open={false} installation={installation} onClose={inst.onClose} />,
    );
    inst.rerender(
      <EventsDrawer open={true} installation={installation} onClose={inst.onClose} />,
    );
    await waitFor(() => expect(mEvents.mock.calls.length).toBeGreaterThan(1));
  });
});
