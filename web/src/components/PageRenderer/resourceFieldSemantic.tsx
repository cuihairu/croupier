/**
 * 字段语义渲染（D1 列表/详情共用）。
 *
 * 列表列与详情项共用同一套业务语义渲染：enum+render:'tag' → 彩色本地化
 * Tag、boolean → 是/否 Tag、datetime/date → 带时区标签格式化、number+unit
 * → 单位后缀、render:'copy' → 可复制文本。详情抽屉按 key 命中列表列复用
 * 其语义元数据（enum/unit/datetime/render），避免两处描述漂移。
 *
 * 无匹配返回 undefined，调用方走各自默认渲染（ProTable valueEnum /
 * renderJSONValueSummary）。
 *
 * @module components/PageRenderer/resourceFieldSemantic
 */

import React from 'react';
import { Tag, Typography } from 'antd';
import { getIntl } from '@umijs/max';
import { localizedText } from '@/utils/localizedText';
import { formatDateTime } from '@/utils/format';
import type { EnumOption, LocalizedText } from '@/types/dashboard';

/** 语义渲染所需的最小字段描述（列表列与详情项同构） */
export interface FieldSemanticInput {
  key?: string;
  dataType?: string;
  enum?: EnumOption[];
  render?: string;
  unit?: LocalizedText;
}

/**
 * 按字段语义渲染值。导出纯函数无法 useIntl：getIntl 调用点求值。
 * @param field 字段描述（enum/render/unit/dataType）
 * @param value 字段值
 * @returns 语义渲染节点；无匹配返回 undefined（走默认渲染）
 */
export function renderFieldSemantic(
  field: FieldSemanticInput,
  value: unknown,
): React.ReactNode | undefined {
  if (value === undefined || value === null || value === '') {
    return '-';
  }

  const stringValue = String(value);
  const unitText = field.unit ? localizedText(field.unit, 'zh-CN', '') : '';

  if (field.enum?.length) {
    const opt = field.enum.find((e) => e.value === stringValue);
    if (opt) {
      if (field.render !== 'tag') {
        return <Typography.Text>{localizedText(opt.label, 'zh-CN', stringValue)}</Typography.Text>;
      }
      return (
        <Tag color={opt.color || 'default'}>{localizedText(opt.label, 'zh-CN', stringValue)}</Tag>
      );
    }
    return stringValue;
  }

  if (field.render === 'copy') {
    return <Typography.Text copyable>{stringValue}</Typography.Text>;
  }

  if (field.dataType === 'boolean') {
    const intl = getIntl();
    return value ? (
      <Tag color="success">
        {intl.formatMessage({
          id: 'component.pageRenderer.resultView.booleanTrue',
          defaultMessage: '是',
        })}
      </Tag>
    ) : (
      <Tag color="default">
        {intl.formatMessage({
          id: 'component.pageRenderer.resultView.booleanFalse',
          defaultMessage: '否',
        })}
      </Tag>
    );
  }

  if (field.dataType === 'datetime' || field.dataType === 'date') {
    return formatDateTime(stringValue, true);
  }

  if (field.dataType === 'number' && unitText) {
    return `${stringValue} ${unitText}`;
  }

  return undefined;
}

/** 合并列表列与详情项语义：详情项显式声明优先，未声明回落列表列 */
export function mergeFieldSemantic(
  col: FieldSemanticInput | undefined,
  field: FieldSemanticInput,
): FieldSemanticInput {
  if (!col) return field;
  return {
    key: field.key,
    dataType: field.dataType ?? col.dataType,
    enum: field.enum ?? col.enum,
    render: field.render ?? col.render,
    unit: field.unit ?? col.unit,
  };
}
