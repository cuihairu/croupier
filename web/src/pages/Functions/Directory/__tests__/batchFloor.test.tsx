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
import { getFunctionSummary } from '@/services/api/functions-enhanced';
import {
  batchSetFunctionVersionFloor,
  deleteFunctionVersionFloor,
  listFunctionVersionFloors,
  listFunctionVersionHistory,
  putFunctionVersionFloor,
} from '@/services/api/functions';

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
  fetchAssignments: jest.fn().mockResolvedValue({ assignments: {} }),
}));

jest.mock('@/services/api/functions-enhanced', () => ({
  getFunctionSummary: jest.fn(),
}));

jest.mock('@/services/api/functions', () => ({
  listFunctionVersionFloors: jest.fn(),
  listFunctionVersionHistory: jest.fn(),
  batchSetFunctionVersionFloor: jest.fn(),
  putFunctionVersionFloor: jest.fn(),
  deleteFunctionVersionFloor: jest.fn(),
}));

// PageSchemaRenderer 依赖图过重，测试目标是批量门槛 hook 行为——按行为最小化复刻。
jest.mock('@/components/page-schema/PageSchemaRenderer', () => ({
  renderSchemaActions: () => null,
}));

jest.mock('@/components/page-schema/icons', () => ({
  resolveSchemaIcon: () => null,
}));

const mockListFloors = jest.mocked(listFunctionVersionFloors);
const mockListHistory = jest.mocked(listFunctionVersionHistory);
const mockBatchSet = jest.mocked(batchSetFunctionVersionFloor);
const mockPutFloor = jest.mocked(putFunctionVersionFloor);
const mockDeleteFloor = jest.mocked(deleteFunctionVersionFloor);
const mockSummary = jest.mocked(getFunctionSummary);

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
    mockSummary.mockResolvedValue([] as never);
    mockListFloors.mockResolvedValue({});
    mockListHistory.mockResolvedValue({});
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
  const renderCell = (record: { id?: string; minVersion?: string }) => {
    const cols = buildDirectoryColumns({
      intl: getIntl(),
      columns: DIRECTORY_PAGE_SCHEMA.columns,
      rowActions: DIRECTORY_PAGE_SCHEMA.rowActions,
      versions: [],
      versionIndex: {},
      onFloorChange: () => {},
      onOpenDetail: () => {},
      onOpenSchema: () => {},
      onInvoke: () => {},
      onOpenAssignments: () => {},
    });
    const col = cols.find((c) => c.dataIndex === 'minVersion');
    return render(
      <AntdApp>
        {col?.render?.(undefined, { id: 'a.fn', ...record } as never) as React.ReactNode}
      </AntdApp>,
    );
  };

  it('有门槛：行内下拉选中项显示 ≥ v 版本值（#26 起为下拉非 Tag）', () => {
    renderCell({ minVersion: '0.3.0' });
    expect(screen.getByTitle('≥ v0.3.0')).toBeInTheDocument();
  });

  it('未配置：下拉占位「未设置」（清空 = 门槛未设置）', () => {
    renderCell({});
    expect(screen.getByText('未设置')).toBeInTheDocument();
  });
});

describe('行内修改版本门槛（#26：changeSingleFloor 经列下拉接线）', () => {
  function FloorHarness() {
    const page = useDirectoryPage();
    const col = page.columns.find((c) => c.dataIndex === 'minVersion');
    const row = page.processedData.find((r) => r.id === 'player.ban');
    return (
      <div data-testid="floor-cell">
        {row && col ? (col.render?.(undefined, row as never) as React.ReactNode) : null}
      </div>
    );
  }

  beforeEach(() => {
    jest.clearAllMocks();
    mockSummary.mockResolvedValue([
      { id: 'player.ban', version: '1.0.0', enabled: true, tags: [] },
    ] as never);
    mockListFloors.mockResolvedValue({ 'player.ban': '0.2.0' });
    mockListHistory.mockResolvedValue({ 'player.ban': ['0.2.0', '0.3.0'] });
  });

  const renderHarness = async () => {
    render(
      <AntdApp>
        <FloorHarness />
      </AntdApp>,
    );
    await waitFor(() => {
      expect(screen.getByTitle('≥ v0.2.0')).toBeInTheDocument();
    });
  };

  it('下拉选历史版本 → PUT 新门槛并提示成功', async () => {
    await renderHarness();
    mockPutFloor.mockResolvedValue({ functionId: 'player.ban', minVersion: '0.3.0' } as never);

    fireEvent.mouseDown(screen.getByRole('combobox'));
    await waitFor(() => {
      expect(screen.getByTitle('≥ v0.3.0')).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTitle('≥ v0.3.0'));

    await waitFor(() => {
      expect(mockPutFloor).toHaveBeenCalledWith('player.ban', '0.3.0');
    });
    expect(await screen.findByText('版本门槛已更新')).toBeInTheDocument();
  });

  it('清空下拉 → DELETE 门槛并提示已清除', async () => {
    await renderHarness();

    // antd 6 Select 清除钮 = <button class="ant-select-clear">（click 触发）
    const clear = screen.getByTestId('floor-cell').querySelector('button.ant-select-clear');
    expect(clear).not.toBeNull();
    fireEvent.click(clear as Element);

    await waitFor(() => {
      expect(mockDeleteFloor).toHaveBeenCalledWith('player.ban');
    });
    expect(await screen.findByText('版本门槛已清除')).toBeInTheDocument();
  });

  it('PUT 失败 → error 提示且单元格保持原值（不静默丢弃）', async () => {
    await renderHarness();
    mockPutFloor.mockRejectedValue(new Error('forbidden'));

    fireEvent.mouseDown(screen.getByRole('combobox'));
    await waitFor(() => {
      expect(screen.getByTitle('≥ v0.3.0')).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTitle('≥ v0.3.0'));

    expect(await screen.findByText('forbidden')).toBeInTheDocument();
    // 选中项容器（非下拉里同名 option）回滚为原值
    await waitFor(() => {
      const content = screen
        .getByTestId('floor-cell')
        .querySelector('.ant-select-content-has-value');
      expect(content).toHaveAttribute('title', '≥ v0.2.0');
    });
  });
});
