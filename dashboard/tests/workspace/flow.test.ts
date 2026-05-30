import { request } from '@umijs/max';
import {
  clearAllCache,
  listPublishedWorkspaceConfigs,
  loadWorkspaceConfig,
  publishWorkspaceConfig,
  rollbackWorkspaceVersion,
  saveWorkspaceConfig,
} from '@/services/workspaceConfig';
import type { WorkspaceConfig } from '@/types/workspace';

const requestMock = request as jest.Mock;

describe('workspace flow integration (service-level)', () => {
  const draftConfig: WorkspaceConfig = {
    objectKey: 'player',
    title: '玩家工作台',
    layout: {
      type: 'tabs',
      tabs: [
        {
          key: 'main',
          title: '主列表',
          functions: ['player.list'],
          layout: {
            type: 'list',
            listFunction: 'player.list',
            columns: [{ key: 'id', title: 'ID' }],
          },
        },
      ],
    },
  };

  beforeEach(() => {
    requestMock.mockReset();
    clearAllCache();
  });

  it('核心成功链路：保存草稿 -> 发布 -> 控制台可读 -> 回滚', async () => {
    requestMock
      .mockResolvedValueOnce(draftConfig)
      .mockResolvedValueOnce({ published: true, objectKey: 'player' })
      .mockResolvedValueOnce({ items: [{ ...draftConfig, published: true }] })
      .mockResolvedValueOnce({ objectKey: 'player', version: 2 });

    const saved = await saveWorkspaceConfig(draftConfig);
    expect(saved.objectKey).toBe('player');

    const published = await publishWorkspaceConfig('player');
    expect(published.published).toBe(true);

    const publishedList = await listPublishedWorkspaceConfigs();
    expect(publishedList.length).toBe(1);
    expect(publishedList[0].objectKey).toBe('player');

    const rollback = await rollbackWorkspaceVersion('player', '2');
    expect(rollback.version).toBe(2);

    expect(requestMock).toHaveBeenNthCalledWith(
      1,
      '/api/v1/workspaces/player/config',
      expect.objectContaining({ method: 'PUT' }),
    );
    expect(requestMock).toHaveBeenNthCalledWith(
      2,
      '/api/v1/workspaces/player/publish',
      expect.objectContaining({ method: 'POST' }),
    );
    expect(requestMock).toHaveBeenNthCalledWith(
      3,
      '/api/v1/workspaces/published',
      expect.objectContaining({ method: 'GET' }),
    );
    expect(requestMock).toHaveBeenNthCalledWith(
      4,
      '/api/v1/workspaces/player/rollback',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('关键失败链路：发布失败后保留草稿可读', async () => {
    requestMock
      .mockResolvedValueOnce(draftConfig)
      .mockRejectedValueOnce(new Error('publish failed'))
      .mockResolvedValueOnce(draftConfig);

    await saveWorkspaceConfig(draftConfig);
    await expect(publishWorkspaceConfig('player')).rejects.toThrow('publish failed');

    const reloaded = await loadWorkspaceConfig('player', { forceRefresh: true });
    expect(reloaded?.objectKey).toBe('player');
    expect(reloaded?.title).toBe('玩家工作台');
  });
});
