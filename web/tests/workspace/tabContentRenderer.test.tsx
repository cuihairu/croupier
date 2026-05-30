import React from 'react';
import { render, screen } from '@testing-library/react';
import TabContentRenderer from '@/components/WorkspaceRenderer/TabContentRenderer';

jest.mock('@/components/WorkspaceRenderer/renderers/FormDetailRenderer', () => ({
  __esModule: true,
  default: () => <div data-testid="renderer-form-detail">FormDetailRenderer</div>,
}));

jest.mock('@/components/WorkspaceRenderer/renderers/ListRenderer', () => ({
  __esModule: true,
  default: () => <div data-testid="renderer-list">ListRenderer</div>,
}));

jest.mock('@/components/WorkspaceRenderer/renderers/FormRenderer', () => ({
  __esModule: true,
  default: () => <div data-testid="renderer-form">FormRenderer</div>,
}));

jest.mock('@/components/WorkspaceRenderer/renderers/DetailRenderer', () => ({
  __esModule: true,
  default: () => <div data-testid="renderer-detail">DetailRenderer</div>,
}));

function createTab(layoutType: string): any {
  return {
    key: `tab-${layoutType}`,
    title: `tab-${layoutType}`,
    functions: [],
    layout: {
      type: layoutType,
    },
  };
}

describe('TabContentRenderer', () => {
  it('tabs: form-detail 分发正确', () => {
    render(<TabContentRenderer tab={createTab('form-detail')} objectKey="player" />);
    expect(screen.getByTestId('renderer-form-detail')).toBeInTheDocument();
  });

  it('tabs: list 分发正确', () => {
    render(<TabContentRenderer tab={createTab('list')} objectKey="player" />);
    expect(screen.getByTestId('renderer-list')).toBeInTheDocument();
  });

  it('tabs: form 分发正确', () => {
    render(<TabContentRenderer tab={createTab('form')} objectKey="player" />);
    expect(screen.getByTestId('renderer-form')).toBeInTheDocument();
  });

  it('tabs: detail 分发正确', () => {
    render(<TabContentRenderer tab={createTab('detail')} objectKey="player" />);
    expect(screen.getByTestId('renderer-detail')).toBeInTheDocument();
  });

  it('tabs: 非 V1 类型返回错误提示', () => {
    render(<TabContentRenderer tab={createTab('kanban')} objectKey="player" />);
    expect(screen.getByText('当前 Tab 布局不在 V1 支持范围')).toBeInTheDocument();
    expect(screen.getByText(/当前类型: kanban/)).toBeInTheDocument();
  });
});
