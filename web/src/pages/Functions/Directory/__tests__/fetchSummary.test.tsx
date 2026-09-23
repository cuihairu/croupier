/**
 * 函数目录数据管道（hook 级）：descriptors 三形态契约（裸数组/{functions}/{descriptors}/
 * 异常降级）、summary 为唯一真值源 + descriptor 仅增强、tags 空数组时才允许 descriptor
 * 兜底、floors 拉取失败降级空表、整页加载失败 message.error、详情抽屉 instances 拉取
 * 失败兜底 0。批量门槛交互见 batchFloor.test.tsx，抽屉动作路由见 drawerActions.test.tsx。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import useDirectoryPage from '../useDirectoryPage';
import { listDescriptors, listFunctionInstances } from '@/services/api';
import { getFunctionSummary } from '@/services/api/functions-enhanced';
import { listFunctionVersionFloors } from '@/services/api/functions';

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
  listDescriptors: jest.fn(),
  listFunctionInstances: jest.fn(),
}));

jest.mock('@/services/api/functions-enhanced', () => ({
  getFunctionSummary: jest.fn(),
}));

jest.mock('@/services/api/functions', () => ({
  listFunctionVersionFloors: jest.fn(),
  batchSetFunctionVersionFloor: jest.fn(),
}));

// PageSchemaRenderer 依赖图过重，测试目标是数据管道 hook 行为——按行为最小化复刻。
jest.mock('@/components/page-schema/PageSchemaRenderer', () => ({
  renderSchemaActions: () => null,
}));

jest.mock('@/components/page-schema/icons', () => ({
  resolveSchemaIcon: () => null,
}));

const mockListDescriptors = jest.mocked(listDescriptors);
const mockGetSummary = jest.mocked(getFunctionSummary);
const mockListFloors = jest.mocked(listFunctionVersionFloors);
const mockInstances = jest.mocked(listFunctionInstances);

const descriptor = {
  id: 'player.list',
  version: 'v9.9.9',
  displayName: '描述侧名称',
  summary: '描述侧摘要',
  resource: 'player',
  operation: 'list',
  tags: ['tag-from-descriptor'],
};

const summaryA = {
  id: 'player.list',
  version: '',
  enabled: true,
  displayName: '',
  summary: '',
  resource: '',
  operation: '',
  tags: [],
};

function Harness() {
  const page = useDirectoryPage();
  return (
    <div>
      <div data-testid="rows">{JSON.stringify(page.processedData)}</div>
      <div data-testid="detail">{JSON.stringify(page.selectedFunction)}</div>
      <button
        type="button"
        data-testid="open-detail"
        onClick={() =>
          page.handleViewDetail({ id: 'player.list', enabled: true, tags: [] } as never)
        }
      >
        open
      </button>
    </div>
  );
}

describe('函数目录数据管道', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockGetSummary.mockResolvedValue([summaryA] as never);
    mockListFloors.mockResolvedValue({});
    mockListDescriptors.mockResolvedValue({ functions: [descriptor] } as never);
  });

  const renderAndLoad = async () => {
    render(
      <AntdApp>
        <Harness />
      </AntdApp>,
    );
    await waitFor(() => {
      expect(JSON.parse(screen.getByTestId('rows').textContent || '[]').length).toBe(1);
    });
    return JSON.parse(screen.getByTestId('rows').textContent || '[]') as Array<
      Record<string, unknown>
    >;
  };

  it('summary 空字段由 descriptor 增强；item.tags 空数组才允许兜底；floors 落到 minVersion', async () => {
    mockListFloors.mockResolvedValue({ 'player.list': '0.3.0' });
    const rows = await renderAndLoad();
    expect(rows[0]).toMatchObject({
      id: 'player.list',
      version: 'v9.9.9',
      displayName: '描述侧名称',
      summary: '描述侧摘要',
      resource: 'player',
      operation: 'list',
      tags: ['tag-from-descriptor'],
      minVersion: '0.3.0',
    });
  });

  it('item 自身值优先于 descriptor 增强', async () => {
    mockGetSummary.mockResolvedValue([
      {
        ...summaryA,
        version: 'v1.0.0',
        displayName: '自身名称',
        tags: ['own-tag'],
      },
    ] as never);
    const rows = await renderAndLoad();
    expect(rows[0]).toMatchObject({
      version: 'v1.0.0',
      displayName: '自身名称',
      tags: ['own-tag'],
    });
  });

  it('descriptors 兼容裸数组形态', async () => {
    mockListDescriptors.mockResolvedValue([descriptor] as never);
    const rows = await renderAndLoad();
    expect(rows[0].displayName).toBe('描述侧名称');
  });

  it('descriptors 兼容 {descriptors:[...]} 信封形态', async () => {
    mockListDescriptors.mockResolvedValue({ descriptors: [descriptor] } as never);
    const rows = await renderAndLoad();
    expect(rows[0].displayName).toBe('描述侧名称');
  });

  it('descriptors 无键信封安全忽略', async () => {
    mockListDescriptors.mockResolvedValue({ unrelated: true } as never);
    const rows = await renderAndLoad();
    expect(rows[0].displayName).toBeUndefined();
  });

  it('descriptors 空 id 条目跳过、有效条目正常建索引', async () => {
    mockListDescriptors.mockResolvedValue({
      functions: [{ ...descriptor, id: '' }, descriptor],
    } as never);
    const rows = await renderAndLoad();
    expect(rows[0].displayName).toBe('描述侧名称');
  });

  it('descriptors 拉取失败仅失去增强，summary 主数据照常', async () => {
    mockListDescriptors.mockRejectedValue(new Error('desc down'));
    const rows = await renderAndLoad();
    expect(rows[0].id).toBe('player.list');
    expect(rows[0].displayName).toBeUndefined();
  });

  it('floors 拉取失败降级空表，minVersion 为空不阻塞列表', async () => {
    mockListFloors.mockRejectedValue(new Error('floors down'));
    const rows = await renderAndLoad();
    expect(rows[0].minVersion).toBeUndefined();
  });

  it('summary 主数据失败：message.error 展示错误信息', async () => {
    mockGetSummary.mockRejectedValue(new Error('summary exploded'));
    render(
      <AntdApp>
        <Harness />
      </AntdApp>,
    );
    expect(await screen.findByText('summary exploded')).toBeInTheDocument();
  });

  it('详情抽屉 instances 拉取失败兜底 0 且抽屉照常打开', async () => {
    mockInstances.mockRejectedValue(new Error('instances down'));
    render(
      <AntdApp>
        <Harness />
      </AntdApp>,
    );
    fireEvent.click(screen.getByTestId('open-detail'));
    await waitFor(() => {
      const detail = JSON.parse(screen.getByTestId('detail').textContent || 'null');
      expect(detail).toMatchObject({ id: 'player.list', instances: 0 });
    });
  });
});
