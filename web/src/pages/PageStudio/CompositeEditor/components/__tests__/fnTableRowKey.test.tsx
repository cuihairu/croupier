/** FnTable.Preview 的 Table rowKey 直测：
 * rowKey 用行索引字符串（`(_, i) => String(i)`），但 Preview 不传 dataSource，
 * 生产路径 rc-table 从不调用它——这里 mock antd Table 捕获 props 后直接调用，
 * 断言索引 → 字符串映射（补 FN 覆盖缺口）。 */
import React from 'react';
import { render } from '@testing-library/react';
import { fnTable } from '../FnTable';
import type { PageNode } from '../../model';

jest.mock('antd', () => {
  const R = require('react') as typeof React;
  const actual = jest.requireActual('antd');
  let captured: Record<string, unknown> | null = null;
  const Table = (props: Record<string, unknown>) => {
    captured = props;
    return R.createElement('div', { 'data-testid': 'table-stub' });
  };
  return { ...actual, Table, __tableProps: () => captured };
});

const antdMock = jest.requireMock('antd') as {
  __tableProps: () => Record<string, unknown> | null;
};

describe('FnTable.Preview rowKey（行索引键映射）', () => {
  it('rowKey 把行索引映射为字符串（String(i)）', () => {
    const node: PageNode = { id: 't1', type: 'fnTable', props: { columns: ['id'] } };
    const { getByTestId } = render(<fnTable.Preview node={node} />);
    expect(getByTestId('table-stub')).toBeInTheDocument();
    const rowKey = antdMock.__tableProps()?.rowKey as
      ((record: unknown, index?: number) => string) | undefined;
    expect(rowKey).toBeInstanceOf(Function);
    expect(rowKey?.(undefined, 0)).toBe('0');
    expect(rowKey?.(undefined, 3)).toBe('3');
    expect(rowKey?.(undefined, 12)).toBe('12');
  });
});
