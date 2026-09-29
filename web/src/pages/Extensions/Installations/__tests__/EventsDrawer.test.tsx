/**
 * 扩展事件抽屉单测（Extensions 簇余量收口第 1 项：EventsDrawer.tsx 286 行 0% → 行覆盖收口）。
 *
 * 锁定契约：打开主链（抽屉标题 displayName 兜底 extensionId、概览三 chip 含
 * total 同步、首拉载荷 page/pageSize/level/keyword 缺省形态）、六列渲染矩阵
 * （formatUnix 时间、payload 有值 code 文本 / 空值 '-'、createdAt=0 → '-'）、
 * 无安装实例守卫（不发起请求 + 默认空态文案）、关键词筛选（trim 后入参 +
 * 生效 Alert「已生效条件」+ chip 关键词）、级别筛选（Select 下拉 → 载荷 +
 * chip 级别）、双条件「 / 」拼接、清空筛选按钮（初始 disabled、点击复位
 * chip/Alert/载荷）、筛选空态与默认空态两套文案、请求失败翼（success:false
 * 静默空表不弹错——全局拦截器 toast 语义）、切换安装实例重置筛选并重拉、
 * onClose 回调（抽屉关闭按钮）。
 *
 * mock 口径：services/api/extensions 仅 listExtensionEvents；adapter
 * （adaptEventListResponse）/formatUnix/SummaryOverview 走真实实现；
 * @umijs/max 本地 mock（对齐 Installations 套件先例）。
 *
 * 边界（诚实）：actionRef.current 的 undefined 翼（filter onChange 里
 * `actionRef.current?.setPageInfo?.()`）经 UI 不可达——筛选栏与 ProTable 同
 * commit 渲染，actionRef 在 ProTable 挂载后即被赋值，用户可交互时必非空；
 * Select 的 allowClear 清除翼与「清空筛选」按钮走同一 onChange/reset 函数体，
 * 不重复铺用例。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import EventsDrawer from '../EventsDrawer';
import { formatUnix } from '../shared';
import { listExtensionEvents } from '@/services/api/extensions';
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

const mList = listExtensionEvents as jest.MockedFunction<typeof listExtensionEvents>;

const inst = (over: Partial<ExtensionInstallationItem> = {}): ExtensionInstallationItem => ({
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
  ...over,
});

const ev = (over: Partial<ExtensionEventItem> = {}): ExtensionEventItem => ({
  eventType: 'install',
  level: 'info',
  message: '安装完成',
  payload: '',
  createdBy: 'admin',
  createdAt: 0,
  ...over,
});

// 两行矩阵：全字段行 + payload/createdAt 空值兜底行
const twoEvents = {
  items: [
    ev({
      eventType: 'health_check',
      level: 'warn',
      message: '探活超时',
      payload: '{"status":"unknown"}',
      createdBy: 'system',
      createdAt: 1727500000,
    }),
    ev({ eventType: 'upgrade', message: '升级到 1.4.1' }),
  ],
  total: 42,
};

function renderDrawer(props?: {
  open?: boolean;
  installation?: ExtensionInstallationItem | null;
  onClose?: () => void;
}) {
  const onClose = props?.onClose ?? jest.fn();
  const view = render(
    <App>
      <EventsDrawer
        open={props?.open ?? true}
        installation={props?.installation === undefined ? inst() : props.installation}
        onClose={onClose}
      />
    </App>,
  );
  return { onClose, ...view };
}

/** 等首拉落定（ProTable 20ms 防抖：挂载自动请求与 effect reload 双触发内部合并） */
async function waitFirstLoad(marker: string) {
  expect(await screen.findByText(marker)).toBeInTheDocument();
  await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
}

/** 打开级别下拉并选择一项（antd6：mouseDown 落 .ant-select 根；option 收窄到可见 content） */
async function pickLevel(label: string) {
  // 筛选栏 Select 是抽屉 body 内首个 .ant-select（分页 size changer 在表格之后）
  const selectRoot = document.querySelector('.ant-drawer-body .ant-select') as HTMLElement;
  expect(selectRoot).not.toBeNull();
  fireEvent.mouseDown(selectRoot);
  const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
  expect(dropdown).not.toBeNull();
  fireEvent.click(
    within(dropdown).getByText(label, { selector: '.ant-select-item-option-content' }),
  );
}

/** 输入关键词（Input 在抽屉 body 内，placeholder 唯一） */
function typeKeyword(value: string) {
  fireEvent.change(screen.getByPlaceholderText('筛选事件/内容/操作者'), {
    target: { value },
  });
}

beforeEach(() => {
  jest.clearAllMocks();
  mList.mockResolvedValue({ ...twoEvents });
});

describe('EventsDrawer 打开主链与渲染矩阵', () => {
  it('标题/概览三 chip（total 同步）/首拉载荷/六列渲染', async () => {
    renderDrawer();
    await waitFirstLoad('health_check');

    // 抽屉标题：displayName 拼接 #id
    expect(screen.getByText('扩展事件: ChatOps (#7)')).toBeInTheDocument();
    // 概览：total 来自 adaptEventListResponse 归一（42），未筛选两 chip 走缺省文案
    expect(screen.getByText('事件 42')).toBeInTheDocument();
    expect(screen.getByText('全部级别')).toBeInTheDocument();
    expect(screen.getByText('未设置关键词')).toBeInTheDocument();

    // 首拉载荷：分页缺省 + level/keyword 未筛选
    expect(mList).toHaveBeenCalledWith(7, {
      level: undefined,
      keyword: undefined,
      page: 1,
      pageSize: 10,
    });

    // 全字段行
    expect(screen.getByText('探活超时')).toBeInTheDocument();
    expect(screen.getByText('{"status":"unknown"}')).toBeInTheDocument();
    expect(screen.getByText('system')).toBeInTheDocument();
    expect(screen.getByText(formatUnix(1727500000))).toBeInTheDocument();
    // 空值兜底行：payload '-' / createdAt=0 → '-'
    expect(screen.getByText('升级到 1.4.1')).toBeInTheDocument();
    expect(screen.getByText('admin')).toBeInTheDocument();
    const dashes = screen.getAllByText('-');
    expect(dashes.length).toBeGreaterThanOrEqual(2);
  });

  it('displayName 空兜底 extensionId', async () => {
    renderDrawer({ installation: inst({ id: 8, displayName: '', extensionId: 'wiki' }) });
    await waitFirstLoad('health_check');
    expect(screen.getByText('扩展事件: wiki (#8)')).toBeInTheDocument();
  });

  it('无安装实例守卫：不发起请求 + 默认空态文案', async () => {
    renderDrawer({ installation: null });
    expect(
      await screen.findByText('暂时没有事件数据，后续有安装动作后会显示在这里。'),
    ).toBeInTheDocument();
    expect(mList).not.toHaveBeenCalled();
  });
});

describe('EventsDrawer 筛选契约', () => {
  it('关键词：trim 入参 + 生效 Alert + chip', async () => {
    renderDrawer();
    await waitFirstLoad('health_check');

    typeKeyword('  abc  ');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(7, {
        level: undefined,
        keyword: 'abc',
        page: 1,
        pageSize: 10,
      }),
    );
    expect(screen.getByText('关键词 abc')).toBeInTheDocument();
    expect(screen.getByText('当前正在查看筛选后的事件范围')).toBeInTheDocument();
    expect(screen.getByText('已生效条件：关键词 abc')).toBeInTheDocument();
  });

  it('级别：下拉选择 → 载荷 + chip + Alert 单条件', async () => {
    renderDrawer();
    await waitFirstLoad('health_check');

    await pickLevel('error');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(7, {
        level: 'error',
        keyword: undefined,
        page: 1,
        pageSize: 10,
      }),
    );
    expect(screen.getByText('级别 error')).toBeInTheDocument();
    expect(screen.getByText('已生效条件：级别 error')).toBeInTheDocument();
  });

  it('双条件拼接：关键词 / 级别', async () => {
    renderDrawer();
    await waitFirstLoad('health_check');

    typeKeyword('abc');
    await pickLevel('warn');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(7, {
        level: 'warn',
        keyword: 'abc',
        page: 1,
        pageSize: 10,
      }),
    );
    expect(screen.getByText('已生效条件：关键词 abc / 级别 warn')).toBeInTheDocument();
  });

  it('清空筛选：初始 disabled，设条件后可点并全复位', async () => {
    renderDrawer();
    await waitFirstLoad('health_check');

    const clear = screen.getByRole('button', { name: '清空筛选' });
    expect(clear).toBeDisabled();

    typeKeyword('abc');
    await pickLevel('error');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(7, {
        level: 'error',
        keyword: 'abc',
        page: 1,
        pageSize: 10,
      }),
    );
    expect(clear).toBeEnabled();

    fireEvent.click(clear);
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(7, {
        level: undefined,
        keyword: undefined,
        page: 1,
        pageSize: 10,
      }),
    );
    expect(screen.getByText('未设置关键词')).toBeInTheDocument();
    expect(screen.getByText('全部级别')).toBeInTheDocument();
    expect(screen.queryByText('当前正在查看筛选后的事件范围')).not.toBeInTheDocument();
  });

  it('筛选空态文案（区别于默认空态）', async () => {
    mList.mockResolvedValue({ items: [], total: 0 });
    renderDrawer();
    await waitFirstLoad('暂时没有事件数据，后续有安装动作后会显示在这里。');

    typeKeyword('nope');
    expect(
      await screen.findByText('当前筛选条件下没有匹配事件，请调整筛选后重试。'),
    ).toBeInTheDocument();
    expect(screen.getByText('事件 0')).toBeInTheDocument();
  });
});

describe('EventsDrawer 生命周期与失败翼', () => {
  it('请求失败：success:false 静默空表（不本地弹错）', async () => {
    mList.mockRejectedValue(new Error('boom'));
    renderDrawer();
    expect(
      await screen.findByText('暂时没有事件数据，后续有安装动作后会显示在这里。'),
    ).toBeInTheDocument();
    expect(screen.getByText('事件 0')).toBeInTheDocument();
  });

  it('切换安装实例：重置筛选并以新 id 重拉', async () => {
    const { rerender } = renderDrawer();
    await waitFirstLoad('health_check');

    typeKeyword('abc');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(7, {
        level: undefined,
        keyword: 'abc',
        page: 1,
        pageSize: 10,
      }),
    );

    rerender(
      <App>
        <EventsDrawer
          open
          installation={inst({ id: 9, extensionId: 'wiki' })}
          onClose={jest.fn()}
        />
      </App>,
    );
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(9, {
        level: undefined,
        keyword: undefined,
        page: 1,
        pageSize: 10,
      }),
    );
    expect(screen.getByText('未设置关键词')).toBeInTheDocument();
  });

  it('onClose：抽屉关闭按钮回调', async () => {
    const onClose = jest.fn();
    renderDrawer({ onClose });
    await waitFirstLoad('health_check');

    const closeBtn = document.querySelector('.ant-drawer-close') as HTMLElement;
    expect(closeBtn).not.toBeNull();
    fireEvent.click(closeBtn);
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
