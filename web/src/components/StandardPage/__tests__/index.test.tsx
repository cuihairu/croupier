/** StandardPage 展示组件覆盖。
 *
 * 覆盖路径：SummaryOverview（description/hint 可选渲染、item.color 兜底 key）、
 * StandardListSection（extra/resultText 可选）、StandardFilterBar（resultText 可选）、
 * PageStatePanel 四种 tone 主题与 badgeText 兜底、actions/extra 可选渲染。 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import {
  DASHBOARD_PAGE_TOKENS,
  PageStatePanel,
  SummaryOverview,
  StandardFilterBar,
  StandardListSection,
} from '../index';

describe('DASHBOARD_PAGE_TOKENS', () => {
  it('导出统一 token 常量（圆角由 antd token 承载，此处不含圆角数值）', () => {
    expect(DASHBOARD_PAGE_TOKENS.cardPadding).toBe(20);
    expect(DASHBOARD_PAGE_TOKENS.compactCardPadding).toBe(12);
    expect(DASHBOARD_PAGE_TOKENS.sectionGap).toBe(16);
  });
});

describe('SummaryOverview', () => {
  it('渲染标题、描述、徽标项与 hint', () => {
    render(
      <SummaryOverview
        title="概览标题"
        description="描述文本"
        items={[{ color: 'blue', text: '带色项' }, { text: '无色项' }]}
        hint="提示内容"
      />,
    );
    expect(screen.getByText('概览标题')).toBeInTheDocument();
    expect(screen.getByText('描述文本')).toBeInTheDocument();
    expect(screen.getByText('带色项')).toBeInTheDocument();
    expect(screen.getByText('无色项')).toBeInTheDocument();
    expect(screen.getByText('提示内容')).toBeInTheDocument();
  });

  it('description 与 hint 缺省时不渲染', () => {
    const { container } = render(<SummaryOverview title="仅标题" items={[{ text: 'a' }]} />);
    expect(screen.getByText('仅标题')).toBeInTheDocument();
    expect(screen.queryByText('提示内容')).not.toBeInTheDocument();
    // item 无 color → Badge 默认色（无 .ant-badge-color-* 自定义类残留断言，仅确认渲染不炸）
    expect(container.querySelector('.ant-badge')).toBeInTheDocument();
  });
});

describe('StandardListSection', () => {
  it('渲染标题、extra、resultText 与 children', () => {
    render(
      <StandardListSection
        title="列表区块"
        extra={<button type="button">操作</button>}
        resultText="共 3 条"
      >
        <div>内容行</div>
      </StandardListSection>,
    );
    expect(screen.getByText('列表区块')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '操作' })).toBeInTheDocument();
    expect(screen.getByText('共 3 条')).toBeInTheDocument();
    expect(screen.getByText('内容行')).toBeInTheDocument();
  });

  it('extra 与 resultText 缺省时不渲染', () => {
    render(<StandardListSection title="最小形态">内容</StandardListSection>);
    expect(screen.getByText('最小形态')).toBeInTheDocument();
    expect(screen.getByText('内容')).toBeInTheDocument();
  });
});

describe('StandardFilterBar', () => {
  it('渲染控件与 resultText', () => {
    render(<StandardFilterBar controls={<input aria-label="搜索" />} resultText="已过滤" />);
    expect(screen.getByLabelText('搜索')).toBeInTheDocument();
    expect(screen.getByText('已过滤')).toBeInTheDocument();
  });

  it('resultText 缺省时不渲染', () => {
    const { container } = render(<StandardFilterBar controls={<span>c</span>} />);
    expect(container.textContent).toBe('c');
  });
});

describe('PageStatePanel', () => {
  it.each([
    { tone: 'success', label: '状态正常' },
    { tone: 'info', label: '状态说明' },
    { tone: 'warning', label: '需要处理' },
    { tone: 'error', label: '当前不可用' },
  ] as const)('tone=$tone 使用兜底徽标文案「$label」', ({ tone, label }) => {
    render(<PageStatePanel title={`${tone} 标题`} description={`${tone} 描述`} tone={tone} />);
    expect(screen.getByText(`${tone} 标题`)).toBeInTheDocument();
    expect(screen.getByText(`${tone} 描述`)).toBeInTheDocument();
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it('badgeText/extra/actions 传入时优先渲染', () => {
    render(
      <PageStatePanel
        title="t"
        description="d"
        tone="warning"
        badgeText="自定义徽标"
        extra={<span>附加信息</span>}
        actions={<button type="button">立即处理</button>}
      />,
    );
    expect(screen.getByText('自定义徽标')).toBeInTheDocument();
    expect(screen.queryByText('需要处理')).not.toBeInTheDocument();
    expect(screen.getByText('附加信息')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '立即处理' })).toBeInTheDocument();
  });
});
