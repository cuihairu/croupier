/** PageEditor shell（页面信息卡片）：挂载菜单插槽渲染在 body 内、
 * 旧「分类 key」编辑入口已移除（menu_items 挂载是导航唯一事实源）。 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import PageEditor from '../index';
import type { PageSpec } from '@/types/dashboard';

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
  useIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage,
    locale: 'zh-CN',
  }),
}));

const spec: PageSpec = {
  pageKey: 'operation--inventory.consume',
  type: 'operation',
  title: { 'zh-CN': '消耗道具' },
  category: { key: 'inventory', labels: {} },
  status: 'draft',
} as unknown as PageSpec;

describe('PageEditor shell', () => {
  it('渲染挂载菜单插槽且不再出现分类 key 输入', () => {
    render(
      <PageEditor
        value={spec}
        onChange={() => {}}
        mountMenuSlot={
          <div data-testid="mount-slot">
            <span>挂载菜单</span>
          </div>
        }
      />,
    );
    expect(screen.getByTestId('mount-slot')).toBeInTheDocument();
    expect(screen.queryByText('分类 key')).not.toBeInTheDocument();
    expect(screen.getByText('页面标题（多语言）')).toBeInTheDocument();
  });

  it('未提供插槽时正常渲染（预览等只读场景）', () => {
    render(<PageEditor value={spec} onChange={() => {}} readonly />);
    expect(screen.queryByTestId('mount-slot')).not.toBeInTheDocument();
    expect(screen.queryByText('分类 key')).not.toBeInTheDocument();
  });
});
