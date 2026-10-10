/**
 * CompareValueView 单测（docs/design/incident-reports.md §5.4 渲染约定）：
 * - 空值 → 主值 '-'；
 * - missing=true → 「无数据」占位（不出箭头、不算假数）；
 * - delta 正/负 → 箭头方向 + 颜色语义（默认升=好，lowerIsBetter 反转）；
 * - delta=null（对照期无样本）→ dash 占位；
 * - pct=null 不渲染百分比；有 pct 渲染绝对值。
 *
 * mock 口径：@umijs/max 的 FormattedMessage 用小词典渲染中文（断言可读文案）。
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { CompareValueView, DeltaView } from './index';
import type { CompareDelta, CompareValue } from '@/services/api/incident';

jest.mock('@umijs/max', () => {
  const DICT: Record<string, string> = {
    'pages.incidentReports.compare.prev': '环比',
    'pages.incidentReports.compare.yoy': '同比',
    'pages.incidentReports.compare.noData': '无数据',
    'pages.incidentReports.compare.missing': '对照期无数据（不与 0 比较）',
  };
  return {
    __esModule: true,
    FormattedMessage: ({ id, defaultMessage }: { id?: string; defaultMessage?: string }) =>
      (id && DICT[id]) || defaultMessage || id || '',
    useIntl: () => ({
      formatMessage: ({ id, defaultMessage }: { id?: string; defaultMessage?: string }) =>
        (id && DICT[id]) || defaultMessage || id || '',
    }),
  };
});

describe('DeltaView', () => {
  it('无 delta → 占位 -', () => {
    render(<DeltaView />);
    expect(screen.getByText('-')).toBeInTheDocument();
  });

  it('missing=true → 「无数据」占位，不出箭头', () => {
    render(<DeltaView delta={{ missing: true }} />);
    expect(screen.getByText('无数据')).toBeInTheDocument();
    expect(document.querySelector('.anticon-arrow-up')).toBeNull();
    expect(document.querySelector('.anticon-arrow-down')).toBeNull();
  });

  it('delta 正数 → 上升箭头（默认升=好 → success）', () => {
    render(<DeltaView delta={{ delta: 3, pct: 30 }} />);
    expect(document.querySelector('.anticon-arrow-up')).not.toBeNull();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('30.0%')).toBeInTheDocument();
    expect(document.querySelector('.ant-typography-success')).not.toBeNull();
  });

  it('lowerIsBetter：负 delta → success + 下降箭头', () => {
    render(<DeltaView delta={{ delta: -2 }} lowerIsBetter />);
    expect(document.querySelector('.anticon-arrow-down')).not.toBeNull();
    expect(document.querySelector('.ant-typography-success')).not.toBeNull();
  });

  it('lowerIsBetter：正 delta → danger', () => {
    render(<DeltaView delta={{ delta: 2 }} lowerIsBetter />);
    expect(document.querySelector('.ant-typography-danger')).not.toBeNull();
  });

  it('delta=null → dash 占位（对照期无样本，不与 0 比较）', () => {
    render(<DeltaView delta={{ delta: null, pct: null }} />);
    expect(document.querySelector('.anticon-dash')).not.toBeNull();
    expect(screen.queryByText('%')).toBeNull();
  });

  it('delta=0 → dash + 无箭头', () => {
    render(<DeltaView delta={{ delta: 0 }} />);
    expect(document.querySelector('.anticon-dash')).not.toBeNull();
    expect(document.querySelector('.anticon-arrow-up')).toBeNull();
  });
});

describe('CompareValueView', () => {
  const mk = (v: Partial<CompareValue>): CompareValue => v as CompareValue;

  it('空 value → 主值 -，环比/同比标签在位', () => {
    render(<CompareValueView value={mk({})} />);
    expect(screen.getAllByText('-').length).toBeGreaterThan(0);
    expect(screen.getByText('环比')).toBeInTheDocument();
    expect(screen.getByText('同比')).toBeInTheDocument();
  });

  it('主值 + 环比正常、同比 missing 三段同屏', () => {
    const value = mk({
      value: 12,
      prev: { value: 10, delta: 2, pct: 20 },
      yoy: { missing: true },
    });
    render(<CompareValueView value={value} />);
    expect(screen.getByText('12')).toBeInTheDocument();
    expect(screen.getByText('无数据')).toBeInTheDocument();
  });

  it('unit 与 precision 进主值格式化', () => {
    render(<CompareValueView value={mk({ value: 3.14159 })} unit="h" precision={1} />);
    expect(screen.getByText('3.1h')).toBeInTheDocument();
  });
});
