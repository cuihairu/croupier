/**
 * F3：Select 系自定义 RJSF widgets（TreeSelect/Cascader/Rate）。
 *
 * 数据约定：树/级联选项经 FormFieldSpec.widgetProps（x-widget-props）透传——
 * TreeSelect 读 options.treeData，Cascader 读 options.cascaderOptions，
 * 避免 antd `options` 与 rjsf options prop 撞名。值语义：
 * TreeSelect 单选 string / 多选 string[]（multiple 或 schema.type=array）；
 * Cascader 值取最后一级 string；Rate 为 number。
 */

import { useState, type FocusEvent } from 'react';
import { Cascader, Rate, Select as AntdSelect, TreeSelect as AntdTreeSelect } from 'antd';
import { Widgets as AntdWidgets } from '@rjsf/antd';
import type { GenericObjectType, RJSFSchema, WidgetProps } from '@rjsf/utils';
import { useRemoteOptions } from './useRemoteOptions';
import type { RemoteOptionsSpec } from '@/types/dashboard';

type WidgetPropsAlias = WidgetProps;

type AntdTreeNode = NonNullable<React.ComponentProps<typeof AntdTreeSelect>['treeData']>[number];
type AntdCascaderOption = NonNullable<React.ComponentProps<typeof Cascader>['options']>[number];

function toValueArray(value: unknown): string[] {
  if (Array.isArray(value)) return value.map(String);
  if (typeof value === 'string' && value) return [value];
  return [];
}

function isMultiple(props: GenericObjectType, schema: RJSFSchema): boolean {
  if (typeof props.multiple === 'boolean') return props.multiple;
  return schema.type === 'array';
}

function normalizeNode(raw: unknown, labelKey: string, valueKey: string) {
  const node = (raw ?? {}) as Record<string, unknown>;
  return {
    labelValue: String(node.label ?? node.title ?? node[valueKey] ?? ''),
    valueValue: String(node[valueKey] ?? ''),
    disabled: node.disabled === true,
    children: Array.isArray(node.children) ? node.children : undefined,
  };
}

function antdTreeData(raw: unknown, labelKey = 'label', valueKey = 'value'): AntdTreeNode[] {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const { labelValue, valueValue, disabled, children } = normalizeNode(item, labelKey, valueKey);
    const next: AntdTreeNode = { title: labelValue, value: valueValue };
    if (disabled) (next as AntdTreeNode & { disabled: boolean }).disabled = true;
    if (children?.length) next.children = antdTreeData(children, labelKey, valueKey);
    return next;
  });
}

function antdCascaderOptions(
  raw: unknown,
  labelKey = 'label',
  valueKey = 'value',
): AntdCascaderOption[] {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const { labelValue, valueValue, disabled, children } = normalizeNode(item, labelKey, valueKey);
    const next: AntdCascaderOption = { label: labelValue, value: valueValue };
    if (disabled) (next as AntdCascaderOption & { disabled: boolean }).disabled = true;
    if (children?.length) next.children = antdCascaderOptions(children, labelKey, valueKey);
    return next;
  });
}

/** TreeSelect：树形下拉；单选 string / 多选 string[]。支持远程选项源（F9）。 */
export function TreeSelectWidget(widgetProps: WidgetProps) {
  const remote = ((widgetProps.options ?? {}) as GenericObjectType).remoteOptions as
    RemoteOptionsSpec | undefined;
  if (remote?.functionId) return <RemoteTreeSelectBody {...widgetProps} remote={remote} />;
  return <TreeSelectBody {...widgetProps} />;
}

function TreeSelectBody({
  disabled,
  id,
  onBlur,
  onChange,
  onFocus,
  options,
  placeholder,
  readonly,
  value,
  schema,
}: WidgetPropsAlias) {
  const props = (options ?? {}) as GenericObjectType;
  const multiple = isMultiple(props, schema);
  const handleChange = (next: unknown) => {
    onChange(multiple ? toValueArray(next) : typeof next === 'string' ? next : '');
  };
  return (
    <AntdTreeSelect
      id={id}
      style={{ width: '100%' }}
      treeData={antdTreeData(props.treeData)}
      multiple={multiple}
      disabled={disabled || readonly}
      placeholder={placeholder}
      value={multiple ? toValueArray(value) : typeof value === 'string' ? value : undefined}
      showSearch
      treeNodeFilterProp="title"
      allowClear
      onChange={handleChange}
      onBlur={(event: FocusEvent<HTMLInputElement>) => onBlur(id, value)}
      onFocus={(event: FocusEvent<HTMLInputElement>) => onFocus(id, value)}
      data-testid={id}
    />
  );
}

function RemoteTreeSelectBody({
  disabled,
  id,
  onBlur,
  onChange,
  onFocus,
  options,
  placeholder,
  readonly,
  value,
  schema,
  remote,
}: WidgetPropsAlias & { remote: RemoteOptionsSpec }) {
  const [search, setSearch] = useState('');
  const { options: remoteOptions, loading } = useRemoteOptions(remote, search);
  const props = (options ?? {}) as GenericObjectType;
  const multiple = isMultiple(props, schema);
  // 远程选项降级为平铺树（label/value 无层级语义）
  const treeData = remoteOptions.map((option) => ({ title: option.label, value: option.value }));
  return (
    <AntdTreeSelect
      id={id}
      style={{ width: '100%' }}
      treeData={treeData}
      multiple={multiple}
      loading={loading}
      disabled={disabled || readonly}
      placeholder={placeholder}
      value={multiple ? toValueArray(value) : typeof value === 'string' ? value : undefined}
      showSearch
      treeNodeFilterProp="title"
      filterTreeNode={remote.searchParam ? false : true}
      onSearch={remote.searchParam ? (text) => setSearch(text ?? '') : undefined}
      allowClear
      onChange={(next) => {
        onChange(multiple ? toValueArray(next) : typeof next === 'string' ? next : '');
      }}
      onBlur={(event: FocusEvent<HTMLInputElement>) => onBlur(id, value)}
      onFocus={(event: FocusEvent<HTMLInputElement>) => onFocus(id, value)}
      data-testid={id}
    />
  );
}

/** Cascader：级联选择；值取最后一级 string。 */
export function CascaderWidget({
  disabled,
  id,
  onBlur,
  onChange,
  onFocus,
  options,
  placeholder,
  readonly,
  value,
}: WidgetPropsAlias) {
  const props = (options ?? {}) as GenericObjectType;
  const path = toValueArray(value);
  return (
    <Cascader
      id={id}
      style={{ width: '100%' }}
      options={antdCascaderOptions(props.cascaderOptions)}
      disabled={disabled || readonly}
      placeholder={placeholder}
      value={path.length ? path : undefined}
      changeOnSelect={props.changeOnSelect === true}
      onChange={(next) => onChange(next?.length ? next[next.length - 1] : '')}
      onBlur={() => onBlur(id, value)}
      onFocus={() => onFocus(id, value)}
      data-testid={id}
    />
  );
}

/** Rate：星级评分，number 值。 */
export function RateWidget({
  disabled,
  id,
  onBlur,
  onChange,
  onFocus,
  options,
  readonly,
  value,
}: WidgetPropsAlias) {
  const props = (options ?? {}) as GenericObjectType;
  return (
    <Rate
      id={id}
      count={typeof props.count === 'number' ? props.count : 5}
      allowHalf={props.allowHalf === true}
      disabled={disabled || readonly}
      value={typeof value === 'number' ? value : 0}
      onChange={(next) => {
        onChange(next);
        onBlur(id, next);
      }}
      onFocus={() => onFocus(id, value)}
      data-testid={id}
    />
  );
}

/** SchemaFormRenderer 注册的自定义 widget 集。 */
export const customWidgets = {
  treeSelect: TreeSelectWidget,
  cascader: CascaderWidget,
  rate: RateWidget,
  // 覆盖主题内置 select：有 remoteOptions 走远程模式，否则委托内置实现
  select: RemoteSelectWidget,
} as const;

/** 远程选项下拉（F9）：有 remoteOptions 时调函数拉选项；否则委托 antd 内置 select。 */
export function RemoteSelectWidget(widgetProps: WidgetProps) {
  const remote = ((widgetProps.options ?? {}) as GenericObjectType).remoteOptions as
    RemoteOptionsSpec | undefined;
  if (!remote?.functionId) {
    const Default = AntdWidgets.SelectWidget;
    return Default ? <Default {...widgetProps} /> : null;
  }
  return <RemoteSelectBody {...widgetProps} remote={remote} />;
}

function RemoteSelectBody({
  disabled,
  id,
  multiple,
  onBlur,
  onChange,
  onFocus,
  placeholder,
  readonly,
  value,
  remote,
}: WidgetProps & { remote: RemoteOptionsSpec }) {
  const [search, setSearch] = useState('');
  const { options, loading } = useRemoteOptions(remote, search);
  const single = !multiple;
  const normalized = single
    ? typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : undefined
    : Array.isArray(value)
      ? value.map(String)
      : undefined;
  return (
    <AntdSelect
      id={id}
      style={{ width: '100%' }}
      mode={multiple ? 'multiple' : undefined}
      disabled={disabled || readonly}
      placeholder={placeholder}
      value={normalized}
      loading={loading}
      options={options}
      showSearch
      filterOption={remote.searchParam ? false : true}
      onSearch={remote.searchParam ? (text) => setSearch(text ?? '') : undefined}
      onChange={(next) => {
        if (multiple) {
          onChange(Array.isArray(next) ? next.map(String) : []);
        } else {
          onChange(typeof next === 'string' ? next : '');
        }
        onBlur(id, next);
      }}
      onFocus={() => onFocus(id, value)}
      allowClear
      data-testid={id}
    />
  );
}
