/**
 * F3：Select 系自定义 RJSF widgets（TreeSelect/Cascader/Rate）。
 *
 * 数据约定：树/级联选项经 FormFieldSpec.widgetProps（x-widget-props）透传——
 * TreeSelect 读 options.treeData，Cascader 读 options.cascaderOptions，
 * 避免 antd `options` 与 rjsf options prop 撞名。值语义：
 * TreeSelect 单选 string / 多选 string[]（multiple 或 schema.type=array）；
 * Cascader 值取最后一级 string；Rate 为 number。
 *
 * SUI-022：date / date-time / time widget 覆盖（DateWidget/DateTimeWidget/
 * TimeWidget）——antd 主题内置 DateWidget 把 date-time 序列化成 UTC `Z`
 * 串（丢弃本地显式偏移）、清空透传 null（滞留 formData 且撞 AJV
 * type:string）；core 的 TimeWidget（antd 主题无 time widget）`${value}:00`
 * 拼接对 Dayjs 产出非法串。覆盖实现见下方三个导出与 @/utils/datetime。
 */

import { useState } from 'react';
import {
  Cascader,
  DatePicker,
  Rate,
  Select as AntdSelect,
  TimePicker,
  TreeSelect as AntdTreeSelect,
} from 'antd';
import { Widgets as AntdWidgets } from '@rjsf/antd';
import type { Dayjs } from 'dayjs';
import type { GenericObjectType, RJSFSchema, WidgetProps } from '@rjsf/utils';
import { ariaDescribedByIds } from '@rjsf/utils';
import { useRemoteOptions } from './useRemoteOptions';
import type { RemoteOptionsSpec } from '@/types/dashboard';
import { primarySchemaType } from '@/utils/schemaDialect';
import {
  parseDateTimeSeed,
  parseTimeSeed,
  serializeDateValue,
  serializeDateTimeValue,
  serializeTimeValue,
} from '@/utils/datetime';

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
  // SUI-009：type 数组（可空/联合）以主类型判定，`["array","null"]` 仍是多值。
  return primarySchemaType(schema) === 'array';
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
      onBlur={() => onBlur(id, value)}
      onFocus={() => onFocus(id, value)}
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
      onBlur={() => onBlur(id, value)}
      onFocus={() => onFocus(id, value)}
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

// ---------------------------------------------------------------------------
// SUI-022：date / date-time / time widget 覆盖
//
// registry key 与 rjsf getWidget 的 widgetMap 一致（DateWidget /
// DateTimeWidget / TimeWidget），一次注册同时覆盖两条路径：schema.format
// 缺省路径（format: date / date-time / time → 同名 registry key）与
// x-widget 路径（widgetToRjsf：DatePicker→'date'、TimePicker→'time'）。
//
// 数据约定：date 提交 `YYYY-MM-DD`（本地日历日，无时间无偏移）；
// date-time 提交带显式本地偏移的 RFC3339（`YYYY-MM-DDTHH:mm:ssZ` token，
// 如 `2026-01-31T08:00:00+08:00`——不再像内置实现那样平移到 UTC `Z`）；
// time 提交 `HH:mm:ss`（不再像 core TimeWidget 那样拼 `:00`）。
// 清空一律产出 undefined（rjsf v6 只认 undefined 移除键，null 会滞留）。
// 显示种子经 @/utils/datetime 的归一：naive 串按本地墙钟、带偏移串按瞬时。
// ---------------------------------------------------------------------------

/** 日期/时刻 picker 共用本体：showTime 决定序列化契约（SUI-022）。 */
type DayPickerWidgetProps = WidgetProps & { showTime?: boolean };

function DayPickerBody({
  disabled,
  id,
  onBlur,
  onChange,
  onFocus,
  placeholder,
  readonly,
  registry,
  value,
  showTime = false,
}: DayPickerWidgetProps) {
  const { readonlyAsDisabled = true } = registry.formContext ?? {};
  const handleChange = (next: Dayjs | null) =>
    onChange(next ? (showTime ? serializeDateTimeValue(next) : serializeDateValue(next)) : undefined);
  return (
    <DatePicker
      disabled={disabled || (readonlyAsDisabled && readonly)}
      getPopupContainer={(node) => node.parentNode as HTMLElement}
      id={id}
      name={id}
      onBlur={!readonly ? () => onBlur(id, value) : undefined}
      onChange={!readonly ? handleChange : undefined}
      onFocus={!readonly ? () => onFocus(id, value) : undefined}
      placeholder={placeholder}
      showTime={showTime}
      style={{ width: '100%' }}
      value={parseDateTimeSeed(value)}
      aria-describedby={ariaDescribedByIds(id)}
    />
  );
}

/** DateWidget 覆盖：format:date / x-widget DatePicker → 本地 `YYYY-MM-DD`；清空 → undefined（键移除）。 */
export function DateWidget(props: WidgetProps) {
  return <DayPickerBody {...props} />;
}

/** DateTimeWidget 覆盖：format:date-time → 带显式本地偏移的 RFC3339；清空 → undefined。 */
export function DateTimeWidget(props: WidgetProps) {
  return <DayPickerBody {...props} showTime />;
}

/** TimeWidget 覆盖：antd 主题无 time widget（回退 core 的 `${value}:00` 拼接对
 *  Dayjs 产出非法串）——antd TimePicker 实现：`HH:mm:ss`，清空 → undefined。 */
export function TimeWidget({
  disabled,
  id,
  onBlur,
  onChange,
  onFocus,
  placeholder,
  readonly,
  value,
}: WidgetProps) {
  const handleChange = (next: Dayjs | null) => {
    onChange(next ? serializeTimeValue(next) : undefined);
  };
  return (
    <TimePicker
      id={id}
      style={{ width: '100%' }}
      disabled={disabled || readonly}
      placeholder={placeholder}
      allowClear
      value={parseTimeSeed(value)}
      onChange={!readonly ? handleChange : undefined}
      onBlur={() => onBlur(id, value)}
      onFocus={() => onFocus(id, value)}
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
  // SUI-022：date/date-time/time 覆盖（registry key 与 rjsf widgetMap 一致，
  // format 缺省路径与 x-widget 路径同一次注册全收）
  DateWidget,
  DateTimeWidget,
  TimeWidget,
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
