/**
 * 实时大屏工具栏：阈值输入由原生 input 迁移到 antd InputNumber 后的
 * 交互回归（输入值回调、null 归零）；「最后更新」文案随 token 着色。
 * CSV 导出与 RangePicker 交互不在本套件（jsdom 交互不可靠，逻辑在
 * fetchRealtimeSeries 的 service 测试与 e2e 覆盖）。
 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import Toolbar from '../Toolbar';
import { fetchRealtimeSeries } from '@/services/api/analytics';
import type { StreamStatus } from '../types';

jest.mock('@/services/api/analytics', () => ({
  fetchRealtimeSeries: jest.fn(),
}));

jest.mock('@/utils/export', () => ({
  exportToCSV: jest.fn(),
}));

const baseProps = () => ({
  streamStatus: 'connected' as StreamStatus,
  lastMessageAt: null,
  loading: false,
  auto: false,
  onToggleAuto: jest.fn(),
  onRefresh: jest.fn(),
  onClearTrend: jest.fn(),
  thrOnline: 0,
  onThrOnlineChange: jest.fn(),
  thrA5: 0,
  onThrA5Change: jest.fn(),
  ptsOnline: [] as [number, number][],
  ptsA5: [] as [number, number][],
  ptsA15: [] as [number, number][],
  ptsRev5: [] as [number, number][],
});

describe('Toolbar 阈值输入（InputNumber）', () => {
  it('连接状态 Tag 与「最后更新」渲染', () => {
    render(<Toolbar {...baseProps()} />);
    expect(screen.getByText('已连接')).toBeInTheDocument();
    expect(screen.getByText(/最后更新:/)).toBeInTheDocument();
  });

  it('输入在线阈值回调数值', () => {
    const props = baseProps();
    render(<Toolbar {...props} />);
    // aria-label 可能透传到 input 本身或包在 wrapper 上，两者兼容
    const el = screen.getByLabelText('threshold-online');
    const input = (el.tagName === 'INPUT' ? el : el.querySelector('input')) as HTMLInputElement;
    fireEvent.change(input, { target: { value: '100' } });
    expect(props.onThrOnlineChange).toHaveBeenCalledWith(100);
  });

  it('清空输入归零（null → 0）', () => {
    const props = baseProps();
    render(<Toolbar {...props} />);
    const el = screen.getByLabelText('threshold-active5m');
    const input = (el.tagName === 'INPUT' ? el : el.querySelector('input')) as HTMLInputElement;
    fireEvent.change(input, { target: { value: '' } });
    expect(props.onThrA5Change).toHaveBeenCalledWith(0);
  });
});
