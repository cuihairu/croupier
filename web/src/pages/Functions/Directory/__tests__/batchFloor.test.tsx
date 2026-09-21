/**
 * 函数目录批量版本门槛（hook 级，避开整页重型组件）：
 * 勾选 → 统一值批量设置/清除 → 门槛列局部刷新 + 勾选清空。
 * 部分失败 warning 列明细；请求级失败 error 且保留勾选便于重试。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { getIntl } from '@umijs/max';
import useDirectoryPage from '../useDirectoryPage';
import { buildDirectoryColumns } from '../columns';
import { DIRECTORY_PAGE_SCHEMA } from '../schema';
import { batchSetFunctionVersionFloor, listFunctionVersionFloors } from '@/services/api/functions';

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

jest.mock('@/services/api/functions', () => ({
  listFunctionVersionFloors: jest.fn(),
  batchSetFunctionVersionFloor: jest.fn(),
}));

// PageSchemaRenderer 依赖图过重，测试目标是批量门槛 hook 行为——按行为最小化复刻。
jest.mock('@/components/page-schema/PageSchemaRenderer', () => ({
  renderSchemaActions: () => null,
}));

jest.mock('@/components/page-schema/icons', () => ({
  resolveSchemaIcon: () => null,
}));

const mockListFloors = jest.mocked(listFunctionVersionFloors);
const mockBatchSet = jest.mocked(batchSetFunctionVersionFloor);

function Harness() {
  const page = useDirectoryPage();
  return (
    <div>
      <button
        type="button"
        data-testid="select-rows"
        onClick={() => page.rowSelection.onChange(['player.ban', 'mail.send'])}
      >
        select
      </button>
      <button type="button" data-testid="apply" onClick={() => page.applyBatchFloor('0.3.0')}>
        apply
      </button>
      <button type="button" data-testid="clear" onClick={() => page.applyBatchFloor('')}>
        clear
      </button>
      <div data-testid="sel">{page.selectedRowKeys.join(',')}</div>
    </div>
  );
}

describe('函数目录批量版本门槛', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockListFloors.mockResolvedValue({});
    mockBatchSet.mockResolvedValue({ updated: 2, failed: [] });
  });

  const renderHarness = () =>
    render(
      <AntdApp>
        <Harness />
      </AntdApp>,
    );

  const selectRows = async () => {
    renderHarness();
    await waitFor(() => {
      expect(mockListFloors).toHaveBeenCalled();
    });
    fireEvent.click(screen.getByTestId('select-rows'));
    expect(screen.getByTestId('sel').textContent).toBe('player.ban,mail.send');
  };

  it('applyBatchFloor 成功：service 以勾选 id + 统一版本调用，刷新门槛并清空勾选', async () => {
    await selectRows();
    mockListFloors.mockClear();
    mockListFloors.mockResolvedValue({ 'player.ban': '0.3.0', 'mail.send': '0.3.0' });

    fireEvent.click(screen.getByTestId('apply'));

    await waitFor(() => {
      expect(mockBatchSet).toHaveBeenCalledWith(['player.ban', 'mail.send'], '0.3.0');
    });
    // 成功后 reloadFloors 再拉一次门槛 + 勾选清空
    await waitFor(() => {
      expect(mockListFloors).toHaveBeenCalled();
      expect(screen.getByTestId('sel').textContent).toBe('');
    });
    expect(await screen.findByText('2 个函数的版本门槛已设为 0.3.0')).toBeInTheDocument();
  });

  it('批量清除：minVersion 空串提交', async () => {
    await selectRows();
    mockListFloors.mockClear();

    fireEvent.click(screen.getByTestId('clear'));

    await waitFor(() => {
      expect(mockBatchSet).toHaveBeenCalledWith(['player.ban', 'mail.send'], '');
    });
    expect(await screen.findByText('2 个函数的版本门槛已清除')).toBeInTheDocument();
  });

  it('部分失败：warning 列出失败明细，成功后仍清空勾选', async () => {
    await selectRows();
    mockBatchSet.mockResolvedValue({ updated: 1, failed: ['mail.send'] });

    fireEvent.click(screen.getByTestId('apply'));

    expect(await screen.findByText('已更新 1 个，失败 1 个：mail.send')).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByTestId('sel').textContent).toBe('');
    });
  });

  it('请求级失败：error 提示且勾选保留（便于重试）', async () => {
    await selectRows();
    mockBatchSet.mockRejectedValue(new Error('forbidden'));

    fireEvent.click(screen.getByTestId('apply'));

    expect(await screen.findByText('forbidden')).toBeInTheDocument();
    await waitFor(() => {
      // 选择保留
      expect(screen.getByTestId('sel').textContent).toBe('player.ban,mail.send');
    });
  });
});

describe('目录 minVersion 列渲染', () => {
  const renderCell = (record: { minVersion?: string }) => {
    const cols = buildDirectoryColumns({
      intl: getIntl(),
      columns: DIRECTORY_PAGE_SCHEMA.columns,
      rowActions: DIRECTORY_PAGE_SCHEMA.rowActions,
      versions: [],
      onOpenDetail: () => {},
      onOpenSchema: () => {},
      onInvoke: () => {},
    });
    const col = cols.find((c) => c.dataIndex === 'minVersion');
    return render(
      <AntdApp>{col?.render?.(undefined, record as never) as React.ReactNode}</AntdApp>,
    );
  };

  it('有门槛：蓝 Tag ≥ 版本值', () => {
    renderCell({ minVersion: '0.3.0' });
    expect(screen.getByText('≥ 0.3.0')).toBeInTheDocument();
  });

  it('未配置：显示 -（与 version 列空态一致）', () => {
    renderCell({});
    expect(screen.getByText('-')).toBeInTheDocument();
  });
});
