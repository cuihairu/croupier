/**
 * SUI-027 验收：同页挂载同 schema 的两个表单时，label/错误/说明关联各自
 * 实例；同一实例重渲染时关联稳定。
 *
 * 真实 RJSF 引擎：断言两实例的控件 id 前缀互异（首个实例保持默认 `root`，
 * 并发实例领 `root-2`…），label htmlFor 指向本实例控件，卸载归还后串行
 * 渲染回到默认前缀（既有 `#root_*` 选择器不受影响）。
 */
import React from 'react';
import { render, screen, act } from '@testing-library/react';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

const spec: FormPresentationSpec = {
  jsonSchema: {
    type: 'object',
    properties: { name: { type: 'string', title: '名称' } },
  } as unknown as JSONSchema,
  layout: 'vertical',
};

function nameInputOf(scope: ParentNode): HTMLInputElement {
  const input = scope.querySelector('input[type="text"]');
  expect(input).not.toBeNull();
  return input as HTMLInputElement;
}

describe('SchemaFormRenderer unique field ids across instances (SUI-027)', () => {
  test('two same-schema forms get distinct prefixes; labels bind to their own input', () => {
    const { unmount } = render(
      <div>
        <section data-testid="first">
          <SchemaFormRenderer spec={spec} hideSubmit />
        </section>
        <section data-testid="second">
          <SchemaFormRenderer spec={spec} hideSubmit />
        </section>
      </div>,
    );

    const first = nameInputOf(screen.getByTestId('first'));
    const second = nameInputOf(screen.getByTestId('second'));
    // id 互异且覆盖两种前缀形态
    expect(first.id).not.toBe(second.id);
    expect([first.id, second.id]).toEqual(
      expect.arrayContaining([expect.stringMatching(/^root_/), expect.stringMatching(/^root-2_/)]),
    );

    // label htmlFor 关联本实例控件（首实例 label 不指向第二实例输入框）
    const firstLabel = screen.getByTestId('first').querySelector('label');
    expect(firstLabel?.getAttribute('for')).toBe(first.id);
    const secondLabel = screen.getByTestId('second').querySelector('label');
    expect(secondLabel?.getAttribute('for')).toBe(second.id);

    unmount();
  });

  test('rerender keeps the instance id stable; unmount releases the prefix for reuse', () => {
    const harness = render(<SchemaFormRenderer spec={spec} hideSubmit />);
    const before = nameInputOf(harness.container).id;
    // 同实例重渲染：前缀与控件 id 稳定
    harness.rerender(<SchemaFormRenderer spec={spec} hideSubmit />);
    expect(nameInputOf(harness.container).id).toBe(before);
    expect(before).toMatch(/^root_/);
    harness.unmount();

    // 卸载归还后，新实例重新领取默认 `root`（串行渲染不漂移）
    const next = render(<SchemaFormRenderer spec={spec} hideSubmit />);
    expect(nameInputOf(next.container).id).toMatch(/^root_/);
    next.unmount();
  });

  test('input id and label survive a spec-value rerender without id churn', () => {
    const harness = render(
      <div data-testid="host">
        <SchemaFormRenderer spec={spec} initialValues={{ name: 'a' }} hideSubmit />
      </div>,
    );
    const inputBefore = nameInputOf(screen.getByTestId('host'));
    const labelBefore = screen.getByTestId('host').querySelector('label');
    act(() => {
      harness.rerender(
        <div data-testid="host">
          <SchemaFormRenderer spec={spec} initialValues={{ name: 'b' }} hideSubmit />
        </div>,
      );
    });
    const inputAfter = nameInputOf(screen.getByTestId('host'));
    const labelAfter = screen.getByTestId('host').querySelector('label');
    expect(inputAfter.id).toBe(inputBefore.id);
    expect(labelAfter?.getAttribute('for')).toBe(labelBefore?.getAttribute('for'));
    // 初值变化照常回种（关联稳定不牺牲既有行为）
    expect(inputAfter.value).toBe('b');
    harness.unmount();
  });
});
