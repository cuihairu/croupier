import { request } from '@umijs/max';
import {
  clearAllCache,
  exportWorkspaceBackupBundle,
  importWorkspaceConfig,
  listWorkspaceVersions,
  loadWorkspaceConfig,
  publishWorkspaceConfig,
  rollbackWorkspaceVersion,
  saveWorkspaceConfig,
  unpublishWorkspaceConfig,
} from '@/services/workspaceConfig';
import type { WorkspaceConfig } from '@/types/workspace';

const requestMock = request as jest.Mock;

describe('workspace service api branches', () => {
  const baseConfig: WorkspaceConfig = {
    objectKey: 'player',
    title: '玩家工作台',
    layout: {
      type: 'tabs',
      tabs: [
        {
          key: 'base',
          title: '基础',
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

  it('publish/unpublish 成功分支', async () => {
    requestMock
      .mockResolvedValueOnce({ published: true, objectKey: 'player' })
      .mockResolvedValueOnce({ published: false, objectKey: 'player' });

    const publishResult = await publishWorkspaceConfig('player');
    const unpublishResult = await unpublishWorkspaceConfig('player');

    expect(publishResult).toEqual({ published: true, objectKey: 'player' });
    expect(unpublishResult).toEqual({ published: false, objectKey: 'player' });
    expect(requestMock).toHaveBeenNthCalledWith(
      1,
      '/api/v1/workspaces/player/publish',
      expect.objectContaining({ method: 'POST' }),
    );
    expect(requestMock).toHaveBeenNthCalledWith(
      2,
      '/api/v1/workspaces/player/unpublish',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('listWorkspaceVersions 返回数组与空数组分支', async () => {
    requestMock.mockResolvedValueOnce({
      items: [
        {
          id: '2',
          objectKey: 'player',
          version: 2,
          config: baseConfig,
        },
      ],
    });
    const versions = await listWorkspaceVersions('player');
    expect(versions).toHaveLength(1);
    expect(versions[0].version).toBe(2);

    requestMock.mockResolvedValueOnce({});
    const empty = await listWorkspaceVersions('player');
    expect(empty).toEqual([]);
  });

  it('listWorkspaceVersions 透传时间过滤参数', async () => {
    requestMock.mockResolvedValueOnce({ items: [] });
    await listWorkspaceVersions('player', {
      from: '2026-03-01T00:00:00.000Z',
      to: '2026-03-07T00:00:00.000Z',
    });
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/player/versions',
      expect.objectContaining({
        method: 'GET',
        params: {
          from: '2026-03-01T00:00:00.000Z',
          to: '2026-03-07T00:00:00.000Z',
        },
      }),
    );
  });

  it('rollbackWorkspaceVersion 成功并触发缓存清理', async () => {
    requestMock
      .mockResolvedValueOnce(baseConfig)
      .mockResolvedValueOnce({ objectKey: 'player', version: 3 })
      .mockResolvedValueOnce(null);

    await saveWorkspaceConfig(baseConfig);
    const cached = await loadWorkspaceConfig('player');
    expect(cached?.title).toBe('玩家工作台');

    const rollbackResult = await rollbackWorkspaceVersion('player', '3');
    expect(rollbackResult).toEqual({ objectKey: 'player', version: 3 });

    const afterRollback = await loadWorkspaceConfig('player');
    expect(afterRollback).toBeNull();
  });

  it('publish 失败分支透传错误', async () => {
    requestMock.mockRejectedValueOnce(new Error('publish failed'));
    await expect(publishWorkspaceConfig('player')).rejects.toThrow('publish failed');
  });

  it('importWorkspaceConfig 支持冲突 key 重写并强制生成草稿', async () => {
    requestMock.mockResolvedValueOnce({
      ...baseConfig,
      objectKey: 'player',
      status: 'draft',
      published: false,
    });
    const raw = JSON.stringify({
      ...baseConfig,
      objectKey: 'order',
      status: 'published',
      published: true,
      publishedAt: '2026-03-07T12:00:00.000Z',
      publishedBy: 'tester',
    });
    const result = await importWorkspaceConfig(raw, {
      targetObjectKey: 'player',
      forceDraft: true,
    });
    expect(result.objectKey).toBe('player');
    expect(requestMock).toHaveBeenCalledWith(
      '/api/v1/workspaces/player/config',
      expect.objectContaining({
        method: 'PUT',
        data: expect.objectContaining({
          objectKey: 'player',
          status: 'draft',
          published: false,
          publishedAt: undefined,
          publishedBy: undefined,
        }),
      }),
    );
  });

  it('exportWorkspaceBackupBundle 返回草稿/发布版/版本快照', async () => {
    requestMock
      .mockResolvedValueOnce({
        ...baseConfig,
        objectKey: 'player',
        status: 'draft',
        published: false,
      })
      .mockResolvedValueOnce({
        items: [
          {
            id: '1',
            objectKey: 'player',
            version: 1,
            isCurrentPublished: true,
            config: { ...baseConfig, published: true, status: 'published' },
          },
        ],
      });
    const content = await exportWorkspaceBackupBundle('player');
    const bundle = JSON.parse(content);
    expect(bundle.objectKey).toBe('player');
    expect(bundle.currentDraft?.objectKey).toBe('player');
    expect(bundle.currentPublished?.published).toBe(true);
    expect(Array.isArray(bundle.versions)).toBe(true);
    expect(bundle.versions).toHaveLength(1);
  });
});
