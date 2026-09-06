import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { StaticFormLive } from '../PreviewRuntime';
import type { PageNode } from '../model';

function staticFormNode(schema: unknown): PageNode {
  return {
    id: 'sf1',
    type: 'staticForm',
    props: {
      title: '封禁原因',
      staticSchema: typeof schema === 'string' ? schema : JSON.stringify(schema),
    },
  };
}

const demoSchema = {
  type: 'object',
  properties: {
    banReason: {
      type: 'string',
      title: '封禁原因',
      enum: ['恶意刷单', '使用外挂', '辱骂他人'],
    },
    days: { type: 'integer', title: '封禁天数' },
  },
};

describe('StaticFormLive（预览态常量表单：真实控件可交互）', () => {
  it('按 staticSchema 渲染可交互控件（下拉 + 输入框）', () => {
    render(<StaticFormLive node={staticFormNode(demoSchema)} />);
    expect(screen.getByRole('combobox')).toBeTruthy();
    expect(screen.getByLabelText(/封禁天数/)).toBeTruthy();
  });

  it('文本字段输入经防抖后触发 onChange（值并入预览页面状态）', async () => {
    const onChange = jest.fn();
    render(
      <StaticFormLive node={staticFormNode(demoSchema)} onChange={onChange} debounceMs={50} />,
    );
    fireEvent.change(screen.getByLabelText(/封禁天数/), { target: { value: '3' } });
    await waitFor(() => expect(onChange).toHaveBeenCalled(), { timeout: 1000 });
    const all = onChange.mock.calls.at(-1)![0] as Record<string, unknown>;
    expect(all.days).toBe(3);
  });

  it('非法 JSON 显示警告且不渲染表单', () => {
    render(<StaticFormLive node={staticFormNode('{broken')} />);
    expect(screen.getByText('字段定义 JSON 无效')).toBeTruthy();
    expect(screen.queryByRole('combobox')).toBeNull();
  });

  it('无字段 schema 渲染为空表单不报错', () => {
    render(<StaticFormLive node={staticFormNode({ type: 'object', properties: {} })} />);
    expect(screen.queryByRole('combobox')).toBeNull();
  });
});
