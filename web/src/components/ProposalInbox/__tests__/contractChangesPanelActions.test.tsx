/**
 * ContractChangesPanel 行级操作链路（一键重新发布之外）：
 * 重发布（有 draftRevision 走 publishPageDraft / 无则 republish）、
 * 重生成 Proposal（成功 modal / 失败显式 error 反馈）、
 * 删除页面（confirm → deleteVersioningPage）、
 * 自动合并（结果计数 modal）、处理冲突（预览失败提示 / dryRun 打开冲突
 * 弹窗并经 onSubmit 提交 manual 合并 / 提交失败提示）。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { render, screen, fireEvent, waitFor, within, configure } from '@testing-library/react';
import ContractChangesPanel from '../ContractChangesPanel';
import { bulkRepublishPages, publishPageDraft } from '@/services/api/pages';
import { mergeChanges, regenerateProposal, republish } from '@/services/dashboard';
import { deleteVersioningPage } from '@/services/api/versioning';
import type { MergeResponse } from '@/services/api/versioning';
import type { ContractChangeInfo } from '@/types/dashboard';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

// MergeConflictModal 替身：捕获最近一次 props 供断言/触发 onSubmit
let mergeModalProps: {
  open: boolean;
  loading: boolean;
  preview: MergeResponse | null;
  onCancel: () => void;
  onSubmit: (payload: { conflicts: unknown[]; reason?: string }) => Promise<void>;
} | null = null;

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
    useIntl: () => ({ formatMessage, locale: 'zh-CN' }),
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
  default: (props: {
    open: boolean;
    loading: boolean;
    preview: MergeResponse | null;
    onCancel: () => void;
    onSubmit: (payload: { conflicts: unknown[]; reason?: string }) => Promise<void>;
  }) => {
    mergeModalProps = props;
    return props.open ? <div data-testid="merge-conflict-modal" /> : null;
  },
}));

jest.mock('@/components/SelectorSync/SelectorSyncReportModal', () => ({
  __esModule: true,
  default: () => null,
}));

const mockPublishPageDraft = jest.mocked(publishPageDraft);
const mockMergeChanges = jest.mocked(mergeChanges);
const mockRegenerate = jest.mocked(regenerateProposal);
const mockRepublish = jest.mocked(republish);
const mockDeletePage = jest.mocked(deleteVersioningPage);
void bulkRepublishPages;

function makeRecord(overrides: Partial<ContractChangeInfo>): ContractChangeInfo {
  return {
    pageKey: 'resource--player',
    pageType: 'resource',
    kind: 'published',
    ...overrides,
  } as ContractChangeInfo;
}

function renderPanel(records: ContractChangeInfo[]) {
  const onChanged = jest.fn().mockResolvedValue(undefined);
  render(
    <AntdApp>
      <ContractChangesPanel records={records} loading={false} onChanged={onChanged} />
    </AntdApp>,
  );
  return { onChanged };
}

/** 打开行内「更多」下拉并点击菜单项（「更多」= 行内最后一个 icon-only 按钮） */
async function clickMenuItem(label: RegExp) {
  const row = document.querySelector('tr.ant-table-row');
  expect(row).not.toBeNull();
  const rowButtons = within(row as HTMLElement).getAllByRole('button');
  fireEvent.click(rowButtons[rowButtons.length - 1]);
  const item = await screen.findByRole('menuitem', { name: label });
  fireEvent.click(item);
}

beforeEach(() => {
  jest.clearAllMocks();
  mergeModalProps = null;
});

describe('重发布', () => {
  it('draftRevision>0 走草稿快照发布', async () => {
    const { onChanged } = renderPanel([
      makeRecord({ draftRevision: 7, title: { 'zh-CN': '玩家' } }),
    ]);
    mockPublishPageDraft.mockResolvedValue({
      pageKey: 'resource--player',
      publishedVersion: 3,
    } as unknown as ReturnType<typeof publishPageDraft>);

    fireEvent.click(screen.getByRole('button', { name: /重\s*发\s*布/ }));
    fireEvent.click(await screen.findByRole('button', { name: /OK|确\s*定/ }));

    await waitFor(() => expect(mockPublishPageDraft).toHaveBeenCalledWith('resource--player', 7));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it('无 draftRevision 走 republish 全量重生成', async () => {
    const { onChanged } = renderPanel([makeRecord({ draftRevision: undefined })]);
    mockRepublish.mockResolvedValue({ message: 'ok' } as unknown as MergeResponse);

    fireEvent.click(screen.getByRole('button', { name: /重\s*发\s*布/ }));
    fireEvent.click(await screen.findByRole('button', { name: /OK|确\s*定/ }));

    await waitFor(() => expect(mockRepublish).toHaveBeenCalledWith('resource--player'));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });
});

describe('重生成 Proposal', () => {
  it('成功展示结果 modal 并刷新', async () => {
    const { onChanged } = renderPanel([makeRecord({})]);
    mockRegenerate.mockResolvedValue({ message: '已生成' } as unknown as MergeResponse);

    await clickMenuItem(/重生成/);

    await waitFor(() => expect(mockRegenerate).toHaveBeenCalledWith('resource--player'));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it('失败显式反馈错误诊断（不静默成功）', async () => {
    renderPanel([makeRecord({})]);
    mockRegenerate.mockRejectedValue(new Error('schema 非法'));

    await clickMenuItem(/重生成/);

    await waitFor(() => expect(screen.getAllByText('重新生成失败').length).toBeGreaterThan(0));
    expect(await screen.findAllByText('schema 非法')).not.toHaveLength(0);
  });
});

describe('删除页面', () => {
  it('confirm 后删除并提示', async () => {
    const { onChanged } = renderPanel([makeRecord({})]);
    mockDeletePage.mockResolvedValue(undefined);

    await clickMenuItem(/删除页面/);
    fireEvent.click(await screen.findByRole('button', { name: /删\s*除|OK/ }));

    await waitFor(() => expect(mockDeletePage).toHaveBeenCalledWith('resource--player'));
    await waitFor(() => expect(screen.getAllByText('页面已删除').length).toBeGreaterThan(0));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });
});

describe('自动合并', () => {
  it('展示合并结果计数', async () => {
    const { onChanged } = renderPanel([makeRecord({})]);
    mockMergeChanges.mockResolvedValue({
      message: '安全合并完成',
      merged: 2,
      conflicts: 1,
    } as unknown as MergeResponse);

    await clickMenuItem(/自动合并/);

    await waitFor(() =>
      expect(mockMergeChanges).toHaveBeenCalledWith('resource--player', { strategy: 'auto' }),
    );
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });
});

describe('处理冲突（手动合并）', () => {
  it('预览失败提示不打开弹窗', async () => {
    renderPanel([makeRecord({})]);
    mockMergeChanges.mockRejectedValue(new Error('boom'));

    await clickMenuItem(/处理冲突/);

    await waitFor(() => expect(screen.getByText('加载冲突预览失败')).toBeInTheDocument());
    expect(mergeModalProps?.open ?? false).toBe(false);
  });

  it('dryRun 预览成功打开冲突弹窗，提交走 manual 合并', async () => {
    const { onChanged } = renderPanel([makeRecord({})]);
    mockMergeChanges
      .mockResolvedValueOnce({ message: 'preview', conflicts: 2 } as unknown as MergeResponse)
      .mockResolvedValueOnce({
        message: 'done',
        draftRevision: 9,
      } as unknown as MergeResponse);

    await clickMenuItem(/处理冲突/);
    await waitFor(() =>
      expect(mockMergeChanges).toHaveBeenCalledWith('resource--player', {
        strategy: 'manual',
        dryRun: true,
      }),
    );
    await waitFor(() => expect(screen.getByTestId('merge-conflict-modal')).toBeInTheDocument());

    await mergeModalProps!.onSubmit({ conflicts: [], reason: '手动' });
    await waitFor(() =>
      expect(mockMergeChanges).toHaveBeenLastCalledWith('resource--player', {
        strategy: 'manual',
        conflicts: [],
        reason: '手动',
      }),
    );
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
    // 提交成功后弹窗关闭并清理状态
    await waitFor(() => expect(mergeModalProps?.open ?? true).toBe(false));
  });

  it('提交失败提示手动合并失败', async () => {
    renderPanel([makeRecord({})]);
    mockMergeChanges
      .mockResolvedValueOnce({ message: 'preview', conflicts: 1 } as unknown as MergeResponse)
      .mockRejectedValueOnce(new Error('conflict'));

    await clickMenuItem(/处理冲突/);
    await waitFor(() => expect(screen.getByTestId('merge-conflict-modal')).toBeInTheDocument());

    await mergeModalProps!.onSubmit({ conflicts: [] });
    await waitFor(() => expect(screen.getByText('手动合并失败')).toBeInTheDocument());
  });
});
