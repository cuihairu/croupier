import React from 'react';
import { render, screen } from '@testing-library/react';
import WorkspaceRenderer from '@/components/WorkspaceRenderer';
import type { WorkspaceConfig } from '@/types/workspace';

jest.mock('@/components/WorkspaceRenderer/TabContentRenderer', () => ({
  __esModule: true,
  default: () => <div data-testid="tab-content">TabContent</div>,
}));

function workspace(): WorkspaceConfig {
  return {
    objectKey: 'player.ban',
    title: '封禁玩家',
    published: true,
    layout: {
      type: 'tabs',
      tabs: [
        {
          key: 'main',
          title: '主页面',
          functions: [],
          layout: {
            type: 'detail',
            detailFunction: 'player.detail',
            sections: [],
          },
        },
      ],
    },
  };
}

describe('WorkspaceRenderer', () => {
  it('console 运行态只渲染用户配置内容，不显示系统说明外壳', () => {
    render(<WorkspaceRenderer config={workspace()} context={{ runtimeMode: 'console' }} />);

    expect(screen.getByTestId('tab-content')).toBeInTheDocument();
    expect(screen.queryByText('正式运行态')).not.toBeInTheDocument();
    expect(screen.queryByText('页面标签')).not.toBeInTheDocument();
    expect(screen.queryByText('装配结果视图')).not.toBeInTheDocument();
  });
});
