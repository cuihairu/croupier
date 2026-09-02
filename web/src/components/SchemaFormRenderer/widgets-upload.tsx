/**
 * F4：Upload 系（Upload/ImageUpload/FileUpload 共用）与 KeyValue 自定义 RJSF 扩展。
 *
 * 值语义：Upload 系为 URL 字符串（schema.type=string，单值）或 URL 字符串数组
 * （schema.type=array）；上传完成（status=done）后取 file.url ?? response?.url，
 * 无 action/失败（error 状态）不计入值。
 * KeyValue 值为 object——object 类型在 rjsf 中走 ObjectField（忽略 ui:widget），
 * 故注册为自定义 field（ui:field: 'keyValue'），见 applyFieldPresentation。
 */

import { useEffect, useRef, useState } from 'react';
import { Button, Input, Space, Upload as AntdUpload } from 'antd';
import type { UploadFile } from 'antd';
import type { FieldProps, GenericObjectType, UiSchema, WidgetProps } from '@rjsf/utils';
import { getUiOptions } from '@rjsf/utils';

type WidgetPropsAlias = WidgetProps;

type JSONValue = string | number | boolean | null | JSONValue[] | { [key: string]: JSONValue };

function isObject(value: unknown): value is Record<string, JSONValue> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function toValueArray(value: unknown): string[] {
  if (Array.isArray(value)) return value.map(String).filter(Boolean);
  if (typeof value === 'string' && value) return [value];
  return [];
}

function urlToName(url: string): string {
  const path = url.split(/[?#]/)[0];
  const last = path.slice(path.lastIndexOf('/') + 1);
  return decodeURIComponent(last) || url;
}

/** antd Upload fileList → 组件值；仅收集 done 且可提取 URL 的文件。 */
export function uploadValueFromFileList(
  fileList: UploadFile[],
  multiple: boolean,
): string | string[] | undefined {
  const urls: string[] = [];
  for (const item of fileList) {
    if (item.status !== 'done') continue;
    const response = item.response as { url?: unknown } | string | undefined;
    const url =
      (typeof item.url === 'string' && item.url) ||
      (response && typeof response === 'object' && typeof response.url === 'string'
        ? response.url
        : undefined) ||
      (typeof response === 'string' ? response : undefined);
    if (url) urls.push(url);
  }
  if (!multiple) return urls[0] ?? '';
  return urls;
}

function valueToFileList(value: unknown, multiple: boolean): UploadFile[] {
  return toValueArray(value).map((url, index) => ({
    uid: `${index}-${url}`,
    name: urlToName(url),
    url,
    status: 'done' as const,
  }));
}

/** Upload：值为 URL（string 或 string[]）；上传端点等经 widgetProps.action 配置。 */
export function UploadWidget({
  disabled,
  id,
  onChange,
  options,
  placeholder,
  readonly,
  value,
  schema,
}: WidgetPropsAlias) {
  const props = (options ?? {}) as GenericObjectType;
  const multiple = props.multiple === true || schema.type === 'array';
  const action = typeof props.action === 'string' ? props.action : undefined;
  const listType = typeof props.listType === 'string' ? props.listType : 'text';
  const maxCount = typeof props.maxCount === 'number' ? props.maxCount : undefined;
  const accept = typeof props.accept === 'string' ? props.accept : undefined;

  const handleChange = (info: { fileList: UploadFile[] }) => {
    const next = uploadValueFromFileList(info.fileList, multiple);
    if (next !== undefined) onChange(next);
  };

  return (
    <AntdUpload
      id={id}
      action={action}
      accept={accept}
      maxCount={maxCount}
      multiple={multiple}
      listType={listType === 'picture' || listType === 'picture-card' ? listType : 'text'}
      fileList={valueToFileList(value, multiple)}
      disabled={disabled || readonly}
      onChange={handleChange}
      data-testid={id}
    >
      <Button disabled={disabled || readonly} type="default">
        {placeholder || '上传'}
      </Button>
    </AntdUpload>
  );
}

interface KeyValueRow {
  key: string;
  value: string;
}

function rowsFromValue(value: unknown): KeyValueRow[] {
  if (!isObject(value)) return [];
  return Object.entries(value).map(([key, v]) => ({
    key,
    value: typeof v === 'string' ? v : JSON.stringify(v),
  }));
}

/** KeyValue：键值对编辑器 field；值为 object（输入按 string，游戏方按需解析）。 */
export function KeyValueField({
  disabled,
  fieldPathId,
  idSchema,
  onChange,
  readonly,
  schema,
  uiSchema,
  formData,
}: FieldProps) {
  const id = idSchema?.$id ?? 'keyValue';
  const uiOptions = getUiOptions(uiSchema as UiSchema) as GenericObjectType;
  const placeholder = typeof uiOptions.placeholder === 'string' ? uiOptions.placeholder : '';
  const value = formData;
  const [rows, setRows] = useState<KeyValueRow[]>(() => rowsFromValue(value));
  const committedRef = useRef(JSON.stringify(isObject(value) ? value : {}));

  useEffect(() => {
    const external = JSON.stringify(isObject(value) ? value : {});
    if (external !== committedRef.current) {
      committedRef.current = external;
      setRows(rowsFromValue(value));
    }
  }, [value]);

  const commit = (nextRows: KeyValueRow[]) => {
    const obj: Record<string, JSONValue> = {};
    for (const row of nextRows) {
      const key = row.key.trim();
      if (key) obj[key] = row.value;
    }
    committedRef.current = JSON.stringify(obj);
    setRows(nextRows);
    onChange(obj, fieldPathId?.path ?? []);
  };

  const updateRow = (index: number, patch: Partial<KeyValueRow>) => {
    commit(rows.map((row, i) => (i === index ? { ...row, ...patch } : row)));
  };

  void schema;

  return (
    <Space orientation="vertical" style={{ width: '100%' }} data-testid={id}>
      {rows.map((row, index) => (
        <Space.Compact key={index} style={{ width: '100%' }} data-testid={`${id}-row-${index}`}>
          <Input
            style={{ width: '40%' }}
            placeholder="键"
            value={row.key}
            disabled={disabled || readonly}
            onChange={(e) => updateRow(index, { key: e.target.value })}
          />
          <Input
            style={{ width: '60%' }}
            placeholder={placeholder || '值'}
            value={row.value}
            disabled={disabled || readonly}
            onChange={(e) => updateRow(index, { value: e.target.value })}
          />
          <Button
            disabled={disabled || readonly}
            onClick={() => commit(rows.filter((_, i) => i !== index))}
          >
            删除
          </Button>
        </Space.Compact>
      ))}
      <Button
        disabled={disabled || readonly}
        onClick={() => commit([...rows, { key: '', value: '' }])}
        data-testid={`${id}-add`}
      >
        添加
      </Button>
    </Space>
  );
}

/** Upload widget 注册集（ui:widget: 'upload'）。 */
export const uploadWidgets = {
  upload: UploadWidget,
} as const;

/** KeyValue 自定义 field 注册集（ui:field: 'keyValue'）。 */
export const uploadFields = {
  keyValue: KeyValueField,
} as const;
