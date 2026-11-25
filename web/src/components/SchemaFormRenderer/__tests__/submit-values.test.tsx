/**
 * SUI-014 验收测试：统一表单的提交取值接口
 *
 * - projectSubmitValues 是唯一提交投影（隐藏键按当前值剔除，其余键原样保留）
 * - 原生提交（onFinish）与命令式 getSubmitValues() 输出恒等（同一取值逻辑）
 * - 隐藏前后的编辑值可恢复（值保留在表单态，getValues 含隐藏字段）
 * - deriveRuntimeSchema.hiddenKeys 与 computeHiddenKeys 恒等（渲染与投影不漂移）
 *
 * SUI-030：隐藏切换断言等待真实 UI 回填（onFinish 到达 / 字段值恢复）后才读值，
 * 消除点击与派生 schema 重算之间的竞态；jest 预算放宽到 20s（负载下 5s 曾误报）。
 */
jest.setTimeout(20000);

import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import {
  computeHiddenKeys,
  deriveRuntimeSchema,
  projectSubmitValues,
} from '@/components/SchemaFormRenderer';
import SchemaFormRenderer, { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
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
        remark: { type: 'string', title: '备注' },
      },
    }),
    fields: [
      { key: 'mode', widget: 'Radio' },
      {
        key: 'targetPlayer',
        visibleWhen: { kind: 'equals', path: '/mode', value: 'single' },
      },
      { key: 'remark' },
    ],
  };
}

describe('SUI-014: projectSubmitValues 统一投影（纯函数）', () => {
  test('visibleWhen 不满足的字段被剔除，可见字段原样保留', () => {
    const spec = makeSpec();
    const values = { mode: 'batch', targetPlayer: 'p1', remark: 'r' };
    expect(projectSubmitValues(values, spec)).toEqual({ mode: 'batch', remark: 'r' });
  });

  test('条件满足时全量保留；无隐藏即原值透传', () => {
    const spec = makeSpec();
    const values = { mode: 'single', targetPlayer: 'p1', remark: 'r' };
    expect(projectSubmitValues(values, spec)).toEqual(values);
    expect(projectSubmitValues(values, spec)).toBe(values);
  });

  test('visible:false 静态隐藏同样剔除（显式给出值也不出网）', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        type: 'object',
        properties: { a: { type: 'string' }, internal: { type: 'string' } },
      }),
      fields: [{ key: 'a' }, { key: 'internal', visible: false }],
    };
    expect(projectSubmitValues({ a: 'x', internal: 'secret' }, spec)).toEqual({ a: 'x' });
  });

  test('spec 无 fields 时投影为透传（资源模板裸 jsonSchema）', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({ type: 'object', properties: { a: { type: 'string' } } }),
    };
    const values = { a: 'x', extra: 1 };
    expect(projectSubmitValues(values, spec)).toEqual(values);
  });

  test('computeHiddenKeys 与 deriveRuntimeSchema.hiddenKeys 恒等（渲染/投影同源）', () => {
    const spec = makeSpec(['mode', 'targetPlayer']);
    for (const values of [
      { mode: 'single', remark: 'r' },
      { mode: 'batch', remark: 'r', targetPlayer: 'p1' },
      {},
    ]) {
      expect(deriveRuntimeSchema(spec, values).hiddenKeys).toEqual(computeHiddenKeys(spec, values));
    }
  });
});

describe('SUI-014: 原生提交与 getSubmitValues 同一取值逻辑', () => {
  test('隐藏时两者一致剔除；恢复后两者一致带回编辑值', async () => {
    const onFinish = jest.fn();
    const ref = React.createRef<SchemaFormRendererHandle>();
    render(
      <SchemaFormRenderer
        ref={ref}
        spec={makeSpec()}
        initialValues={{ mode: 'single', targetPlayer: 'p1', remark: 'note' }}
        onFinish={onFinish}
      />,
    );

    // 编辑隐藏前改值：命令式取值读实时值
    fireEvent.change(screen.getByLabelText('目标玩家'), { target: { value: 'p2' } });
    expect(ref.current?.getSubmitValues()).toEqual({
      mode: 'single',
      targetPlayer: 'p2',
      remark: 'note',
    });

    // 切到 batch → 等字段从 DOM 消失（派生 schema 重算落地）再提交
    fireEvent.click(screen.getByRole('radio', { name: 'batch' }));
    await waitFor(() => {
      expect(screen.queryByLabelText('目标玩家')).toBeNull();
    });
    // 原生提交与 getSubmitValues 输出恒等（都不含隐藏字段）
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({ mode: 'batch', remark: 'note' });
    expect(ref.current?.getSubmitValues()).toEqual(onFinish.mock.calls[0][0]);

    // 隐藏字段的编辑值仍留在表单态（getValues 含隐藏字段），供恢复
    expect(ref.current?.getValues()).toEqual({
      mode: 'batch',
      targetPlayer: 'p2',
      remark: 'note',
    });

    // 切回 single → 等编辑值真实回填 DOM 后再断言命令式取值
    fireEvent.click(screen.getByRole('radio', { name: 'single' }));
    await waitFor(() => {
      const restored = screen.getByLabelText('目标玩家') as HTMLInputElement;
      expect(restored.value).toBe('p2');
    });
    expect(ref.current?.getSubmitValues()).toEqual({
      mode: 'single',
      targetPlayer: 'p2',
      remark: 'note',
    });
  });

  test('隐藏的必填字段不阻断命令式取值（校验照常豁免）', async () => {
    const ref = React.createRef<SchemaFormRendererHandle>();
    render(
      <SchemaFormRenderer
        ref={ref}
        spec={makeSpec(['mode', 'targetPlayer'])}
        initialValues={{ mode: 'batch' }}
      />,
    );
    expect(ref.current?.validate()).toBe(true);
    expect(ref.current?.getSubmitValues()).toEqual({ mode: 'batch' });
  });
});
