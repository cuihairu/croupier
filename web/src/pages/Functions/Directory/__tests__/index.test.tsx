/**
 * 函数目录页单测（覆盖率巡检：Functions/Directory/index.tsx 651 行 0% →
 * 收口——全仓最大零覆盖页面组件；同目录既有六个测试只打 hook/columns/
 * schema/BatchFloorModal，无人 import 页面本体）。
 *
 * 锁定契约：
 * - summary 汇总（enabled 过滤、resource 缺省并入「未声明」参与 distinct
 *   计数、operation 真值计数、topResource 降序取首；空数据 → topResource
 *   缺省 `|| undeclaredLabel` / `|| 0` 兜底翼，紫 Tag 条件渲染翼）；
 * - intro/Alert/Footer 三处导航回调（resource-catalog 直跳、测试调用
 *   selectedFunction 有无两翼：buildInvokePath(id) vs /functions/invoke）；
 * - 空范围提示（!loading && 空表才显示；scope.gameId/env 缺省 '-' 兜底）；
 * - 批量门槛链（已选 N 项告警、设置门槛开弹窗、Popconfirm 清除 →
 *   applyBatchFloor('')、取消选择；BatchFloorModal props 接线：open/
 *   count/submitting/versionOptions 去重并集、onSubmit/onClose 回流）；
 * - ProTable 数据透传（行渲染 + showTotal 共 N 个函数）；
 * - 详情抽屉（富/贫/无三形态：version/resource/operation/instances 缺省
 *   兜底、enabled 徽标双翼、displayName/summary/tags 三卡条件渲染、
 *   drawerActions extra、onClose 回调、Footer 双导航）。
 *
 * mock 口径：`../useDirectoryPage` 整体 mock（hook 面已由 batchFloor/
 * drawerActions/directoryBranches/fetchSummary 覆盖，本文件只锁页面渲染
 * 分支）；BatchFloorModal 以桩替换（内部已由 BatchFloorModal.test 覆盖，
 * 桩暴露 props 数据与 onSubmit/onClose 触发钮锁接线）；antd/
 * pro-components/@umijs/History 同款真实实现；@umijs/max 本地 mock
 * （useIntl 带 values 插值 + locale，FormattedMessage 直出 defaultMessage）。
 *
 * 边界（诚实）：intro「测试函数调用」依赖 selectedFunction（由行点击链
 * 设置，hook 域），此处经 mock 直供两翼值，不经过表格交互。
 */
import React from 'react';
import {
  cleanup,
  configure,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { App } from 'antd';
import DirectoryPage from '../index';
import useDirectoryPage from '../useDirectoryPage';
import type { DetailRow, SummaryRow } from '../types';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

// mock* 前缀变量：babel-jest hoist 白名单（工厂体内惰性求值，无 TDZ）
const mockIntl = {
  locale: 'zh-CN',
  formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, unknown>) => {
    let text = opts.defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
    }
    return text;
  },
};

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => mockIntl,
  history: { push: jest.fn() },
}));

jest.mock('../useDirectoryPage', () => ({
  __esModule: true,
  default: jest.fn(),
}));

jest.mock('../BatchFloorModal', () => {
  const R = require('react');
  return {
    __esModule: true,
    default: (props: {
      open: boolean;
      count: number;
      submitting: boolean;
      versionOptions: string[];
      onSubmit: (minVersion: string) => void;
      onClose: () => void;
    }) =>
      R.createElement(
        'div',
        {
          'data-testid': 'batch-floor-stub',
          'data-open': String(props.open),
          'data-count': String(props.count),
          'data-submitting': String(props.submitting),
          'data-options': JSON.stringify(props.versionOptions),
        },
        R.createElement(
          'button',
          { type: 'button', onClick: () => props.onSubmit('v9.0.0') },
          '桩·提交',
        ),
        R.createElement('button', { type: 'button', onClick: () => props.onClose() }, '桩·关闭'),
      ),
  };
});

import { history } from '@umijs/max';

const mHook = jest.mocked(useDirectoryPage);
const mPush = history.push as jest.Mock;

type HookReturn = ReturnType<typeof useDirectoryPage>;

function setHook(overrides: Partial<HookReturn> = {}): HookReturn {
  const base: HookReturn = {
    loading: false,
    processedData: [],
    columns: [{ title: '函数ID', dataIndex: 'id', key: 'id' }],
    headerActions: <span data-testid="header-actions">页头动作</span>,
    scope: { gameId: 'demo', env: 'prod' },
    detailVisible: false,
    setDetailVisible: jest.fn(),
    selectedFunction: null,
    drawerActions: <button type="button">抽屉动作</button>,
    buildInvokePath: jest.fn((id: string) => `/invoke/${id}`),
    selectedRowKeys: [],
    setSelectedRowKeys: jest.fn(),
    rowSelection: { type: 'checkbox', selectedRowKeys: [], onChange: jest.fn() },
    batchModalOpen: false,
    setBatchModalOpen: jest.fn(),
    batchSubmitting: false,
    applyBatchFloor: jest.fn(),
    versionIndex: {},
    floorSubmitting: false,
    handleViewDetail: jest.fn(),
  };
  const merged = { ...base, ...overrides };
  mHook.mockReturnValue(merged);
  return merged;
}

function renderPage() {
  return render(
    <App>
      <DirectoryPage />
    </App>,
  );
}

/** 混合数据：2 启用/1 禁用、resource player×2+缺省、operation 真值×2 */
const mixedRows: SummaryRow[] = [
  { id: 'fn.query', enabled: true, resource: 'player', operation: 'query' },
  { id: 'fn.mutate', enabled: false, resource: 'player', operation: 'mutate' },
  { id: 'fn.ghost', enabled: true },
];

const fullDetail: DetailRow = {
  id: 'fn.query',
  enabled: true,
  version: 'v1.2.0',
  resource: 'player',
  operation: 'query',
  instances: 2,
  displayName: { 'zh-CN': '查询玩家' },
  summary: { 'zh-CN': '按 ID 查询玩家档案' },
  tags: ['hot', 'read'],
};

beforeEach(() => {
  jest.clearAllMocks();
  setHook();
});

describe('函数目录页概览汇总', () => {
  it('混合数据：页头/标签/SummaryOverview/resultText/表格行/分页总数', async () => {
    setHook({ processedData: mixedRows });
    renderPage();

    // PageContainer 骨架
    expect(screen.getByText('函数目录')).toBeInTheDocument();
    expect(screen.getByText(/函数目录只管理原子能力契约/)).toBeInTheDocument();
    expect(screen.getByTestId('header-actions')).toBeInTheDocument();

    // intro 标签：能力供给层 / 可装配 2 / 已声明操作 2 / 紫色 topResource
    expect(screen.getByText('能力供给层')).toBeInTheDocument();
    expect(screen.getByText('可装配函数 2')).toBeInTheDocument();
    expect(screen.getByText('已声明操作 2')).toBeInTheDocument();
    const purple = document.querySelector('.ant-tag-purple');
    expect(purple).not.toBeNull();
    expect(purple?.textContent).toContain('当前最大资源 player · 2');

    // SummaryOverview：总数 3/启用 2/禁用 1/资源 2（player + 未声明 去重）
    expect(screen.getByText('总数 3')).toBeInTheDocument();
    expect(screen.getByText('启用 2')).toBeInTheDocument();
    expect(screen.getByText('禁用 1')).toBeInTheDocument();
    expect(screen.getByText('资源 2')).toBeInTheDocument();

    // 列表区：resultText + 行渲染 + showTotal
    expect(screen.getByText('当前结果 3 个函数')).toBeInTheDocument();
    expect(await screen.findByText('fn.query')).toBeInTheDocument();
    expect(screen.getByText('共 3 个函数')).toBeInTheDocument();
  });

  it('空数据：计数全 0，topResource 缺省翼——紫 Tag 不渲染', () => {
    setHook({ processedData: [] });
    renderPage();

    expect(screen.getByText('可装配函数 0')).toBeInTheDocument();
    expect(screen.getByText('已声明操作 0')).toBeInTheDocument();
    expect(screen.getByText('总数 0')).toBeInTheDocument();
    expect(screen.getByText('启用 0')).toBeInTheDocument();
    expect(screen.getByText('禁用 0')).toBeInTheDocument();
    expect(screen.getByText('资源 0')).toBeInTheDocument();
    expect(document.querySelector('.ant-tag-purple')).toBeNull();
  });
});

describe('函数目录页导航回调', () => {
  it('intro「查看资源/页面候选」与 Alert「查看资源」→ resource-catalog', () => {
    renderPage();
    fireEvent.click(screen.getAllByRole('button', { name: /查看资源\/页面候选/ })[0]);
    fireEvent.click(screen.getByRole('button', { name: '查看资源' }));
    expect(mPush).toHaveBeenCalledWith('/functions/resource-catalog');
    expect(mPush).toHaveBeenCalledTimes(2);
  });

  it('「测试函数调用」两翼：无选中 → /functions/invoke；有选中 → buildInvokePath(id)', () => {
    const h = setHook();
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: '测试函数调用' }));
    expect(mPush).toHaveBeenCalledWith('/functions/invoke');
    expect(h.buildInvokePath).not.toHaveBeenCalled();

    cleanup();
    const h2 = setHook({ selectedFunction: fullDetail });
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: '测试函数调用' }));
    expect(h2.buildInvokePath).toHaveBeenCalledWith('fn.query');
    expect(mPush).toHaveBeenCalledWith('/invoke/fn.query');
  });
});

describe('空范围提示（scope empty alert）', () => {
  it('非 loading + 空表 → 提示显示，scope 有值直显', () => {
    setHook({ processedData: [], scope: { gameId: 'demo', env: 'prod' } });
    renderPage();
    expect(screen.getByTestId('directory-scope-empty')).toBeInTheDocument();
    expect(screen.getByText(/当前查询范围：游戏 demo \/ 环境 prod/)).toBeInTheDocument();
  });

  it('scope 缺省 → 游戏 - / 环境 - 兜底', () => {
    setHook({ processedData: [], scope: {} });
    renderPage();
    expect(screen.getByText(/当前查询范围：游戏 - \/ 环境 -/)).toBeInTheDocument();
  });

  it('loading 或非空表 → 提示不渲染（左操作数两翼）', () => {
    setHook({ loading: true, processedData: [] });
    renderPage();
    expect(screen.queryByTestId('directory-scope-empty')).toBeNull();

    cleanup();
    setHook({ processedData: mixedRows });
    renderPage();
    expect(screen.queryByTestId('directory-scope-empty')).toBeNull();
  });
});

describe('批量门槛链', () => {
  it('选中态告警：已选 N 项 + 设置门槛开弹窗 + 取消选择', () => {
    const h = setHook({ selectedRowKeys: ['fn.query', 'fn.mutate'] });
    renderPage();

    expect(screen.getByText('已选 2 项')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '批量设置门槛' }));
    expect(h.setBatchModalOpen).toHaveBeenCalledWith(true);

    fireEvent.click(screen.getByRole('button', { name: '取消选择' }));
    expect(h.setSelectedRowKeys).toHaveBeenCalledWith([]);
  });

  it('未选中 → 批量告警不渲染', () => {
    setHook({ selectedRowKeys: [] });
    renderPage();
    expect(screen.queryByText(/已选/)).toBeNull();
    expect(screen.queryByRole('button', { name: '批量清除' })).toBeNull();
  });

  it('批量清除走 Popconfirm 确认 → applyBatchFloor(空串)', async () => {
    const h = setHook({ selectedRowKeys: ['fn.query'] });
    renderPage();

    fireEvent.click(screen.getByRole('button', { name: '批量清除' }));
    expect(await screen.findByText('清除已选函数的版本门槛？')).toBeInTheDocument();
    const ok = document.querySelector('.ant-popconfirm .ant-btn-primary') as HTMLElement;
    expect(ok).not.toBeNull();
    fireEvent.click(ok);
    await waitFor(() => expect(h.applyBatchFloor).toHaveBeenCalledWith(''));
  });

  it('BatchFloorModal 接线：版本并集去重 + onSubmit/onClose 回流', () => {
    const h = setHook({
      selectedRowKeys: ['fn.query', 'fn.mutate'],
      batchModalOpen: true,
      batchSubmitting: true,
      versionIndex: { 'fn.query': ['v2', 'v1'], 'fn.mutate': ['v1', 'v3'] },
    });
    renderPage();

    const stub = screen.getByTestId('batch-floor-stub');
    expect(stub).toHaveAttribute('data-open', 'true');
    expect(stub).toHaveAttribute('data-count', '2');
    expect(stub).toHaveAttribute('data-submitting', 'true');
    expect(JSON.parse(stub.getAttribute('data-options') ?? '[]')).toEqual(['v2', 'v1', 'v3']);

    fireEvent.click(screen.getByRole('button', { name: '桩·提交' }));
    expect(h.applyBatchFloor).toHaveBeenCalledWith('v9.0.0');

    fireEvent.click(screen.getByRole('button', { name: '桩·关闭' }));
    expect(h.setBatchModalOpen).toHaveBeenCalledWith(false);
  });

  it('batchModalOpen=false → 桩收到 open=false', () => {
    setHook({ batchModalOpen: false });
    renderPage();
    expect(screen.getByTestId('batch-floor-stub')).toHaveAttribute('data-open', 'false');
  });
});

describe('详情抽屉', () => {
  function drawerRoot(): HTMLElement {
    return screen.getByText('函数详情').closest('.ant-drawer') as HTMLElement;
  }

  it('富形态：全字段 + 三卡 + extra + 关闭回调', async () => {
    const h = setHook({ detailVisible: true, selectedFunction: fullDetail });
    renderPage();

    const drawer = await waitFor(() => drawerRoot());
    const item = (label: string): HTMLElement =>
      within(drawer).getByText(label).closest('.ant-descriptions-item') as HTMLElement;

    expect(within(item('函数ID')).getByText('fn.query')).toBeInTheDocument();
    expect(within(item('版本')).getByText('v1.2.0')).toBeInTheDocument();

    const resTag = item('资源').querySelector('.ant-tag') as HTMLElement;
    expect(resTag.className).toContain('ant-tag-geekblue');
    expect(within(item('资源')).getByText('player')).toBeInTheDocument();

    expect(within(item('操作')).getByText('query')).toBeInTheDocument();
    expect(within(item('状态')).getByText('启用')).toBeInTheDocument();
    expect(item('状态').querySelector('.ant-badge-status-success')).not.toBeNull();
    expect(within(item('覆盖实例')).getByText('2 个实例')).toBeInTheDocument();

    expect(within(drawer).getByText('查询玩家')).toBeInTheDocument();
    expect(within(drawer).getByText('按 ID 查询玩家档案')).toBeInTheDocument();
    expect(within(drawer).getByText('hot')).toBeInTheDocument();
    expect(within(drawer).getByText('read')).toBeInTheDocument();

    expect(within(drawer).getByRole('button', { name: '抽屉动作' })).toBeInTheDocument();

    fireEvent.click(drawer.querySelector('.ant-drawer-close') as HTMLElement);
    expect(h.setDetailVisible).toHaveBeenCalledWith(false);
  });

  it('富形态 Footer：resource-catalog 直跳 + 测试调用走 buildInvokePath', () => {
    const h = setHook({ detailVisible: true, selectedFunction: fullDetail });
    renderPage();
    const drawer = drawerRoot();

    fireEvent.click(within(drawer).getAllByRole('button', { name: /查看资源\/页面候选/ })[0]);
    expect(mPush).toHaveBeenCalledWith('/functions/resource-catalog');

    fireEvent.click(within(drawer).getByRole('button', { name: /测试调用/ }));
    expect(h.buildInvokePath).toHaveBeenCalledWith('fn.query');
    expect(mPush).toHaveBeenCalledWith('/invoke/fn.query');
  });

  it('贫形态：version/resource/operation/instances 缺省兜底 + 三卡不渲染 + 禁用翼', () => {
    setHook({
      detailVisible: true,
      selectedFunction: { id: 'fn.min', enabled: false },
    });
    renderPage();
    const drawer = drawerRoot();
    const item = (label: string): HTMLElement =>
      within(drawer).getByText(label).closest('.ant-descriptions-item') as HTMLElement;

    expect(within(item('版本')).getByText('未指定')).toBeInTheDocument();
    expect(within(item('资源')).getAllByText('未声明')).toHaveLength(1);
    expect(item('资源').querySelector('.ant-tag-geekblue')).toBeNull();
    expect(within(item('操作')).getByText('未声明')).toBeInTheDocument();
    expect(within(item('状态')).getByText('禁用')).toBeInTheDocument();
    expect(item('状态').querySelector('.ant-badge-status-default')).not.toBeNull();
    expect(within(item('覆盖实例')).getByText('未知')).toBeInTheDocument();

    expect(within(drawer).queryByText('显示名称')).toBeNull();
    expect(within(drawer).queryByText('函数描述')).toBeNull();
    expect(within(drawer).queryByText('标签')).toBeNull();
  });

  it('无选中（selectedFunction=null）→ 抽屉壳在、内容卡不渲染', () => {
    setHook({ detailVisible: true, selectedFunction: null });
    renderPage();
    expect(screen.getByText('函数详情')).toBeInTheDocument();
    expect(screen.queryByText('基本信息')).toBeNull();
  });

  it('关闭态 → 抽屉不渲染', () => {
    setHook({ detailVisible: false, selectedFunction: fullDetail });
    renderPage();
    expect(screen.queryByText('函数详情')).toBeNull();
  });
});
