import { request } from '@umijs/max';
import type { PageProposal } from '@/types/dashboard';
import {
  acceptAndPublishProposal,
  acceptProposal,
  getChangeChain,
  getContract,
  getProposal,
  getResourceDetail,
  getResourceSemanticConflicts,
  getResourceSemanticVersions,
  getVersionDiff,
  listContracts,
  listProposalInbox,
  listProposals,
  listResourceCatalog,
  mergeChanges,
  regenerateProposal,
  rejectProposal,
  republish,
  resolveResourceSemanticConflict,
  rollbackDraft,
  rollbackPublish,
  updateResourceSemantics,
} from './dashboard';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('dashboard resource-catalog API service', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('lists resource catalog entries with optional filters', async () => {
    const payload = { items: [], total: 0 };
    mockedRequest.mockResolvedValue(payload);

    await expect(listResourceCatalog({ category: 'player', query: '充值' })).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/resource-catalog', {
      method: 'GET',
      params: { category: 'player', query: '充值' },
    });

    await expect(listResourceCatalog()).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/resource-catalog', {
      method: 'GET',
      params: undefined,
    });
  });

  it('fetches a resource detail', async () => {
    const detail = { resourceKey: 'player', labels: {}, status: 'identified', functions: [] };
    mockedRequest.mockResolvedValue(detail);

    await expect(getResourceDetail('player')).resolves.toBe(detail);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/resource-catalog/player', {
      method: 'GET',
    });
  });

  it('updates resource semantics via PUT', async () => {
    const payload = { version: 2, source: 'platform_review', message: 'ok' };
    mockedRequest.mockResolvedValue(payload);

    await expect(
      updateResourceSemantics('player', { identityField: 'id', changeReason: '修正' }),
    ).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/resource-catalog/player/semantics', {
      method: 'PUT',
      data: { identityField: 'id', changeReason: '修正' },
    });
  });

  it('gets resource semantic conflicts', async () => {
    const payload = { conflicts: [], provenance: [] };
    mockedRequest.mockResolvedValue(payload);

    await expect(getResourceSemanticConflicts('player')).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/resource-catalog/player/conflicts', {
      method: 'GET',
    });
  });

  it('lists resource semantic versions with optional pagination', async () => {
    const payload = { items: [], total: 0 };
    mockedRequest.mockResolvedValue(payload);

    await expect(getResourceSemanticVersions('player', { limit: 5, offset: 5 })).resolves.toBe(
      payload,
    );
    expect(mockedRequest).toHaveBeenCalledWith(
      '/api/v1/resource-catalog/player/semantics/versions',
      {
        method: 'GET',
        params: { limit: 5, offset: 5 },
      },
    );

    await expect(getResourceSemanticVersions('player')).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenLastCalledWith(
      '/api/v1/resource-catalog/player/semantics/versions',
      {
        method: 'GET',
        params: undefined,
      },
    );
  });

  it('resolves a semantic conflict field via POST', async () => {
    const payload = { message: 'ok' };
    mockedRequest.mockResolvedValue(payload);

    await expect(
      resolveResourceSemanticConflict('player', 'identityField', {
        chosenSource: 'platform_review',
        reason: '人工裁定',
      }),
    ).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenCalledWith(
      '/api/v1/resource-catalog/player/conflicts/identityField/resolve',
      {
        method: 'POST',
        data: { chosenSource: 'platform_review', reason: '人工裁定' },
      },
    );
  });
});

describe('dashboard versioning API service', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('gets the page change chain', async () => {
    const payload = { pageKey: 'player-list', resourceKey: 'player', items: [], current: {} };
    mockedRequest.mockResolvedValue(payload);

    await expect(getChangeChain('player-list')).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/versioning/pages/player-list/chain', {
      method: 'GET',
    });
  });

  it('gets the version diff with from/to params', async () => {
    const payload = { changes: [], summary: '无差异' };
    mockedRequest.mockResolvedValue(payload);

    await expect(getVersionDiff('player-list', { fromVersion: 1, toVersion: 3 })).resolves.toBe(
      payload,
    );
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/versioning/pages/player-list/diff', {
      method: 'GET',
      params: { fromVersion: 1, toVersion: 3 },
    });
  });

  it('merges page changes via POST', async () => {
    const payload = { merged: 1, conflicts: 0, message: '已合并' };
    mockedRequest.mockResolvedValue(payload);

    await expect(mergeChanges('player-list', { strategy: 'auto' })).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/versioning/pages/player-list/merge', {
      method: 'POST',
      data: { strategy: 'auto' },
    });
  });

  it('rolls back draft and publish by version', async () => {
    const payload = { message: '已回滚' };
    mockedRequest.mockResolvedValue(payload);

    await expect(rollbackDraft('player-list', { version: 3, reason: '配置错误' })).resolves.toBe(
      payload,
    );
    expect(mockedRequest).toHaveBeenCalledWith(
      '/api/v1/versioning/pages/player-list/rollback-draft',
      {
        method: 'POST',
        data: { version: 3, reason: '配置错误' },
      },
    );

    await expect(rollbackPublish('player-list', { version: 2 })).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenLastCalledWith(
      '/api/v1/versioning/pages/player-list/rollback-publish',
      {
        method: 'POST',
        data: { version: 2 },
      },
    );
  });

  it('regenerates proposals with and without force', async () => {
    const payload = { message: '已重新生成' };
    mockedRequest.mockResolvedValue(payload);

    await expect(regenerateProposal('player-list', { force: true })).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/versioning/pages/player-list/regenerate', {
      method: 'POST',
      data: { force: true },
    });

    await expect(regenerateProposal('player-list')).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenLastCalledWith(
      '/api/v1/versioning/pages/player-list/regenerate',
      {
        method: 'POST',
        data: undefined,
      },
    );
  });

  it('republishes with and without reason', async () => {
    const payload = { version: 4, message: '已重新发布' };
    mockedRequest.mockResolvedValue(payload);

    await expect(republish('player-list', { reason: '修复' })).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/versioning/pages/player-list/republish', {
      method: 'POST',
      data: { reason: '修复' },
    });

    await expect(republish('player-list')).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenLastCalledWith(
      '/api/v1/versioning/pages/player-list/republish',
      {
        method: 'POST',
        data: undefined,
      },
    );
  });
});

describe('dashboard contracts & proposals API service', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('lists contracts and fetches one by function id', async () => {
    const contracts = [{ functionId: 'player.list' }];
    mockedRequest.mockResolvedValue(contracts);

    await expect(listContracts()).resolves.toBe(contracts);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/contracts', { method: 'GET' });

    const detail = { functionId: 'player.list', inputSchema: {} };
    mockedRequest.mockResolvedValue(detail);

    await expect(getContract('player.list')).resolves.toBe(detail);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/contracts/player.list', { method: 'GET' });
  });

  it('lists proposals with optional filters', async () => {
    const proposals: PageProposal[] = [];
    mockedRequest.mockResolvedValue(proposals);

    await expect(listProposals({ status: 'pending', resourceKey: 'player' })).resolves.toBe(
      proposals,
    );
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/proposals', {
      method: 'GET',
      params: { status: 'pending', resourceKey: 'player' },
    });

    await expect(listProposals()).resolves.toBe(proposals);
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/proposals', {
      method: 'GET',
      params: undefined,
    });
  });

  it('gets the proposal inbox scoped by resource', async () => {
    const inbox = {
      publishable: [],
      needsReview: [],
      blockedIssues: [],
      contractChanges: [],
      summary: { publishable: 0, needsReview: 0, blockedIssues: 0, contractChanges: 0 },
    };
    mockedRequest.mockResolvedValue(inbox);

    await expect(listProposalInbox({ resourceKey: 'player' })).resolves.toBe(inbox);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/proposals/inbox', {
      method: 'GET',
      params: { resourceKey: 'player' },
    });

    await expect(listProposalInbox()).resolves.toBe(inbox);
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/proposals/inbox', {
      method: 'GET',
      params: undefined,
    });
  });

  it('gets, accepts, accept-and-publishes and rejects proposals', async () => {
    const proposal = { proposalKey: 'pr-1', status: 'pending' };
    mockedRequest.mockResolvedValue(proposal);

    await expect(getProposal('pr-1')).resolves.toBe(proposal);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/proposals/pr-1', { method: 'GET' });

    mockedRequest.mockResolvedValue({ message: '已接受' });
    await expect(acceptProposal('pr-1')).resolves.toEqual({ message: '已接受' });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/proposals/pr-1/accept', { method: 'POST' });

    mockedRequest.mockResolvedValue({
      pageKey: 'player-list',
      draftRevision: 2,
      publishedVersion: 3,
    });
    await expect(acceptAndPublishProposal('pr-1')).resolves.toEqual({
      pageKey: 'player-list',
      draftRevision: 2,
      publishedVersion: 3,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/proposals/pr-1/accept-and-publish', {
      method: 'POST',
    });

    mockedRequest.mockResolvedValue({ message: '已拒绝' });
    await expect(rejectProposal('pr-1')).resolves.toEqual({ message: '已拒绝' });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/proposals/pr-1/reject', { method: 'POST' });
  });
});
