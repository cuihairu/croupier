import React from 'react';
import { Space, Tooltip, Typography } from 'antd';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DashOutlined,
  QuestionCircleOutlined,
} from '@ant-design/icons';
import { FormattedMessage } from '@umijs/max';
import type { CompareDelta, CompareValue } from '@/services/api/incident';

/**
 * 三列对比的渲染约定（docs/design/incident-reports.md §5.4）：
 * - pct=null（基数 0 或对照期无样本）→ 只显示差值箭头；
 * - missing=true（对照期无数据）→ 「无数据」占位，不算假数、不置 0。
 */
export function DeltaView({
  delta,
  unit,
  lowerIsBetter,
}: {
  delta?: CompareDelta;
  unit?: string;
  lowerIsBetter?: boolean;
}) {
  if (!delta) return <span>-</span>;
  if (delta.missing) {
    return (
      <Tooltip title={<FormattedMessage id="pages.incidentReports.compare.missing" />}>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          <QuestionCircleOutlined /> <FormattedMessage id="pages.incidentReports.compare.noData" />
        </Typography.Text>
      </Tooltip>
    );
  }
  const good = lowerIsBetter ? (delta.delta ?? 0) < 0 : (delta.delta ?? 0) > 0;
  const color = delta.delta ? (good ? 'success' : 'danger') : 'secondary';
  return (
    <Space size={4}>
      <Typography.Text type={color} style={{ fontSize: 12 }}>
        {delta.delta !== null && delta.delta !== undefined ? (
          <>
            {(delta.delta ?? 0) > 0 ? (
              <ArrowUpOutlined />
            ) : (delta.delta ?? 0) < 0 ? (
              <ArrowDownOutlined />
            ) : (
              <DashOutlined />
            )}{' '}
            {Math.abs(delta.delta ?? 0).toLocaleString()}
            {unit || ''}
          </>
        ) : (
          <DashOutlined /> /* delta=null：对照期无样本 */
        )}
      </Typography.Text>
      {delta.pct !== null && delta.pct !== undefined && (
        <Typography.Text type={color} style={{ fontSize: 12 }}>
          {Math.abs(delta.pct).toFixed(1)}%
        </Typography.Text>
      )}
    </Space>
  );
}

export function CompareValueView({
  value,
  unit,
  precision = 0,
  lowerIsBetter,
}: {
  value?: CompareValue;
  unit?: string;
  precision?: number;
  lowerIsBetter?: boolean;
}) {
  const text = (v?: number | null) =>
    v === null || v === undefined
      ? '-'
      : v.toLocaleString(undefined, { maximumFractionDigits: precision });
  return (
    <Space orientation="vertical" size={0}>
      <Typography.Text strong>
        {text(value?.value)}
        {unit}
      </Typography.Text>
      <Space size={12}>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          <FormattedMessage id="pages.incidentReports.compare.prev" />
        </Typography.Text>
        <DeltaView delta={value?.prev} unit={unit} lowerIsBetter={lowerIsBetter} />
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          <FormattedMessage id="pages.incidentReports.compare.yoy" />
        </Typography.Text>
        <DeltaView delta={value?.yoy} unit={unit} lowerIsBetter={lowerIsBetter} />
      </Space>
    </Space>
  );
}

export default CompareValueView;
