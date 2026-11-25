/**
 * SUI-023 验收：枚举可见标签按值关联，且标签变化不改变提交的枚举值。
 *
 * 真实 RJSF 引擎 + 真实 antd Select（jsdom 下下拉经 portal 渲染，
 * fireEvent.mouseDown 打开、textContent 匹配点选），提交路径与
 * schema.enum 全程真实：点选「标签」断言 onFinish 收到「原始枚举值」。
 */
import React from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

function specWith(
  properties: Record<string, unknown>,
  fields?: Record<string, unknown>[],
): FormPresentationSpec {
  return {
    jsonSchema: { type: 'object', properties } as unknown as JSONSchema,
    fields: fields as FormPresentationSpec['fields'],
  };
}

async function openOptions(): Promise<Element[]> {
  const combobox = screen.getByRole('combobox');
  fireEvent.mouseDown(combobox);
  await waitFor(() => {
    expect(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option').length).toBeGreaterThan(0);
  });
  // rjsf antd Select 在首项渲染空占位选项（未选态），真实枚举项带文本
  return Array.from(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option')).filter(
    (el) => (el.textContent ?? '').length > 0,
  );
}

async function clickOptionByText(text: string): Promise<void> {
  const options = await openOptions();
  const target = options.find((el) => el.textContent === text);
  expect(target).toBeDefined();
  await act(async () => {
    fireEvent.click(target as Element);
  });
}

function submitForm(): void {
  fireEvent.click(screen.getByRole('button', { name: /提\s*交|submit/i }));
}

describe('SUI-023: 标签变化不改变提交的枚举值（真实 RJSF 提交路径）', () => {
  it('x-enum-options 数值枚举：点选标签提交原始数值', async () => {
    // portal 下拉 + waitFor 在高负载机器上可超 jest 默认 5s，显式放宽

    const onFinish = jest.fn();
    const spec = specWith({
      level: {
        type: 'integer',
        title: '等级',
        enum: [0, 1, 2],
        'x-enum-options': [
          { value: 2, label: { 'zh-CN': '高级' } },
          { value: 0, label: { 'zh-CN': '普通' } },
        ],
      },
    });
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);

    // 下拉显示标签（值 1 缺标签 → 原始值字符串），且按值关联而非下标
    const labels = (await openOptions()).map((el) => el.textContent);
    expect(labels).toEqual(['普通', '1', '高级']);

    await clickOptionByText('高级');
    submitForm();
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    // 提交的是原始枚举值 2，不是标签「高级」
    expect(onFinish.mock.calls[0][0]).toEqual({ level: 2 });
  }, 15000);

  it('fields.enumOptions 声明顺序与 enum 无关：点选标签提交原始字符串值', async () => {
    const onFinish = jest.fn();
    const spec = specWith(
      { mode: { type: 'string', title: '类型', enum: ['mail', 'broadcast'] } },
      [
        {
          key: 'mode',
          enumOptions: [
            { value: 'broadcast', label: { 'zh-CN': '全服广播' } },
            { value: 'mail', label: { 'zh-CN': '邮件' } },
          ],
        },
      ],
    );
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);

    // enumOptions 重排后下拉仍按 schema.enum 顺序关联标签
    const labels = (await openOptions()).map((el) => el.textContent);
    expect(labels).toEqual(['邮件', '全服广播']);

    await clickOptionByText('全服广播');
    submitForm();
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({ mode: 'broadcast' });
  }, 15000);
});
