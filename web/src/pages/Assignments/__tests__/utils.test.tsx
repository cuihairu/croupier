/** Assignments 工具函数单元回归（OPEN-ISSUES #37）：
 * formatDateTime 各形态解析；renderHistoryDetail 数组/对象/标量分支；
 * renderHistoryDiff 截断边界与空值处理。 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { formatDateTime, renderHistoryDetail, renderHistoryDiff } from '../utils';

jest.mock('@umijs/max', () => ({
  getIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage ?? '',
  }),
}));

describe('formatDateTime（#37）', () => {
  it('空值返回占位符', () => {
    expect(formatDateTime()).toBe('-');
    expect(formatDateTime(null)).toBe('-');
    expect(formatDateTime('')).toBe('-');
    expect(formatDateTime('   ')).toBe('-');
  });

  it('Date 实例如法解析；非法 Date 回退占位符', () => {
    expect(formatDateTime(new Date('2026-09-27T08:00:00Z'))).toBe(
      new Date('2026-09-27T08:00:00Z').toLocaleString('zh-CN'),
    );
    expect(formatDateTime(new Date('invalid'))).toBe('-');
  });

  it('数字时间戳：秒（<1e12）自动放大为毫秒', () => {
    expect(formatDateTime(1727424000)).toBe(new Date(1727424000 * 1000).toLocaleString('zh-CN'));
    expect(formatDateTime(1727424000000)).toBe(new Date(1727424000000).toLocaleString('zh-CN'));
  });

  it('数字字符串同走时间戳路径；普通字符串走 Date 解析', () => {
    expect(formatDateTime('1727424000')).toBe(new Date(1727424000 * 1000).toLocaleString('zh-CN'));
    expect(formatDateTime('2026-09-27T08:00:00Z')).toBe(
      new Date('2026-09-27T08:00:00Z').toLocaleString('zh-CN'),
    );
    expect(formatDateTime('not-a-date')).toBe('-');
  });
});

describe('renderHistoryDetail（#37）', () => {
  it('数组 ≤8 逐项渲染为 Tag；空数组渲染 0', () => {
    render(<>{renderHistoryDetail('k', ['a', 'b'])}</>);
    expect(screen.getByText('a')).toBeInTheDocument();
    expect(screen.getByText('b')).toBeInTheDocument();
    render(<>{renderHistoryDetail('k', [])}</>);
    expect(screen.getByText('0')).toBeInTheDocument();
  });

  it('数组 >8 收敛为 N 项计数标签', () => {
    render(
      <>
        {renderHistoryDetail(
          'k',
          Array.from({ length: 12 }, (_, i) => String(i)),
        )}
      </>,
    );
    expect(screen.getByText('12 项')).toBeInTheDocument();
  });

  it('对象渲染 JSON pre；标量原样转字符串', () => {
    render(
      <>
        {renderHistoryDetail('obj', { note: 'x', list: [1, 2] })}
        {renderHistoryDetail('num', 42)}
      </>,
    );
    expect(screen.getByText(/"note": "x"/)).toBeInTheDocument();
    expect(screen.getByText('42')).toBeInTheDocument();
  });
});

describe('renderHistoryDiff 边界（#37）', () => {
  it('details 缺失或五组全空：返回 null 不渲染', () => {
    const { container } = render(<>{renderHistoryDiff(undefined)}</>);
    expect(container).toBeEmptyDOMElement();
    const { container: c2 } = render(
      <>{renderHistoryDiff({ before: [], after: [], added: [], removed: [], unknown: [] })}</>,
    );
    expect(c2).toBeEmptyDOMElement();
  });

  it('恰好 8 项不显示 +N；第 9 项起出现余量标签', () => {
    const eight = Array.from({ length: 8 }, (_, i) => `f${i + 1}`);
    const nine = [...eight, 'f9'];
    const { container } = render(
      <>{renderHistoryDiff({ before: [], after: [], added: eight, removed: [], unknown: [] })}</>,
    );
    expect(screen.getByText('新增')).toBeInTheDocument();
    expect(screen.getByText('f8')).toBeInTheDocument();
    expect(container.textContent).not.toContain('+1');

    const { container: c2 } = render(
      <>{renderHistoryDiff({ before: [], after: [], added: nine, removed: [], unknown: [] })}</>,
    );
    expect(screen.getByText('+1')).toBeInTheDocument();
    // 第 9 项不在摘要行，完整清单在折叠面板中（对应 historyModal 用例）
    expect(c2.textContent).not.toContain('f9');
  });

  it('非数组值按空白处理：五组全无效时整个 diff 区不渲染也不崩溃（防御旧数据）', () => {
    const { container } = render(
      <>
        {renderHistoryDiff({
          before: 'not-an-array',
          after: null,
          added: 'x',
          removed: [],
          unknown: undefined,
        })}
      </>,
    );
    expect(container).toBeEmptyDOMElement();
  });
});
