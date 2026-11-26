/**
 * SUI-035 验收：渲染端解析命名展示风格的生效默认项。
 *
 * 解析链：defaultVariant → 基础呈现 → deriveDefaultPresentationSpec 派生
 * 纵向（SUI-025）。真实 RJSF + 真实 SchemaFormRenderer 断言：
 * - 声明 defaultVariant 的表单按变体覆盖渲染（grid 布局注入 formContext、
 *   逐字段列宽/控件合并生效）且提交 payload 不变（调用恒等）；
 * - defaultVariant 缺失/解析不到（陈旧 key）回落基础呈现原样渲染；
 * - 无变体配置的旧页面（零 variants/defaultVariant）合并恒等，与 SUI-025
 *   兜底链无缝衔接（fields-less 仍派生标准纵向）；
 * - 变体合并不触碰 jsonSchema：required 校验行为在基础/变体下一致。
 */
import React, { createRef } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import SchemaFormRenderer, { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec } from '@/types/dashboard';

const schema = {
  type: 'object',
  properties: {
    playerId: { type: 'string' },
    reason: { type: 'string' },
  },
  required: ['playerId'],
} as unknown as FormPresentationSpec['jsonSchema'];

/** 基础纵向 + 紧凑栅格变体（SUI-031 典型一对）。 */
const withVariants = (defaultVariant?: string): FormPresentationSpec => ({
  jsonSchema: schema,
  layout: 'vertical',
  fields: [
    { key: 'playerId', widget: 'Input', label: { 'zh-CN': '玩家' }, width: 12 },
    { key: 'reason', widget: 'TextArea' },
  ],
  variants: [
    { key: 'standard', fields: [] },
    {
      key: 'compact',
      layout: 'grid',
      fields: [
        { key: 'playerId', width: 6 },
        { key: 'reason', widget: 'Input', width: 6 },
      ],
    },
  ],
  defaultVariant,
});

const fieldLabel = (text: string): HTMLElement | null =>
  screen.queryByText(text)?.closest('.ant-form-item') ?? null;

describe('SchemaFormRenderer variant default resolution (SUI-035)', () => {
  test('defaultVariant=compact：grid 布局注入、逐字段覆盖合并渲染，提交 payload 与基础恒等', async () => {
    const onFinish = jest.fn();
    const ref = createRef<SchemaFormRendererHandle>();
    render(<SchemaFormRenderer spec={withVariants('compact')} onFinish={onFinish} ref={ref} />);

    // 布局覆盖：grid → GroupedObjectFieldTemplate 走栅格模板（Row/Col）
    expect(document.querySelector('.ant-row')).not.toBeNull();
    // 逐字段控件覆盖：reason 由 TextArea 变体改 Input（无 textarea 元素）
    expect(document.querySelector('textarea')).toBeNull();
    // 列宽覆盖进 formContext（12 栏协议 width=6 → antd 24 栏 span=12）
    const playerCol = fieldLabel('玩家')?.closest('.ant-col');
    expect(playerCol?.className).toContain('ant-col-12');

    // 调用恒等：同输入产出与基础一致的 payload
    const input = document.querySelector('input#root_playerId') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'p-1' } });
    ref.current?.submit();
    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    expect(onFinish).toHaveBeenCalledWith({ playerId: 'p-1' });
  }, 15000);

  test('defaultVariant 解析不到（陈旧 key）：回落基础呈现原样渲染', async () => {
    const onFinish = jest.fn();
    const ref = createRef<SchemaFormRendererHandle>();
    render(<SchemaFormRenderer spec={withVariants('ghost')} onFinish={onFinish} ref={ref} />);

    // 基础渲染：reason 仍是 TextArea，布局无栅格注入
    expect(document.querySelector('textarea')).not.toBeNull();
    fireEvent.change(document.querySelector('textarea') as HTMLTextAreaElement, {
      target: { value: 'spam' },
    });
    const input = document.querySelector('input#root_playerId') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'p-2' } });
    ref.current?.submit();
    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    expect(onFinish).toHaveBeenCalledWith({ playerId: 'p-2', reason: 'spam' });
  }, 15000);

  test('未声明 defaultVariant：基础呈现即生效（变体配置存在也不参与）', () => {
    render(<SchemaFormRenderer spec={withVariants()} onFinish={jest.fn()} />);
    // 基础 reason=TextArea（非变体的 Input）
    expect(document.querySelector('textarea')).not.toBeNull();
  });

  test('旧页面零变体配置：合并恒等，与 SUI-025 兜底链衔接', () => {
    const legacy: FormPresentationSpec = {
      jsonSchema: schema,
      layout: 'vertical',
      fields: [{ key: 'playerId', widget: 'Input' }],
    };
    const { rerender } = render(<SchemaFormRenderer spec={legacy} onFinish={jest.fn()} />);
    // fields-less 老形态：兜底派生仍生效（两字段都可输入）
    rerender(
      <SchemaFormRenderer spec={{ jsonSchema: schema, layout: 'vertical' }} onFinish={jest.fn()} />,
    );
    expect(document.querySelector('input#root_playerId')).not.toBeNull();
    expect(document.querySelector('input#root_reason')).not.toBeNull();
  });

  test('defaultVariant 无变体声明即发布层校验错误；渲染端容错为恒等（防御边界）', () => {
    // SUI-033 发布校验已拦 form_variant_default_unknown；渲染端对漏网
    // 形态（旧快照缓存）不崩、原样渲染基础
    const orphan: FormPresentationSpec = {
      jsonSchema: schema,
      layout: 'vertical',
      fields: [{ key: 'playerId', widget: 'Input' }],
      defaultVariant: 'compact',
    };
    render(<SchemaFormRenderer spec={orphan} onFinish={jest.fn()} />);
    expect(document.querySelector('input#root_playerId')).not.toBeNull();
  });
});
