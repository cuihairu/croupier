/**
 * 提案详情/预览弹窗（ProposalDetailModal / ProposalPreviewModal）单测：
 * 此前两组件在所有套件中均被 jest.mock 替身顶替（0% 覆盖）——本套件渲染
 * 真实组件：详情弹窗的元信息 Descriptions、诊断表（级别配色/字段兜底）与
 * 预览弹窗的 PageSpec 渲染、空态、以及「预览不执行函数」的显式拒绝。
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import type { DiagnosticInfo, PageProposal } from '@/types/dashboard';
import ProposalDetailModal from '../ProposalDetailModal';
import ProposalPreviewModal from '../ProposalPreviewModal';
import PageRenderer from '@/components/PageRenderer';

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

// PageRenderer 替身：透传 props 快照 + 可控的 onExecute 句柄
let capturedExecute: (() => Promise<unknown>) | null = null;
jest.mock('@/components/PageRenderer', () => ({
  __esModule: true,
  default: (props: {
    preview?: boolean;
    onExecute?: () => Promise<unknown>;
    pageSpec?: unknown;
  }) => {
    capturedExecute = props.onExecute ?? null;
    return (
      <div data-testid="page-renderer-stub" data-preview={String(props.preview === true)}>
        renderer:{JSON.stringify(props.pageSpec)}
      </div>
    );
  },
}));

const proposal = (overrides: Partial<PageProposal> = {}): PageProposal => ({
  id: 1,
  proposalKey: 'resource--players',
  pageKey: 'players',
  pageType: 'resource',
  quality: 'ready',
  generatorVersion: 'v1',
  title: { 'zh-CN': '玩家列表' },
  pageSpec: { type: 'resource' } as PageProposal['pageSpec'],
  status: 'pending',
  pageExists: false,
  updatedAt: '2026-09-19T00:00:00Z',
  ...overrides,
});

const diag = (over: Partial<DiagnosticInfo> & { code: string }): DiagnosticInfo => ({
  severity: 'warning',
  message: '字段漂移',
  ...over,
});

describe('ProposalDetailModal（提案详情弹窗）', () => {
  it('元信息 Descriptions：key/页面/本地化标题/类型·质量·状态标签', () => {
    render(<ProposalDetailModal open proposal={proposal()} onClose={jest.fn()} />);

    expect(screen.getByText('resource--players')).toBeInTheDocument();
    expect(screen.getByText('players')).toBeInTheDocument();
    expect(screen.getByText('玩家列表')).toBeInTheDocument();
    expect(screen.getByText('资源')).toBeInTheDocument();
    expect(screen.getByText('可直接发布')).toBeInTheDocument();
    expect(screen.getByText('待处理')).toBeInTheDocument();
  });

  it('摘要字段缺省显示「-」，有值显示原文', () => {
    const { rerender } = render(
      <ProposalDetailModal proposal={proposal()} open onClose={jest.fn()} />,
    );
    // functionDigest/semanticsDigest 均未提供 → 两处「-」
    expect(screen.getAllByText('-').length).toBe(2);

    rerender(
      <ProposalDetailModal
        open
        proposal={proposal({ functionDigest: 'fn:abc123', semanticsDigest: 'sem:def456' })}
        onClose={jest.fn()}
      />,
    );
    expect(screen.getByText('fn:abc123')).toBeInTheDocument();
    expect(screen.getByText('sem:def456')).toBeInTheDocument();
  });

  it('诊断表：级别按严重度配色，缺字段兜底「-」，说明原文渲染', () => {
    render(
      <ProposalDetailModal
        open
        proposal={proposal({
          diagnostics: [
            diag({ code: 'input_schema_stale', severity: 'error', field: 'playerId' }),
            diag({ code: 'shape_drift', severity: 'warning' }),
            diag({ code: 'info_only', severity: 'info', message: '仅提示' }),
          ],
        })}
        onClose={jest.fn()}
      />,
    );

    expect(screen.getByText('input_schema_stale')).toBeInTheDocument();
    // error/warning/info 三种级别 Tag（class 前缀 ant-tag-<color>）——info 行
    // 在诊断表内断言，避免命中上方页面类型 Tag 的 ant-tag-blue
    expect(document.querySelector('.ant-tag-error')?.textContent).toBe('error');
    expect(document.querySelector('.ant-tag-warning')?.textContent).toBe('warning');
    const infoRow = screen.getByText('info_only').closest('tr');
    expect(infoRow?.querySelector('.ant-tag-blue')?.textContent).toBe('info');
    // shape_drift 行缺 field → 「-」兜底
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('仅提示')).toBeInTheDocument();
  });

  it('无诊断时不渲染表格；PageSpec 说明常驻', () => {
    render(<ProposalDetailModal open proposal={proposal()} onClose={jest.fn()} />);
    expect(document.querySelector('.ant-table')).toBeNull();
    expect(
      screen.getByText(
        'PageSpec 为发布快照输入，不在正常路径手工编辑 JSON；如需调整请进入 Page Studio 编辑器。',
      ),
    ).toBeInTheDocument();
  });

  it('proposal 为 null 时不渲染任何内容区', () => {
    render(<ProposalDetailModal open proposal={null} onClose={jest.fn()} />);
    expect(document.querySelector('.ant-descriptions')).toBeNull();
    expect(document.querySelector('.ant-table')).toBeNull();
  });
});

describe('ProposalPreviewModal（默认页面预览弹窗）', () => {
  it('有 pageSpec：只读渲染 PageRenderer（preview 标记透传）', () => {
    render(
      <ProposalPreviewModal
        open
        proposal={proposal({ pageSpec: { type: 'resource' } as PageProposal['pageSpec'] })}
        onClose={jest.fn()}
      />,
    );
    const stub = screen.getByTestId('page-renderer-stub');
    expect(stub.getAttribute('data-preview')).toBe('true');
    expect(stub.textContent).toContain('resource');
    expect(screen.queryByText('暂无可预览页面')).not.toBeInTheDocument();
  });

  it('「Proposal 预览不执行函数」：onExecute 显式拒绝', async () => {
    render(
      <ProposalPreviewModal
        open
        proposal={proposal({ pageSpec: { type: 'resource' } as PageProposal['pageSpec'] })}
        onClose={jest.fn()}
      />,
    );
    expect(capturedExecute).not.toBeNull();
    await expect(capturedExecute!()).rejects.toThrow(
      'Proposal 预览不执行函数；发布后请在运行控制台执行。',
    );
  });

  it('无 pageSpec：空态「暂无可预览页面」，不渲染 PageRenderer', () => {
    render(
      <ProposalPreviewModal
        open
        proposal={proposal({ pageSpec: undefined as never })}
        onClose={jest.fn()}
      />,
    );
    expect(screen.getByText('暂无可预览页面')).toBeInTheDocument();
    expect(screen.queryByTestId('page-renderer-stub')).toBeNull();
  });

  it('proposal 为 null：空态', () => {
    render(<ProposalPreviewModal open proposal={null} onClose={jest.fn()} />);
    expect(screen.getByText('暂无可预览页面')).toBeInTheDocument();
  });
});
