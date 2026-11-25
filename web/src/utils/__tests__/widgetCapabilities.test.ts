/** SUI-020：共享 widget 能力表测试——枚举全覆盖、关键 widget 事实与字段完整性。 */
import type { FormWidget } from '@/types/dashboard';
import {
  WIDGET_CAPABILITIES,
  widgetCapability,
  widgetCapabilityHint,
} from '../widgetCapabilities';

/** FormWidget 联合的 26 个取值，独立于被测表声明，防止表漏项时测试同步漏掉。 */
const ALL_WIDGETS: FormWidget[] = [
  'Input',
  'TextArea',
  'InputNumber',
  'Password',
  'Select',
  'MultiSelect',
  'Radio',
  'Checkbox',
  'Switch',
  'DatePicker',
  'TimePicker',
  'DateRange',
  'Upload',
  'ImageUpload',
  'FileUpload',
  'RichText',
  'Code',
  'Cascader',
  'TreeSelect',
  'Color',
  'Slider',
  'Rate',
  'JSON',
  'KeyValue',
  'Array',
  'Object',
];

/** 当前 FormPresentationEditor 提供的 12 个选项（不 import React 组件）。 */
const EDITOR_OPTIONS: FormWidget[] = [
  'Input',
  'TextArea',
  'InputNumber',
  'Password',
  'Select',
  'MultiSelect',
  'Switch',
  'DatePicker',
  'DateRange',
  'Upload',
  'Array',
  'Object',
];

describe('WIDGET_CAPABILITIES：枚举覆盖与字段完整性', () => {
  it('26 个 FormWidget 取值都有能力条目', () => {
    expect(ALL_WIDGETS).toHaveLength(26);
    expect(Object.keys(WIDGET_CAPABILITIES)).toHaveLength(26);
    for (const widget of ALL_WIDGETS) {
      expect(widgetCapability(widget)).toBeDefined();
    }
  });

  it('未知 widget 与 undefined 返回 undefined', () => {
    expect(widgetCapability(undefined)).toBeUndefined();
    expect(widgetCapability('NotAWidget')).toBeUndefined();
    expect(widgetCapabilityHint('NotAWidget')).toBeUndefined();
  });

  it('每条能力的 acceptedTypes/valueShape/fallback 均非空', () => {
    for (const widget of ALL_WIDGETS) {
      const item = widgetCapability(widget);
      expect(item).toBeDefined();
      if (!item) continue;
      expect(item.acceptedTypes.length).toBeGreaterThan(0);
      for (const type of item.acceptedTypes) expect(type.trim()).not.toBe('');
      expect(item.valueShape.trim()).not.toBe('');
      expect(item.fallback.trim()).not.toBe('');
      expect(typeof item.implemented).toBe('boolean');
    }
  });

  it('提示串包含类型、输出与回退三段', () => {
    for (const widget of ALL_WIDGETS) {
      const hint = widgetCapabilityHint(widget);
      expect(hint).toContain('可接受 schema 类型：');
      expect(hint).toContain('输出：');
      expect(hint).toContain('回退：');
    }
  });
});

describe('关键 widget 事实', () => {
  it('DateRange 与 RichText 已声明但无实现，文档化回退到 schema 类型默认控件', () => {
    for (const widget of ['DateRange', 'RichText'] as const) {
      const item = widgetCapability(widget);
      expect(item?.implemented).toBe(false);
      expect(item?.fallback).toContain('未实现');
      expect(item?.fallback).toContain('回退');
      expect(item?.rjsfName).toBeUndefined();
    }
  });

  it('Switch 映射到 checkbox，输出布尔值', () => {
    const item = widgetCapability('Switch');
    expect(item?.rjsfName).toContain('checkbox');
    expect(item?.valueShape).toContain('布尔');
    expect(item?.acceptedTypes).toContain('boolean');
  });

  it('Code 与 JSON 映射到 textarea，输出普通字符串', () => {
    for (const widget of ['Code', 'JSON'] as const) {
      const item = widgetCapability(widget);
      expect(item?.implemented).toBe(true);
      expect(item?.rjsfName).toBe('textarea');
      expect(item?.acceptedTypes).toContain('string');
      expect(item?.valueShape).toContain('字符串');
    }
    // JSON 不解析成对象；object 的根字段由 ObjectField 接管。
    expect(widgetCapability('JSON')?.valueShape).toContain('不解析');
    expect(widgetCapability('JSON')?.fallback).toContain('ObjectField');
  });

  it('Array/Object 是 renderer 管理的复合字段，KeyValue 走自定义 field', () => {
    expect(widgetCapability('Array')?.rjsfName).toContain('ArrayField');
    expect(widgetCapability('Object')?.rjsfName).toContain('ObjectField');
    expect(widgetCapability('KeyValue')?.rjsfName).toContain('keyValue');
  });
});

describe('编辑器选项', () => {
  it('当前 12 个选项都是已声明的能力子集', () => {
    expect(EDITOR_OPTIONS).toHaveLength(12);
    for (const widget of EDITOR_OPTIONS) {
      const item = widgetCapability(widget);
      expect(item).toBeDefined();
      expect(item?.valueShape).not.toBe('');
    }
    // DateRange 被编辑器暴露，但只是声明、没有渲染实现。
    expect(widgetCapability('DateRange')?.implemented).toBe(false);
  });
});
