/**
 * MarkdownText 受控子集回归：
 * - 块级：三级标题 / 无序有序列表归组 / 空行分段 / 段内换行保留
 * - 行内：粗体 斜体（先匹配 **）/ 行内代码 / 链接协议白名单
 * - 安全：raw HTML 按纯文本展示（React 转义）、javascript: 链接不渲染、
 *   未闭合标记不吞内容、斜体不吞粗体星号
 * - 空 source 不渲染
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { MarkdownText, parseBlocks, parseInline } from '../index';

describe('parseInline', () => {
  it('纯文本单节点', () => {
    expect(parseInline('hello 世界')).toEqual([{ kind: 'text', value: 'hello 世界' }]);
  });

  it('粗体成对解析（先于斜体）', () => {
    expect(parseInline('a **b** c')).toEqual([
      { kind: 'text', value: 'a ' },
      { kind: 'bold', children: [{ kind: 'text', value: 'b' }] },
      { kind: 'text', value: ' c' },
    ]);
  });

  it('斜体不成对星号按字面保留', () => {
    expect(parseInline('a *b c')).toEqual([{ kind: 'text', value: 'a *b c' }]);
    expect(parseInline('**未闭合')).toEqual([{ kind: 'text', value: '**未闭合' }]);
  });

  it('斜体边界不吞相邻粗体', () => {
    expect(parseInline('*a* **b**')).toEqual([
      { kind: 'italic', children: [{ kind: 'text', value: 'a' }] },
      { kind: 'text', value: ' ' },
      { kind: 'bold', children: [{ kind: 'text', value: 'b' }] },
    ]);
  });

  it('行内代码成对，内部语法不再解析', () => {
    expect(parseInline('run `a * b` now')).toEqual([
      { kind: 'text', value: 'run ' },
      { kind: 'code', value: 'a * b' },
      { kind: 'text', value: ' now' },
    ]);
  });

  it('链接：http/https 接受，其余协议与空 label 按字面', () => {
    expect(parseInline('[文档](https://example.com/x)')).toEqual([
      { kind: 'link', label: '文档', url: 'https://example.com/x' },
    ]);
    expect(parseInline('[x](javascript:alert(1))')).toEqual([
      { kind: 'text', value: '[x](javascript:alert(1))' },
    ]);
    expect(parseInline('[](https://a.b)')).toEqual([{ kind: 'text', value: '[](https://a.b)' }]);
  });

  it('嵌套：粗体内可再有斜体', () => {
    expect(parseInline('**a *b* c**')).toEqual([
      {
        kind: 'bold',
        children: [
          { kind: 'text', value: 'a ' },
          { kind: 'italic', children: [{ kind: 'text', value: 'b' }] },
          { kind: 'text', value: ' c' },
        ],
      },
    ]);
  });
});

describe('parseBlocks', () => {
  it('三级标题与正文段落', () => {
    const blocks = parseBlocks('# 大标题\n\n## 二级\n\n### 三级\n\n正文');
    expect(blocks.map((b) => b.kind)).toEqual(['heading', 'heading', 'heading', 'paragraph']);
    expect((blocks[0] as { level: number }).level).toBe(1);
    expect((blocks[3] as { children: unknown[] }).children).toEqual([
      { kind: 'text', value: '正文' },
    ]);
  });

  it('无序列表连续行归组，空行断开', () => {
    const blocks = parseBlocks('- a\n- b\n\n- c');
    expect(blocks.map((b) => b.kind)).toEqual(['list', 'list']);
    const first = blocks[0] as { items: unknown[][] };
    expect(first.items).toHaveLength(2);
  });

  it('有序/无序不混组', () => {
    const blocks = parseBlocks('- a\n1. b\n2. c');
    const kinds = blocks.map((b) => b.kind);
    expect(kinds).toEqual(['list', 'list']);
    expect((blocks[0] as { ordered: boolean }).ordered).toBe(false);
    expect((blocks[1] as { ordered: boolean }).ordered).toBe(true);
  });

  it('正文中段换行保留同一段落', () => {
    const blocks = parseBlocks('第一行\n第二行');
    expect(blocks).toHaveLength(1);
    expect(blocks[0].children).toEqual([{ kind: 'text', value: '第一行\n第二行' }]);
  });

  it('正文行首井号多于三级不算标题', () => {
    const blocks = parseBlocks('#### 四个');
    expect(blocks[0].kind).toBe('paragraph');
  });

  it('CRLF 归一化', () => {
    expect(parseBlocks('a\r\n\r\nb').map((b) => b.kind)).toEqual(['paragraph', 'paragraph']);
  });
});

describe('MarkdownText 渲染', () => {
  it('空 source 不渲染', () => {
    const { container } = render(<MarkdownText source="" />);
    expect(container.querySelector('[data-testid="markdown-text"]')).toBeNull();
  });

  it('标题/粗体/列表/行内代码完整渲染', () => {
    render(<MarkdownText source={'# 维护公告\n公告正文 **重要**\n- 项一\n- 项二'} />);
    expect(screen.getByText('维护公告')).toBeInTheDocument();
    const strong = screen.getByText('重要');
    expect(strong.tagName).toBe('STRONG');
    // 段落级文本聚合（正文+粗体同段落），断言段整体而非中间文本节点
    expect(document.querySelector('[data-testid="markdown-text"]')?.textContent).toContain(
      '公告正文',
    );
    expect(screen.getByText('项一')).toBeInTheDocument();
    expect(screen.getByText('项二')).toBeInTheDocument();
  });

  it('raw HTML 按纯文本展示（无注入面）', () => {
    render(<MarkdownText source={'<script>alert(1)</script>\n<img src=x onerror=alert(2)>'} />);
    // 原文按纯文本展示，且页面无 script/img 真实元素
    const body = document.querySelector('[data-testid="markdown-text"]')?.textContent ?? '';
    expect(body).toContain('<script>alert(1)</script>');
    expect(body).toContain('<img src=x onerror=alert(2)>');
    expect(document.querySelector('script')).toBeNull();
    expect(document.querySelector('img')).toBeNull();
  });

  it('链接白名单渲染：合法链接新窗口 + rel 安全；危险协议不渲染', () => {
    render(
      <MarkdownText
        source={'[外链](https://example.com/op)\n[x](javascript:alert(1))\n[x](data:text/html,hi)'}
      />,
    );
    const link = screen.getByText('外链');
    const a = link.closest('a');
    expect(a).not.toBeNull();
    expect(a?.getAttribute('href')).toBe('https://example.com/op');
    expect(a?.getAttribute('target')).toBe('_blank');
    expect(a?.getAttribute('rel')).toContain('noopener');
    // 危险协议的写法整体按文本展示（与相邻行同段落，断言容器聚合文本）
    const container = document.querySelector('[data-testid="markdown-text"]')?.textContent ?? '';
    expect(container).toContain('[x](javascript:alert(1))');
    expect(container).toContain('[x](data:text/html,hi)');
    expect(document.querySelectorAll('a')).toHaveLength(1);
  });

  it('序号列表按书写值展示', () => {
    render(<MarkdownText source={'1. 首\n2. 次'} />);
    const items = document.querySelectorAll('li');
    expect(items).toHaveLength(2);
    expect(items[0].textContent).toBe('首');
    expect(items[1].textContent).toBe('次');
  });
});
