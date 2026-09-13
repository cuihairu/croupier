/** Console/TemplatePlayerManage：玩家管理 CRUD 模板预览页。
 *
 * 页面本身是纯壳（渲染内置 demo 模板、不触发真实函数调用），
 * 模板组件重量级且有自己的行为（由其自身测试覆盖），
 * 这里 mock 后验证壳的挂载与透传。 */
import { render, screen } from '@testing-library/react';
import PlayerManageTemplate from '@/components/PageRenderer/templates/playerManage';
import TemplatePlayerManagePage from './TemplatePlayerManage';

jest.mock('@/components/PageRenderer/templates/playerManage', () => ({
  __esModule: true,
  default: jest.fn(() => <div data-testid="player-manage-template" />),
}));

const mockedTemplate = jest.mocked(PlayerManageTemplate);

describe('Console/TemplatePlayerManage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('挂载即渲染玩家管理模板（无 props）', () => {
    render(<TemplatePlayerManagePage />);

    expect(screen.getByTestId('player-manage-template')).toBeInTheDocument();
    expect(mockedTemplate).toHaveBeenCalledTimes(1);
    expect(mockedTemplate.mock.calls[0][0]).toEqual({});
  });
});
