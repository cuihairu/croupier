/**
 * 2.2 回归：rjsf 渲染抛错降级为结构化告警（而非整页白屏），
 * 重试按钮与 spec 变化（key 复位）两条恢复路径。
 */
import { fireEvent, render, screen } from '@testing-library/react';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

jest.mock('@rjsf/antd', () => {
  const React: typeof import('react') = require('react');
  const state = { crash: true };
  return {
    __esModule: true,
    default: () => {
      if (state.crash) throw new Error('boom: bad schema');
      return React.createElement('div', { 'data-testid': 'rjsf-ok' });
    },
    __state: state,
  };
});

const stubState = jest.requireMock('@rjsf/antd').__state as { crash: boolean };

const specOf = (seed: string): FormPresentationSpec =>
  ({
    jsonSchema: {
      type: 'object',
      properties: { a: { type: 'string', title: seed } },
    },
  }) as unknown as FormPresentationSpec;

beforeEach(() => {
  stubState.crash = true;
  jest.spyOn(console, 'error').mockImplementation(() => {});
});

afterEach(() => {
  (console.error as jest.Mock).mockRestore?.();
});

describe('SchemaFormRenderer 渲染错误边界', () => {
  it('渲染抛错 → Alert 降级 + 重试按钮，不再击穿页面', () => {
    render(<SchemaFormRenderer spec={specOf('a')} onFinish={jest.fn()} />);
    expect(screen.getByText('表单渲染失败')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /重试渲染/ })).toBeInTheDocument();
  });

  it('重试：schema 修复后点重试恢复渲染', () => {
    render(<SchemaFormRenderer spec={specOf('a')} onFinish={jest.fn()} />);
    expect(screen.getByText('表单渲染失败')).toBeInTheDocument();
    stubState.crash = false;
    fireEvent.click(screen.getByRole('button', { name: /重试渲染/ }));
    expect(screen.getByTestId('rjsf-ok')).toBeInTheDocument();
  });

  it('spec 变化复位边界：恢复后换回坏 spec 重新降级', () => {
    stubState.crash = false;
    const { rerender } = render(<SchemaFormRenderer spec={specOf('a')} onFinish={jest.fn()} />);
    expect(screen.getByTestId('rjsf-ok')).toBeInTheDocument();

    stubState.crash = true;
    rerender(<SchemaFormRenderer spec={specOf('b')} onFinish={jest.fn()} />);
    expect(screen.getByText('表单渲染失败')).toBeInTheDocument();

    stubState.crash = false;
    rerender(<SchemaFormRenderer spec={specOf('c')} onFinish={jest.fn()} />);
    expect(screen.getByTestId('rjsf-ok')).toBeInTheDocument();
  });
});
