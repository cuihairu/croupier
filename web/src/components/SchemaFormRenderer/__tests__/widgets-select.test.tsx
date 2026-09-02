/**
 * F3 验收测试：Select 系 widget 映射与自定义 widgets 渲染
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { deriveRuntimeSchema } from '@/components/SchemaFormRenderer';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import { RateWidget } from '@/components/SchemaFormRenderer/widgets';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

const schemaOf = (value: Record<string, unknown>): JSONSchema => value as unknown as JSONSchema;

describe('F3: uiSchema widget 映射', () => {
  test('Select → 内置 select', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: { type: 'object', properties: { mode: { type: 'string', enum: ['a', 'b'] } } },
      fields: [{ key: 'mode', widget: 'Select' }],
    };
    const { uiSchema } = deriveRuntimeSchema(spec, {});
    expect((uiSchema.mode as Record<string, string>)['ui:widget']).toBe('select');
  });

  test('MultiSelect → select + ui:options.multiple', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: {
        type: 'object',
        properties: { tags: { type: 'array', items: { type: 'string', enum: ['x', 'y'] } } },
      },
      fields: [{ key: 'tags', widget: 'MultiSelect' }],
    };
    const { uiSchema } = deriveRuntimeSchema(spec, {});
    const ui = uiSchema.tags as Record<string, unknown>;
    expect(ui['ui:widget']).toBe('select');
    expect(ui['ui:options']).toMatchObject({ multiple: true });
  });

  test('TreeSelect/Cascader/Rate → 自定义 widget 名', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: {
        type: 'object',
        properties: {
          area: { type: 'string' },
          channel: { type: 'string' },
          score: { type: 'integer' },
        },
      },
      fields: [
        { key: 'area', widget: 'TreeSelect' },
        { key: 'channel', widget: 'Cascader' },
        { key: 'score', widget: 'Rate' },
      ],
    };
    const { uiSchema } = deriveRuntimeSchema(spec, {});
    expect((uiSchema.area as Record<string, string>)['ui:widget']).toBe('treeSelect');
    expect((uiSchema.channel as Record<string, string>)['ui:widget']).toBe('cascader');
    expect((uiSchema.score as Record<string, string>)['ui:widget']).toBe('rate');
  });
});

describe('F3: 自定义 widgets 渲染', () => {
  test('TreeSelect 渲染并可选值（treeData 来自 widgetProps）', async () => {
    const spec: FormPresentationSpec = {
      jsonSchema: { type: 'object', properties: { area: { type: 'string', title: '大区' } } },
      fields: [
        {
          key: 'area',
          widget: 'TreeSelect',
          widgetProps: {
            treeData: [
              { title: '华东', value: 'east' },
              { title: '华南', value: 'south' },
            ],
          },
        },
      ],
    };
    const onFinish = jest.fn();
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);
    const selector = screen.getByLabelText('大区').closest('.ant-select');
    expect(selector).toBeTruthy();
    fireEvent.mouseDown(selector as Element);
    await waitFor(() => expect(screen.getByText('华东')).toBeTruthy());
    fireEvent.click(screen.getByText('华东'));
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() =>
      expect(onFinish).toHaveBeenCalledWith(expect.objectContaining({ area: 'east' })),
    );
  });

  test('TreeSelect 多选（schema.type=array）值为数组', async () => {
    const spec: FormPresentationSpec = {
      jsonSchema: {
        type: 'object',
        properties: { areas: { type: 'array', items: { type: 'string' }, title: '多选大区' } },
      },
      fields: [
        {
          key: 'areas',
          widget: 'TreeSelect',
          widgetProps: {
            treeData: [
              { title: '华东', value: 'east' },
              { title: '华南', value: 'south' },
            ],
          },
        },
      ],
    };
    const onFinish = jest.fn();
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);
    const selector = screen.getByLabelText('多选大区').closest('.ant-select');
    expect(selector).toBeTruthy();
    fireEvent.mouseDown(selector as Element);
    await waitFor(() => expect(screen.getByText('华东')).toBeTruthy());
    fireEvent.click(screen.getByText('华东'));
    fireEvent.click(screen.getByText('华南'));
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() =>
      expect(onFinish).toHaveBeenCalledWith(expect.objectContaining({ areas: ['east', 'south'] })),
    );
  });

  test('Cascader 渲染并取最后一级值', async () => {
    const spec: FormPresentationSpec = {
      jsonSchema: { type: 'object', properties: { channel: { type: 'string', title: '渠道' } } },
      fields: [
        {
          key: 'channel',
          widget: 'Cascader',
          widgetProps: {
            cascaderOptions: [
              {
                label: '安卓',
                value: 'android',
                children: [{ label: '官方包', value: 'official' }],
              },
            ],
          },
        },
      ],
    };
    const onFinish = jest.fn();
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);
    const selector = screen.getByLabelText('渠道').closest('.ant-select');
    expect(selector).toBeTruthy();
    fireEvent.mouseDown(selector as Element);
    await waitFor(() => expect(screen.getByText('安卓')).toBeTruthy());
    fireEvent.click(screen.getByText('安卓'));
    await waitFor(() => expect(screen.getByText('官方包')).toBeTruthy());
    fireEvent.click(screen.getByText('官方包'));
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() =>
      expect(onFinish).toHaveBeenCalledWith(expect.objectContaining({ channel: 'official' })),
    );
  });

  test('Rate 渲染并产出 number', async () => {
    function Harness({ onChange }: { onChange: (v: unknown) => void }) {
      const [value, setValue] = React.useState(0);
      const props = {
        id: 'score',
        value,
        schema: { type: 'integer' },
        uiSchema: {},
        options: { count: 5 },
        formContext: {},
        registry: {} as never,
        label: '评分',
        multiple: false,
        autofocus: false,
        disabled: false,
        readonly: false,
        hideError: false,
        rawErrors: [],
        placeholder: '',
        required: false,
        onChange: (next: unknown) => {
          setValue(next as number);
          onChange(next);
        },
        onBlur: jest.fn(),
        onFocus: jest.fn(),
      } as never;
      return <RateWidget {...props} />;
    }
    const onChange = jest.fn();
    const { container } = render(<Harness onChange={onChange} />);
    const stars = container.querySelectorAll('.ant-rate-star');
    expect(stars.length).toBe(5);
    // rc-rate 的点击处理器在内层 div[role=radio]
    fireEvent.click(stars[3].querySelector('[role="radio"]') as Element);
    expect(onChange).toHaveBeenCalledWith(4);
  });
});
