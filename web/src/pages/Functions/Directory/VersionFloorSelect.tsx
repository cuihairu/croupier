import React from 'react';
import { Select, Typography } from 'antd';
import { useIntl } from '@umijs/max';

const { Text } = Typography;

type VersionFloorSelectProps = {
  /** 当前门槛值（空串/undefined = 未设置） */
  value?: string;
  /** 候选版本：只允许选历史出现过的版本（#26，服务端契约版本流聚合） */
  options: string[];
  submitting?: boolean;
  /** 允许清空（行内编辑 = 清除门槛；批量弹窗清除走独立按钮时传 false） */
  allowClear?: boolean;
  placeholder?: string;
  style?: React.CSSProperties;
  onChange: (next: string | undefined) => void;
};

// 版本门槛下拉（#26）：门槛编辑改下拉而非手输，选项来自服务端函数历史
// 版本索引——杜绝手拼不存在的版本号。数据源异常（options 为空）时仍可
// 清空，不可编造选项。
export default function VersionFloorSelect({
  value,
  options,
  submitting = false,
  allowClear = true,
  placeholder,
  style,
  onChange,
}: VersionFloorSelectProps) {
  const intl = useIntl();
  const resolvedPlaceholder =
    placeholder ??
    intl.formatMessage({
      id: 'pages.functionsDirectory.floorSelect.placeholder',
      defaultMessage: '未设置',
    });
  // 当前门槛可能来自历史手输（不在服务端历史索引里）——补进选项，避免
  // rc-select 回退成裸值显示（丢「≥ v」前缀）且无法重新选中当前值。
  const items = value && !options.includes(value) ? [value, ...options] : options;
  return (
    <Select
      size="small"
      showSearch
      allowClear={allowClear}
      disabled={submitting}
      variant="borderless"
      value={value || undefined}
      placeholder={resolvedPlaceholder}
      style={{ minWidth: 110, ...style }}
      options={items.map((v) => ({ value: v, label: `≥ v${v}` }))}
      onChange={(next) => {
        onChange(typeof next === 'string' ? next : undefined);
      }}
      notFoundContent={
        <Text type="secondary">
          {intl.formatMessage({
            id: 'pages.functionsDirectory.floorSelect.empty',
            defaultMessage: '暂无历史版本',
          })}
        </Text>
      }
    />
  );
}
