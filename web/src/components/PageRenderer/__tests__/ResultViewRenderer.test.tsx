/** ResultViewRenderer 覆盖。
 *
 * 覆盖路径：renderJSONValueSummary 全类型分派（空值/布尔/标量/数组/对象/空对象）、
 * 组件三分支（无数据 Alert / 未声明 fields Alert / 对象 Descriptions）、
 * 非对象数据时 result 单字段直通与结构不匹配告警、title 本地化回退。 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import ResultViewRenderer, { renderJSONValueSummary } from '../ResultViewRenderer';
import type { ResultFieldSpec, ResultViewSpec } from '@/types/dashboard';

const fields = (...list: Partial<ResultFieldSpec>[]): ResultViewSpec => ({
  fields: list.map((f) => ({
    key: f.key ?? 'k',
    title: f.title,
    dataType: f.dataType ?? 'string',
  })),
});

describe('renderJSONValueSummary', () => {
  const cases: { name: string; value: unknown; text: string | RegExp }[] = [
    { name: 'undefined/null 归一为 -', value: undefined, text: '-' },
    { name: 'null 归一为 -', value: null, text: '-' },
    { name: '布尔 true 渲染为「是」', value: true, text: '是' },
    { name: '布尔 false 渲染为「否」', value: false, text: '否' },
    { name: '数字转字符串', value: 42, text: '42' },
    { name: '字符串原样', value: 'hello', text: 'hello' },
    { name: '数组渲染项数', value: [1, 2, 3], text: '数组 3 项' },
    {
      name: '对象列出前 6 个 key',
      value: { a: 1, b: 2, c: 3, d: 4, e: 5, f: 6, g: 7 },
      text: 'a, b, c, d, e, f',
    },
    { name: '空对象提示', value: {}, text: '空对象' },
  ];

  it.each(cases)('$name', ({ value, text }) => {
    const { container } = render(<>{renderJSONValueSummary(value as never)}</>);
    expect(container.textContent).toMatch(typeof text === 'string' ? new RegExp(text) : text);
    // 截断场景：第 7 个 key 不出现
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      expect(container.textContent).not.toContain('g');
    }
  });

  it('对象渲染「对象」标签', () => {
    const { container } = render(<>{renderJSONValueSummary({ x: 1 })}</>);
    expect(container.textContent).toContain('对象');
    expect(container.textContent).toContain('x');
  });
});

describe('ResultViewRenderer', () => {
  it('无数据时提示执行已完成且无结构化返回', () => {
    render(<ResultViewRenderer data={undefined} resultView={fields({ key: 'a' })} />);
    expect(screen.getByText('执行已完成，无结构化返回结果')).toBeInTheDocument();
  });

  it('null 数据同样提示无结构化返回', () => {
    render(<ResultViewRenderer data={null} />);
    expect(screen.getByText('执行已完成，无结构化返回结果')).toBeInTheDocument();
  });

  it('未声明 fields 时告警并使用默认 emptyTitle', () => {
    render(<ResultViewRenderer data={{ a: 1 }} resultView={{ fields: [] }} />);
    expect(screen.getByText('结果视图未配置')).toBeInTheDocument();
    expect(screen.getByText(/resultView\.fields 未声明展示字段/)).toBeInTheDocument();
  });

  it('resultView 缺省同样走未配置分支', () => {
    render(<ResultViewRenderer data={{ a: 1 }} />);
    expect(screen.getByText('结果视图未配置')).toBeInTheDocument();
  });

  it('自定义 emptyTitle 透传', () => {
    render(<ResultViewRenderer data={{ a: 1 }} resultView={{}} emptyTitle="自定义空标题" />);
    expect(screen.getByText('自定义空标题')).toBeInTheDocument();
  });

  it('非对象数据 + 单字段 key=result：直通渲染摘要', () => {
    render(
      <ResultViewRenderer
        data={[1, 2]}
        resultView={fields({ key: 'result', title: { 'zh-CN': '执行结果' } })}
      />,
    );
    expect(screen.getByText('执行结果')).toBeInTheDocument();
    expect(screen.getByText('数组 2 项')).toBeInTheDocument();
  });

  it('非对象数据 + 单字段无 title：label 回退为「结果」', () => {
    render(<ResultViewRenderer data="raw" resultView={fields({ key: 'result' })} />);
    expect(screen.getByText('结果')).toBeInTheDocument();
    expect(screen.getByText('raw')).toBeInTheDocument();
  });

  it('非对象数据 + 其他字段组合：告警结构不匹配', () => {
    render(<ResultViewRenderer data={[1]} resultView={fields({ key: 'a' }, { key: 'result' })} />);
    expect(screen.getByText('结果结构与 ResultViewSpec 不匹配')).toBeInTheDocument();
  });

  it('对象数据按 fields 逐项渲染，缺省 title 回退 field.key', () => {
    render(
      <ResultViewRenderer
        data={{ ok: true, count: 3, extra: 'x' }}
        resultView={fields(
          { key: 'ok', title: { 'zh-CN': '是否成功' } },
          { key: 'count', title: { 'zh-CN': '数量' } },
          { key: 'extra' },
        )}
      />,
    );
    expect(screen.getByText('是否成功')).toBeInTheDocument();
    expect(screen.getByText('是')).toBeInTheDocument();
    expect(screen.getByText('数量')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    // 缺省 title → localizedText 回退 key
    expect(screen.getByText('extra')).toBeInTheDocument();
    expect(screen.getByText('x')).toBeInTheDocument();
  });
});
