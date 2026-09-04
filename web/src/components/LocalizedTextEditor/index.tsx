/**
 * LocalizedTextEditor - 多语言文本编辑器（紧凑型）
 *
 * 契约（types/dashboard.ts LocalizedText）：key 为 BCP47 locale，与后端
 * spec.LocalizedText 一致。
 *
 * 语言选项统一来自全局支持列表（src/locales/supported.ts，与界面语言
 * 切换器同源），下拉直接展示全集；已录入文案的语言带 ✓ 标记，当前
 * 界面语言默认选中。全局之外的语言（如 ru-RU）通过 🌐 气泡添加
 * 自定义 BCP47 locale。
 *
 * 布局：单行 [locale 切换][输入框][🌐]，适配 inline 表单；
 * placeholder 为主语言文案，方便翻译对照。
 */

import React, { useMemo, useState } from 'react';
import { Button, Input, Popover, Select, Space, Typography } from 'antd';
import { GlobalOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import type { LocalizedText } from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';
import { REQUIRED_LOCALE, SUPPORTED_LOCALES, SUPPORTED_LOCALE_LABELS } from '@/locales/supported';

const { Text } = Typography;

// 粗粒度 BCP47 校验：language-REGION / language-SCRIPT-REGION
const BCP47_PATTERN = /^[a-z]{2,3}(-[A-Za-z]{4})?(-[A-Z]{2}|[0-9]{3})$/i;

export interface LocalizedTextEditorProps {
  /** 缺省时由 antd Form.Item 注入（受控模式） */
  value?: LocalizedText;
  onChange?: (value: LocalizedText) => void;
  /** 输入框占位（默认取主 locale 文案做翻译对照） */
  placeholder?: string;
  size?: 'small' | 'middle';
  disabled?: boolean;
  /** 初始编辑的语言（默认跟随当前界面语言） */
  defaultLocale?: string;
  style?: React.CSSProperties;
}

export default function LocalizedTextEditor({
  value,
  onChange,
  placeholder,
  size = 'middle',
  disabled = false,
  defaultLocale,
  style,
}: LocalizedTextEditorProps) {
  const { locale: uiLocale } = useIntl();

  const present = useMemo(
    () => Object.keys(value || {}).filter((k) => (value || {})[k] !== undefined),
    [value],
  );

  const [activeLocale, setActiveLocale] = useState<string>(() => {
    if (defaultLocale && present.includes(defaultLocale)) return defaultLocale;
    if (present.includes(uiLocale)) return uiLocale;
    if (present.includes(REQUIRED_LOCALE)) return REQUIRED_LOCALE;
    return present[0] || REQUIRED_LOCALE;
  });
  const [popoverOpen, setPopoverOpen] = useState(false);
  const [customLocale, setCustomLocale] = useState('');

  // 下拉 = 全局支持语言 ∪ 已录入的自定义语言
  const selectableLocales = useMemo(() => {
    const extra = present.filter((k) => !SUPPORTED_LOCALE_LABELS[k]);
    return [...SUPPORTED_LOCALES.map((item) => item.value), ...extra];
  }, [present]);

  const setText = (locale: string, text: string) => {
    onChange?.({ ...(value || {}), [locale]: text });
  };

  const addCustomLocale = (locale: string) => {
    if (!locale || (value || {})[locale] !== undefined) return;
    onChange?.({ ...(value || {}), [locale]: '' });
    setActiveLocale(locale);
  };

  const primaryHint = localizedText(value, REQUIRED_LOCALE, '');

  const customContent = (
    <div style={{ width: 280 }}>
      <Space orientation="vertical" size={4} style={{ width: '100%' }}>
        <Text type="secondary" style={{ fontSize: 12 }}>
          下方列出平台界面支持的全部 {SUPPORTED_LOCALES.length} 种语言。需要界面之外的语言 （如
          ru-RU、ko-KR）时，在此输入自定义 BCP47 locale：
        </Text>
        <Space.Compact style={{ width: '100%', marginTop: 4 }}>
          <Input
            size="small"
            placeholder="自定义 BCP47，如 ko-KR"
            value={customLocale}
            onChange={(e) => setCustomLocale(e.target.value)}
            onPressEnter={() => {
              if (BCP47_PATTERN.test(customLocale.trim())) {
                addCustomLocale(customLocale.trim());
                setCustomLocale('');
                setPopoverOpen(false);
              }
            }}
          />
          <Button
            size="small"
            disabled={!BCP47_PATTERN.test(customLocale.trim())}
            onClick={() => {
              addCustomLocale(customLocale.trim());
              setCustomLocale('');
              setPopoverOpen(false);
            }}
          >
            添加
          </Button>
        </Space.Compact>
        <Text type="secondary" style={{ fontSize: 12 }}>
          后端契约为 BCP47 locale 键；{REQUIRED_LOCALE} 为必选基线。清除输入框内容即删除该语言文案。
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
        style={{ minWidth: 130 }}
        options={selectableLocales.map((locale) => ({
          value: locale,
          label: (
            <Space size={4}>
              <span>{SUPPORTED_LOCALE_LABELS[locale] || locale}</span>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {locale}
              </Text>
              {(value || {})[locale] ? <span style={{ color: '#52c41a' }}>✓</span> : null}
            </Space>
          ),
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
        content={customContent}
        trigger="click"
        open={popoverOpen}
        onOpenChange={setPopoverOpen}
        placement="bottomRight"
      >
        <Button size={size} disabled={disabled} icon={<GlobalOutlined />} />
      </Popover>
    </Space.Compact>
  );
}
