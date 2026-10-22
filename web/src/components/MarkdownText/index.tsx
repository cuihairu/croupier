import React from 'react';
import { Typography } from 'antd';

/**
 * MarkdownText —— 受控 Markdown 子集渲染器（零依赖、零 HTML 注入面）。
 *
 * 背景（#50 系统公告核对）：公告 contentMd 承诺 Markdown，但消费端此前只做
 * pre-wrap 纯文本展示，`**加粗**` 等语法原样露出。引入 react-markdown 需把
 * unified/remark 全 ESM 生态逐包加进 jest transformIgnorePatterns（脆），故
 * 自写受控子集：React 元素直出（天然转义），不支持 raw HTML（按纯文本展示，
 * 无 XSS 面）。
 *
 * 支持的语法（块级）：
 * - `# / ## / ###` 标题（弹窗语境缩放为 Title level 4/5 与加粗段）
 * - `- ` 或 `* ` 开头的无序列表（连续行归组）
 * - `1. ` 形式的有序列表（连续行归组，序号按书写值展示）
 * - 空行分段；段内换行保留
 * 支持的行内语法：
 * - `**粗体**`、`*斜体*`（先匹配 ** 防止吞并）、`` `行内代码` ``
 * - `[文案](http(s)://…)` 链接——仅 http/https，其余按纯文本；新窗口 +
 *   rel="noopener noreferrer"
 * 不支持（按字面展示）：嵌套结构、代码块、转义序列、raw HTML、表格、图片。
 */

const { Text, Title, Paragraph } = Typography;

/** 行内节点：React key 由渲染层按下标派生（解析结果无状态） */
export type InlineNode =
  | { kind: 'text'; value: string }
  | { kind: 'bold'; children: InlineNode[] }
  | { kind: 'italic'; children: InlineNode[] }
  | { kind: 'code'; value: string }
  | { kind: 'link'; label: string; url: string };

export type Block =
  | { kind: 'heading'; level: 1 | 2 | 3; children: InlineNode[] }
  | { kind: 'paragraph'; children: InlineNode[] }
  | { kind: 'list'; ordered: boolean; items: InlineNode[][] };

const LINK_PROTOCOL = /^https?:\/\//i;

/** 解析 [label](url)，url 仅接受 http/https（防 javascript: 等注入） */
function matchLink(
  text: string,
  start: number,
): { label: string; url: string; end: number } | null {
  if (text[start] !== '[') return null;
  const closeBracket = text.indexOf('](', start + 1);
  if (closeBracket < 0) return null;
  const openParen = closeBracket + 1;
  const closeParen = text.indexOf(')', openParen + 1);
  if (closeParen < 0) return null;
  const url = text.slice(openParen + 1, closeParen).trim();
  const label = text.slice(start + 1, closeBracket);
  if (!label || !url || !LINK_PROTOCOL.test(url)) return null;
  return { label, url, end: closeParen + 1 };
}

/** 行内解析：粗体/斜体/行内代码/链接；其余字符累积为 text 节点 */
export function parseInline(text: string): InlineNode[] {
  const nodes: InlineNode[] = [];
  let plain = '';
  const flush = () => {
    if (plain) {
      nodes.push({ kind: 'text', value: plain });
      plain = '';
    }
  };
  let i = 0;
  while (i < text.length) {
    // 行内代码：反引号成对（内部不再解析其他语法）
    if (text[i] === '`') {
      const close = text.indexOf('`', i + 1);
      if (close > i + 1) {
        flush();
        nodes.push({ kind: 'code', value: text.slice(i + 1, close) });
        i = close + 1;
        continue;
      }
    }
    // 链接优先于粗斜体判定（[ 与 * 无冲突，但位置判断需先于通配匹配）
    if (text[i] === '[') {
      const link = matchLink(text, i);
      if (link) {
        flush();
        nodes.push({ kind: 'link', label: link.label, url: link.url });
        i = link.end;
        continue;
      }
    }
    if (text.startsWith('**', i)) {
      const close = text.indexOf('**', i + 2);
      if (close > i + 2) {
        flush();
        nodes.push({ kind: 'bold', children: parseInline(text.slice(i + 2, close)) });
        i = close + 2;
        continue;
      }
    }
    if (text[i] === '*') {
      const close = text.indexOf('*', i + 1);
      if (close > i + 1) {
        flush();
        nodes.push({ kind: 'italic', children: parseInline(text.slice(i + 1, close)) });
        i = close + 1;
        continue;
      }
    }
    plain += text[i];
    i += 1;
  }
  flush();
  return nodes;
}

const HEADING_RE = /^(#{1,3})\s+(.*)$/;
const UL_RE = /^[-*]\s+(.*)$/;
const OL_RE = /^(\d+)[.、]\s+(.*)$/;

/** 块级解析：标题/列表（连续行归组）/空行分段；段内换行保留为独立行 */
export function parseBlocks(source: string): Block[] {
  const blocks: Block[] = [];
  const lines = source.replace(/\r\n?/g, '\n').split('\n');
  let paragraph: string[] = [];
  let list: { ordered: boolean; items: InlineNode[][] } | null = null;

  const flushParagraph = () => {
    if (paragraph.length) {
      blocks.push({ kind: 'paragraph', children: parseInline(paragraph.join('\n')) });
      paragraph = [];
    }
  };
  const flushList = () => {
    if (list) {
      blocks.push({ kind: 'list', ordered: list.ordered, items: list.items });
      list = null;
    }
  };

  for (const raw of lines) {
    const line = raw.trimEnd();
    const heading = HEADING_RE.exec(line);
    if (heading) {
      flushParagraph();
      flushList();
      const level = heading[1].length as 1 | 2 | 3;
      blocks.push({ kind: 'heading', level, children: parseInline(heading[2].trim()) });
      continue;
    }
    if (!line.trim()) {
      flushParagraph();
      flushList();
      continue;
    }
    const ul = UL_RE.exec(line);
    if (ul) {
      flushParagraph();
      if (!list || list.ordered) {
        flushList();
        list = { ordered: false, items: [] };
      }
      list.items.push(parseInline(ul[1].trim()));
      continue;
    }
    const ol = OL_RE.exec(line);
    if (ol) {
      flushParagraph();
      if (!list || !list.ordered) {
        flushList();
        list = { ordered: true, items: [] };
      }
      list.items.push(parseInline(ol[2].trim()));
      continue;
    }
    // 普通文本行：终结未闭合列表，进入段落
    flushList();
    paragraph.push(line);
  }
  flushParagraph();
  flushList();
  return blocks;
}

/** 行内节点 → React 元素（key 按位置派生；文本经 React 转义，无注入面） */
export function renderInline(nodes: InlineNode[]): React.ReactNode[] {
  return nodes.map((node, idx) => {
    switch (node.kind) {
      case 'bold':
        return <strong key={idx}>{renderInline(node.children)}</strong>;
      case 'italic':
        return <em key={idx}>{renderInline(node.children)}</em>;
      case 'code':
        return (
          <Text key={idx} code>
            {node.value}
          </Text>
        );
      case 'link':
        // 原生 a：antd Text link 形态在不同版本渲染差异大，直接可控
        return (
          <a
            key={idx}
            href={node.url}
            target="_blank"
            rel="noopener noreferrer"
            style={{ color: 'var(--brand-2)' }}
          >
            {node.label}
          </a>
        );
      default:
        return <React.Fragment key={idx}>{node.value}</React.Fragment>;
    }
  });
}

export type MarkdownTextProps = {
  source: string;
  /** 容器 className（透传给最外层 div） */
  className?: string;
};

/** 渲染入口：块级结构 → Typography 元素；source 为空时不渲染 */
export function MarkdownText({ source, className }: MarkdownTextProps) {
  if (!source || !source.trim()) return null;
  const blocks = parseBlocks(source);
  return (
    <div className={className} data-testid="markdown-text">
      {blocks.map((block, idx) => {
        switch (block.kind) {
          case 'heading':
            // 弹窗语境缩放：# → Title 4，## → Title 5，### → 加粗文本
            if (block.level === 1) {
              return (
                <Title key={idx} level={4} style={{ marginTop: 0 }}>
                  {renderInline(block.children)}
                </Title>
              );
            }
            if (block.level === 2) {
              return (
                <Title key={idx} level={5} style={{ marginTop: 0 }}>
                  {renderInline(block.children)}
                </Title>
              );
            }
            return (
              <Paragraph key={idx} style={{ marginBottom: 4 }}>
                <Text strong>{renderInline(block.children)}</Text>
              </Paragraph>
            );
          case 'list': {
            const ListTag = block.ordered ? 'ol' : 'ul';
            return (
              <ListTag key={idx} style={{ margin: '0 0 8px', paddingLeft: 20 }}>
                {block.items.map((item, itemIdx) => (
                  <li key={itemIdx}>{renderInline(item)}</li>
                ))}
              </ListTag>
            );
          }
          default:
            return (
              <Paragraph key={idx} style={{ whiteSpace: 'pre-wrap', marginBottom: 8 }}>
                {renderInline(block.children)}
              </Paragraph>
            );
        }
      })}
    </div>
  );
}

export default MarkdownText;
