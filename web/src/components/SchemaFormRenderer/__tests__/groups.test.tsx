/**
 * F7 验收测试：FormGroupSpec 分组渲染（自定义根级 ObjectFieldTemplate）
 */
import { fireEvent, render, screen, within } from '@testing-library/react';
import { deriveRuntimeSchema } from '@/components/SchemaFormRenderer';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

const schemaOf = (value: Record<string, unknown>): JSONSchema => value as unknown as JSONSchema;

describe('F7: formContext 分组元数据', () => {
  test('deriveRuntimeSchema 注入 __groups/__fieldGroups/__fieldWidths', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        type: 'object',
        properties: { a: { type: 'string' }, b: { type: 'integer' } },
      }),
      groups: [{ key: 'g1', title: { 'zh-CN': '组一' }, fields: ['a'] }],
      fields: [{ key: 'b', width: 6 }],
    };
    const { formContext } = deriveRuntimeSchema(spec, {});
    expect(formContext.__groups).toEqual([
      { key: 'g1', title: { 'zh-CN': '组一' }, fields: ['a'] },
    ]);
    expect(formContext.__fieldGroups).toEqual({ a: 'g1' });
    expect(formContext.__fieldWidths).toEqual({ b: 6 });
  });

  test('无 groups 时不注入分组元数据', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({ type: 'object', properties: { a: { type: 'string' } } }),
    };
    const { formContext } = deriveRuntimeSchema(spec, {});
    expect(formContext.__groups).toBeUndefined();
    expect(formContext.__fieldWidths).toBeUndefined();
  });
});

describe('F7: 分组渲染', () => {
  test('分组渲染为 Card（本地化标题），字段归属正确', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        type: 'object',
        properties: {
          title: { type: 'string', title: '标题' },
          level: { type: 'integer', title: '等级' },
          solo: { type: 'string', title: '独立字段' },
        },
      }),
      groups: [{ key: 'basic', title: { 'zh-CN': '基本信息' }, fields: ['title', 'level'] }],
    };
    const { container } = render(<SchemaFormRenderer spec={spec} onFinish={jest.fn()} />);
    const card = container.querySelector('[data-testid="group-basic"]');
    expect(card).toBeTruthy();
    expect(screen.getByText('基本信息')).toBeTruthy();
    // title/level 在卡内，solo 在卡外
    expect(within(card as HTMLElement).getByLabelText('标题')).toBeTruthy();
    expect(within(card as HTMLElement).getByLabelText('等级')).toBeTruthy();
    expect(within(card as HTMLElement).queryByLabelText('独立字段')).toBeNull();
    expect(screen.getByLabelText('独立字段')).toBeTruthy();
  });

  test('collapsible 分组用 Collapse 可展开', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        type: 'object',
        properties: { adv: { type: 'string', title: '高级项' } },
      }),
      groups: [
        {
          key: 'advanced',
          title: { 'zh-CN': '高级选项' },
          fields: ['adv'],
          collapsible: true,
          collapsed: true,
        },
      ],
    };
    const { container } = render(<SchemaFormRenderer spec={spec} onFinish={jest.fn()} />);
    const collapse = container.querySelector('[data-testid="group-advanced"]') as HTMLElement;
    expect(collapse).toBeTruthy();
    const item = collapse.querySelector('.ant-collapse-item') as HTMLElement;
    // 默认折叠
    expect(item.className).not.toContain('ant-collapse-item-active');
    // 点击头部展开
    fireEvent.click(collapse.querySelector('.ant-collapse-header') as Element);
    expect(item.className).toContain('ant-collapse-item-active');
  });

  test('width 生效为 Col 栅格宽度', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        type: 'object',
        properties: { half: { type: 'string', title: '半宽' } },
      }),
      fields: [{ key: 'half', width: 6 }],
    };
    const { container } = render(<SchemaFormRenderer spec={spec} onFinish={jest.fn()} />);
    const col = container.querySelector('.ant-col-6');
    expect(col).toBeTruthy();
    expect(col?.querySelector('input#root_half')).toBeTruthy();
  });

  test('无分组时不产生 Card，行为与默认一致', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        type: 'object',
        properties: { plain: { type: 'string', title: '普通' } },
      }),
    };
    const { container } = render(<SchemaFormRenderer spec={spec} onFinish={jest.fn()} />);
    expect(container.querySelector('.ant-card')).toBeNull();
    expect(screen.getByLabelText('普通')).toBeTruthy();
  });
});
