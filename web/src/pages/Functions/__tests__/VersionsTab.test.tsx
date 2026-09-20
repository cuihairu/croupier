/**
 * B2：VersionsTab（契约变更历史）组件行为——列表渲染、快照 Drawer、
 * 两版对比（含 breaking 告警）、接口失败静默空表。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor, configure } from '@testing-library/react';
import { VersionsTab } from '../DetailTabs';
import { App as AntdApp } from 'antd';
import {
  deleteFunctionVersionFloor,
  diffContractVersions,
  getContractVersion,
  getFunctionVersionFloor,
  listContractVersions,
  putFunctionVersionFloor,
} from '@/services/api/functions';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

jest.mock('@/services/api/functions', () => ({
  deleteFunctionVersionFloor: jest.fn(),
  diffContractVersions: jest.fn(),
  getContractVersion: jest.fn(),
  getFunctionAnalytics: jest.fn(),
  getFunctionVersionFloor: jest.fn(),
  listContractVersions: jest.fn(),
  listFunctionWarnings: jest.fn(),
  putFunctionVersionFloor: jest.fn(),
}));

const mockList = jest.mocked(listContractVersions);
const mockGet = jest.mocked(getContractVersion);
const mockDiff = jest.mocked(diffContractVersions);
const mockGetFloor = jest.mocked(getFunctionVersionFloor);
const mockPutFloor = jest.mocked(putFunctionVersionFloor);
const mockDelFloor = jest.mocked(deleteFunctionVersionFloor);

const rows = [
  {
    seq: 3,
    version: '1.2.0',
    source: 'sdk',
    changeType: 'removed' as const,
    breaking: false,
    actor: 'admin',
    createdAt: '2026-09-19T08:00:00Z',
    diff: [{ field: 'enabled', from: 'true', to: 'false' }],
  },
  {
    seq: 2,
    version: '1.1.0',
    source: 'openapi',
    changeType: 'updated' as const,
    breaking: true,
    actor: 'system',
    createdAt: '2026-09-18T08:00:00Z',
    diff: [
      { field: 'risk', from: 'safe', to: 'danger' },
      {
        field: 'inputSchema',
        change: 'schema_replaced',
        findings: [
          {
            severity: 'breaking',
            source: 'inputSchema',
            path: '/reason',
            reason: '已声明字段被删除',
          },
        ],
      },
    ],
  },
  {
    seq: 1,
    version: '1.0.0',
    source: 'sdk',
    changeType: 'created' as const,
    breaking: false,
    createdAt: '2026-09-17T08:00:00Z',
  },
];

beforeEach(() => {
  jest.clearAllMocks();
  mockList.mockResolvedValue({ items: rows, total: 3, page: 1, size: 10 });
  mockGetFloor.mockResolvedValue({ functionId: 'player.ban', minVersion: '' });
  mockPutFloor.mockImplementation(async (functionId, minVersion) => ({ functionId, minVersion }));
  mockDelFloor.mockResolvedValue(undefined);
});

const renderTab = () =>
  render(
    <AntdApp>
      <VersionsTab functionId="player.ban" />
    </AntdApp>,
  );

describe('VersionsTab', () => {
  it('按 functionId 拉取历史：类型/兼容性/变更字段列渲染', async () => {
    render(<VersionsTab functionId="player.ban" />);
    await waitFor(() =>
      expect(mockList).toHaveBeenCalledWith('player.ban', { page: 1, pageSize: 10 }),
    );
    expect(await screen.findByText('破坏性')).toBeInTheDocument();
    expect(screen.getAllByText('兼容')).toHaveLength(2);
    expect(screen.getByText('新建')).toBeInTheDocument();
    expect(screen.getByText('删除')).toBeInTheDocument();
    expect(screen.getByText('risk、inputSchema')).toBeInTheDocument();
    expect(screen.getAllByText('system').length).toBe(1);
  });

  it('接口失败 → 空表不崩', async () => {
    mockList.mockRejectedValue(new Error('boom'));
    render(<VersionsTab functionId="player.ban" />);
    await waitFor(() => expect(mockList).toHaveBeenCalled());
    expect(await screen.findByText('两版对比：')).toBeInTheDocument();
  });

  it('点快照 → 拉取对应版本并以 JSON 展示', async () => {
    mockGet.mockResolvedValue({
      ...rows[0],
      snapshot: { id: 'player.ban', version: '1.2.0', risk: 'danger' },
    });
    render(<VersionsTab functionId="player.ban" />);
    fireEvent.click((await screen.findAllByText('快照'))[0]);
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('player.ban', 3));
    expect(await screen.findByText(/"version": "1.2.0"/)).toBeInTheDocument();
  });

  it('选择两版 → 对比：breaking 告警 + findings 明细', async () => {
    mockDiff.mockResolvedValue({
      fromSeq: 1,
      toSeq: 2,
      breaking: true,
      changes: [
        { field: 'risk', from: 'safe', to: 'danger' },
        {
          field: 'inputSchema',
          change: 'schema_replaced',
          findings: [
            {
              severity: 'breaking',
              source: 'inputSchema',
              path: '/reason',
              reason: '已声明字段被删除',
            },
          ],
        },
      ],
    });
    render(<VersionsTab functionId="player.ban" />);
    await screen.findByText('破坏性');

    const selects = screen.getAllByRole('combobox');
    fireEvent.mouseDown(selects[0]);
    let options = await screen.findAllByText('#1');
    fireEvent.click(options[options.length - 1]);
    fireEvent.mouseDown(selects[1]);
    options = await screen.findAllByText('#2');
    fireEvent.click(options[options.length - 1]);
    fireEvent.click(screen.getByRole('button', { name: /对比|比 对/ }));

    await waitFor(() => expect(mockDiff).toHaveBeenCalledWith('player.ban', 1, 2));
    expect(await screen.findByText(/存在破坏性 schema 变更/)).toBeInTheDocument();
    // findings 单元格内 path 与 reason 是兄弟文本节点，用子串匹配
    expect(screen.getByText(/已声明字段被删除/)).toBeInTheDocument();
    expect(screen.getByText('breaking')).toBeInTheDocument();
  });

  it('未选两版时对比按钮禁用', async () => {
    render(<VersionsTab functionId="player.ban" />);
    const button = await screen.findByRole('button', { name: /对比|比 对/ });
    expect(button).toBeDisabled();
  });
});

describe('VersionsTab 版本门槛设置', () => {
  it('已配置门槛：回显当前值与「当前门槛」标签', async () => {
    mockGetFloor.mockResolvedValue({ functionId: 'player.ban', minVersion: '0.3.0' });
    renderTab();
    await waitFor(() => expect(mockGetFloor).toHaveBeenCalledWith('player.ban'));
    expect(await screen.findByText('当前门槛', { exact: false })).toBeInTheDocument();
    expect(screen.getByDisplayValue('0.3.0')).toBeInTheDocument();
  });

  it('输入新版本并保存 → PUT 并更新回显', async () => {
    renderTab();
    await waitFor(() => expect(mockGetFloor).toHaveBeenCalled());
    fireEvent.change(screen.getByPlaceholderText('0.3.0'), { target: { value: '0.4.0' } });
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    await waitFor(() => expect(mockPutFloor).toHaveBeenCalledWith('player.ban', '0.4.0'));
    expect(await screen.findByText('当前门槛', { exact: false })).toBeInTheDocument();
    expect(screen.getByDisplayValue('0.4.0')).toBeInTheDocument();
  });

  it('清除门槛 → DELETE 并回到未设置', async () => {
    mockGetFloor.mockResolvedValue({ functionId: 'player.ban', minVersion: '0.3.0' });
    renderTab();
    expect(await screen.findByText('当前门槛', { exact: false })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /清\s*除/ }));
    await waitFor(() => expect(mockDelFloor).toHaveBeenCalledWith('player.ban'));
    expect(await screen.findByText('未设置')).toBeInTheDocument();
  });
});
