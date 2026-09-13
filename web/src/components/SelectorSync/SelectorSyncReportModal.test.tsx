/** SelectorSyncReportModal 覆盖：
 * dry-run 计划加载（open/pageKey 守卫、bindingIds 透传）、加载失败
 * （Error / 非 Error 两种 err 形态）与重试、空计划与字段缺省（|| 兜底）、
 * 完整报告渲染（7 种 action tag、high/low 置信度、changed/executionModeFixed/
 * functionId、输入/输出/需人工区块及 newTarget/newSource/required 缺省形态）、
 * 变更计数、剩余错误级诊断 warning（含 slice(0,8)）与清零 success、
 * Popconfirm 确认后 apply 成功（message.success/onApplied/按钮撤下）与
 * apply 失败（Error / 非 Error）。 */
import React from 'react';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { message } from 'antd';
import SelectorSyncReportModal from './SelectorSyncReportModal';
import { getPageDraft, syncPageSelectors } from '@/services/api/pages';
import type { BindingSelectorSyncReport, Diagnostic, PageSpecDraft } from '@/types/dashboard';

jest.mock('@/services/api/pages');

const mockedGetDraft = jest.mocked(getPageDraft);
const mockedSync = jest.mocked(syncPageSelectors);
const successSpy = jest.spyOn(message, 'success');
const errorSpy = jest.spyOn(message, 'error');

const PAGE_KEY = 'demo-page';
type SyncResp = Awaited<ReturnType<typeof syncPageSelectors>>;

const syncResponse = (partial: Partial<SyncResp>): SyncResp =>
  ({
    pageKey: PAGE_KEY,
    dryRun: true,
    applied: false,
    draftRevision: 7,
    ...partial,
  }) as SyncResp;

const errorDiag = (code: string): Diagnostic => ({
  code,
  severity: 'error',
  message: `错误-${code}`,
});

/** 完整报告：覆盖全部 action、置信度三态、required/newTarget/newSource 有无 */
const fullReport: BindingSelectorSyncReport[] = [
  {
    bindingId: 'bind-1',
    functionId: 'player.list',
    changed: true,
    executionModeFixed: true,
    input: [
      { target: 'uid', action: 'kept', reason: '输入-保留' },
      {
        target: 'oldName',
        action: 'renamed',
        newTarget: 'newName',
        confidence: 'high',
        reason: '输入-重映射',
      },
      { target: 'legacy', action: 'removed', confidence: 'low', reason: '输入-摘除' },
      { target: 'extra', action: 'added', reason: '输入-补齐' },
      { target: 'score', action: 'type_changed', reason: '输入-类型变化' },
    ],
    output: [
      {
        stateKey: 'rows',
        source: 'items',
        action: 'shape_updated',
        required: true,
        confidence: 'high',
        reason: '输出-形状更新',
      },
      {
        stateKey: 'total',
        source: 'count',
        action: 'renamed',
        newSource: 'count.v2',
        required: false,
        reason: '输出-重映射',
      },
      {
        stateKey: 'flag',
        source: 'ok',
        action: 'manual_required',
        required: false,
        reason: '输出-需人工',
      },
    ],
    manual: [{ code: 'M001', severity: 'warning', message: '人工项-1' }],
  },
  {
    bindingId: 'bind-2',
    changed: false,
    output: [
      {
        stateKey: 'empty-src',
        source: '',
        action: 'kept',
        newSource: '',
        required: false,
        reason: '输出-空串',
      },
    ],
  },
  { bindingId: 'bind-3', changed: false },
];

const renderModal = (props?: Partial<React.ComponentProps<typeof SelectorSyncReportModal>>) =>
  render(<SelectorSyncReportModal open pageKey={PAGE_KEY} onClose={jest.fn()} {...props} />);

/** Popconfirm 弹层 OK 按钮（antd 默认英文 locale） */
const clickPopconfirmOk = async (title: string) => {
  const root = await waitFor(() => {
    const node = Array.from(document.querySelectorAll('.ant-popover')).find((n) =>
      n.textContent?.includes(title),
    );
    if (!node) throw new Error(`popconfirm ${title} 未渲染`);
    return node as HTMLElement;
  });
  fireEvent.click(within(root).getByRole('button', { name: 'OK' }));
};

beforeEach(() => {
  jest.clearAllMocks();
  mockedGetDraft.mockResolvedValue({ draftRevision: 7 } as unknown as PageSpecDraft);
});

describe('SelectorSyncReportModal：dry-run 计划加载', () => {
  it('open=false / pageKey 为空 不触发加载；随后打开才请求', async () => {
    const { rerender } = render(
      <SelectorSyncReportModal open={false} pageKey={PAGE_KEY} onClose={jest.fn()} />,
    );
    expect(mockedGetDraft).not.toHaveBeenCalled();

    rerender(<SelectorSyncReportModal open pageKey={PAGE_KEY} onClose={jest.fn()} />);
    await waitFor(() => expect(mockedGetDraft).toHaveBeenCalledWith(PAGE_KEY));
  });

  it('pageKey 为空字符串时同样短路', () => {
    renderModal({ pageKey: '' });
    expect(mockedGetDraft).not.toHaveBeenCalled();
  });

  it('打开即 dry-run：带 draftRevision 与 bindingIds，标题含 pageKey', async () => {
    mockedSync.mockResolvedValueOnce(syncResponse({ syncedBindings: fullReport }));
    renderModal({ bindingIds: ['bind-1'] });

    await screen.findByText('bind-1');
    expect(mockedSync).toHaveBeenCalledWith(PAGE_KEY, {
      draftRevision: 7,
      dryRun: true,
      bindingIds: ['bind-1'],
    });
    expect(screen.getByText('一键同步 Selector（demo-page）')).toBeInTheDocument();
  });

  it('加载失败（Error）：错误 Alert 展示 message，重试后恢复', async () => {
    mockedGetDraft.mockRejectedValueOnce(new Error('后端 500'));
    renderModal();

    expect(await screen.findByText('加载同步计划失败')).toBeInTheDocument();
    expect(screen.getByText('后端 500')).toBeInTheDocument();
    fireEvent.click(screen.getByText('重试'));
    mockedSync.mockResolvedValueOnce(syncResponse({}));
    expect(await screen.findByText('没有需要同步的 binding')).toBeInTheDocument();
    expect(mockedGetDraft).toHaveBeenCalledTimes(2);
  });

  it('dry-run 抛非 Error：errorText 走 String(err)', async () => {
    mockedSync.mockRejectedValueOnce('裸字符串错误');
    renderModal();

    expect(await screen.findByText('裸字符串错误')).toBeInTheDocument();
    expect(screen.getByText('加载同步计划失败')).toBeInTheDocument();
  });
});

describe('SelectorSyncReportModal：报告渲染', () => {
  it('空计划（响应缺字段走 || 兜底）：info Alert、无应用按钮、无尾部 Alert', async () => {
    mockedSync.mockResolvedValueOnce({
      pageKey: PAGE_KEY,
      dryRun: true,
      applied: false,
      draftRevision: 7,
    } as SyncResp);
    renderModal();

    expect(await screen.findByText('没有需要同步的 binding')).toBeInTheDocument();
    expect(screen.queryByText('应用同步到草稿')).toBeNull();
    expect(screen.queryByText('同步后无错误级诊断，可尝试发布')).toBeNull();
    expect(screen.queryByText(/条错误级诊断/)).toBeNull();
  });

  it('完整报告：7 种动作 tag、双置信度、区块与计数', async () => {
    mockedSync.mockResolvedValueOnce(
      syncResponse({
        syncedBindings: fullReport,
        remainingDiagnostics: [
          errorDiag('E1'),
          { code: 'W1', severity: 'warning', message: '警告-1' },
        ],
      }),
    );
    renderModal();

    expect(await screen.findByText('bind-1')).toBeInTheDocument();
    // 动作 tag 全集（kept 在 bind-1 输入与 bind-2 输出各出现一次，用 AllBy）
    for (const label of ['保留', '重映射', '摘除', '补齐', '类型变化', '形状更新']) {
      expect(screen.getAllByText(label).length).toBeGreaterThan(0);
    }
    expect(screen.getAllByText('需人工处理').length).toBeGreaterThanOrEqual(2);
    // 置信度（high 在输入/输出各一处，用 AllBy）
    expect(screen.getAllByText('精确匹配').length).toBeGreaterThan(0);
    expect(screen.getByText('启发式')).toBeInTheDocument();
    // binding 级标记
    expect(screen.getByText('有变更')).toBeInTheDocument();
    expect(screen.getAllByText('无变更').length).toBe(2);
    expect(screen.getByText('执行模式已修正')).toBeInTheDocument();
    expect(screen.getByText('player.list')).toBeInTheDocument();
    // 区块标题与重映射箭头两侧（输出映射在 bind-1/bind-2 各一处）
    expect(screen.getByText('输入映射')).toBeInTheDocument();
    expect(screen.getAllByText('输出映射').length).toBe(2);
    expect(screen.getByText('oldName')).toBeInTheDocument();
    expect(screen.getByText('newName')).toBeInTheDocument();
    // 输出必填、空串占位（newSource:'' 三元为假不渲染 → 仅 source 一处）与非空 newSource
    expect(screen.getByText('必需')).toBeInTheDocument();
    expect(screen.getAllByText('""').length).toBe(1);
    expect(screen.getByText('count.v2')).toBeInTheDocument();
    // 人工项
    expect(screen.getByText('M001')).toBeInTheDocument();
    expect(screen.getByText('人工项-1')).toBeInTheDocument();
    // 变更计数（FormattedMessage 不插值，按字面断言）
    expect(screen.getByText('{count} 个 binding 将被修改；其余保留原样')).toBeInTheDocument();
    // 剩余错误级诊断 warning（只计 error，warning 被过滤）
    const alert = screen
      .getByText('同步后仍有 1 条错误级诊断，发布会继续被阻断；请处理「需人工处理」项后重试')
      .closest('.ant-alert');
    expect(alert).not.toBeNull();
    expect(within(alert as HTMLElement).getByText('E1')).toBeInTheDocument();
    expect(within(alert as HTMLElement).queryByText('W1')).toBeNull();
    // 应用按钮可见
    expect(screen.getByRole('button', { name: '应用同步到草稿' })).toBeInTheDocument();
  });

  it('剩余错误超 8 条时仅展示前 8 条', async () => {
    mockedSync.mockResolvedValueOnce(
      syncResponse({
        syncedBindings: fullReport,
        remainingDiagnostics: Array.from({ length: 9 }, (_, i) => errorDiag(`E${i + 1}`)),
      }),
    );
    renderModal();

    const alert = await waitFor(() => {
      const node = screen
        .getByText('同步后仍有 9 条错误级诊断，发布会继续被阻断；请处理「需人工处理」项后重试')
        .closest('.ant-alert');
      if (!node) throw new Error('warning alert 未渲染');
      return node as HTMLElement;
    });
    const items = within(alert).getAllByRole('listitem');
    expect(items.length).toBe(8);
    expect(within(alert).queryByText('E9')).toBeNull();
  });
});

describe('SelectorSyncReportModal：应用同步', () => {
  it('Popconfirm 确认后 apply：message.success、onApplied、按钮撤下、成功 Alert', async () => {
    mockedSync
      // dry-run
      .mockResolvedValueOnce(syncResponse({ syncedBindings: fullReport, remainingDiagnostics: [] }))
      // apply（响应缺 syncedBindings/remainingDiagnostics → || 兜底为 []）
      .mockResolvedValueOnce(syncResponse({ dryRun: false, applied: true, draftRevision: 8 }));
    const onApplied = jest.fn();
    renderModal({ onApplied });

    expect(await screen.findByText('同步后无错误级诊断，可尝试发布')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '应用同步到草稿' }));
    await clickPopconfirmOk('应用同步到草稿？');

    await waitFor(() => expect(mockedSync).toHaveBeenCalledTimes(2));
    expect(mockedSync).toHaveBeenLastCalledWith(PAGE_KEY, {
      draftRevision: 7,
      dryRun: false,
      bindingIds: undefined,
    });
    await waitFor(() =>
      expect(successSpy).toHaveBeenCalledWith('已应用到草稿（版本 8），请检查后手动发布'),
    );
    expect(onApplied).toHaveBeenCalledWith(8);
    // Alert 与 message toast（spy 透传真实渲染）各一处
    expect(
      screen.getAllByText('已应用到草稿（版本 8），请检查后手动发布').length,
    ).toBeGreaterThanOrEqual(1);
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: '应用同步到草稿' })).toBeNull(),
    );
  });

  it('apply 失败（Error）：message.error 带原因，按钮保留', async () => {
    mockedSync
      .mockResolvedValueOnce(syncResponse({ syncedBindings: fullReport, remainingDiagnostics: [] }))
      .mockRejectedValueOnce(new Error('版本冲突'));
    renderModal();

    await screen.findByText('bind-1');
    fireEvent.click(screen.getByRole('button', { name: '应用同步到草稿' }));
    await clickPopconfirmOk('应用同步到草稿？');

    await waitFor(() => expect(errorSpy).toHaveBeenCalledWith('应用同步失败：版本冲突'));
    expect(screen.getByRole('button', { name: '应用同步到草稿' })).toBeInTheDocument();
  });

  it('apply 失败（非 Error）：String(err) 拼接', async () => {
    mockedSync
      .mockResolvedValueOnce(syncResponse({ syncedBindings: fullReport, remainingDiagnostics: [] }))
      .mockRejectedValueOnce('oops');
    renderModal();

    await screen.findByText('bind-1');
    fireEvent.click(screen.getByRole('button', { name: '应用同步到草稿' }));
    await clickPopconfirmOk('应用同步到草稿？');

    await waitFor(() => expect(errorSpy).toHaveBeenCalledWith('应用同步失败：oops'));
  });

  it('关闭按钮与右上角 X 均回调 onClose', async () => {
    const onClose = jest.fn();
    mockedSync.mockResolvedValueOnce(syncResponse({}));
    renderModal({ onClose });

    await screen.findByText('没有需要同步的 binding');
    // antd 双字按钮自动插空格，用正则容忍
    fireEvent.click(screen.getByRole('button', { name: /关\s*闭/ }));
    expect(onClose).toHaveBeenCalledTimes(1);

    const closeIcon = document.querySelector('.ant-modal-close') as HTMLElement;
    expect(closeIcon).not.toBeNull();
    fireEvent.click(closeIcon);
    expect(onClose).toHaveBeenCalledTimes(2);
  });
});
