/** StatCard 迁移语义回归（2026-09 由手写 Card+Statistic+spark 槽
 * 迁移到 pro-components StatisticCard 的 chart slot）：
 * 1. 基础形态渲染标题与数值；
 * 2. 传 spark 时 chart slot 渲染序列图；
 * 3. 不传 spark 时不渲染图表占位（快照指标无死 sparkline）；
 * 4. loading 时不渲染数值内容。 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import StatCard from '../StatCard';

describe('StatCard', () => {
  it('基础形态：渲染标题与数值', () => {
    render(<StatCard title="在线" value={42} />);
    expect(screen.getByText('在线')).toBeInTheDocument();
    expect(screen.getByText('42')).toBeInTheDocument();
  });

  it('带 spark：chart slot 渲染 sparkline（svg）', () => {
    const { container } = render(
      <StatCard
        title="QPS"
        value={7}
        spark={[
          [-2, 1],
          [-1, 3],
          [0, 2],
        ]}
      />,
    );
    expect(container.querySelector('svg')).toBeInTheDocument();
  });

  it('无 spark：不渲染图表占位', () => {
    const { container } = render(<StatCard title="在线" value={1} />);
    expect(container.querySelector('svg')).not.toBeInTheDocument();
  });

  it('loading：不渲染数值内容', () => {
    render(<StatCard loading title="在线" value={42} />);
    expect(screen.queryByText('42')).not.toBeInTheDocument();
  });

  it('contentStyle：映射 antd 6 styles.content（valueStyle 已废弃）', () => {
    const { container } = render(
      <StatCard title="实时在线" value={42} contentStyle={{ color: '#cf1322' }} />,
    );
    const content = container.querySelector('.ant-statistic-content');
    expect(content).toBeInTheDocument();
    expect(content).toHaveStyle({ color: 'rgb(207, 19, 34)' });
  });
});
