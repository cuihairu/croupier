import React from 'react';
import { StatisticCard } from '@ant-design/pro-components';
import Spark, { SPARK_HEIGHT } from './Spark';

/** 实时指标卡：StatisticCard + chart slot sparkline。仅快照指标（无趋势
 * 序列数据源）不传 spark 时渲染等高空白占位（而非死 sparkline），保证
 * 网格内所有卡片等高。卡片自身 height:100% 填满 grid 单元格。 */
export default function StatCard({
  loading,
  title,
  value,
  precision,
  prefix,
  suffix,
  contentStyle,
  style,
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
  /** 卡片根样式（网格等高由调用方配合 grid 布局使用）。 */
  style?: React.CSSProperties;
  spark?: [number, number][];
}) {
  return (
    <StatisticCard
      loading={loading}
      style={{ height: '100%', ...style }}
      statistic={{
        title,
        value,
        precision,
        prefix,
        suffix,
        styles: contentStyle ? { content: contentStyle } : undefined,
      }}
      chart={spark ? <Spark data={spark} /> : <div style={{ height: SPARK_HEIGHT }} aria-hidden />}
    />
  );
}
