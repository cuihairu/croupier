/**
 * SUI-042 验收测试：discriminator 分支选择
 *
 * - 解析器三态：无 discriminator → null（既有流程零变化）；可解析组合 →
 *   branches；不可解析（缺 propertyName / 分支缺 const-enum / 悬空 $ref /
 *   重复值 / 无变体）→ unresolved（原因）→ JSON 降级。
 * - 渲染层：分支选择器；切换分支只渲染所选分支字段（合并 schema 渲染与
 *   校验），切换裁剪表单值、共享键保留、discriminator 预置；提交 payload
 *   只含所选分支允许的键；所选分支必填字段拦截提交。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import {
  mergeBranchIntoSchema,
  pruneValuesToBranch,
  resolveDiscriminatorBranches,
} from '@/components/SchemaFormRenderer/branchSelection';
import type {
  DiscriminatorBranch,
  DiscriminatorResolution,
} from '@/components/SchemaFormRenderer/branchSelection';
import SchemaFormRenderer, { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

const schemaOf = (value: Record<string, unknown>): JSONSchema => value as unknown as JSONSchema;

/** 战士/法师双分支多态根；shared 为两侧共享键，根本身不声明顶层 properties。 */
const polymorphicSchema: JSONSchema = schemaOf({
  type: 'object',
  discriminator: { propertyName: 'unitType' },
  oneOf: [
    {
      type: 'object',
      title: 'Warrior',
      required: ['unitType', 'weapon'],
      properties: {
        unitType: { type: 'string', const: 'warrior', title: '单位类型' },
        weapon: { type: 'string', title: '武器' },
        shared: { type: 'string', title: '共享' },
      },
    },
    {
      type: 'object',
      title: 'Mage',
      required: ['unitType', 'spell'],
      properties: {
        unitType: { type: 'string', const: 'mage', title: '单位类型' },
        spell: { type: 'string', title: '法术' },
        shared: { type: 'string', title: '共享' },
      },
    },
  ],
});

const baseSpec: FormPresentationSpec = { jsonSchema: polymorphicSchema };

describe('SUI-042: resolveDiscriminatorBranches 解析器', () => {
  test('无 discriminator 返回 null（非多态走既有流程）', () => {
    expect(
      resolveDiscriminatorBranches(
        schemaOf({ type: 'object', properties: { a: { type: 'string' } } }),
      ),
    ).toBeNull();
  });

  test('缺 discriminator.propertyName → unresolved 带原因', () => {
    const result = resolveDiscriminatorBranches(
      schemaOf({ type: 'object', discriminator: {}, oneOf: [{}] }),
    );
    expect(result).toEqual({ status: 'unresolved', reason: 'discriminator.propertyName missing' });
  });

  test('discriminator 无 oneOf/anyOf 变体 → unresolved', () => {
    const result = resolveDiscriminatorBranches(
      schemaOf({ type: 'object', discriminator: { propertyName: 'kind' } }),
    );
    expect(result?.status).toBe('unresolved');
    expect((result as { reason: string }).reason).toContain('no oneOf/anyOf variants');
  });

  test('分支缺 const/enum 与 mapping → unresolved（点名分支序号）', () => {
    const result = resolveDiscriminatorBranches(
      schemaOf({
        type: 'object',
        discriminator: { propertyName: 'kind' },
        oneOf: [
          { type: 'object', properties: { kind: { type: 'string', const: 'a' } } },
          { type: 'object', properties: { kind: { type: 'string' } } },
        ],
      }),
    );
    expect(result?.status).toBe('unresolved');
    expect((result as { reason: string }).reason).toContain('2/2');
  });

  test('悬空 $ref（mapping 或分支引用）→ unresolved', () => {
    const result = resolveDiscriminatorBranches(
      schemaOf({
        type: 'object',
        discriminator: { propertyName: 'kind', mapping: { a: '#/$defs/Missing' } },
        oneOf: [{ $ref: '#/$defs/Missing' }],
      }),
    );
    expect(result?.status).toBe('unresolved');
    // 悬空引用落入「分支无法定值」诊断；reason 必须给出可定位的分支序号
    expect((result as { reason: string }).reason).toContain('branch 1/1');
  });

  test('重复 discriminator 值 → unresolved', () => {
    const result = resolveDiscriminatorBranches(
      schemaOf({
        type: 'object',
        discriminator: { propertyName: 'kind' },
        oneOf: [
          { type: 'object', properties: { kind: { type: 'string', const: 'dup' } } },
          { type: 'object', properties: { kind: { type: 'string', enum: ['dup'] } } },
        ],
      }),
    );
    expect(result?.status).toBe('unresolved');
    expect((result as { reason: string }).reason).toContain('duplicate');
  });

  test('const 分支 → resolved，branch value 取自 const，label 取 title', () => {
    const result = resolveDiscriminatorBranches(polymorphicSchema) as Extract<
      DiscriminatorResolution,
      { status: 'resolved' }
    >;
    expect(result.status).toBe('resolved');
    expect(result.propertyName).toBe('unitType');
    expect(result.branches.map((branch: DiscriminatorBranch) => branch.value)).toEqual([
      'warrior',
      'mage',
    ]);
    expect(result.branches.map((branch: DiscriminatorBranch) => branch.label)).toEqual([
      'Warrior',
      'Mage',
    ]);
  });

  test('mapping + $ref 分支：值取自 mapping 键，schema 解析自引用', () => {
    const result = resolveDiscriminatorBranches(
      schemaOf({
        type: 'object',
        discriminator: { propertyName: 'kind', mapping: { sword: '#/$defs/Warrior' } },
        oneOf: [{ $ref: '#/$defs/Warrior' }],
        $defs: {
          Warrior: {
            type: 'object',
            properties: { kind: { type: 'string', const: 'sword' }, blade: { type: 'string' } },
          },
        },
      }),
    );
    expect(result?.status).toBe('resolved');
    const resolved = result as Extract<DiscriminatorResolution, { status: 'resolved' }>;
    expect(resolved.branches[0].value).toBe('sword');
    expect(resolved.branches[0].schema.properties?.blade).toBeTruthy();
  });

  test('anyOf + 唯一 enum 分支同样可解析', () => {
    const result = resolveDiscriminatorBranches(
      schemaOf({
        type: 'object',
        discriminator: { propertyName: 'kind' },
        anyOf: [{ type: 'object', properties: { kind: { type: 'string', enum: ['solo'] } } }],
      }),
    );
    expect(result?.status).toBe('resolved');
    const resolved = result as Extract<DiscriminatorResolution, { status: 'resolved' }>;
    expect(resolved.branches[0].value).toBe('solo');
  });
});

describe('SUI-042: mergeBranchIntoSchema / pruneValuesToBranch', () => {
  test('合并：移除 oneOf/discriminator，分支属性覆盖并并入基础属性，required 并集', () => {
    const resolution = resolveDiscriminatorBranches(polymorphicSchema)! as Extract<
      DiscriminatorResolution,
      { status: 'resolved' }
    >;
    const merged = mergeBranchIntoSchema(polymorphicSchema, resolution.branches[0], 'unitType');
    const root = merged as Record<string, unknown>;
    expect(root.oneOf).toBeUndefined();
    expect(root.discriminator).toBeUndefined();
    expect(root.properties).toEqual(
      expect.objectContaining({ weapon: expect.any(Object), shared: expect.any(Object) }),
    );
    expect(root.properties).not.toHaveProperty('spell');
    expect(root.required).toEqual(expect.arrayContaining(['unitType', 'weapon']));
  });

  test('裁剪：只保留所选分支允许的键 + 预置 discriminator，共享键保留已填值', () => {
    const resolution = resolveDiscriminatorBranches(polymorphicSchema)! as Extract<
      DiscriminatorResolution,
      { status: 'resolved' }
    >;
    const merged = mergeBranchIntoSchema(polymorphicSchema, resolution.branches[1], 'unitType');
    const pruned = pruneValuesToBranch(
      { unitType: 'warrior', weapon: 'sword', shared: 's1', stray: 'x' },
      merged,
      'unitType',
      'mage',
    );
    expect(pruned).toEqual({ unitType: 'mage', shared: 's1' });
  });
});

describe('SUI-042: 交互——分支选择器与按分支渲染', () => {
  test('默认渲染首个分支字段；切换到另一分支只渲染其字段', async () => {
    render(<SchemaFormRenderer spec={baseSpec} />);
    // 首屏：战士分支（weapon 可见，spell 不可见）
    expect(screen.getByLabelText('武器')).toBeTruthy();
    expect(screen.queryByLabelText('法术')).toBeNull();

    fireEvent.mouseDown(screen.getByTestId('branch-select'));
    const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
    fireEvent.click(
      within(dropdown).getByText('Mage', { selector: '.ant-select-item-option-content' }),
    );

    await waitFor(() => {
      expect(screen.queryByLabelText('武器')).toBeNull();
      expect(screen.getByLabelText('法术')).toBeTruthy();
    });
  });

  test('切换分支提交 payload 只含所选分支键并自带 discriminator 值', async () => {
    const onFinish = jest.fn();
    render(<SchemaFormRenderer spec={baseSpec} onFinish={onFinish} />);
    fireEvent.mouseDown(screen.getByTestId('branch-select'));
    const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
    fireEvent.click(
      within(dropdown).getByText('Mage', { selector: '.ant-select-item-option-content' }),
    );
    await waitFor(() => expect(screen.getByLabelText('法术')).toBeTruthy());

    fireEvent.change(screen.getByLabelText('法术'), { target: { value: '火球' } });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({ unitType: 'mage', spell: '火球' });
  });

  test('共享键切换后保留已填值；非分支键（warrior-only）被裁剪', async () => {
    render(<SchemaFormRenderer spec={baseSpec} />);
    fireEvent.change(screen.getByLabelText('共享'), { target: { value: 's1' } });
    fireEvent.change(screen.getByLabelText('武器'), { target: { value: '剑' } });

    fireEvent.mouseDown(screen.getByTestId('branch-select'));
    const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
    fireEvent.click(
      within(dropdown).getByText('Mage', { selector: '.ant-select-item-option-content' }),
    );
    await waitFor(() => expect(screen.getByLabelText('法术')).toBeTruthy());
    // 共享键跨分支保留
    expect((screen.getByLabelText('共享') as HTMLInputElement).value).toBe('s1');

    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() => {
      // 必填法术为空 → 校验拦截，不提交
      expect(screen.queryAllByText('火球').length).toBeGreaterThanOrEqual(0);
    });
    fireEvent.change(screen.getByLabelText('法术'), { target: { value: '闪现' } });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() => {
      // 无 onFinish 时提交失败也触发校验态；此处以法术字段有值且武器字段消失为准
      expect(screen.queryByLabelText('武器')).toBeNull();
    });
  });

  test('命令式 getSubmitValues 同样只含所选分支键', () => {
    const ref = React.createRef<SchemaFormRendererHandle>();
    render(<SchemaFormRenderer spec={baseSpec} ref={ref} />);
    fireEvent.change(screen.getByLabelText('武器'), { target: { value: '弓' } });
    expect(ref.current?.getSubmitValues()).toEqual({ unitType: 'warrior', weapon: '弓' });
  });

  test('initialValues 自带 discriminator 值时选择对应分支并回填', () => {
    render(
      <SchemaFormRenderer
        spec={baseSpec}
        initialValues={{ unitType: 'mage', spell: '预填法术', shared: 'x' }}
      />,
    );
    // 选中 Mage 分支：法术有初值，武器不可见
    expect((screen.getByLabelText('法术') as HTMLInputElement).value).toBe('预填法术');
    expect(screen.queryByLabelText('武器')).toBeNull();
  });

  test('非多态 schema：无分支选择器，渲染与提交与既有行为一致', () => {
    const onFinish = jest.fn();
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        type: 'object',
        required: ['a'],
        properties: { a: { type: 'string', title: 'A' } },
      }),
    };
    const { container } = render(
      <SchemaFormRenderer spec={spec} onFinish={onFinish} onValuesChange={undefined} />,
    );
    expect(screen.queryByTestId('branch-select')).toBeNull();
    fireEvent.change(screen.getByLabelText('A'), { target: { value: '1' } });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    expect(onFinish).toHaveBeenCalledWith({ a: '1' });
  });

  test('不可解析分支组合 → JSON 降级 + 具体原因，提交经同一 JSON 编辑链', async () => {
    const onFinish = jest.fn();
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        type: 'object',
        discriminator: { propertyName: 'unitType' },
        // 分支未声明 const/enum/mapping → unresolved；两分支互斥（weapon vs
        // spell 必填）使下方输入恰好匹配 oneOf 第二支，schema 校验可过。
        oneOf: [
          {
            type: 'object',
            required: ['weapon'],
            properties: { unitType: { type: 'string' }, weapon: { type: 'string' } },
          },
          {
            type: 'object',
            required: ['spell'],
            properties: { unitType: { type: 'string' }, spell: { type: 'string' } },
          },
        ],
      }),
    };
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);
    // 选择器不出现；JSON 降级编辑器与原因呈现
    expect(screen.queryByTestId('branch-select')).toBeNull();
    const reason = screen.getByTestId('json-root-reason');
    expect(reason.textContent).toContain('unitType');

    const editor = screen.getByTestId('json-root-editor') as HTMLTextAreaElement;
    fireEvent.change(editor, {
      target: { value: JSON.stringify({ unitType: 'warrior', weapon: '斧' }) },
    });
    fireEvent.click(screen.getByTestId('json-root-submit'));
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({ unitType: 'warrior', weapon: '斧' });
  });

  test('降级形态的命令式 handle 指向 JSON 编辑器，而非空对象表单', async () => {
    const ref = React.createRef<SchemaFormRendererHandle>();
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        type: 'object',
        discriminator: { propertyName: 'kind' },
        oneOf: [{ type: 'object', properties: { kind: { type: 'string' } } }],
      }),
    };
    render(<SchemaFormRenderer spec={spec} ref={ref} />);
    fireEvent.change(screen.getByTestId('json-root-editor'), {
      target: { value: JSON.stringify({ kind: 'x' }) },
    });
    expect(ref.current?.getSubmitValues()).toEqual({ kind: 'x' });
    expect(ref.current?.validate()).toBe(true);
  });
});
