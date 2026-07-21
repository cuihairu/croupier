import { request } from '@umijs/max';
import { loadAuthedInitialState, loadConsoleWorkspaceConfigs } from '@/services/initialState';

describe('runtime initial state workspace menus', () => {
  beforeEach(() => {
    (request as jest.Mock).mockClear();
  });

  it('加载已发布工作台作为动态控制台菜单输入', async () => {
    const configs = await loadConsoleWorkspaceConfigs();

    expect(configs).toEqual([
      expect.objectContaining({
        objectKey: 'player.ban',
        title: '封禁玩家',
      }),
    ]);
    expect(request).toHaveBeenCalledWith('/api/v1/workspaces/published', {
      method: 'GET',
      skipErrorHandler: true,
    });
  });

  it('已发布工作台加载失败时直接暴露错误', async () => {
    (request as jest.Mock).mockRejectedValueOnce(new Error('workspace menu failed'));

    await expect(loadConsoleWorkspaceConfigs()).rejects.toThrow('workspace menu failed');
  });

  it('登录态初始化同时返回用户和已发布工作台', async () => {
    const state = await loadAuthedInitialState(async () => ({
      name: 'admin',
      roles: ['admin'],
    }));

    expect(state.currentUser).toEqual({ name: 'admin', roles: ['admin'] });
    expect(state.workspaceConfigs).toEqual([
      expect.objectContaining({
        objectKey: 'player.ban',
      }),
    ]);
  });
});
