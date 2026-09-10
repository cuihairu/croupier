import React from 'react';
import { Card, Statistic } from 'antd';
import Spark from './Spark';

/** 实时指标卡：Statistic + 底部 sparkline。无趋势序列的指标
 * 传 spark 省略（保持空 sparkline 占位）。 */
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
      <div style={{ marginTop: 6 }}>
        <Spark data={spark || []} />
      </div>
    </Card>
  );
}
