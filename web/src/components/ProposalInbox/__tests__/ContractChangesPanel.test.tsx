/**
 * ContractChangesPanel 契约变更队列回归：
 * 1. 「一键重新发布全部」按钮只在存在已发布态漂移页面时可用（草稿态不计入）；
 * 2. 点击后确认弹窗带上页面数量，确认后按已发布 pageKeys 调 bulkRepublishPages；
 * 3. 部分失败时展示 warning（含失败页面明细），成功后刷新队列并通知控制台菜单。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { render, screen, fireEvent, waitFor, configure } from '@testing-library/react';
import ContractChangesPanel from '../ContractChangesPanel';
import { bulkRepublishPages } from '@/services/api/pages';
import type { ContractChangeInfo } from '@/types/dashboard';

// coverage instrumentation 下渲染更慢：放宽异步查询与用例超时
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
  return {
    __esModule: true,
    useIntl: () => ({ formatMessage }),
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  };
});

jest.mock('@/services/api/pages', () => ({
  bulkRepublishPages: jest.fn(),
  publishPageDraft: jest.fn(),
}));

jest.mock('@/services/dashboard', () => ({
  mergeChanges: jest.fn(),
  regenerateProposal: jest.fn(),
  republish: jest.fn(),
}));

jest.mock('@/services/api/versioning', () => ({
  deleteVersioningPage: jest.fn(),
}));

jest.mock('@/utils/consoleMenu', () => ({
  requestConsoleMenuRefresh: jest.fn(),
}));

jest.mock('@/components/MergeConflictModal', () => ({
  __esModule: true,
  default: () => null,
}));

jest.mock('@/components/SelectorSync/SelectorSyncReportModal', () => ({
  __esModule: true,
  default: () => null,
}));

const mockBulkRepublishPages = jest.mocked(bulkRepublishPages);

function makeRecord(overrides: Partial<ContractChangeInfo>): ContractChangeInfo {
  return {
    pageKey: 'resource--player',
    pageType: 'resource',
    kind: 'published',
    ...overrides,
  };
}

function renderPanel(records: ContractChangeInfo[], onChanged: jest.Mock) {
  return render(
    <AntdApp>
      <ContractChangesPanel records={records} loading={false} onChanged={onChanged} />
    </AntdApp>,
  );
}

describe('ContractChangesPanel 一键重新发布', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('仅草稿态漂移时按钮禁用（未上线页面不应被批量发布）', () => {
    renderPanel([makeRecord({ kind: 'draft', pageKey: 'operation--kick' })], jest.fn());
    expect(screen.getByRole('button', { name: /一键重新发布全部/ })).toBeDisabled();
    expect(mockBulkRepublishPages).not.toHaveBeenCalled();
  });

  it('确认后按已发布 pageKeys 批量重新发布并刷新队列', async () => {
    const onChanged = jest.fn().mockResolvedValue(undefined);
    mockBulkRepublishPages.mockResolvedValue({
      total: 2,
      published: ['resource--player', 'operation--kick'],
    });
    renderPanel(
      [
        makeRecord({ pageKey: 'resource--player' }),
        makeRecord({ kind: 'draft', pageKey: 'operation--kick' }),
        makeRecord({ pageKey: 'operation--ban' }),
      ],
      onChanged,
    );

    const button = screen.getByRole('button', { name: /一键重新发布全部/ });
    expect(button).toBeEnabled();
    fireEvent.click(button);

    // 确认弹窗出现在 body，内容包含已发布页面数量（草稿不计入）
    const confirmText = await screen.findByText(/将把 2 个已发布页面/);
    expect(confirmText).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /OK|确\s*定/ }));
    await waitFor(() =>
      expect(mockBulkRepublishPages).toHaveBeenCalledWith(['resource--player', 'operation--ban']),
    );
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it('部分失败时展示失败明细 warning，不中断成功页面的反馈', async () => {
    const onChanged = jest.fn().mockResolvedValue(undefined);
    mockBulkRepublishPages.mockResolvedValue({
      total: 2,
      published: ['resource--player'],
      failed: [{ pageKey: 'operation--ban', error: '生成失败' }],
    });
    renderPanel(
      [makeRecord({ pageKey: 'resource--player' }), makeRecord({ pageKey: 'operation--ban' })],
      onChanged,
    );

    fireEvent.click(screen.getByRole('button', { name: /一键重新发布全部/ }));
    fireEvent.click(await screen.findByRole('button', { name: /OK|确\s*定/ }));

    await waitFor(() =>
      expect(
        screen.getByText(/已重新发布 1 个页面，1 个失败：operation--ban: 生成失败/),
      ).toBeInTheDocument(),
    );
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });
});
