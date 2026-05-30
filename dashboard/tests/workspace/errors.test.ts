import { getWorkspaceErrorMessage, parseWorkspaceError } from '@/services/workspace/errors';

describe('workspace errors', () => {
  it('parseWorkspaceError: 404 映射为 workspace_not_found', () => {
    const parsed = parseWorkspaceError({
      response: {
        status: 404,
        data: {
          code: 'workspace_not_found',
          message: 'workspace config not found',
          request_id: 'rid-1',
        },
      },
    });

    expect(parsed.code).toBe('workspace_not_found');
    expect(parsed.message).toBe('workspace config not found');
    expect(parsed.requestId).toBe('rid-1');
  });

  it('parseWorkspaceError: 403 映射为 forbidden', () => {
    const parsed = parseWorkspaceError({
      response: {
        status: 403,
        data: {
          code: 'forbidden',
          message: 'permission denied',
        },
      },
    });
    expect(parsed.code).toBe('forbidden');
  });

  it('parseWorkspaceError: workspace_version_not_found 保持原语义', () => {
    const parsed = parseWorkspaceError({
      response: {
        status: 400,
        data: {
          code: 'workspace_version_not_found',
          message: 'workspace version not found',
        },
      },
    });
    expect(parsed.code).toBe('workspace_version_not_found');
    expect(parsed.message).toBe('workspace version not found');
  });

  it('getWorkspaceErrorMessage: Request failed 使用 fallback 兜底', () => {
    const message = getWorkspaceErrorMessage(
      {
        message: 'Request failed',
      },
      '操作失败',
    );

    expect(message).toBe('操作失败');
  });
});
