/**
 * IconPicker - 菜单图标选择器
 *
 * 选项来自 utils/menuIcon.tsx 的 MENU_ICON_NAMES（与运行期菜单图标
 * 解析器同源，选了必能渲染）；存值为图标名字符串（PageSpec.Icon 契约），
 * 支持按名称搜索与清空。
 */
import React, { useMemo } from 'react';
import { Select } from 'antd';
import { MENU_ICON_NAMES, resolveMenuIcon } from '@/utils/menuIcon';

export interface IconPickerProps {
  value?: string;
  onChange?: (value: string | undefined) => void;
  placeholder?: string;
  allowClear?: boolean;
  style?: React.CSSProperties;
}

export default function IconPicker({
  value,
  onChange,
  placeholder,
  allowClear = true,
  style,
}: IconPickerProps) {
  const options = useMemo(
    () =>
      MENU_ICON_NAMES.map((name) => ({
        value: name,
        label: (
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
            {resolveMenuIcon(name)}
            <span>{name}</span>
          </span>
        ),
      })),
    [],
  );

  return (
    <Select
      value={value || undefined}
      onChange={(v) => onChange?.(v)}
      onClear={() => onChange?.(undefined)}
      options={options}
      showSearch
      optionFilterProp="value"
      allowClear={allowClear}
      placeholder={placeholder}
      style={{ width: '100%', ...style }}
    />
  );
}
