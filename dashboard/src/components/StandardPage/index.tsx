import React from 'react';
import { Alert, Badge, Card, Space, Typography } from 'antd';

export const DASHBOARD_PAGE_TOKENS = {
  cardPadding: 20,
  compactCardPadding: 12,
  heroCardPadding: 24,
  cardRadius: 16,
  compactRadius: 12,
  chipRadius: 999,
  sectionGap: 16,
  itemGap: 10,
  mutedTextSize: 12,
  softSurface: 'rgba(0,0,0,0.03)',
  filterSurface: 'rgba(0,0,0,0.025)',
  infoGradient: 'linear-gradient(180deg, rgba(22,119,255,0.04) 0%, rgba(22,119,255,0) 100%)',
} as const;

type SummaryItem = {
  color?: string;
  text: string;
};

export function SummaryOverview({
  title,
  description,
  items,
  hint,
  hintType = 'info',
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  items: SummaryItem[];
  hint?: React.ReactNode;
  hintType?: 'success' | 'info' | 'warning' | 'error';
}) {
  return (
    <Card
      styles={{
        body: {
          padding: DASHBOARD_PAGE_TOKENS.cardPadding,
          background: DASHBOARD_PAGE_TOKENS.infoGradient,
        },
      }}
    >
      <Space direction="vertical" size={DASHBOARD_PAGE_TOKENS.sectionGap} style={{ width: '100%' }}>
        <Space direction="vertical" size={6} style={{ width: '100%' }}>
          <Typography.Title level={4} style={{ margin: 0 }}>
            {title}
          </Typography.Title>
          {description ? <Typography.Text type="secondary">{description}</Typography.Text> : null}
        </Space>
        <Space wrap size={[10, 10]}>
          {items.map((item) => (
            <div
              key={`${item.color || 'default'}-${item.text}`}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 8,
                minHeight: 36,
                padding: '6px 12px',
                borderRadius: DASHBOARD_PAGE_TOKENS.chipRadius,
                background: DASHBOARD_PAGE_TOKENS.softSurface,
              }}
            >
              <Badge color={item.color} />
              <Typography.Text strong>{item.text}</Typography.Text>
            </div>
          ))}
        </Space>
        {hint ? <Alert type={hintType} showIcon message={hint} /> : null}
      </Space>
    </Card>
  );
}

export function StandardListSection({
  title,
  extra,
  resultText,
  children,
}: {
  title: React.ReactNode;
  extra?: React.ReactNode;
  resultText?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <Card
      title={
        <Typography.Title level={5} style={{ margin: 0 }}>
          {title}
        </Typography.Title>
      }
      extra={extra}
      styles={{
        body: {
          paddingTop: 18,
          paddingLeft: DASHBOARD_PAGE_TOKENS.cardPadding,
          paddingRight: DASHBOARD_PAGE_TOKENS.cardPadding,
          paddingBottom: DASHBOARD_PAGE_TOKENS.cardPadding,
        },
      }}
    >
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        {resultText ? <Typography.Text type="secondary">{resultText}</Typography.Text> : null}
        {children}
      </Space>
    </Card>
  );
}

export function StandardFilterBar({
  controls,
  resultText,
}: {
  controls: React.ReactNode;
  resultText?: React.ReactNode;
}) {
  return (
    <Space
      style={{
        width: '100%',
        justifyContent: 'space-between',
        padding: DASHBOARD_PAGE_TOKENS.compactCardPadding,
        borderRadius: DASHBOARD_PAGE_TOKENS.compactRadius,
        background: DASHBOARD_PAGE_TOKENS.filterSurface,
      }}
      wrap
    >
      <Space wrap size={[8, 8]}>
        {controls}
      </Space>
      {resultText ? <Typography.Text type="secondary">{resultText}</Typography.Text> : null}
    </Space>
  );
}

const PAGE_STATE_THEME: Record<
  'success' | 'info' | 'warning' | 'error',
  {
    accent: string;
    badgeStatus: 'success' | 'processing' | 'warning' | 'error';
    label: string;
    background: string;
  }
> = {
  success: {
    accent: '#52c41a',
    badgeStatus: 'success',
    label: '状态正常',
    background: 'linear-gradient(135deg, rgba(82,196,26,0.12) 0%, rgba(22,119,255,0.03) 100%)',
  },
  info: {
    accent: '#1677ff',
    badgeStatus: 'processing',
    label: '状态说明',
    background: 'linear-gradient(135deg, rgba(22,119,255,0.1) 0%, rgba(114,46,209,0.03) 100%)',
  },
  warning: {
    accent: '#faad14',
    badgeStatus: 'warning',
    label: '需要处理',
    background: 'linear-gradient(135deg, rgba(250,173,20,0.12) 0%, rgba(22,119,255,0.03) 100%)',
  },
  error: {
    accent: '#ff4d4f',
    badgeStatus: 'error',
    label: '当前不可用',
    background: 'linear-gradient(135deg, rgba(255,77,79,0.12) 0%, rgba(22,119,255,0.03) 100%)',
  },
};

export function PageStatePanel({
  title,
  description,
  tone = 'info',
  badgeText,
  extra,
  actions,
}: {
  title: React.ReactNode;
  description: React.ReactNode;
  tone?: 'success' | 'info' | 'warning' | 'error';
  badgeText?: React.ReactNode;
  extra?: React.ReactNode;
  actions?: React.ReactNode;
}) {
  const theme = PAGE_STATE_THEME[tone];

  return (
    <Card
      styles={{
        body: {
          padding: DASHBOARD_PAGE_TOKENS.heroCardPadding,
          background: theme.background,
        },
      }}
    >
      <Space direction="vertical" size={DASHBOARD_PAGE_TOKENS.sectionGap} style={{ width: '100%' }}>
        <Space wrap size={[8, 8]}>
          <Badge status={theme.badgeStatus} text={badgeText || theme.label} />
          {extra}
        </Space>
        <Space direction="vertical" size={6} style={{ width: '100%' }}>
          <Typography.Title level={4} style={{ margin: 0, color: theme.accent }}>
            {title}
          </Typography.Title>
          <Typography.Text type="secondary">{description}</Typography.Text>
        </Space>
        {actions ? <Space wrap size={[8, 8]}>{actions}</Space> : null}
      </Space>
    </Card>
  );
}
