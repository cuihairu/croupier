/** ErrorDetailList：统一错误体 details 的内联明细列表（保存/发布弹窗与
 * 组合页预览栏共用）。覆盖 map 形态、表单式数组形态与无 details 三种输入。 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import ErrorDetailList from '../ErrorDetailList';

describe('ErrorDetailList', () => {
  it('map 形态 details：字段路径 code + 原因逐条列出', () => {
    render(
      <ErrorDetailList
        error={{
          data: {
            error: 'validation_failed',
            message: 'composite page cannot be generated',
            details: {
              'sections[1].functionId': 'function no.such.fn contract not found',
              'sections[0].key': 'duplicate section key "k1"',
            },
          },
        }}
      />,
    );

    const items = screen.getAllByRole('listitem');
    expect(items).toHaveLength(2);
    expect(items[0].textContent).toBe(
      'sections[1].functionId: function no.such.fn contract not found',
    );
    expect(items[1].textContent).toBe('sections[0].key: duplicate section key "k1"');
    expect(items[0].querySelector('code')?.textContent).toBe('sections[1].functionId');
    expect(document.querySelectorAll('code').length).toBe(2);
  });

  it('表单式数组 details：缺 field 时只渲染 message（不渲染 code 字段）', () => {
    render(
      <ErrorDetailList error={{ data: { details: [{ field: '', message: '整体校验失败' }] } }} />,
    );

    expect(screen.getByText('整体校验失败')).toBeInTheDocument();
    expect(document.querySelectorAll('code').length).toBe(0);
  });

  it('无 details：不渲染任何内容', () => {
    const { container } = render(<ErrorDetailList error={{ data: { message: 'boom' } }} />);
    expect(container).toBeEmptyDOMElement();
  });
});
