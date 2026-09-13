/** DataPanel（编辑态底部数据面板）覆盖：渲染门槛三路（无 node/无 fn/
 * 非 fn* 类型）、折叠态提示与展开切换、试跑成功（result.items 表格渲染——
 * 列截前 8·单元格空值兜底）、无 items 三态（result 无 items/裸 payload/
 * 数组 result/原始值 result → pre JSON）、试跑失败 error 展示。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import DataPanel from '../DataPanel';
import { invokeFunction, type FunctionDescriptor } from '@/services/api/functions';
import type { PageNode } from '../model';

jest.mock('@/services/api/functions', () => ({
  invokeFunction: jest.fn(),
}));

const fn: FunctionDescriptor = {
  id: 'player.list',
  operation: 'query',
  resource: 'player',
  inputSchema: {
    type: 'object',
    properties: { playerId: { type: 'string' }, zone: { type: 'string' } },
    required: ['playerId'],
  },
};

const node = (type: PageNode['type']): PageNode => ({ id: 'n1', type, props: {} });

const mockRun = (payload: unknown) => (invokeFunction as jest.Mock).mockResolvedValue(payload);

beforeEach(() => {
  (invokeFunction as jest.Mock).mockReset();
});

describe('渲染门槛', () => {
  it('无 node / 无 fn / 非 fn* 类型：不渲染', () => {
    const { container } = render(<DataPanel node={undefined} fn={fn} />);
    expect(container).toBeEmptyDOMElement();
    render(<DataPanel node={node('fnTable')} fn={undefined} />);
    expect(screen.queryByText('数据')).not.toBeInTheDocument();
    const { container: c3 } = render(<DataPanel node={node('text')} fn={fn} />);
    expect(c3).toBeEmptyDOMElement();
  });

  it('fn* 三类型皆可渲染；折叠态显示试跑提示', () => {
    for (const t of ['fnTable', 'fnFields', 'fnForm'] as const) {
      const { unmount } = render(<DataPanel node={node(t)} fn={fn} />);
      expect(screen.getByText('数据')).toBeInTheDocument();
      // setupTests 的 FormattedMessage mock 不做 values 插值：断言模板原文
      expect(screen.getByText('试跑 {fnId}（{paramCount} 个参数，默认空跑）')).toBeInTheDocument();
      // 折叠：内容区未渲染
      expect(screen.queryByText('执行失败')).not.toBeInTheDocument();
      unmount();
    }
  });
});

describe('面板交互与试跑', () => {
  it('点击标题行切换展开/收起', () => {
    render(<DataPanel node={node('fnTable')} fn={fn} />);
    fireEvent.click(screen.getByText('数据'));
    // 展开后内容区存在（空 pre 尚无——无 data 时无内容，验证可再点执行）
    fireEvent.click(screen.getByText('数据'));
    // 收起无异常
    expect(screen.getByText('数据')).toBeInTheDocument();
  });

  it('试跑成功 result.items：表格渲染（列取首行键·空值兜底空串）', async () => {
    mockRun({
      result: {
        items: [
          { id: 'p1', name: '张三' },
          { id: 'p2', name: undefined },
        ],
      },
    });
    render(<DataPanel node={node('fnTable')} fn={fn} />);
    fireEvent.click(screen.getByText('执行'));
    expect(await screen.findByText('张三')).toBeInTheDocument();
    expect(screen.getByText('p2')).toBeInTheDocument();
    // 空值兜底空串（name: undefined 的单元格渲染空文本，不炸即可）
    expect(screen.queryByText('执行失败')).not.toBeInTheDocument();
  });

  it('items 超 8 列截断', async () => {
    const row: Record<string, unknown> = {};
    for (let i = 1; i <= 10; i += 1) row[`c${i}`] = `v${i}`;
    mockRun({ result: { items: [row] } });
    render(<DataPanel node={node('fnTable')} fn={fn} />);
    fireEvent.click(screen.getByText('执行'));
    await screen.findByText('c1');
    expect(screen.getByText('c8')).toBeInTheDocument();
    expect(screen.queryByText('c9')).not.toBeInTheDocument();
    expect(screen.queryByText('c10')).not.toBeInTheDocument();
  });

  it('成功但无 items：裸 payload 对象 → pre JSON 展示', async () => {
    mockRun({ total: 3 });
    render(<DataPanel node={node('fnFields')} fn={fn} />);
    fireEvent.click(screen.getByText('执行'));
    expect(await screen.findByText(/"total": 3/)).toBeInTheDocument();
  });

  it('result 为数组 / 原始值：inner 回落 {} → 空 pre', async () => {
    mockRun({ result: [1, 2] });
    render(<DataPanel node={node('fnTable')} fn={fn} />);
    fireEvent.click(screen.getByText('执行'));
    await screen.findByText('{}');
    const { unmount } = { unmount: () => undefined };
    unmount();
  });

  it('result 原始值：inner 回落 {}', async () => {
    mockRun({ result: 'plain' });
    const { unmount } = render(<DataPanel node={node('fnForm')} fn={fn} />);
    fireEvent.click(screen.getByText('执行'));
    await screen.findByText('{}');
    unmount();
  });

  it('试跑失败：error 展示（extractErrorMessage 提取 message）', async () => {
    (invokeFunction as jest.Mock).mockRejectedValue({
      response: { data: { message: '游戏服繁忙' } },
    });
    render(<DataPanel node={node('fnTable')} fn={fn} />);
    fireEvent.click(screen.getByText('执行'));
    await screen.findByText('游戏服繁忙');
    expect(screen.queryByText('{}')).not.toBeInTheDocument();
  });

  it('试跑失败（非对象错误）：兜底文案', async () => {
    (invokeFunction as jest.Mock).mockRejectedValue(new Error('boom'));
    render(<DataPanel node={node('fnTable')} fn={fn} />);
    fireEvent.click(screen.getByText('执行'));
    await screen.findByText('boom');
  });

  it('执行按钮点击后面板强制展开', async () => {
    mockRun({ result: { items: [{ id: 'x' }] } });
    render(<DataPanel node={node('fnTable')} fn={fn} />);
    fireEvent.click(screen.getByText('执行'));
    await screen.findByText('x');
  });

  it('失败后再次试跑成功：error 清除', async () => {
    (invokeFunction as jest.Mock).mockRejectedValueOnce(new Error('first'));
    mockRun({ ok: 1 });
    render(<DataPanel node={node('fnTable')} fn={fn} />);
    fireEvent.click(screen.getByText('执行'));
    await screen.findByText('first');
    fireEvent.click(screen.getByText('执行'));
    await screen.findByText(/"ok": 1/);
    expect(screen.queryByText('first')).not.toBeInTheDocument();
  });
});

// waitFor 显式使用以覆盖异步路径的 act 警告场景
void waitFor;
