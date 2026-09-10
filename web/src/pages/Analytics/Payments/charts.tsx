import React from 'react';
import { Button } from 'antd';
import type { DimData, ProductData, TrendData, TrendPoint } from './types';

/** 支付分析页 SVG 图表组件：Top 榜（收入/成功率/组合/转化）、维度 CSV 导出、SKU 趋势双面板。 */

export const TopProducts: React.FC<{ data: ProductData[] }> = ({ data }) => {
  try {
    const items = (data || [])
      .slice(0)
      .sort(
        (a: ProductData, b: ProductData) =>
          Number(b.revenueCents || 0) - Number(a.revenueCents || 0),
      )
      .slice(0, 10);
    if (!items.length) return null;
    const max = Math.max(...items.map((x: ProductData) => Number(x.revenueCents || 0)), 1);
    const w = 600,
      barH = 18,
      gap = 6,
      left = 140,
      right = 60,
      h = items.length * (barH + gap) + 20;
    const scale = (v: number) => ((w - left - right) * v) / max;
    return (
      <div style={{ marginTop: 12 }}>
        <b>Top 商品（按收入）</b>
        <svg
          width={w}
          height={h}
          style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
        >
          {items.map((it: ProductData, idx: number) => {
            const y = 10 + idx * (barH + gap);
            const val = Number(it.revenueCents || 0);
            return (
              <g key={idx}>
                <text x={4} y={y + barH - 4} fontSize={12} fill="#555">
                  {String(it.productId || '-')}
                </text>
                <rect x={left} y={y} width={Math.max(2, scale(val))} height={barH} fill="#1677ff" />
                <text
                  x={left + Math.max(2, scale(val)) + 6}
                  y={y + barH - 4}
                  fontSize={12}
                  fill="#333"
                >
                  {val}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch {
    return null;
  }
};

export const TopDimBar: React.FC<{ data: DimData[]; dimKey: string; title: string }> = ({
  data,
  dimKey,
  title,
}) => {
  try {
    const items = (data || [])
      .slice(0)
      .sort((a, b) => Number(b.revenueCents || 0) - Number(a.revenueCents || 0))
      .slice(0, 10);
    if (!items.length) return null;
    const max = Math.max(...items.map((x) => Number(x.revenueCents || 0)), 1);
    const w = 600,
      barH = 18,
      gap = 6,
      left = 140,
      right = 60,
      h = items.length * (barH + gap) + 20;
    const scale = (v: number) => ((w - left - right) * v) / max;
    return (
      <div style={{ marginTop: 12 }}>
        <b>{title}</b>
        <svg
          width={w}
          height={h}
          style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
        >
          {items.map((it, idx: number) => {
            const y = 10 + idx * (barH + gap);
            const val = Number(it.revenueCents || 0);
            return (
              <g key={idx}>
                <text x={4} y={y + barH - 4} fontSize={12} fill="#555">
                  {String((it as Record<string, string | number>)[dimKey] || '-')}
                </text>
                <rect x={left} y={y} width={Math.max(2, scale(val))} height={barH} fill="#73d13d" />
                <text
                  x={left + Math.max(2, scale(val)) + 6}
                  y={y + barH - 4}
                  fontSize={12}
                  fill="#333"
                >
                  {val}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch {
    return null;
  }
};

export const TopDimRate: React.FC<{ data: DimData[]; dimKey: string; title: string }> = ({
  data,
  dimKey,
  title,
}) => {
  try {
    const items = (data || [])
      .map((x) => ({ ...x, successRate: Number(x.successRate || 0) }))
      .filter((x) => isFinite(x.successRate))
      .sort((a, b) => b.successRate - a.successRate)
      .slice(0, 10);
    if (!items.length) return null;
    const max = Math.max(...items.map((x) => Number(x.successRate || 0)), 1);
    const w = 600,
      barH = 18,
      gap = 6,
      left = 140,
      right = 60,
      h = items.length * (barH + gap) + 20;
    const scale = (v: number) => ((w - left - right) * v) / max;
    return (
      <div style={{ marginTop: 12 }}>
        <b>{title}</b>
        <svg
          width={w}
          height={h}
          style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
        >
          {items.map((it, idx: number) => {
            const y = 10 + idx * (barH + gap);
            const val = Number(it.successRate || 0);
            return (
              <g key={idx}>
                <text x={4} y={y + barH - 4} fontSize={12} fill="#555">
                  {String((it as Record<string, string | number>)[dimKey] || '-')}
                </text>
                <rect x={left} y={y} width={Math.max(2, scale(val))} height={barH} fill="#faad14" />
                <text
                  x={left + Math.max(2, scale(val)) + 6}
                  y={y + barH - 4}
                  fontSize={12}
                  fill="#333"
                >
                  {val}%
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch {
    return null;
  }
};

// Combined revenue (bar) + success_rate (line) on same chart (two scales approximated visually)
export const TopDimCombo: React.FC<{ data: DimData[]; dimKey: string; title: string }> = ({
  data,
  dimKey,
  title,
}) => {
  try {
    const items = (data || [])
      .slice(0)
      .sort((a, b) => Number(b.revenueCents || 0) - Number(a.revenueCents || 0))
      .slice(0, 10);
    if (!items.length) return null;
    const maxRev = Math.max(...items.map((x) => Number(x.revenueCents || 0)), 1);
    const maxRate = Math.max(...items.map((x) => Number(x.successRate || 0)), 1);
    const w = 720,
      barH = 18,
      gap = 10,
      left = 140,
      right = 80,
      topm = 16,
      h = items.length * (barH + gap) + topm + 10;
    const sRev = (v: number) => ((w - left - right) * v) / maxRev;
    const sRate = (v: number) => ((w - left - right) * v) / maxRate;
    return (
      <div style={{ marginTop: 12 }}>
        <b>{title}</b>
        <svg
          width={w}
          height={h}
          style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
        >
          {items.map((it, idx: number) => {
            const y = topm + idx * (barH + gap);
            const rev = Number(it.revenueCents || 0);
            const rate = Number(it.successRate || 0);
            return (
              <g key={idx}>
                <text x={4} y={y + barH - 4} fontSize={12} fill="#555">
                  {String((it as Record<string, string | number>)[dimKey] || '-')}
                </text>
                {/* revenue bar */}
                <rect x={left} y={y} width={Math.max(2, sRev(rev))} height={barH} fill="#69c0ff" />
                <text
                  x={left + Math.max(2, sRev(rev)) + 6}
                  y={y + barH - 4}
                  fontSize={12}
                  fill="#333"
                >
                  {rev}
                </text>
                {/* success rate markers (line) */}
                <circle
                  cx={left + Math.max(2, sRate(rate))}
                  cy={y + barH / 2}
                  r={3}
                  fill="#fa541c"
                />
                <text
                  x={left + Math.max(2, sRate(rate)) + 6}
                  y={y + barH / 2 + 4}
                  fontSize={10}
                  fill="#666"
                >
                  {rate}%
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch {
    return null;
  }
};

// Product conversion compare (success vs total)
export const TopProductConv: React.FC<{ data: ProductData[] }> = ({ data }) => {
  try {
    const items = (data || [])
      .slice(0)
      .sort((a: ProductData, b: ProductData) => Number(b.total || 0) - Number(a.total || 0))
      .slice(0, 10);
    if (!items.length) return null;
    const max = Math.max(...items.map((x: ProductData) => Number(x.total || 0)), 1);
    const w = 720,
      barH = 16,
      gap = 8,
      left = 160,
      right = 80,
      topm = 16,
      h = items.length * (barH + gap) + topm + 14;
    const s = (v: number) => ((w - left - right) * v) / max;
    return (
      <div style={{ marginTop: 12 }}>
        <b>Top 商品（转化：成功 vs 总数）</b>
        <svg
          width={w}
          height={h}
          style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
        >
          {items.map((it: ProductData, idx: number) => {
            const y = topm + idx * (barH + gap);
            const succ = Number(it.success || 0);
            const tot = Number(it.total || 0);
            return (
              <g key={idx}>
                <text x={4} y={y + barH - 2} fontSize={12} fill="#555">
                  {String(it.productId || '-')}
                </text>
                {/* total (background) */}
                <rect x={left} y={y} width={Math.max(2, s(tot))} height={barH} fill="#f0f0f0" />
                {/* success */}
                <rect x={left} y={y} width={Math.max(2, s(succ))} height={barH} fill="#52c41a" />
                <text
                  x={left + Math.max(2, s(succ)) + 6}
                  y={y + barH - 2}
                  fontSize={12}
                  fill="#333"
                >
                  {succ}/{tot}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch {
    return null;
  }
};

// Export current dimension rows to CSV
export const ExportDimCSV: React.FC<{
  data: DimData[];
  dimKey: string;
  name: string;
  includeConv?: boolean;
}> = ({ data, dimKey, name, includeConv }) => (
  <Button
    style={{ marginTop: 8 }}
    onClick={() => {
      try {
        const rows: string[][] = [['dim', 'revenue_cents', 'success', 'total', 'success_rate(%)']];
        (data || []).forEach((r) => {
          const record = r as Record<string, string | number>;
          rows.push([
            String(record[dimKey] ?? ''),
            String(record.revenueCents ?? 0),
            String(record.success ?? 0),
            String(record.total ?? 0),
            String(record.successRate ?? 0),
          ]);
        });
        if (includeConv) rows.push([]);
        const csv = rows.map((r) => r.map((x) => String(x ?? '')).join(',')).join('\n');
        const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `payments_${name}.csv`;
        a.click();
        URL.revokeObjectURL(url);
      } catch {}
    }}
  >
    导出 {name} CSV
  </Button>
);

// TrendChart: two panels (revenue & success_rate) for multiple products
export const TrendChart: React.FC<{ data: TrendData[] }> = ({ data }) => {
  try {
    const prods = (data || []) as TrendData[];
    if (!prods.length) return null;
    const colors = [
      '#1677ff',
      '#fa541c',
      '#52c41a',
      '#faad14',
      '#722ed1',
      '#13c2c2',
      '#eb2f96',
      '#2f54eb',
      '#a0d911',
      '#fa8c16',
    ];
    // collect times and compute scales
    const timesSet: Record<string, number> = {};
    let maxRev = 1,
      maxRate = 100;
    prods.forEach((p: TrendData) =>
      (p.points || []).forEach((pt: TrendPoint) => {
        timesSet[String(pt.time)] = 1;
        maxRev = Math.max(maxRev, Number(pt.amount || 0));
        const succ = Number(pt.amount || 0);
        const tot = Number(pt.count || 0);
        const rate = tot > 0 ? (succ * 100) / tot : 0;
        maxRate = Math.max(maxRate, rate);
      }),
    );
    const times = Object.keys(timesSet).sort();
    const w = 720,
      h = 180,
      left = 40,
      bottom = 24,
      right = 10,
      topm = 16;
    const sx = (t: string) => {
      const i = times.indexOf(t);
      if (i < 0) return left;
      return left + ((w - left - right) * i) / Math.max(1, times.length - 1);
    };
    const syRev = (v: number) => topm + (h - topm - bottom) * (1 - v / Math.max(1, maxRev));
    const syRate = (v: number) => topm + (h - topm - bottom) * (1 - v / Math.max(1, maxRate));
    const Path = ({
      vals,
      yfn,
      color,
    }: {
      vals: [string, number][];
      yfn: (v: number) => number;
      color: string;
    }) => {
      const d = vals
        .map(
          (pt: [string, number], idx: number) =>
            `${idx ? 'L' : 'M'}${sx(pt[0])},${yfn(Number(pt[1]))}`,
        )
        .join(' ');
      return <path d={d} fill="none" stroke={color} strokeWidth={2} />;
    };
    return (
      <div>
        <div style={{ marginTop: 8 }}>
          <b>收入趋势</b>
          <svg
            width={w}
            height={h}
            style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
          >
            <line x1={left} y1={topm} x2={left} y2={h - bottom} stroke="#ddd" />
            <line x1={left} y1={h - bottom} x2={w - right} y2={h - bottom} stroke="#ddd" />
            {prods.map((p: TrendData, i: number) => (
              <Path
                key={p.productId || i}
                vals={(p.points || []).map((pt: TrendPoint) => [pt.time, Number(pt.amount || 0)])}
                yfn={syRev}
                color={colors[i % colors.length]}
              />
            ))}
          </svg>
        </div>
        <div style={{ marginTop: 8 }}>
          <b>成功率趋势</b>
          <svg
            width={w}
            height={h}
            style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
          >
            <line x1={left} y1={topm} x2={left} y2={h - bottom} stroke="#ddd" />
            <line x1={left} y1={h - bottom} x2={w - right} y2={h - bottom} stroke="#ddd" />
            {prods.map((p: TrendData, i: number) => (
              <Path
                key={p.productId || i}
                vals={(p.points || []).map((pt: TrendPoint) => {
                  const succ = Number(pt.amount || 0);
                  const tot = Number(pt.count || 0);
                  const rate = tot > 0 ? (succ * 100) / tot : 0;
                  return [pt.time, rate] as [string, number];
                })}
                yfn={syRate}
                color={colors[i % colors.length]}
              />
            ))}
          </svg>
        </div>
        <div style={{ marginTop: 4 }}>
          <b>图例：</b>
          {prods.map((p: TrendData, i: number) => (
            <span key={p.productId || i} style={{ marginRight: 12 }}>
              <span
                style={{
                  display: 'inline-block',
                  width: 10,
                  height: 10,
                  background: colors[i % colors.length],
                  marginRight: 4,
                }}
              />
              {String(p.productId || '-')}
            </span>
          ))}
        </div>
      </div>
    );
  } catch {
    return null;
  }
};
