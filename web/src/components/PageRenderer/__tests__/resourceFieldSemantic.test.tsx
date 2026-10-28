/** resourceFieldSemantic 覆盖。
 *
 * 详情抽屉复用列表列语义：enum+tag → 彩色本地化 Tag、（非 tag）枚举 → 文案、
 * copy → 可复制、boolean → 是/否 Tag、number+unit → 单位后缀、datetime/date →
 * 带时区标签；详情项显式声明优先、未声明回落列表列；无匹配返回 undefined 由
 * 默认渲染兜底；空值统一 '-'。
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
jest.mock('@umijs/max', () => ({
  getIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
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

  it('enum（非 tag）渲染本地化文案，未知值回退原值', () => {
    renderNode(
      renderFieldSemantic(
        { dataType: 'enum', enum: [{ value: 'a', label: { 'zh-CN': '甲' } }] },
        'a',
      ),
    );
    expect(screen.getByText('甲')).toBeInTheDocument();
    renderNode(
      renderFieldSemantic(
        { dataType: 'enum', enum: [{ value: 'a', label: { 'zh-CN': '甲' } }] },
        'unknown',
      ),
    );
    expect(screen.getAllByText('unknown').length).toBeGreaterThan(0);
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

  it('number + unit 渲染单位后缀', () => {
    renderNode(renderFieldSemantic({ dataType: 'number', unit: { 'zh-CN': '金' } }, 100));
    expect(screen.getByText('100 金')).toBeInTheDocument();
  });

  it('datetime 带时区标签（列表与详情同构）', () => {
    renderNode(renderFieldSemantic({ dataType: 'datetime' }, '2026-01-01T10:00:00Z'));
    expect(screen.getByText('2026/01/01 10:00:00 (UTC+08:00)')).toBeInTheDocument();
  });

  it('空值统一 -, 无匹配返回 undefined（默认渲染兜底）', () => {
    expect(renderFieldSemantic({ dataType: 'string' }, null)).toBe('-');
    expect(renderFieldSemantic({ dataType: 'string', render: 'text' }, 'x')).toBeUndefined();
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
