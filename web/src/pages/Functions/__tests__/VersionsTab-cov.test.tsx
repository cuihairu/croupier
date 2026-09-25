/**
 * VersionsTab 残余分支（契约版本历史）：
 * 1. 门槛拉取失败 → 静默回退「未设置」；
 * 2. 保存/清除门槛失败的 Error 与非 Error 两种文案形态；
 * 3. 快照拉取失败 → 抽屉不开、不崩；
 * 4. 两版对比失败 → 不出告警；
 * 5. 非 breaking 对比 → 「变更兼容」info 告警；
 * 6. 快照含循环引用 → prettyJSON 的 stringify 异常兜底；
 * 7. 分页翻页 → setPage/setPageSize 接线。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { VersionsTab } from '../DetailTabs';
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
  },
  {
    seq: 2,
    version: '1.1.0',
    source: 'openapi',
    changeType: 'updated' as const,
    breaking: true,
    actor: 'system',
    createdAt: '2026-09-18T08:00:00Z',
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
  mockGetFloor.mockResolvedValue({ functionId: 'player.ban', minVersion: '0.3.0' });
  mockPutFloor.mockImplementation(async (functionId, minVersion) => ({ functionId, minVersion }));
  mockDelFloor.mockResolvedValue(undefined);
});

const renderTab = () =>
  render(
    <AntdApp>
      <VersionsTab functionId="player.ban" />
    </AntdApp>,
  );

describe('VersionsTab 残余分支', () => {
  it('门槛拉取失败 → 静默回退未设置', async () => {
    mockGetFloor.mockRejectedValueOnce(new Error('floor down'));
    renderTab();
    expect(await screen.findByText('未设置')).toBeInTheDocument();
    const input = screen.getByPlaceholderText('0.3.0') as HTMLInputElement;
    expect(input.value).toBe('');
  });

  it('保存门槛失败（Error）→ 透出错误文案', async () => {
    mockPutFloor.mockRejectedValueOnce(new Error('floor boom'));
    renderTab();
    fireEvent.change(await screen.findByPlaceholderText('0.3.0'), {
      target: { value: '0.4.0' },
    });
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    expect(await screen.findByText('floor boom')).toBeInTheDocument();
    // finally 分支：保存中状态复位，按钮恢复可用
    await waitFor(() => expect(screen.getByRole('button', { name: /保\s*存/ })).toBeEnabled());
  });

  it('保存门槛失败（非 Error）→ String(err) 文案', async () => {
    (
      mockPutFloor as unknown as { mockRejectedValueOnce: (v: unknown) => void }
    ).mockRejectedValueOnce('plain-floor-error');
    renderTab();
    fireEvent.change(await screen.findByPlaceholderText('0.3.0'), {
      target: { value: '0.4.0' },
    });
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    expect(await screen.findByText('plain-floor-error')).toBeInTheDocument();
  });

  it('清除门槛失败 → 透出错误文案且保持当前门槛', async () => {
    mockDelFloor.mockRejectedValueOnce(new Error('clear boom'));
    renderTab();
    expect(await screen.findByText('当前门槛', { exact: false })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /清\s*除/ }));
    expect(await screen.findByText('clear boom')).toBeInTheDocument();
    expect(screen.getByDisplayValue('0.3.0')).toBeInTheDocument();
  });

  it('快照拉取失败 → 抽屉不开、列表不崩', async () => {
    mockGet.mockRejectedValueOnce(new Error('snapshot boom'));
    renderTab();
    fireEvent.click((await screen.findAllByText('快照'))[0]);
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('player.ban', 3));
    await new Promise((resolve) => setTimeout(resolve, 40));
    expect(screen.queryByText(/"version"/)).not.toBeInTheDocument();
    expect(await screen.findByText('1.2.0')).toBeInTheDocument();
  });

  it('快照含循环引用 → prettyJSON 走 stringify 异常兜底', async () => {
    const circular: Record<string, unknown> = { id: 'player.ban', version: '1.2.0' };
    circular.self = circular;
    mockGet.mockResolvedValueOnce({
      ...rows[0],
      snapshot: circular,
    } as Awaited<ReturnType<typeof getContractVersion>>);
    renderTab();
    fireEvent.click((await screen.findAllByText('快照'))[0]);
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('player.ban', 3));
    // stringify 抛错 → String(value) 兜底，抽屉仍能展示
    expect(await screen.findByText('[object Object]')).toBeInTheDocument();
  });

  it('两版对比失败 → 不出告警', async () => {
    mockDiff.mockRejectedValueOnce(new Error('diff boom'));
    renderTab();
    await screen.findByText('两版对比：');

    const selects = screen.getAllByRole('combobox');
    fireEvent.mouseDown(selects[0]);
    let options = await screen.findAllByText('#1');
    fireEvent.click(options[options.length - 1]);
    fireEvent.mouseDown(selects[1]);
    options = await screen.findAllByText('#2');
    fireEvent.click(options[options.length - 1]);
    fireEvent.click(screen.getByRole('button', { name: /对比|比 对/ }));

    await waitFor(() => expect(mockDiff).toHaveBeenCalledWith('player.ban', 1, 2));
    await new Promise((resolve) => setTimeout(resolve, 40));
    expect(screen.queryByText(/存在破坏性 schema 变更/)).not.toBeInTheDocument();
    expect(screen.queryByText(/变更兼容/)).not.toBeInTheDocument();
  });

  it('两版对比兼容 → info 告警「变更兼容」', async () => {
    mockDiff.mockResolvedValueOnce({
      fromSeq: 1,
      toSeq: 3,
      breaking: false,
      changes: [{ field: 'summary', from: 'a', to: 'b' }],
    });
    renderTab();
    await screen.findByText('两版对比：');

    const selects = screen.getAllByRole('combobox');
    fireEvent.mouseDown(selects[0]);
    let options = await screen.findAllByText('#1');
    fireEvent.click(options[options.length - 1]);
    fireEvent.mouseDown(selects[1]);
    options = await screen.findAllByText('#3');
    fireEvent.click(options[options.length - 1]);
    fireEvent.click(screen.getByRole('button', { name: /对比|比 对/ }));

    await waitFor(() => expect(mockDiff).toHaveBeenCalledWith('player.ban', 1, 3));
    expect(await screen.findByText(/变更兼容/)).toBeInTheDocument();
    expect(screen.queryByText(/存在破坏性/)).not.toBeInTheDocument();
    expect(screen.getByText('summary')).toBeInTheDocument();
  });

  it('total > pageSize → 翻页触发 setPage/setPageSize', async () => {
    const many = Array.from({ length: 10 }, (_, i) => ({
      ...rows[0],
      seq: 10 - i,
      version: `2.0.${10 - i}`,
    }));
    mockList.mockResolvedValue({ items: many, total: 11, page: 1, size: 10 });
    renderTab();
    await screen.findByText('2.0.10');

    fireEvent.click(screen.getByTitle('2'));
    await waitFor(() =>
      expect(mockList).toHaveBeenLastCalledWith('player.ban', { page: 2, pageSize: 10 }),
    );
  });

  it('门槛输入全空白 → 保存早退不发请求', async () => {
    renderTab();
    const input = await screen.findByPlaceholderText('0.3.0');
    fireEvent.change(input, { target: { value: '   ' } });
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    expect(mockPutFloor).not.toHaveBeenCalled();
  });

  it('清除门槛失败（非 Error）→ String(err) 兜底文案', async () => {
    (
      mockDelFloor as unknown as { mockRejectedValueOnce: (v: unknown) => void }
    ).mockRejectedValueOnce('plain-clear-error');
    renderTab();
    fireEvent.click(await screen.findByRole('button', { name: /清\s*除/ }));
    expect(await screen.findByText('plain-clear-error')).toBeInTheDocument();
  });

  it('门槛拉取在卸载之后才失败 → cancelled 早退不再 setState', async () => {
    let rejectFloor: (reason: unknown) => void = () => undefined;
    mockGetFloor.mockImplementation(
      () =>
        new Promise<never>((_resolve, reject) => {
          rejectFloor = reject;
        }),
    );
    const { unmount } = renderTab();
    expect(mockGetFloor).toHaveBeenCalledWith('player.ban');
    unmount();
    rejectFloor(new Error('late floor failure'));
    await new Promise((resolve) => setTimeout(resolve, 20));
    // 卸载后 cancelled=true → catch 内 return，不再写 state（否则 React 警告）
    expect(mockGetFloor).toHaveBeenCalledTimes(1);
  });

  it('未知变更类型原样透出；空 version/source 列兜底 -', async () => {
    mockList.mockResolvedValue({
      items: [{ ...rows[0], seq: 9, version: '', source: '', changeType: 'renamed' as const }],
      total: 1,
      page: 1,
      size: 10,
    });
    renderTab();

    expect(await screen.findByText('renamed')).toBeInTheDocument();
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(2);
  });

  it('对比结果 rowKey 三级兜底 + findings 双色 Tag', async () => {
    mockDiff.mockResolvedValueOnce({
      fromSeq: 1,
      toSeq: 3,
      breaking: true,
      changes: [
        {
          field: 'alpha',
          change: 'renamed',
          from: 'a',
          to: 'b',
          findings: [
            { severity: 'breaking', source: 'schema', path: '$.id', reason: '类型变更' },
            { severity: 'warning', source: 'schema', path: '$.name', reason: '可选转必填' },
          ],
        },
        { field: 'beta', from: 'x', to: 'y' },
        { field: 'gamma' },
      ],
    });
    renderTab();
    await screen.findByText('两版对比：');

    const selects = screen.getAllByRole('combobox');
    fireEvent.mouseDown(selects[0]);
    let options = await screen.findAllByText('#1');
    fireEvent.click(options[options.length - 1]);
    fireEvent.mouseDown(selects[1]);
    options = await screen.findAllByText('#3');
    fireEvent.click(options[options.length - 1]);
    fireEvent.click(screen.getByRole('button', { name: /对比|比 对/ }));

    expect(await screen.findByText('alpha')).toBeInTheDocument();
    expect(screen.getByText('beta')).toBeInTheDocument();
    expect(screen.getByText('gamma')).toBeInTheDocument();
    expect(screen.getByText(/类型变更/)).toBeInTheDocument();
    expect(screen.getByText(/可选转必填/)).toBeInTheDocument();
    expect(await screen.findByText(/存在破坏性/)).toBeInTheDocument();
  });

  it('快照抽屉经关闭按钮收起（onClose 接线）', async () => {
    mockGet.mockResolvedValueOnce({ ...rows[0], snapshot: { id: 'player.ban' } });
    renderTab();

    fireEvent.click((await screen.findAllByText('快照'))[0]);
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('player.ban', 3));
    await waitFor(() =>
      expect(document.querySelector('.ant-drawer.ant-drawer-open')).not.toBeNull(),
    );

    const closeButton = document.querySelector('.ant-drawer-close');
    expect(closeButton).toBeTruthy();
    fireEvent.click(closeButton as HTMLElement);
    await waitFor(() => expect(document.querySelector('.ant-drawer.ant-drawer-open')).toBeNull());
  });
});
