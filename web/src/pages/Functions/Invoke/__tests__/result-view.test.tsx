/**
 * F10 验收测试：outputSchema 驱动结果渲染
 */
import { render, screen, within } from '@testing-library/react';
import InvocationResponse from '@/pages/Functions/Invoke/InvocationResponse';
import { deriveResultSpec, isArrayOfObjects } from '@/utils/resultSpec';
import type { JSONSchema } from '@/types/dashboard';

const schemaOf = (value: Record<string, unknown>): JSONSchema => value as unknown as JSONSchema;

describe('F10: deriveResultSpec', () => {
  test('object + properties → 字段卡片规格（title 缺失人性化）', () => {
    const derived = deriveResultSpec(
      schemaOf({
        type: 'object',
        properties: {
          banned: { type: 'boolean', title: '已封禁' },
          missingCount: { type: 'integer' },
        },
      }),
    );
    expect(derived?.shape).toBe('object');
    expect(derived?.spec.fields).toEqual([
      { key: 'banned', title: { 'zh-CN': '已封禁', 'en-US': '已封禁' }, dataType: 'boolean' },
      {
        key: 'missingCount',
        title: { 'zh-CN': 'Missing Count', 'en-US': 'Missing Count' },
        dataType: 'integer',
      },
    ]);
  });

  test('array + items.properties → 表格列规格', () => {
    const derived = deriveResultSpec(
      schemaOf({
        type: 'array',
        items: {
          type: 'object',
          properties: { id: { type: 'string', title: 'ID' }, gold: { type: 'integer' } },
        },
      }),
    );
    expect(derived?.shape).toBe('arrayOfObjects');
    expect(derived?.spec.fields?.map((f) => f.key)).toEqual(['id', 'gold']);
  });

  test('标量/无 properties/空 schema → undefined（JSON 兜底）', () => {
    expect(deriveResultSpec(schemaOf({ type: 'string' }))).toBeUndefined();
    expect(deriveResultSpec(schemaOf({ type: 'object' }))).toBeUndefined();
    expect(deriveResultSpec(null)).toBeUndefined();
  });

  test('isArrayOfObjects 判定', () => {
    expect(isArrayOfObjects([{ a: 1 }])).toBe(true);
    expect(isArrayOfObjects([1, 2])).toBe(false);
    expect(isArrayOfObjects([])).toBe(false);
    expect(isArrayOfObjects({ a: 1 })).toBe(false);
  });
});

describe('F10: InvocationResponse 结构化渲染', () => {
  test('object 结果优先展示结构化 Tab（字段卡片）', () => {
    render(
      <InvocationResponse
        responseRaw='{"banned": true}'
        error=""
        duration={12}
        response={{ banned: true }}
        outputSchema={schemaOf({
          type: 'object',
          properties: { banned: { type: 'boolean', title: '已封禁' } },
        })}
        onCopy={jest.fn()}
      />,
    );
    expect(screen.getByText('结构化')).toBeTruthy();
    expect(screen.getByText('已封禁')).toBeTruthy();
    expect(screen.getByText('是')).toBeTruthy();
  });

  test('对象数组结果渲染为表格', () => {
    render(
      <InvocationResponse
        responseRaw='[{"id":"p1"}]'
        error=""
        duration={12}
        response={[{ id: 'p1' }, { id: 'p2' }]}
        outputSchema={schemaOf({
          type: 'array',
          items: { type: 'object', properties: { id: { type: 'string', title: 'ID' } } },
        })}
        onCopy={jest.fn()}
      />,
    );
    expect(screen.getByRole('table')).toBeTruthy();
    expect(screen.getByText('p1')).toBeTruthy();
    expect(screen.getByText('p2')).toBeTruthy();
  });

  test('outputSchema 缺失时无结构化 Tab（JSON 兜底）', () => {
    render(
      <InvocationResponse
        responseRaw='{"any": 1}'
        error=""
        duration={12}
        response={{ any: 1 }}
        outputSchema={null}
        onCopy={jest.fn()}
      />,
    );
    expect(screen.queryByText('结构化')).toBeNull();
    expect(screen.getByText('格式化')).toBeTruthy();
  });

  test('错误响应渲染 details 结构化明细', () => {
    render(
      <InvocationResponse
        responseRaw=""
        error="请求参数无效"
        errorDetails={[
          { field: 'playerId', message: '不能为空' },
          { field: 'amount', message: '必须大于 0' },
        ]}
        duration={5}
        onCopy={jest.fn()}
      />,
    );
    expect(screen.getByText('调用失败')).toBeTruthy();
    const alert = screen.getByRole('alert');
    expect(within(alert).getByText('playerId')).toBeTruthy();
    expect(within(alert).getByText(/不能为空/)).toBeTruthy();
    expect(within(alert).getByText(/amount/)).toBeTruthy();
    expect(within(alert).getByText(/必须大于 0/)).toBeTruthy();
  });
});
