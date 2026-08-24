/**
 * LocalizedTextEditor - 多语言文本编辑器（紧凑型）
 *
 * 契约（types/dashboard.ts LocalizedText）：key 为 BCP47 locale，与后端
 * spec.LocalizedText 一致。默认提供 zh-CN / en-US 两种语言行，可通过
 * "管理语言"气泡启用任意其他 BCP47 locale（ja-JP / ko-KR / ...）或输入
 * 自定义 locale。
 *
 * 布局：单行 [locale 切换][输入框][管理语言按钮]，适配 inline 表单；
 * placeholder 为主 locale 文案，方便翻译对照。
 */

import React, { useMemo, useState } from 'react';
import { Button, Input, Popover, Select, Space, Tag, Typography } from 'antd';
import { CheckOutlined, CloseOutlined, GlobalOutlined } from '@ant-design/icons';
import type { LocalizedText } from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';

const { Text } = Typography;

/** 内置常用 BCP47 locale（管理面板候选；zh-CN 为必选基线不可移除） */
export const COMMON_LOCALES: Array<{ value: string; label: string }> = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'zh-TW', label: '繁體中文' },
  { value: 'en-US', label: 'English (US)' },
  { value: 'en-GB', label: 'English (UK)' },
  { value: 'ja-JP', label: '日本語' },
  { value: 'ko-KR', label: '한국어' },
  { value: 'ru-RU', label: 'Русский' },
  { value: 'es-ES', label: 'Español' },
  { value: 'pt-BR', label: 'Português (Brasil)' },
  { value: 'fr-FR', label: 'Français' },
  { value: 'de-DE', label: 'Deutsch' },
  { value: 'it-IT', label: 'Italiano' },
  { value: 'tr-TR', label: 'Türkçe' },
  { value: 'vi-VN', label: 'Tiếng Việt' },
  { value: 'th-TH', label: 'ไทย' },
  { value: 'id-ID', label: 'Bahasa Indonesia' },
  { value: 'hi-IN', label: 'हिन्दी' },
  { value: 'ar-SA', label: 'العربية' },
];

export const DEFAULT_LOCALES = ['zh-CN', 'en-US'];
/** 必选基线语言（不可移除） */
export const REQUIRED_LOCALE = 'zh-CN';

// 粗粒度 BCP47 校验：language-REGION / language-SCRIPT-REGION
const BCP47_PATTERN = /^[a-z]{2,3}(-[A-Za-z]{4})?(-[A-Z]{2}|[0-9]{3})$/i;

export interface LocalizedTextEditorProps {
  value: LocalizedText | undefined;
  onChange: (value: LocalizedText) => void;
  /** 输入框占位（默认取主 locale 文案做翻译对照） */
  placeholder?: string;
  size?: 'small' | 'middle';
  disabled?: boolean;
  /** 初始编辑的语言 */
  defaultLocale?: string;
  style?: React.CSSProperties;
}

export default function LocalizedTextEditor({
  value,
  onChange,
  placeholder,
  size = 'middle',
  disabled = false,
  defaultLocale = 'zh-CN',
  style,
}: LocalizedTextEditorProps) {
  const present = useMemo(() => {
    const keys = Object.keys(value || {}).filter((k) => (value || {})[k] !== undefined);
    return keys;
  }, [value]);

  const [activeLocale, setActiveLocale] = useState<string>(() => {
    if (present.includes(defaultLocale)) return defaultLocale;
    if (present.includes('zh-CN')) return 'zh-CN';
    return present[0] || 'zh-CN';
  });
  const [manageOpen, setManageOpen] = useState(false);
  const [customLocale, setCustomLocale] = useState('');

  // 可切换语言 = 已存在 ∪ 默认两语言
  const selectableLocales = useMemo(() => {
    const set = new Set<string>([...present, ...DEFAULT_LOCALES]);
    return Array.from(set);
  }, [present]);

  const setText = (locale: string, text: string) => {
    onChange({ ...(value || {}), [locale]: text });
  };

  const addLocale = (locale: string) => {
    if (!locale || (value || {})[locale] !== undefined) return;
    onChange({ ...(value || {}), [locale]: '' });
    setActiveLocale(locale);
  };

  const removeLocale = (locale: string) => {
    if (locale === REQUIRED_LOCALE) return;
    const next = { ...(value || {}) };
    delete next[locale];
    onChange(next);
    if (activeLocale === locale) setActiveLocale(REQUIRED_LOCALE);
  };

  const primaryHint = localizedText(value, 'zh-CN', '');

  const manageContent = (
    <div style={{ width: 280 }}>
      <Space direction="vertical" size={4} style={{ width: '100%' }}>
        {COMMON_LOCALES.map((item) => {
          const enabled = present.includes(item.value);
          return (
            <Space
              key={item.value}
              style={{ width: '100%', justifyContent: 'space-between' }}
              onClick={() => (enabled ? removeLocale(item.value) : addLocale(item.value))}
            >
              <Space size={8} style={{ cursor: 'pointer' }}>
                {enabled ? (
                  <CheckOutlined style={{ color: '#1677ff' }} />
                ) : (
                  <span style={{ display: 'inline-block', width: 14 }} />
                )}
                <Text>{item.label}</Text>
                <Tag style={{ marginInlineEnd: 0 }}>{item.value}</Tag>
              </Space>
              {enabled && item.value !== REQUIRED_LOCALE ? (
                <Button
                  type="text"
                  size="small"
                  danger
                  icon={<CloseOutlined />}
                  onClick={(e) => {
                    e.stopPropagation();
                    removeLocale(item.value);
                  }}
                />
              ) : null}
            </Space>
          );
        })}
        <Space.Compact style={{ width: '100%', marginTop: 8 }}>
          <Input
            size="small"
            placeholder="自定义 BCP47，如 nl-NL"
            value={customLocale}
            onChange={(e) => setCustomLocale(e.target.value)}
            onPressEnter={() => {
              if (BCP47_PATTERN.test(customLocale.trim())) {
                addLocale(customLocale.trim());
                setCustomLocale('');
              }
            }}
          />
          <Button
            size="small"
            disabled={!BCP47_PATTERN.test(customLocale.trim())}
            onClick={() => {
              addLocale(customLocale.trim());
              setCustomLocale('');
            }}
          >
            添加
          </Button>
        </Space.Compact>
        <Text type="secondary" style={{ fontSize: 12 }}>
          后端契约为 BCP47 locale 键；zh-CN 为必选基线。
        </Text>
      </Space>
    </div>
  );

  return (
    <Space.Compact style={{ width: '100%', ...style }}>
      <Select
        size={size}
        disabled={disabled}
        value={selectableLocales.includes(activeLocale) ? activeLocale : REQUIRED_LOCALE}
        onChange={setActiveLocale}
        style={{ width: 110 }}
        options={selectableLocales.map((locale) => ({
          value: locale,
          label: locale,
        }))}
      />
      <Input
        size={size}
        disabled={disabled}
        value={(value || {})[activeLocale] ?? ''}
        placeholder={placeholder || primaryHint}
        onChange={(e) => setText(activeLocale, e.target.value)}
      />
      <Popover
        content={manageContent}
        trigger="click"
        open={manageOpen}
        onOpenChange={setManageOpen}
        placement="bottomRight"
      >
        <Button size={size} disabled={disabled} icon={<GlobalOutlined />} />
      </Popover>
    </Space.Compact>
  );
}
