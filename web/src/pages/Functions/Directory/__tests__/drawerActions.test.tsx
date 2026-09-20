/**
 * 函数目录行详情抽屉动作（hook 级，避开整页重型组件）：
 * 「变更历史」直达该函数详情页 ?tab=versions（契约版本历史），
 * 「详情页」保持跳转详情首页。此前抽屉只有「详情页/调用函数」，
 * 用户无法从目录页一步进入变更历史。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import useDirectoryPage from '../useDirectoryPage';
import { history } from '@umijs/max';
import { listFunctionInstances } from '@/services/api';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

jest.mock('@umijs/max', () => {
  const formatMessage = jest.fn(
    ({ defaultMessage }: { defaultMessage: string }, values?: Record<string, unknown>) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  );
  // 必须返回稳定引用：useDirectoryPage 的拉数 effect 依赖 intl，
  // 每次渲染新建对象会无限重建（Maximum update depth）。
  const intl = { formatMessage };
  return {
    __esModule: true,
    useIntl: () => intl,
    getIntl: () => intl,
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
    history: { push: jest.fn() },
  };
});

jest.mock('@/services/api', () => ({
  listDescriptors: jest.fn().mockResolvedValue({ functions: [] }),
  listFunctionInstances: jest.fn(),
}));

jest.mock('@/services/api/functions-enhanced', () => ({
  getFunctionSummary: jest.fn().mockResolvedValue([]),
}));

// PageSchemaRenderer 依赖图过重（整页 schema 渲染链），测试目标是
// drawerActions 的 key 路由分支而非渲染器本身——按行为最小化复刻。
jest.mock('@/components/page-schema/PageSchemaRenderer', () => ({
  renderSchemaActions: (
    props: { onAction: (key: string) => void; flags: Record<string, boolean> },
    actions: Array<{ key: string; label: string; disabledWhen?: string[] }>,
  ) =>
    actions.map((action) => (
      <button
        key={action.key}
        type="button"
        disabled={action.disabledWhen?.some((flag) => props.flags[flag])}
        onClick={() => props.onAction(action.key)}
      >
        {action.label}
      </button>
    )),
}));

jest.mock('@/components/page-schema/icons', () => ({
  resolveSchemaIcon: () => null,
}));

const mockInstances = jest.mocked(listFunctionInstances);
const mockPush = jest.mocked(history.push);

function Harness() {
  const page = useDirectoryPage();
  return (
    <div>
      <button
        type="button"
        data-testid="open-detail"
        onClick={() =>
          page.handleViewDetail({ id: 'player.list', enabled: true, tags: [] } as never)
        }
      >
        open
      </button>
      <div data-testid="drawer-extra">{page.drawerActions}</div>
    </div>
  );
}

describe('函数目录详情抽屉动作', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockInstances.mockResolvedValue({ instances: [] } as never);
  });

  const openDrawer = async () => {
    render(
      <AntdApp>
        <Harness />
      </AntdApp>,
    );
    fireEvent.click(screen.getByTestId('open-detail'));
    await waitFor(() => {
      expect(screen.getByTestId('drawer-extra').querySelectorAll('button').length).toBeGreaterThan(
        0,
      );
    });
  };

  it('「变更历史」直达详情页 ?tab=versions', async () => {
    await openDrawer();
    const button = await screen.findByRole('button', { name: /变更历史/ });
    fireEvent.click(button);
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith('/functions/player.list?tab=versions');
    });
  });

  it('「详情页」跳转详情首页（回归）', async () => {
    await openDrawer();
    const button = await screen.findByRole('button', { name: /详情页/ });
    fireEvent.click(button);
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith('/functions/player.list');
    });
  });
});
