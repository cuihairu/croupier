/**
 * studio 变更抽屉真实渲染单测（DiffDrawer / ChangeChainDrawer）：
 * 两组件此前无直接测试（仅经 studio 主页传递命中约 38%），本套件按
 * proposalModals.test.tsx 口径渲染真实组件——变更对比的自动合并/冲突
 * 卡、明细 changeType 配色与 pre JSON、变更链时间线与版本兜底，均为
 * 真实 antd 树断言（不 jest.mock 顶替组件本体）。
 */
import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import DiffDrawer from '../DiffDrawer';
import ChangeChainDrawer from '../ChangeChainDrawer';
import type { ChangeChain, DiffChange, DiffResponse } from '@/services/api/versioning';

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
    // shared.ts 于模块级调用 getIntl（changeChain → ./shared 链）
    getIntl: () => ({ locale: 'zh-CN', formatMessage }),
    useIntl: () => ({ locale: 'zh-CN', formatMessage }),
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => <>{defaultMessage}</>,
  };
});

const change = (over: Partial<DiffChange> & { path: string }): DiffChange => ({
  oldValue: null,
  newValue: null,
  changeType: 'modified',
  isSemantic: false,
  ...over,
});

const diffData = (): DiffResponse => ({
  summary: '共 3 处字段变更',
  autoMergeItems: [
    { field: 'title', reason: '仅草稿侧修改' },
    { field: 'sortOrder', reason: '草稿与最新一致' },
  ],
  conflictItems: [{ field: 'query.params', reason: '双方同时修改' }],
  changes: [
    {
      path: 'players.hp',
      oldValue: { hp: 100 },
      newValue: { hp: 120 },
      changeType: 'modified',
      isSemantic: true,
    },
    change({ path: 'players.rev', changeType: 'added' }),
    change({ path: 'players.obsolete', changeType: 'removed' }),
  ],
});

describe('DiffDrawer（变更对比抽屉）', () => {
  it('data 为 null：空态「暂无变更」；loading 时骨架屏替换内容', () => {
    const { rerender } = render(
      <DiffDrawer open data={null} loading={false} onClose={jest.fn()} onMerge={jest.fn()} />,
    );
    expect(screen.getByText('暂无变更')).toBeInTheDocument();

    rerender(<DiffDrawer open data={null} loading onClose={jest.fn()} onMerge={jest.fn()} />);
    expect(screen.queryByText('暂无变更')).not.toBeInTheDocument();
    expect(document.body.querySelector('.ant-skeleton')).not.toBeNull();
  });

  it('自动合并/冲突卡：count 插值标题、级别 Tag、字段与原因', () => {
    render(
      <DiffDrawer open data={diffData()} loading={false} onClose={jest.fn()} onMerge={jest.fn()} />,
    );
    expect(screen.getByText('共 3 处字段变更')).toBeInTheDocument();
    expect(screen.getByText('可自动合并 2 个展示字段')).toBeInTheDocument();
    expect(screen.getByText('必须人工确认 1 个冲突字段')).toBeInTheDocument();

    const autoTags = screen.getAllByText('auto');
    expect(autoTags.length).toBe(2);
    for (const tag of autoTags) {
      expect(tag.classList.contains('ant-tag-blue')).toBe(true);
    }
    expect(screen.getByText('title')).toBeInTheDocument();
    expect(screen.getByText('仅草稿侧修改')).toBeInTheDocument();
    expect(screen.getByText('sortOrder')).toBeInTheDocument();

    const conflictTag = screen.getByText('conflict');
    expect(conflictTag.classList.contains('ant-tag-red')).toBe(true);
    expect(screen.getByText('query.params')).toBeInTheDocument();
    expect(screen.getByText('双方同时修改')).toBeInTheDocument();
  });

  it('明细：added/removed/modified 配色、语义变更标记、old/new pre JSON', () => {
    render(
      <DiffDrawer open data={diffData()} loading={false} onClose={jest.fn()} onMerge={jest.fn()} />,
    );
    expect(screen.getByText('added').classList.contains('ant-tag-green')).toBe(true);
    expect(screen.getByText('removed').classList.contains('ant-tag-red')).toBe(true);
    expect(screen.getByText('modified').classList.contains('ant-tag-orange')).toBe(true);
    expect(screen.getByText('语义变更')).toBeInTheDocument();
    expect(screen.getByText('players.hp')).toBeInTheDocument();
    expect(screen.getByText('players.rev')).toBeInTheDocument();

    const pres = Array.from(document.body.querySelectorAll('pre'));
    expect(pres.map((p) => JSON.parse(p.textContent || ''))).toEqual([{ hp: 100 }, { hp: 120 }]);
  });

  it('autoMergeItems/conflictItems 缺省与空数组不渲染卡片；无旧/新值时无 pre', () => {
    render(
      <DiffDrawer
        open
        data={{ summary: '仅草稿侧调整', changes: [change({ path: 'x.y' })] }}
        loading={false}
        onClose={jest.fn()}
        onMerge={jest.fn()}
      />,
    );
    expect(screen.queryByText(/可自动合并/)).not.toBeInTheDocument();
    expect(screen.queryByText(/必须人工确认/)).not.toBeInTheDocument();
    expect(document.body.querySelectorAll('.ant-card').length).toBe(1);
    expect(document.body.querySelectorAll('pre').length).toBe(0);
  });

  it('交互：合并按钮触达 onMerge，抽屉关闭按钮触达 onClose', () => {
    const onMerge = jest.fn();
    const onClose = jest.fn();
    render(<DiffDrawer open data={null} loading={false} onClose={onClose} onMerge={onMerge} />);
    fireEvent.click(screen.getByRole('button', { name: /合并变更/ }));
    expect(onMerge).toHaveBeenCalledTimes(1);

    fireEvent.click(document.body.querySelector('.ant-drawer-close')!);
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});

describe('ChangeChainDrawer（变更链抽屉）', () => {
  const chainData = (over: Partial<ChangeChain> = {}): ChangeChain => ({
    pageKey: 'resource--players',
    resourceKey: 'players',
    current: {
      functionVersion: 'v3',
      semanticVersion: 7,
      proposalVersion: 2,
      draftRevision: 12,
      publishedVersion: 5,
    },
    items: [
      {
        type: 'publish',
        timestamp: '2026-09-19T00:00:00Z',
        summary: '发布 v5',
        version: 5,
        actor: 'alice',
      },
      { type: 'draft_update', timestamp: '2026-09-20T00:00:00Z', summary: '草稿修订' },
    ],
    ...over,
  });

  it('chain 为 null：空态「暂无变更记录」', () => {
    render(<ChangeChainDrawer open chain={null} loading={false} onClose={jest.fn()} />);
    expect(screen.getByText('暂无变更记录')).toBeInTheDocument();
  });

  it('完整链：页面/资源标题、当前状态五段全值、时间线条目与操作人', () => {
    render(<ChangeChainDrawer open chain={chainData()} loading={false} onClose={jest.fn()} />);
    expect(screen.getByText('resource--players')).toBeInTheDocument();
    expect(screen.getByText('players')).toBeInTheDocument();
    expect(
      screen.getByText('函数版本: v3, 语义版本: 7, 提案版本: 2, 草稿版本: 12, 发布版本: 5'),
    ).toBeInTheDocument();

    const stamp = (v: string) => new Date(v).toLocaleString();
    expect(screen.getByText(`${stamp('2026-09-19T00:00:00Z')} - alice`)).toBeInTheDocument();
    expect(screen.getByText(stamp('2026-09-20T00:00:00Z'))).toBeInTheDocument();
    const publishTag = screen.getByText('publish');
    expect(publishTag.classList.contains('ant-tag-blue')).toBe(true);
    expect(screen.getByText('发布 v5')).toBeInTheDocument();
    expect(screen.getByText('草稿修订')).toBeInTheDocument();
  });

  it('当前状态版本缺省一律「-」兜底', () => {
    render(
      <ChangeChainDrawer
        open
        chain={chainData({ current: {} })}
        loading={false}
        onClose={jest.fn()}
      />,
    );
    expect(
      screen.getByText('函数版本: -, 语义版本: -, 提案版本: -, 草稿版本: -, 发布版本: -'),
    ).toBeInTheDocument();
  });
});
