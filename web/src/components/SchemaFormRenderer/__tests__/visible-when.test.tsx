/**
 * F8 验收测试：visibleWhen 条件隐藏不丢值
 *
 * SUI-030：隐藏切换用例以「真实 UI 事件 + 显式回填等待」稳定——点击 Radio 后
 * 必须 waitFor 字段从 DOM 消失（或重现）再继续断言，消除派生 schema 重算与
 * antd 重渲染之间的异步窗口；jest 预算放宽到 20s，覆盖满载机器上 transformer
 * 与渲染排队（历史上默认 5s 曾在负载下误报超时）。
 */
jest.setTimeout(20000);

import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { deriveRuntimeSchema } from '@/components/SchemaFormRenderer';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

const schemaOf = (value: Record<string, unknown>): JSONSchema => value as unknown as JSONSchema;

function makeSpec(required: string[] = ['mode']): FormPresentationSpec {
  return {
    jsonSchema: schemaOf({
      type: 'object',
      required,
      properties: {
        mode: { type: 'string', enum: ['single', 'batch'], title: '模式' },
        targetPlayer: { type: 'string', title: '目标玩家' },
      },
    }),
    fields: [
      { key: 'mode', widget: 'Radio' },
      {
        key: 'targetPlayer',
        visibleWhen: { kind: 'equals', path: '/mode', value: 'single' },
      },
    ],
  };
}

describe('F8: deriveRuntimeSchema 隐藏策略', () => {
  test('隐藏字段保留在 schema（ui:widget hidden）且移出 required，返回 hiddenKeys', () => {
    const spec = makeSpec(['mode', 'targetPlayer']);
    const { schema, hiddenKeys } = deriveRuntimeSchema(spec, { mode: 'batch' });
    expect(hiddenKeys).toEqual(['targetPlayer']);
    const properties = schema.properties as Record<string, Record<string, unknown>>;
    // 字段保留（保值），required 摘除
    expect(properties.targetPlayer).toBeTruthy();
    expect(schema.required).toEqual(['mode']);
    // 隐藏由 uiSchema 承担
    const { uiSchema } = deriveRuntimeSchema(spec, { mode: 'batch' });
    expect((uiSchema.targetPlayer as Record<string, string>)['ui:widget']).toBe('hidden');
  });

  test('条件满足时字段可见、无 hiddenKeys', () => {
    const spec = makeSpec(['mode', 'targetPlayer']);
    const { hiddenKeys } = deriveRuntimeSchema(spec, { mode: 'single' });
    expect(hiddenKeys).toEqual([]);
  });
});

describe('F8: 交互——切回恢复、提交剔除', () => {
  test('切到 batch 提交不含隐藏字段；切回 single 值恢复', async () => {
    const onFinish = jest.fn();
    const { container } = render(
      <SchemaFormRenderer
        spec={makeSpec()}
        initialValues={{ mode: 'single', targetPlayer: 'p1' }}
        onFinish={onFinish}
      />,
    );
    // 初始可见且带值
    const targetInput = screen.getByLabelText('目标玩家') as HTMLInputElement;
    expect(targetInput.value).toBe('p1');

    // 切到 batch → 隐藏
    fireEvent.click(screen.getByRole('radio', { name: 'batch' }));
    await waitFor(() => {
      expect(screen.queryByLabelText('目标玩家')).toBeNull();
    });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({ mode: 'batch' });

    // 切回 single → 等字段重新挂载并回填旧值再断言（避免读到重渲染前的 DOM）
    fireEvent.click(screen.getByRole('radio', { name: 'single' }));
    await waitFor(() => {
      const restored = screen.getByLabelText('目标玩家') as HTMLInputElement;
      expect(restored.value).toBe('p1');
    });
    void container;
  });

  test('隐藏的必填字段不阻断提交', async () => {
    const onFinish = jest.fn();
    render(
      <SchemaFormRenderer
        spec={makeSpec(['mode', 'targetPlayer'])}
        initialValues={{ mode: 'batch' }}
        onFinish={onFinish}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() => expect(onFinish).toHaveBeenCalledWith({ mode: 'batch' }));
  });
});
