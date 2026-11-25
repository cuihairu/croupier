/**
 * SUI-028 验收：协议 12 栏宽度与 antd 24 栏换算明确，宽屏多列、窄屏可读
 * 可操作；分组与长字段在响应式栅格下仍各自成形。
 *
 * 真实渲染链：SchemaFormRenderer + 自定义 ObjectFieldTemplate，断言
 * width×2 落地、xs/md/lg 断点跨度、历史 width=0/越界按未设置处理、
 * 分组容器与长文本字段在窄屏/宽屏都保持可读（整行）。
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import { responsiveSpanFor } from '@/components/SchemaFormRenderer/templates';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

function schemaOf(properties: Record<string, unknown>): JSONSchema {
  return { type: 'object', properties } as unknown as JSONSchema;
}

/** rjsf antd 的 FieldTemplate 自带一层 Form.Item > Col，模板栅格 Col 在其
 * 外层；用 `.ant-row > .ant-col` 定位模板列（内层 Form.Item 的 Col 是
 * `ant-col-24` 固定标签包裹列）。 */
function colOf(container: HTMLElement, name: string): HTMLElement {
  const input = screen.getByLabelText(name);
  const cols = Array.from(container.querySelectorAll<HTMLElement>('.ant-col'));
  const col = cols.find(
    (candidate) =>
      candidate.querySelector(`#${CSS.escape(input.id)}`) !== null &&
      !candidate.classList.contains('ant-form-item-control'),
  );
  expect(col).not.toBeNull();
  return col as HTMLElement;
}

describe('SchemaFormRenderer responsive width (SUI-028)', () => {
  test('12-column protocol width maps to 24-column antd span and breakpoints', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        quarter: { type: 'string', title: '四分之一' },
        third: { type: 'string', title: '三分之一' },
      }),
      fields: [
        { key: 'quarter', width: 3 },
        { key: 'third', width: 4 },
      ],
    };
    const { container } = render(<SchemaFormRenderer spec={spec} hideSubmit />);
    const quarter = colOf(container, '四分之一');
    const third = colOf(container, '三分之一');
    // 协议 12 栏 ×2：3 → 6 栏（1/4 行），4 → 8 栏（1/3 行）
    expect(quarter.className).toContain('ant-col-6');
    expect(third.className).toContain('ant-col-8');
    // xs 窄屏独占整行；md 半行起；lg 完整跨度（宽屏多列）
    for (const col of [quarter, third]) {
      expect(col.className).toContain('ant-col-xs-24');
      expect(col.className).toContain('ant-col-md-12');
    }
    expect(quarter.className).toContain('ant-col-lg-6');
    expect(third.className).toContain('ant-col-lg-8');
  });

  test('breakpoint helper: full-width fields stay full at every size', () => {
    expect(responsiveSpanFor(24)).toEqual({ xs: 24, md: 24, lg: 24 });
    expect(responsiveSpanFor(12)).toEqual({ xs: 24, md: 12, lg: 12 });
    expect(responsiveSpanFor(8)).toEqual({ xs: 24, md: 12, lg: 8 });
    // 跨度不足整行时中屏放大到至少半行，避免窄栏不可操作
    expect(responsiveSpanFor(2)).toEqual({ xs: 24, md: 12, lg: 2 });
  });

  test('width 0 (unset) and out-of-range legacy values fall back to type defaults', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        a: { type: 'string', title: '甲' },
        b: { type: 'string', title: '乙' },
      }),
      // 0 = 未设置；13 = 越界历史配置（校验器也拒绝）——均按未设置处理
      fields: [
        { key: 'a', width: 0 },
        { key: 'b', width: 13 },
      ],
    };
    const { container } = render(<SchemaFormRenderer spec={spec} hideSubmit />);
    // 两个字段同属一行 → 类型默认半行（24/2=12）
    expect(colOf(container, '甲').className).toContain('ant-col-12');
    expect(colOf(container, '乙').className).toContain('ant-col-12');
  });

  test('grouped long text field stays full-width and readable at every breakpoint', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        title: { type: 'string', title: '标题' },
        body: { type: 'string', title: '正文' },
      }),
      fields: [
        { key: 'title', width: 6 },
        { key: 'body', widget: 'TextArea' },
      ],
      groups: [{ key: 'g', title: { 'zh-CN': '内容' }, fields: ['title', 'body'] }],
    };
    const { container } = render(<SchemaFormRenderer spec={spec} hideSubmit />);
    const group = container.querySelector('[data-testid="group-g"]');
    expect(group).not.toBeNull();
    const title = colOf(container, '标题');
    const body = colOf(container, '正文');
    expect(title.className).toContain('ant-col-12');
    // 长文本字段整行、窄屏仍整行
    expect(body.className).toContain('ant-col-24');
    expect(body.className).toContain('ant-col-xs-24');
    expect(body.className).toContain('ant-col-lg-24');
  });
});
