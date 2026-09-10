import React from 'react';
import { StatisticCard } from '@ant-design/pro-components';
import Spark from './Spark';

/** 实时指标卡：StatisticCard + chart slot sparkline。仅快照指标（无趋势
 * 序列数据源）不传 spark，此时不渲染图表占位，避免死 sparkline。 */
export default function StatCard({
  loading,
  title,
  value,
  precision,
  prefix,
  suffix,
  contentStyle,
  spark,
}: {
  loading?: boolean;
  title: string;
  value: number | string;
  precision?: number;
  prefix?: string;
  suffix?: string;
  /** 数值内容样式（映射 antd 6 Statistic styles.content；valueStyle 已废弃）。 */
  contentStyle?: React.CSSProperties;
  spark?: [number, number][];
}) {
  return (
    <StatisticCard
      loading={loading}
      statistic={{
        title,
        value,
        precision,
        prefix,
        suffix,
        styles: contentStyle ? { content: contentStyle } : undefined,
      }}
      chart={spark ? <Spark data={spark} /> : undefined}
    />
  );
}
