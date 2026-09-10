import React from 'react';
import { Card, Statistic } from 'antd';
import Spark from './Spark';

/** 实时指标卡：Statistic + 底部 sparkline。仅快照指标（无趋势
 * 序列数据源）不传 spark，此时不渲染图表占位，避免死 sparkline。 */
export default function StatCard({
  loading,
  title,
  value,
  precision,
  prefix,
  suffix,
  valueStyle,
  spark,
}: {
  loading?: boolean;
  title: string;
  value: number | string;
  precision?: number;
  prefix?: string;
  suffix?: string;
  valueStyle?: React.CSSProperties;
  spark?: [number, number][];
}) {
  return (
    <Card loading={loading}>
      <Statistic
        title={title}
        value={value}
        precision={precision}
        prefix={prefix}
        suffix={suffix}
        valueStyle={valueStyle}
      />
      {spark ? (
        <div style={{ marginTop: 6 }}>
          <Spark data={spark} />
        </div>
      ) : null}
    </Card>
  );
}
