/**
 * 实时大屏网格结构（2026-09 卡片统一网格改造）：auto-fit CSS Grid 取代
 * 固定 span 的 Row/Col；主次指标三分组（实时活跃/规模与存量/收入转化）；
 * 全部 12 张指标卡渲染。hook 整体 mock（SSE 不进 jsdom），交互逻辑由
 * useRealtimeStream.test.tsx 与 Toolbar.test.tsx 覆盖。
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import AnalyticsRealtimePage from '../index';
import type { StreamStatus } from '../types';

jest.mock('../useRealtimeStream', () => ({
  useRealtimeStream: () => ({
    data: { online: 12, active5M: 34, dauToday: 56 },
    loading: false,
    auto: true,
    setAuto: jest.fn(),
    streamStatus: 'connected' as StreamStatus,
    lastMessageAt: null,
    ptsOnline: [],
    ptsA5: [],
    ptsA15: [],
    ptsRev5: [],
    thrOnline: 0,
    setThrOnline: jest.fn(),
    thrA5: 0,
    setThrA5: jest.fn(),
    refresh: jest.fn(),
    clearTrend: jest.fn(),
  }),
}));

describe('实时大屏网格结构', () => {
  it('三个指标分组标题渲染', () => {
    render(<AnalyticsRealtimePage />);
    expect(screen.getByText('实时活跃')).toBeInTheDocument();
    expect(screen.getByText('规模与存量')).toBeInTheDocument();
    expect(screen.getByText('收入转化')).toBeInTheDocument();
  });

  it('12 张指标卡全部渲染（分组归位）', () => {
    render(<AnalyticsRealtimePage />);
    // 实时活跃组
    expect(screen.getByText('实时在线')).toBeInTheDocument();
    expect(screen.getByText('1分钟活跃')).toBeInTheDocument();
    expect(screen.getByText('5分钟活跃')).toBeInTheDocument();
    expect(screen.getByText('15分钟活跃')).toBeInTheDocument();
    // 规模与存量组
    expect(screen.getByText('今日峰值在线')).toBeInTheDocument();
    expect(screen.getByText('历史峰值在线')).toBeInTheDocument();
    expect(screen.getByText('今日DAU')).toBeInTheDocument();
    expect(screen.getByText('今日新增')).toBeInTheDocument();
    expect(screen.getByText('注册用户总数')).toBeInTheDocument();
    // 收入转化组
    expect(screen.getByText('5分钟订单额(元)')).toBeInTheDocument();
    expect(screen.getByText('支付成功率')).toBeInTheDocument();
    expect(screen.getByText('今日充值(元)')).toBeInTheDocument();
  });

  it('网格容器使用 auto-fit CSS Grid（自适应列数）', () => {
    const { container } = render(<AnalyticsRealtimePage />);
    const grids = container.querySelectorAll<HTMLElement>('[data-testid="stat-grid"]');
    expect(grids).toHaveLength(3);
    grids.forEach((grid) => {
      expect(grid.style.display).toBe('grid');
      expect(grid.style.gridTemplateColumns).toBe('repeat(auto-fit, minmax(240px, 1fr))');
    });
  });

  it('流状态 Alert 正常渲染', () => {
    render(<AnalyticsRealtimePage />);
    expect(screen.getByText(/实时流已连接；当前没有业务事件时/)).toBeInTheDocument();
  });
});
