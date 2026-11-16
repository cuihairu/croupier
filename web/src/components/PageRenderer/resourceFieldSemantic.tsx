/**
 * 字段语义渲染（D1 列表/详情共用）。
 *
 * 列表列与详情项共用同一套业务语义渲染：enum+render:'tag' → 彩色本地化
 * Tag、boolean → 是/否 Tag、datetime/date → 带时区标签格式化、number+unit
 * → 单位后缀、render:'copy' → 可复制文本。详情抽屉按 key 命中列表列复用
 * 其语义元数据（enum/unit/datetime/render），避免两处描述漂移。
 *
 * 未知与无效显式标注（GUX-005/006）：未声明的枚举值不伪装成普通文本，
 * 无效时间不输出 Invalid Date 拼时区；枚举文案与单位跟随当前界面 locale
 * （GUX-007），不再固定 zh-CN。
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

  const intl = getIntl();
  // umi 只部署 zh-CN / en-US 两套文案；其余 locale 按默认 zh 兜底。
  const locale = intl.locale === 'en-US' ? 'en-US' : 'zh-CN';
  const stringValue = String(value);
  const unitText = field.unit ? localizedText(field.unit, locale, '') : '';

  if (field.enum?.length) {
    const opt = field.enum.find((e) => e.value === stringValue);
    if (opt) {
      const label = localizedText(opt.label, locale, stringValue);
      if (field.render !== 'tag') {
        return <Typography.Text>{label}</Typography.Text>;
      }
      return <Tag color={opt.color || 'default'}>{label}</Tag>;
    }
    // GUX-005：枚举集合外的值显式标注未知——文案保留原始值便于排查，
    // 且不只依赖颜色（Tag 与文字两种渲染都带完整「未知」字样）。
    const label = intl.formatMessage(
      {
        id: 'component.pageRenderer.fieldSemantic.unknownEnumValue',
        defaultMessage: '未知（{value}）',
      },
      { value: stringValue },
    );
    return field.render === 'tag' ? (
      <Tag color="default">{label}</Tag>
    ) : (
      <Typography.Text type="warning">{label}</Typography.Text>
    );
  }

  if (field.render === 'copy') {
    return <Typography.Text copyable>{stringValue}</Typography.Text>;
  }

  if (field.dataType === 'boolean') {
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
    // GUX-006：无效值显式标注（new Date 不抛错，NaN 时间会漏成
    // "Invalid Date (UTC±xx)" 拼接），空值已在入口统一 '-'。
    if (Number.isNaN(new Date(stringValue).getTime())) {
      const label = intl.formatMessage(
        {
          id: 'component.pageRenderer.fieldSemantic.invalidDateTime',
          defaultMessage: '无效时间（{value}）',
        },
        { value: stringValue },
      );
      return <Typography.Text type="warning">{label}</Typography.Text>;
    }
    return formatDateTime(stringValue, true);
  }

  if (field.dataType === 'number' && unitText) {
    // GUX-007：资产/数量单位跟随当前界面 locale；单位是声明侧给的展示
    // 语义（不做换算），未声明 unit 的 number 走默认渲染不加单位。
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
