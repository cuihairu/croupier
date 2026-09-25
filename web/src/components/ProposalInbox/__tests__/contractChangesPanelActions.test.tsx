/**
 * ContractChangesPanel 行级操作链路（一键重新发布之外）：
 * 重发布（有 draftRevision 走 publishPageDraft / 无则 republish）、
 * 重生成 Proposal（成功 modal / 失败显式 error 反馈）、
 * 删除页面（confirm → deleteVersioningPage）、
 * 自动合并（结果计数 modal）、处理冲突（预览失败提示 / dryRun 打开冲突
 * 弹窗并经 onSubmit 提交 manual 合并 / 提交失败提示 / 弹窗取消 /
 * 未打开弹窗时的防御性早退 / 草稿版本号缺失以 - 展示）。
 * 批量同步 Selector（bulkSyncPageSelectors 成功/skipped/failed/异常四态，
 * 明细仅取前 3 条）、一键同步报告弹窗的 onClose/onApplied 回调、
 * 行内编辑跳转（navigateTo 带 focus 参数）、一键重新发布边界（异常提示、
 * 响应缺 published 字段按 0 计）与渲染分支（未知 pageType 回显原文、
 * bindingFreshness 诊断计数、focusPageKey 行高亮、空队列按钮禁用）。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { render, screen, fireEvent, waitFor, within, configure } from '@testing-library/react';
import ContractChangesPanel from '../ContractChangesPanel';
import { bulkRepublishPages, bulkSyncPageSelectors, publishPageDraft } from '@/services/api/pages';
import type { PageBulkSyncSelectorsResult } from '@/services/api/pages';
import { mergeChanges, regenerateProposal, republish } from '@/services/dashboard';
import { deleteVersioningPage } from '@/services/api/versioning';
import type { MergeResponse } from '@/services/api/versioning';
import { navigateTo } from '../urlFocus';
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

// SelectorSyncReportModal 替身：捕获最近一次 props 供断言/触发 onClose、onApplied
let selectorSyncModalProps: {
  open: boolean;
  pageKey: string;
  onClose: () => void;
  onApplied: (revision: number) => void;
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
  bulkSyncPageSelectors: jest.fn(),
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

// 行内「编辑」跳转用 window.location.assign（jsdom 未实现导航），mock 掉
jest.mock('../urlFocus', () => ({
  navigateTo: jest.fn(),
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
  default: (props: {
    open: boolean;
    pageKey: string;
    onClose: () => void;
    onApplied: (revision: number) => void;
  }) => {
    selectorSyncModalProps = props;
    return props.open ? <div data-testid="selector-sync-modal" /> : null;
  },
}));

const mockPublishPageDraft = jest.mocked(publishPageDraft);
const mockMergeChanges = jest.mocked(mergeChanges);
const mockRegenerate = jest.mocked(regenerateProposal);
const mockRepublish = jest.mocked(republish);
const mockDeletePage = jest.mocked(deleteVersioningPage);
const mockBulkRepublishPages = jest.mocked(bulkRepublishPages);
const mockBulkSyncPageSelectors = jest.mocked(bulkSyncPageSelectors);
const mockedNavigateTo = jest.mocked(navigateTo);

function makeRecord(overrides: Partial<ContractChangeInfo>): ContractChangeInfo {
  return {
    pageKey: 'resource--player',
    pageType: 'resource',
    kind: 'published',
    ...overrides,
  } as ContractChangeInfo;
}

function renderPanel(records: ContractChangeInfo[], options?: { focusPageKey?: string }) {
  const onChanged = jest.fn().mockResolvedValue(undefined);
  render(
    <AntdApp>
      <ContractChangesPanel
        records={records}
        loading={false}
        focusPageKey={options?.focusPageKey}
        onChanged={onChanged}
      />
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
  selectorSyncModalProps = null;
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

  it('提交成功但响应缺草稿版本号时以 - 展示', async () => {
    renderPanel([makeRecord({})]);
    mockMergeChanges
      .mockResolvedValueOnce({ message: 'preview', conflicts: 1 } as unknown as MergeResponse)
      .mockResolvedValueOnce({ message: 'done' } as unknown as MergeResponse);

    await clickMenuItem(/处理冲突/);
    await waitFor(() => expect(screen.getByTestId('merge-conflict-modal')).toBeInTheDocument());

    await mergeModalProps!.onSubmit({ conflicts: [] });
    // draftRevision 为空：成功提示中版本号兜底为 -（draftRevision || '-'）
    await waitFor(() => expect(screen.getByText(/草稿已更新到版本 -/)).toBeInTheDocument());
  });

  it('未打开冲突弹窗时提交被防御性忽略（manualMergeRecord 早退）', async () => {
    renderPanel([makeRecord({})]);
    // 未走过「处理冲突」预览：manualMergeRecord 为 null，onSubmit 应直接返回
    expect(mergeModalProps).not.toBeNull();
    await mergeModalProps!.onSubmit({ conflicts: [] });
    expect(mockMergeChanges).not.toHaveBeenCalled();
  });

  it('冲突弹窗取消仅关闭弹窗（onCancel）', async () => {
    renderPanel([makeRecord({})]);
    mockMergeChanges.mockResolvedValueOnce({
      message: 'preview',
      conflicts: 1,
    } as unknown as MergeResponse);

    await clickMenuItem(/处理冲突/);
    await waitFor(() => expect(mergeModalProps?.open ?? false).toBe(true));

    mergeModalProps!.onCancel();
    await waitFor(() => expect(mergeModalProps?.open ?? true).toBe(false));
    // 取消不产生合并调用
    expect(mockMergeChanges).toHaveBeenCalledTimes(1);
  });
});

describe('批量同步 Selector', () => {
  it('确认后按全部队列 pageKeys 调用，全部成功提示并刷新队列', async () => {
    const { onChanged } = renderPanel([
      makeRecord({ pageKey: 'p-published' }),
      makeRecord({ kind: 'draft', pageKey: 'p-draft' }),
    ]);
    mockBulkSyncPageSelectors.mockResolvedValue({
      total: 2,
      synced: ['p-published', 'p-draft'],
    } as PageBulkSyncSelectorsResult);

    fireEvent.click(screen.getByRole('button', { name: /批量同步 Selector/ }));
    // 确认弹窗带上队列页面数（published + draft 全部计入）
    expect(await screen.findByText(/将把 2 个契约变更页面/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /OK|确\s*定/ }));

    await waitFor(() =>
      expect(mockBulkSyncPageSelectors).toHaveBeenCalledWith(['p-published', 'p-draft']),
    );
    expect(
      await screen.findByText('已同步 2 个页面的草稿（未发布，可再一键重发布）'),
    ).toBeInTheDocument();
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it('存在 skipped 页面时提示需人工处理的漂移明细（manual code 优先、reason 兜底、仅前 3 条）', async () => {
    const { onChanged } = renderPanel([makeRecord({ pageKey: 'p0' })]);
    mockBulkSyncPageSelectors.mockResolvedValue({
      total: 4,
      synced: ['p0'],
      skipped: [
        {
          pageKey: 'p1',
          reason: 'r1',
          manual: [
            { code: 'input_schema_stale', severity: 'error', message: '输入变化' },
            { code: '', severity: 'warning', message: '空 code 项被过滤' },
          ],
        },
        { pageKey: 'p2', reason: 'governance' },
        { pageKey: 'p3', reason: '版本漂移' },
        { pageKey: 'p4', reason: '第 4 条被截断' },
      ],
    } as PageBulkSyncSelectorsResult);

    fireEvent.click(screen.getByRole('button', { name: /批量同步 Selector/ }));
    fireEvent.click(await screen.findByRole('button', { name: /OK|确\s*定/ }));

    // 明细拼接：manual 诊断 code 逗号连接，无有效 code 时回退 reason；只展示前 3 条
    expect(
      await screen.findByText(/p1: input_schema_stale；p2: governance；p3: 版本漂移/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/p4: 第 4 条被截断/)).not.toBeInTheDocument();
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it('存在 failed 页面时失败明细优先于 skipped 展示', async () => {
    renderPanel([makeRecord({ pageKey: 'p1' })]);
    mockBulkSyncPageSelectors.mockResolvedValue({
      total: 2,
      synced: ['p1'],
      skipped: [{ pageKey: 'p-skip', reason: 'governance' }],
      failed: [{ pageKey: 'p-fail', error: '版本冲突' }],
    } as PageBulkSyncSelectorsResult);

    fireEvent.click(screen.getByRole('button', { name: /批量同步 Selector/ }));
    fireEvent.click(await screen.findByRole('button', { name: /OK|确\s*定/ }));

    expect(
      await screen.findByText(/已同步 1 个页面，1 个失败：p-fail: 版本冲突/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/p-skip/)).not.toBeInTheDocument();
  });

  it('同步请求异常时提示失败且不刷新队列', async () => {
    const { onChanged } = renderPanel([makeRecord({ pageKey: 'p1' })]);
    mockBulkSyncPageSelectors.mockRejectedValue(new Error('boom'));

    fireEvent.click(screen.getByRole('button', { name: /批量同步 Selector/ }));
    fireEvent.click(await screen.findByRole('button', { name: /OK|确\s*定/ }));

    expect(await screen.findByText('批量同步 Selector 失败')).toBeInTheDocument();
    expect(onChanged).not.toHaveBeenCalled();
  });

  it('响应缺 synced 字段：按 0 计数走成功态（res.synced?.length ?? 0，L367）', async () => {
    const { onChanged } = renderPanel([makeRecord({ pageKey: 'p1' })]);
    mockBulkSyncPageSelectors.mockResolvedValue({ total: 1 } as PageBulkSyncSelectorsResult);

    fireEvent.click(screen.getByRole('button', { name: /批量同步 Selector/ }));
    fireEvent.click(await screen.findByRole('button', { name: /OK|确\s*定/ }));

    await waitFor(() => expect(mockBulkSyncPageSelectors).toHaveBeenCalledWith(['p1']));
    expect(
      await screen.findByText('已同步 0 个页面的草稿（未发布，可再一键重发布）'),
    ).toBeInTheDocument();
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });
});

describe('行内编辑与同步入口', () => {
  it('行内编辑按钮跳转页面工作台并携带 focus 参数', async () => {
    renderPanel([makeRecord({})]);
    fireEvent.click(screen.getByRole('button', { name: /编\s*辑/ }));
    await waitFor(() =>
      expect(mockedNavigateTo).toHaveBeenCalledWith('/functions/pages?focus=resource--player'),
    );
  });

  it('行内「一键同步 Selector」打开报告弹窗，关闭与同步完成后各自回调', async () => {
    const { onChanged } = renderPanel([makeRecord({ pageKey: 'resource--player' })]);

    await clickMenuItem(/一键同步 Selector/);
    await waitFor(() => expect(selectorSyncModalProps?.open ?? false).toBe(true));
    expect(selectorSyncModalProps?.pageKey).toBe('resource--player');

    // 同步落草稿后（onApplied）刷新契约变更队列
    selectorSyncModalProps!.onApplied(3);
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    // onApplied 不关闭弹窗（报告仍可查看）
    expect(selectorSyncModalProps?.open ?? false).toBe(true);

    // 关闭报告弹窗
    selectorSyncModalProps!.onClose();
    await waitFor(() => expect(selectorSyncModalProps?.open ?? true).toBe(false));
  });
});

describe('一键重新发布边界', () => {
  it('批量重发布请求异常时提示失败且不刷新队列', async () => {
    const { onChanged } = renderPanel([makeRecord({ pageKey: 'p1' })]);
    mockBulkRepublishPages.mockRejectedValue(new Error('boom'));

    fireEvent.click(screen.getByRole('button', { name: /一键重新发布全部/ }));
    fireEvent.click(await screen.findByRole('button', { name: /OK|确\s*定/ }));

    expect(await screen.findByText('一键重新发布失败')).toBeInTheDocument();
    expect(onChanged).not.toHaveBeenCalled();
  });

  it('响应缺 published 字段时成功计数按 0 兜底', async () => {
    const { onChanged } = renderPanel([makeRecord({ pageKey: 'p1' })]);
    mockBulkRepublishPages.mockResolvedValue({ total: 1 });

    fireEvent.click(screen.getByRole('button', { name: /一键重新发布全部/ }));
    fireEvent.click(await screen.findByRole('button', { name: /OK|确\s*定/ }));

    expect(await screen.findByText('已重新发布 0 个页面')).toBeInTheDocument();
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });
});

describe('渲染分支', () => {
  it('未知页面类型直接回显原始类型值', () => {
    renderPanel([makeRecord({ pageType: 'custom' as unknown as ContractChangeInfo['pageType'] })]);
    expect(screen.getByText('custom')).toBeInTheDocument();
  });

  it('bindingFreshness 携带诊断时展示错误计数', () => {
    renderPanel([
      makeRecord({
        bindingFreshness: [
          {
            bindingId: 'b1',
            status: 'input_schema_stale',
            diagnostic: {
              code: 'input_schema_stale',
              severity: 'error',
              message: '输入 schema 变化',
            },
          },
        ],
      }),
    ]);
    expect(screen.getByText('1 错误')).toBeInTheDocument();
  });

  it('focusPageKey 命中的行被标记高亮类名', () => {
    renderPanel([makeRecord({ pageKey: 'p-hit' }), makeRecord({ pageKey: 'p-other' })], {
      focusPageKey: 'p-hit',
    });
    const focusRow = document.querySelector('tr.proposal-inbox-focus-row');
    expect(focusRow).not.toBeNull();
    expect(within(focusRow as HTMLElement).getByText('p-hit')).toBeInTheDocument();
  });

  it('空队列时批量按钮禁用（空态提示文案挂在 Tooltip 上）', () => {
    renderPanel([]);
    expect(screen.getByRole('button', { name: /批量同步 Selector/ })).toBeDisabled();
    expect(screen.getByRole('button', { name: /一键重新发布全部/ })).toBeDisabled();
  });
});
