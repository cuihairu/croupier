/** draftColumns 覆盖补全（与 draftColumns.test.tsx 分工：该文件专注挂载菜单
 * 入口与 shared 的 URL 参数工具，本文件直测导出列定义的全部回调与 render：
 * 1) 六个数据列 render（pageKey/标题/分类/状态/版本/更新时间，含缺省兜底）；
 * 2) 操作列 draft 态发布分支（Popconfirm → onPublish）与非 draft 态取消发布；
 * 3) 编辑/预览两个图标按钮回调；
 * 4) ⋯ 下拉四项（重新生成走 modal.confirm onOk、版本历史、变更链、变更对比）。 */
import React from 'react';
import { App } from 'antd';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { buildDraftColumns } from '../draftColumns';
import type { PageSpecDraftSummary } from '@/types/dashboard';

jest.mock('@umijs/max', () => ({
  __esModule: true,
  // shared.ts 模块级调用 getIntl（pageTypeLabel 求值）
  getIntl: () => ({
    locale: 'zh-CN',
    formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  }),
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  }),
}));

type Handlers = {
  onEdit: jest.Mock;
  onPreview: jest.Mock;
  onPublish: jest.Mock;
  onUnpublish: jest.Mock;
  onRegenerate: jest.Mock;
  onVersions: jest.Mock;
  onChangeChain: jest.Mock;
  onDiff: jest.Mock;
  onMountMenu: jest.Mock;
};

function setup() {
  const handlers: Handlers = {
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
  const confirm = jest.fn();
  const columns = buildDraftColumns(
    handlers,
    { confirm },
    {
      locale: 'zh-CN',
      formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
    },
  );
  const byKey = (key: string) => {
    const col = columns.find((c) => c.key === key);
    if (!col) throw new Error(`column ${key} not found`);
    return col;
  };
  return { handlers, confirm, byKey };
}

const base: PageSpecDraftSummary = {
  pageKey: 'resource--players',
  type: 'resource',
  title: { 'zh-CN': '玩家列表' },
  category: { key: 'players' },
  status: 'draft',
  draftRevision: 5,
  publishedVersion: 'v3',
  updatedAt: '2026-09-19T02:03:04Z',
};

/** 渲染单列单元格（ProColumns.render 返回 ReactNode）。 */
function renderCell(node: React.ReactNode) {
  return render(
    <App>
      <>{node}</>
    </App>,
  );
}

/** 渲染操作列单元格并返回 handlers/confirm。 */
function renderActions(record: PageSpecDraftSummary) {
  const ctx = setup();
  const col = ctx.byKey('actions');
  renderCell(col.render?.(null, record));
  return ctx;
}

/** Popconfirm 浮层（antd 默认英文 locale：OK / Cancel）。 */
async function confirmPopconfirm(title: string) {
  const root = await waitFor(() => {
    const node = Array.from(document.querySelectorAll('.ant-popover')).find((n) =>
      n.textContent?.includes(title),
    );
    if (!node) throw new Error(`popconfirm ${title} 未渲染`);
    return node as HTMLElement;
  });
  fireEvent.click(within(root).getByRole('button', { name: 'OK' }));
}

/** 打开 ⋯ 下拉（每次点击菜单项后浮层关闭，需重开）。 */
async function openDropdown() {
  const more = await waitFor(() => {
    const btn = document.querySelector('.anticon-more')?.closest('button');
    if (!btn) throw new Error('more button not rendered');
    return btn as HTMLElement;
  });
  fireEvent.click(more);
  await screen.findByText('重新生成');
}

describe('draftColumns 数据列 render', () => {
  it('pageKey 列：标识 + 类型标签（pageTypeLabel 走 intl）', () => {
    const { byKey } = setup();
    const col = byKey('pageKey');
    expect(col.title).toBe('页面标识');
    expect(col.width).toBe(220);
    renderCell(col.render?.(null, base));
    expect(screen.getByText('resource--players')).toBeInTheDocument();
    expect(screen.getByText('资源页面')).toBeInTheDocument();
  });

  it('标题列：LocalizedText 按 intl.locale 取值', () => {
    const { byKey } = setup();
    const col = byKey('title');
    expect(col.ellipsis).toBe(true);
    renderCell(col.render?.(null, base));
    expect(screen.getByText('玩家列表')).toBeInTheDocument();
  });

  it('分类列：有 category 取 key、缺失兜底 -', () => {
    const { byKey } = setup();
    const col = byKey('category');
    expect(col.dataIndex).toEqual(['category', 'key']);
    const { unmount } = renderCell(col.render?.(null, base));
    expect(screen.getByText('players')).toBeInTheDocument();
    unmount();
    renderCell(col.render?.(null, { ...base, category: undefined }));
    expect(screen.getByText('-')).toBeInTheDocument();
  });

  it('状态列：状态文本入 Tag', () => {
    const { byKey } = setup();
    const col = byKey('status');
    renderCell(col.render?.(null, base));
    expect(screen.getByText('draft')).toBeInTheDocument();
    const { unmount } = renderCell(col.render?.(null, { ...base, status: 'published' }));
    expect(screen.getByText('published')).toBeInTheDocument();
    unmount();
    renderCell(col.render?.(null, { ...base, status: 'archived' }));
    expect(screen.getByText('archived')).toBeInTheDocument();
  });

  it('版本列：有 publishedVersion 显示原值、缺失兜底 -', () => {
    const { byKey } = setup();
    const col = byKey('version');
    expect(col.fixed).toBeUndefined();
    const { unmount } = renderCell(col.render?.(null, base));
    expect(screen.getByText('v3')).toBeInTheDocument();
    unmount();
    renderCell(col.render?.(null, { ...base, publishedVersion: undefined }));
    expect(screen.getByText('-')).toBeInTheDocument();
  });

  it('更新时间列：formatDate 本地化输出；缺失兜底 -', () => {
    const { byKey } = setup();
    const col = byKey('updatedAt');
    const { unmount } = renderCell(col.render?.(null, base));
    expect(screen.getByText(new Date(base.updatedAt).toLocaleString())).toBeInTheDocument();
    unmount();
    renderCell(col.render?.(null, { ...base, updatedAt: undefined }));
    expect(screen.getByText('-')).toBeInTheDocument();
  });
});

describe('draftColumns 操作列：编辑/预览/发布与取消发布', () => {
  it('编辑按钮 → onEdit(pageKey, type)', async () => {
    const { handlers } = renderActions(base);
    fireEvent.click(await screen.findByRole('button', { name: /edit/i }));
    expect(handlers.onEdit).toHaveBeenCalledWith('resource--players', 'resource');
  });

  it('预览按钮 → onPreview(pageKey)', async () => {
    const { handlers } = renderActions(base);
    fireEvent.click(await screen.findByRole('button', { name: /eye/i }));
    expect(handlers.onPreview).toHaveBeenCalledWith('resource--players');
  });

  it('draft 态：发布 Popconfirm 确认 → onPublish(pageKey, draftRevision)', async () => {
    const { handlers } = renderActions(base);
    // draft 态渲染发布（Rocket）而非取消发布（Stop）
    expect(screen.queryByRole('button', { name: /stop/i })).not.toBeInTheDocument();
    fireEvent.click(await screen.findByRole('button', { name: /rocket/i }));
    await confirmPopconfirm('确认发布此页面？');
    expect(handlers.onPublish).toHaveBeenCalledWith('resource--players', 5);
  });

  it('非 draft 态：取消发布 Popconfirm 确认 → onUnpublish(pageKey)', async () => {
    const { handlers } = renderActions({ ...base, status: 'published' });
    expect(screen.queryByRole('button', { name: /rocket/i })).not.toBeInTheDocument();
    fireEvent.click(await screen.findByRole('button', { name: /stop/i }));
    await confirmPopconfirm('确认取消发布此页面？');
    expect(handlers.onUnpublish).toHaveBeenCalledWith('resource--players');
  });
});

describe('draftColumns ⋯ 下拉四项', () => {
  it('重新生成：modal.confirm 收到标题/内容，onOk → onRegenerate(pageKey, draftRevision)', async () => {
    const { handlers, confirm } = renderActions(base);
    await openDropdown();
    fireEvent.click(screen.getByText('重新生成'));
    expect(confirm).toHaveBeenCalledTimes(1);
    const cfg = confirm.mock.calls[0][0] as {
      title: string;
      content: string;
      onOk: () => void;
    };
    expect(cfg.title).toBe('确认按最新 Proposal 重新生成草稿？');
    expect(cfg.content).toContain('当前草稿修改将被最新 Proposal 覆盖');
    expect(handlers.onRegenerate).not.toHaveBeenCalled();
    cfg.onOk();
    expect(handlers.onRegenerate).toHaveBeenCalledWith('resource--players', 5);
  });

  it('版本历史 → onVersions(pageKey)', async () => {
    const { handlers } = renderActions(base);
    await openDropdown();
    fireEvent.click(screen.getByText('版本历史'));
    expect(handlers.onVersions).toHaveBeenCalledWith('resource--players');
  });

  it('变更链 → onChangeChain(pageKey)', async () => {
    const { handlers } = renderActions(base);
    await openDropdown();
    fireEvent.click(screen.getByText('变更链'));
    expect(handlers.onChangeChain).toHaveBeenCalledWith('resource--players');
  });

  it('变更对比 → onDiff(pageKey)', async () => {
    const { handlers } = renderActions(base);
    await openDropdown();
    fireEvent.click(screen.getByText('变更对比'));
    expect(handlers.onDiff).toHaveBeenCalledWith('resource--players');
  });
});
