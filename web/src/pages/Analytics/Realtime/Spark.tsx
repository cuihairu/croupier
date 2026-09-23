import React from 'react';
import { theme as antdTheme } from 'antd';

/** sparkline 绘制高度；StatCard 的无数据占位与其同高以保持卡片等高。 */
export const SPARK_HEIGHT = 40;

/** 趋势 sparkline。 */
export const Spark: React.FC<{ data: [number, number][] }> = ({ data }) => {
  const { token } = antdTheme.useToken();
  // viewBox + 100% 宽度：折线随卡片自适应，不再以固定 240px 溢出窄卡片。
  const w = 240,
    h = SPARK_HEIGHT,
    p = 3;
  if (!data || data.length < 2) return <div style={{ height: h }} />;
  const xs = data.map((d) => d[0]);
  const ys = data.map((d) => d[1]);
  const x0 = Math.min(...xs),
    x1 = Math.max(...xs),
    y0 = Math.min(...ys),
    y1 = Math.max(...ys);
  const sx = (x: number) => (x1 === x0 ? p : p + ((w - 2 * p) * (x - x0)) / (x1 - x0));
  const sy = (y: number) => (y1 === y0 ? h - p : h - (p + ((h - 2 * p) * (y - y0)) / (y1 - y0)));
  const dstr = data.map((pt, i) => `${i ? 'L' : 'M'}${sx(pt[0])},${sy(pt[1])}`).join(' ');
  return (
    <svg
      viewBox={`0 0 ${w} ${h}`}
      width="100%"
      height={h}
      preserveAspectRatio="none"
      style={{ display: 'block' }}
    >
      <path
        d={dstr}
        fill="none"
        stroke={token.colorPrimary}
        strokeWidth={1.8}
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
};

export default Spark;
