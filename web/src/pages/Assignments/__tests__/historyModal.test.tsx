/** Assignments 变更历史明细回归（OPEN-ISSUES #37）：
 * 1. 每条记录渲染 diff 摘要：新增/移除/未识别着色标签 + 变更前后计数行；
 * 2. 全量 before/after 清单默认折叠，展开可见且不截断；
 * 3. details 缺失或全空：不渲染 diff 区且不崩溃；
 * 4. 未知扩展 key 不被吞：仍走原始键值兜底表格。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { App } from 'antd';
import HistoryModal from '../HistoryModal';
import type { AssignmentHistory } from '../types';

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, v);
      }
      return text;
    },
  }),
  getIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

const entry = (overrides: Partial<AssignmentHistory> = {}): AssignmentHistory => ({
  id: '1',
  gameId: 'demo',
  env: 'prod',
  functionId: 'all',
  action: 'assign',
  count: 2,
  operatedBy: 'alice',
  operatedAt: '2026-09-27T08:00:00Z',
  ...overrides,
});

const renderModal = (history: AssignmentHistory[]) =>
  render(
    <App>
      <HistoryModal
        open
        history={history}
        loading={false}
        page={1}
        pageSize={20}
        total={history.length}
        actionFilter="all"
        onClose={jest.fn()}
        onActionFilterChange={jest.fn()}
        onReload={jest.fn()}
        onPageChange={jest.fn()}
      />
    </App>,
  );

describe('Assignments 变更历史明细（#37）', () => {
  it('渲染 diff 摘要：新增/移除/未识别标签与变更前后计数', () => {
    renderModal([
      entry({
        details: {
          before: ['a', 'b', 'c'],
          after: ['b', 'c', 'd', 'e'],
          added: ['d', 'e'],
          removed: ['a'],
          unknown: ['ghost.fn'],
        },
      }),
    ]);

    expect(screen.getByText('变更前 3 → 变更后 4')).toBeInTheDocument();
    expect(screen.getByText('新增')).toBeInTheDocument();
    expect(screen.getByText('d')).toBeInTheDocument();
    expect(screen.getByText('e')).toBeInTheDocument();
    expect(screen.getByText('移除')).toBeInTheDocument();
    expect(screen.getByText('a')).toBeInTheDocument();
    expect(screen.getByText('未识别')).toBeInTheDocument();
    expect(screen.getByText('ghost.fn')).toBeInTheDocument();
    // 原始英文 key 不再作为展示行
    expect(screen.queryByText('before')).toBeNull();
    expect(screen.queryByText('added')).toBeNull();
  });

  it('全量 before/after 清单默认折叠，展开后全量可见', () => {
    renderModal([
      entry({
        details: {
          before: ['a', 'b'],
          after: ['a', 'b', 'x', 'y'],
          added: ['x', 'y'],
          removed: [],
          unknown: [],
        },
      }),
    ]);

    // 默认折叠态（jsdom + antd cssinjs 下 toBeVisible 不可靠，用 aria 断言）
    const header = screen
      .getByText('展开全量清单')
      .closest('.ant-collapse-header') as HTMLElement | null;
    expect(header).not.toBeNull();
    expect(header?.getAttribute('aria-expanded')).toBe('false');

    // 展开后全量清单渲染：b 同时出现在 before/after 全量行（共 2 处），
    // 内容不被丢弃（antd6 折叠面板 children 惰性渲染，展开后才进 DOM）
    fireEvent.click(screen.getByText('展开全量清单'));
    expect(screen.getAllByText('b')).toHaveLength(2);
  });

  it('details 缺失或全空：不渲染 diff 区且不崩溃', () => {
    const { rerender } = renderModal([entry({ details: undefined })]);
    expect(screen.queryByText('新增')).toBeNull();
    expect(screen.getByText(/操作人: alice/)).toBeInTheDocument();

    rerender(
      <App>
        <HistoryModal
          open
          history={[
            entry({
              details: { before: [], after: [], added: [], removed: [], unknown: [] },
            }),
          ]}
          loading={false}
          page={1}
          pageSize={20}
          total={1}
          actionFilter="all"
          onClose={jest.fn()}
          onActionFilterChange={jest.fn()}
          onReload={jest.fn()}
          onPageChange={jest.fn()}
        />
      </App>,
    );
    expect(screen.queryByText('新增')).toBeNull();
  });

  it('未知扩展 key 走原始键值兜底，不被静默吞掉', () => {
    renderModal([
      entry({
        details: {
          added: ['x'],
          before: [],
          after: [],
          removed: [],
          unknown: [],
          customNote: '人工备注内容',
        },
      }),
    ]);

    expect(screen.getByText('customNote')).toBeInTheDocument();
    expect(screen.getByText('人工备注内容')).toBeInTheDocument();
  });

  it('超过 8 项的分组截断并显示 +N 余量', () => {
    renderModal([
      entry({
        details: {
          before: [],
          after: ['1', '2', '3', '4', '5', '6', '7', '8', '9', '10'],
          added: ['1', '2', '3', '4', '5', '6', '7', '8', '9', '10'],
          removed: [],
          unknown: [],
        },
      }),
    ]);

    // 摘要行只展示前 8 项，余量以 +N 汇总
    expect(screen.getByText('+2')).toBeInTheDocument();

    // 余量项（9/10）完整保留在折叠的全量清单中，展开后可见、不被静默丢弃
    // ——若全量行也被截断则查询结果为 0
    fireEvent.click(screen.getByText('展开全量清单'));
    expect(screen.getAllByText('9').length).toBeGreaterThan(0);
    expect(screen.getAllByText('10').length).toBeGreaterThan(0);
  });
});
