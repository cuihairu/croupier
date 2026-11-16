/** resourceFieldSemantic 覆盖。
 *
 * 详情抽屉复用列表列语义：enum+tag → 彩色本地化 Tag、（非 tag）枚举 → 文案、
 * copy → 可复制、boolean → 是/否 Tag、number+unit → 单位后缀、datetime/date →
 * 带时区标签；详情项显式声明优先、未声明回落列表列；无匹配返回 undefined 由
 * 默认渲染兜底；空值统一 '-'。
 *
 * GUX-005/006/007：未声明枚举值显式标注「未知（原值）」（tag 与非 tag 两种
 * 渲染）、无效时间显式标注、枚举文案与单位跟随当前 locale（不固定 zh-CN）。
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { App } from 'antd';
import { renderFieldSemantic, mergeFieldSemantic } from '../resourceFieldSemantic';
import { formatDateTime } from '@/utils/format';

jest.mock('@/utils/format', () => ({
  ...jest.requireActual('@/utils/format'),
  formatDateTime: jest.fn(() => '2026/01/01 10:00:00 (UTC+08:00)'),
}));
void formatDateTime;

let mockLocale = 'zh-CN';
jest.mock('@umijs/max', () => ({
  getIntl: () => ({
    locale: mockLocale,
    formatMessage: (
      { defaultMessage }: { defaultMessage: string },
      values?: Record<string, unknown>,
    ) => defaultMessage.replace(/\{(\w+)\}/g, (_, k: string) => String(values?.[k] ?? '')),
  }),
}));

const renderNode = (node: React.ReactNode) => render(<App>{node}</App>);

describe('renderFieldSemantic', () => {
  it('enum + render:tag 渲染彩色本地化 Tag', () => {
    renderNode(
      renderFieldSemantic(
        {
          dataType: 'enum',
          render: 'tag',
          enum: [{ value: 'active', label: { 'zh-CN': '正常' }, color: 'green' }],
        },
        'active',
      ),
    );
    const tag = document.querySelector('.ant-tag');
    expect(tag?.textContent).toBe('正常');
  });

  it('enum（非 tag）渲染本地化文案', () => {
    renderNode(
      renderFieldSemantic(
        { dataType: 'enum', enum: [{ value: 'a', label: { 'zh-CN': '甲' } }] },
        'a',
      ),
    );
    expect(screen.getByText('甲')).toBeInTheDocument();
  });

  it('GUX-005: 未声明枚举值 tag 渲染显式未知标注（保留原值、非仅颜色）', () => {
    renderNode(
      renderFieldSemantic(
        { dataType: 'enum', render: 'tag', enum: [{ value: 'a', label: { 'zh-CN': '甲' } }] },
        'weird',
      ),
    );
    const tag = document.querySelector('.ant-tag');
    expect(tag?.textContent).toBe('未知（weird）');
  });

  it('GUX-005: 未声明枚举值非 tag 渲染警告文案', () => {
    renderNode(
      renderFieldSemantic(
        { dataType: 'enum', enum: [{ value: 'a', label: { 'zh-CN': '甲' } }] },
        'weird',
      ),
    );
    expect(screen.getByText('未知（weird）')).toBeInTheDocument();
  });

  it('GUX-007: 枚举文案跟随 locale（en-US 优先取 en-US 标签）', () => {
    mockLocale = 'en-US';
    try {
      renderNode(
        renderFieldSemantic(
          {
            dataType: 'enum',
            render: 'tag',
            enum: [
              { value: 'active', label: { 'zh-CN': '正常', 'en-US': 'Active' }, color: 'green' },
            ],
          },
          'active',
        ),
      );
      expect(document.querySelector('.ant-tag')?.textContent).toBe('Active');
    } finally {
      mockLocale = 'zh-CN';
    }
  });

  it('render:copy 渲染可复制文本', () => {
    const { container } = render(
      <App>{renderFieldSemantic({ dataType: 'string', render: 'copy' }, 'p1')}</App>,
    );
    expect(container.querySelector('.ant-typography-copy')).toBeTruthy();
  });

  it('boolean 渲染是/否 Tag', () => {
    renderNode(renderFieldSemantic({ dataType: 'boolean' }, true));
    expect(document.querySelector('.ant-tag')?.textContent).toBe('是');
  });

  it('number + unit 渲染单位后缀（zh-CN）', () => {
    renderNode(renderFieldSemantic({ dataType: 'number', unit: { 'zh-CN': '金' } }, 100));
    expect(screen.getByText('100 金')).toBeInTheDocument();
  });

  it('GUX-007: number + unit 单位跟随 locale（en-US 取 en-US 单位）', () => {
    mockLocale = 'en-US';
    try {
      renderNode(
        renderFieldSemantic({ dataType: 'number', unit: { 'zh-CN': '金', 'en-US': 'gold' } }, 100),
      );
      expect(screen.getByText('100 gold')).toBeInTheDocument();
    } finally {
      mockLocale = 'zh-CN';
    }
  });

  it('datetime 带时区标签（列表与详情同构）', () => {
    renderNode(renderFieldSemantic({ dataType: 'datetime' }, '2026-01-01T10:00:00Z'));
    expect(screen.getByText('2026/01/01 10:00:00 (UTC+08:00)')).toBeInTheDocument();
  });

  it('GUX-006: 无效时间显式标注，不输出 Invalid Date 拼时区', () => {
    renderNode(renderFieldSemantic({ dataType: 'datetime' }, 'not-a-date'));
    expect(screen.getByText('无效时间（not-a-date）')).toBeInTheDocument();
  });

  it('空值统一 -, 无匹配返回 undefined（默认渲染兜底）', () => {
    expect(renderFieldSemantic({ dataType: 'string' }, null)).toBe('-');
    expect(renderFieldSemantic({ dataType: 'string', render: 'text' }, 'x')).toBeUndefined();
  });
});

describe('formatDateTime 无效值守卫（GUX-006）', () => {
  it('无效输入原样返回，不产生 Invalid Date 拼接', () => {
    const guarded = jest.requireActual('@/utils/format') as typeof import('@/utils/format');
    expect(guarded.formatDateTime('not-a-date', true)).toBe('not-a-date');
    expect(guarded.formatDateTime('', true)).toBe('-');
  });
});

describe('mergeFieldSemantic', () => {
  const LIST = {
    dataType: 'enum' as const,
    render: 'tag',
    enum: [{ value: 'active', label: { 'zh-CN': '正常' } }],
  };
  const DETAIL = { key: 'status', dataType: 'string' as const };

  it('无列表列命中：只返回详情项', () => {
    expect(mergeFieldSemantic(undefined, DETAIL)).toEqual(DETAIL);
  });

  it('详情项未声明回落列表列语义，显式详情声明优先', () => {
    // 详情项 dataType 显式 string 优先；未声明的 enum/render 从列表列回落。
    const merged = mergeFieldSemantic(LIST, DETAIL);
    expect(merged.key).toBe('status');
    expect(merged.dataType).toBe('string');
    expect(merged.render).toBe('tag');
    expect(merged.enum).toEqual(LIST.enum);
    const overridden = mergeFieldSemantic(LIST, {
      key: 'status',
      dataType: 'enum',
      render: 'text',
      enum: [{ value: 'active', label: { 'zh-CN': '自定义' } }],
    });
    expect(overridden.render).toBe('text');
    expect(overridden.enum?.[0].label['zh-CN']).toBe('自定义');
  });
});
