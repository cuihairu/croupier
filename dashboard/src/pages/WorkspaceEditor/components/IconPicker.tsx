import React, { useState } from 'react';
import { Input, Popover, Space, Tooltip } from 'antd';
import * as Icons from '@ant-design/icons';

// 常用图标列表
const COMMON_ICONS = [
  'UserOutlined',
  'TeamOutlined',
  'CrownOutlined',
  'SafetyOutlined',
  'UnorderedListOutlined',
  'OrderedListOutlined',
  'TableOutlined',
  'AppstoreOutlined',
  'SettingOutlined',
  'ToolOutlined',
  'EditOutlined',
  'FormOutlined',
  'SearchOutlined',
  'FilterOutlined',
  'SortAscendingOutlined',
  'SortDescendingOutlined',
  'PlusOutlined',
  'MinusOutlined',
  'DeleteOutlined',
  'CopyOutlined',
  'CheckCircleOutlined',
  'CloseCircleOutlined',
  'InfoCircleOutlined',
  'WarningOutlined',
  'StarOutlined',
  'HeartOutlined',
  'LikeOutlined',
  'DislikeOutlined',
  'HomeOutlined',
  'ShopOutlined',
  'ShoppingCartOutlined',
  'GiftOutlined',
  'DollarOutlined',
  'MoneyCollectOutlined',
  'PayCircleOutlined',
  'WalletOutlined',
  'BarChartOutlined',
  'LineChartOutlined',
  'PieChartOutlined',
  'FundOutlined',
  'BellOutlined',
  'MailOutlined',
  'MessageOutlined',
  'CommentOutlined',
  'LockOutlined',
  'UnlockOutlined',
  'KeyOutlined',
  'EyeOutlined',
  'CloudOutlined',
  'DatabaseOutlined',
  'ApiOutlined',
  'CodeOutlined',
  'FileOutlined',
  'FolderOutlined',
  'UploadOutlined',
  'DownloadOutlined',
  'GlobalOutlined',
  'EnvironmentOutlined',
  'ClockCircleOutlined',
  'CalendarOutlined',
  'PhoneOutlined',
  'MobileOutlined',
  'TabletOutlined',
  'DesktopOutlined',
  'TrophyOutlined',
  'FireOutlined',
  'ThunderboltOutlined',
  'RocketOutlined',
];

export interface IconPickerProps {
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
}

export default function IconPicker({ value, onChange, placeholder }: IconPickerProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');

  const filtered = search
    ? COMMON_ICONS.filter((name) => name.toLowerCase().includes(search.toLowerCase()))
    : COMMON_ICONS;

  const IconComponent = value ? (Icons as any)[value] : null;

  const content = (
    <div style={{ width: 320 }}>
      <Input
        placeholder="搜索图标..."
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        style={{ marginBottom: 8 }}
        allowClear
      />
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, maxHeight: 240, overflowY: 'auto' }}>
        {filtered.map((name) => {
          const Icon = (Icons as any)[name];
          if (!Icon) return null;
          return (
            <Tooltip key={name} title={name} placement="top">
              <div
                onClick={() => {
                  onChange?.(name);
                  setOpen(false);
                  setSearch('');
                }}
                style={{
                  width: 36,
                  height: 36,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  cursor: 'pointer',
                  borderRadius: 4,
                  fontSize: 18,
                  border: value === name ? '2px solid #1677ff' : '1px solid transparent',
                  background: value === name ? '#e6f4ff' : 'transparent',
                }}
                onMouseEnter={(e) => {
                  if (value !== name)
                    (e.currentTarget as HTMLDivElement).style.background = '#f5f5f5';
                }}
                onMouseLeave={(e) => {
                  if (value !== name)
                    (e.currentTarget as HTMLDivElement).style.background = 'transparent';
                }}
              >
                <Icon />
              </div>
            </Tooltip>
          );
        })}
        {filtered.length === 0 && (
          <div style={{ color: '#999', padding: '8px 0', width: '100%', textAlign: 'center' }}>
            未找到匹配图标
          </div>
        )}
      </div>
    </div>
  );

  return (
    <Space.Compact style={{ width: '100%' }}>
      <Popover
        content={content}
        trigger="click"
        open={open}
        onOpenChange={setOpen}
        placement="bottomLeft"
      >
        <div
          style={{
            width: 36,
            height: 32,
            border: '1px solid #d9d9d9',
            borderRight: 'none',
            borderRadius: '6px 0 0 6px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            cursor: 'pointer',
            background: '#fafafa',
            fontSize: 16,
            flexShrink: 0,
          }}
        >
          {IconComponent ? <IconComponent /> : <Icons.AppstoreOutlined style={{ color: '#bbb' }} />}
        </div>
      </Popover>
      <Input
        value={value}
        onChange={(e) => onChange?.(e.target.value)}
        placeholder={placeholder || '选择或输入图标名，如: UserOutlined'}
        allowClear
      />
    </Space.Compact>
  );
}
