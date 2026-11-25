/**
 * SUI-026 验收：能够序列化执行的请求（根数组/标量，SUI-011 degraded 形态）
 * 在生成页面/组合页共用的 SchemaFormRenderer 中获得 JSON 编辑入口——同一
 * handle 契约（validate/getSubmitValues/submit）、同一 AJV 校验器、payload
 * 原样提交（不套对象壳，SUI-017 往返语义），并显示不能渲染结构化表单的
 * 具体原因（根类型）与实际可用入口。
 */
import React, { createRef } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import SchemaFormRenderer, { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

const arraySpec: FormPresentationSpec = {
  jsonSchema: {
    type: 'array',
    items: { type: 'string' },
  } as unknown as JSONSchema,
  layout: 'vertical',
};

const scalarSpec: FormPresentationSpec = {
  jsonSchema: { type: 'integer' } as unknown as JSONSchema,
  layout: 'vertical',
};

const objectSpec: FormPresentationSpec = {
  jsonSchema: {
    type: 'object',
    properties: { name: { type: 'string' } },
  } as unknown as JSONSchema,
  layout: 'vertical',
};

function jsonEditor(): HTMLTextAreaElement {
  const editor = document.querySelector('[data-testid="json-root-editor"]');
  expect(editor).not.toBeNull();
  return editor as HTMLTextAreaElement;
}

describe('SchemaFormRenderer JSON root editing (SUI-026)', () => {
  test('root array renders JSON editor with concrete reason and no object form', () => {
    render(<SchemaFormRenderer spec={arraySpec} hideSubmit />);
    const editor = jsonEditor();
    // SUI-017 种子：根数组 → []
    expect(editor.value).toBe('[]');
    // 具体原因（根类型）+ 实际可用入口说明
    expect(screen.getByTestId('json-root-reason').textContent).toContain('array');
    // 不渲染对象表单控件（无 rjsf 输入框）
    expect(document.querySelector('input[type="text"]')).toBeNull();
  });

  test('submit sends the array verbatim (no object shell)', async () => {
    const onFinish = jest.fn();
    const ref = createRef<SchemaFormRendererHandle>();
    render(<SchemaFormRenderer spec={arraySpec} onFinish={onFinish} ref={ref} />);
    fireEvent.change(jsonEditor(), { target: { value: '["a","b"]' } });
    ref.current?.submit();
    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    expect(onFinish).toHaveBeenCalledWith(['a', 'b']);
  });

  test('invalid JSON blocks submit with a parse error', async () => {
    const onFinish = jest.fn();
    const ref = createRef<SchemaFormRendererHandle>();
    render(<SchemaFormRenderer spec={arraySpec} onFinish={onFinish} ref={ref} />);
    fireEvent.change(jsonEditor(), { target: { value: '["a",,' } });
    expect(ref.current?.validate()).toBe(false);
    ref.current?.submit();
    expect(onFinish).not.toHaveBeenCalled();
    // 解析错误呈现
    expect(await screen.findByText(/JSON 解析失败|Invalid JSON/)).toBeInTheDocument();
  });

  test('schema violation surfaces AJV errors through the same validator', async () => {
    const onFinish = jest.fn();
    const ref = createRef<SchemaFormRendererHandle>();
    render(<SchemaFormRenderer spec={arraySpec} onFinish={onFinish} ref={ref} />);
    // items type: string，数字元素违规
    fireEvent.change(jsonEditor(), { target: { value: '[1,2]' } });
    expect(ref.current?.validate()).toBe(false);
    ref.current?.submit();
    expect(onFinish).not.toHaveBeenCalled();
    expect(
      await screen.findByText(/不符合 schema|does not match the schema/),
    ).toBeInTheDocument();
  });

  test('root scalar submits the typed value verbatim', async () => {
    const onFinish = jest.fn();
    const ref = createRef<SchemaFormRendererHandle>();
    render(<SchemaFormRenderer spec={scalarSpec} onFinish={onFinish} ref={ref} />);
    // SUI-017 种子：根标量 integer → 0
    expect(jsonEditor().value).toBe('0');
    fireEvent.change(jsonEditor(), { target: { value: '5' } });
    ref.current?.submit();
    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    expect(onFinish).toHaveBeenCalledWith(5);
  });

  test('imperative getSubmitValues returns the parsed root value', () => {
    const ref = createRef<SchemaFormRendererHandle>();
    render(<SchemaFormRenderer spec={arraySpec} hideSubmit ref={ref} />);
    fireEvent.change(jsonEditor(), { target: { value: '["x"]' } });
    expect(ref.current?.getSubmitValues()).toEqual(['x']);
  });

  test('initialValues seed the editor and survive submit', async () => {
    const onFinish = jest.fn();
    const ref = createRef<SchemaFormRendererHandle>();
    render(
      <SchemaFormRenderer
        spec={arraySpec}
        initialValues={['seeded'] as unknown as Record<string, never>}
        onFinish={onFinish}
        ref={ref}
      />,
    );
    // 种子为 pretty 格式（JSON.stringify(_, null, 2)），按解析值断言
    expect(JSON.parse(jsonEditor().value)).toEqual(['seeded']);
    ref.current?.submit();
    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    expect(onFinish).toHaveBeenCalledWith(['seeded']);
  });

  test('object-root schemas keep the structured form (no JSON editor)', () => {
    render(<SchemaFormRenderer spec={objectSpec} hideSubmit />);
    expect(document.querySelector('[data-testid="json-root-editor"]')).toBeNull();
    // 对象表单照常渲染
    expect(document.querySelector('#root_name')).not.toBeNull();
  });
});
