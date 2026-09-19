/** draftColumns 操作列挂载入口显式化 + shared 的 mount=1 URL 参数工具：
 * 「挂载菜单」是操作列显式图标按钮（不再埋在 ⋯ 下拉）；⋯ 下拉保留
 * 重新生成/版本历史/变更链/变更对比四项；currentMountFlag/clearMountParam
 * 读取与一次性消费语义（消费后 URL 参数移除、重复调用幂等）。 */
import React from 'react';
import { App } from 'antd';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { buildDraftColumns } from '../draftColumns';
import { clearMountParam, currentMountFlag } from '../shared';
import type { PageSpecDraftSummary } from '@/types/dashboard';

jest.mock('@umijs/max', () => ({
  __esModule: true,
  // shared.ts 模块级调用 getIntl（无组件上下文的 label 求值先例）
  getIntl: () => ({
    locale: 'zh-CN',
    formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  }),
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  }),
}));

const record: PageSpecDraftSummary = {
  pageKey: 'resource--players',
  type: 'resource',
  title: { 'zh-CN': '玩家列表' },
  category: { key: 'players' },
  status: 'published',
  draftRevision: 1,
  updatedAt: '2026-09-19T00:00:00Z',
};

function renderActions() {
  const handlers = {
    onEdit: jest.fn(),
    onPreview: jest.fn(),
    onPublish: jest.fn(),
    onUnpublish: jest.fn(),
    onRegenerate: jest.fn(),
    onVersions: jest.fn(),
    onChangeChain: jest.fn(),
    onDiff: jest.fn(),
    onMountMenu: jest.fn(),
  };
  const columns = buildDraftColumns(
    handlers,
    { confirm: jest.fn() },
    {
      locale: 'zh-CN',
      formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
    },
  );
  const actions = columns.find((column) => column.key === 'actions');
  render(
    <App>
      <div>{actions?.render?.(null, record)}</div>
    </App>,
  );
  return handlers;
}

describe('draftColumns 挂载菜单入口', () => {
  it('「挂载菜单」为操作列显式按钮：点击直接触发 onMountMenu', async () => {
    const handlers = renderActions();
    // Tooltip 在 hover 后出现；按钮由 aria-label（图标按钮）定位
    const mountBtn = await screen.findByRole('button', { name: /apartment/i });
    fireEvent.click(mountBtn);
    expect(handlers.onMountMenu).toHaveBeenCalledWith(record);
    expect(handlers.onEdit).not.toHaveBeenCalled();
  });

  it('⋯ 下拉不再含「挂载菜单」：保留重新生成/版本历史/变更链/变更对比', async () => {
    renderActions();
    const moreBtn = await waitFor(() => {
      const btn = document.querySelector('.anticon-more')?.closest('button');
      if (!btn) throw new Error('not rendered');
      return btn as HTMLElement;
    });
    fireEvent.click(moreBtn);
    expect(await screen.findByText('重新生成')).toBeInTheDocument();
    expect(screen.getByText('版本历史')).toBeInTheDocument();
    expect(screen.getByText('变更链')).toBeInTheDocument();
    expect(screen.getByText('变更对比')).toBeInTheDocument();
    expect(screen.queryByText('挂载菜单')).not.toBeInTheDocument();
  });
});

describe('shared mount=1 URL 参数工具', () => {
  it('currentMountFlag 识别 mount=1；clearMountParam 一次性消费', () => {
    window.history.pushState({}, '', '/functions/pages?focus=players&mount=1');
    expect(currentMountFlag()).toBe(true);

    clearMountParam();
    expect(currentMountFlag()).toBe(false);
    // mount 参数已从 URL 移除，focus 保留
    expect(window.location.search).toBe('?focus=players');

    // 无 mount 参数时 clear 幂等（不触发 replaceState）
    clearMountParam();
    expect(window.location.search).toBe('?focus=players');
  });

  it('非 mount=1（缺失或其它值）一律返回 false', () => {
    window.history.pushState({}, '', '/functions/pages');
    expect(currentMountFlag()).toBe(false);
    window.history.pushState({}, '', '/functions/pages?mount=0');
    expect(currentMountFlag()).toBe(false);
  });
});
