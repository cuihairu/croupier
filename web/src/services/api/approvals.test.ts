import { request } from '@umijs/max';
import { approveApproval, getApproval, listApprovals, rejectApproval } from './approvals';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('approvals API adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('lists approvals with passthrough query params', async () => {
    const payload = {
      approvals: [
        {
          id: 'apr-1',
          createdAt: '2026-09-01T00:00:00Z',
          actor: 'admin',
          functionId: 'player.ban',
          state: 'pending' as const,
          mode: 'invoke' as const,
        },
      ],
      total: 1,
      page: 1,
      size: 20,
    };
    mockedRequest.mockResolvedValue(payload);

    const params = { page: 1, pageSize: 20, status: 'pending' };
    await expect(listApprovals(params)).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/approvals/', { params });
  });

  it('lists approvals with params: undefined when called without arguments', async () => {
    mockedRequest.mockResolvedValue({});

    await expect(listApprovals()).resolves.toEqual({});

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/approvals/', { params: undefined });
  });

  it('URL-encodes approval ids on detail fetch', async () => {
    const row = {
      id: 'a/b c',
      createdAt: '2026-09-01T00:00:00Z',
      actor: 'admin',
      functionId: 'player.ban',
      state: 'pending' as const,
      mode: 'start_job' as const,
      payloadPreview: '{"a":1}',
    };
    mockedRequest.mockResolvedValue(row);

    await expect(getApproval('a/b c')).resolves.toBe(row);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/approvals/a%2Fb%20c');
  });

  it('approves with an otp body when otp is provided', async () => {
    mockedRequest.mockResolvedValue({ id: 'apr/1', state: 'approved' });

    await approveApproval({ id: 'apr/1', otp: '123456' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/approvals/apr%2F1/approve', {
      method: 'POST',
      data: { otp: '123456' },
    });
  });

  it('approves without a body when otp is missing or blank', async () => {
    mockedRequest.mockResolvedValue({ id: 'apr-1', state: 'approved' });

    await approveApproval({ id: 'apr-1' });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/approvals/apr-1/approve', {
      method: 'POST',
      data: undefined,
    });

    await approveApproval({ id: 'apr-1', otp: '' });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/approvals/apr-1/approve', {
      method: 'POST',
      data: undefined,
    });
  });

  it('rejects with a reason body and URL-encoded id', async () => {
    mockedRequest.mockResolvedValue({ id: 'apr 1', state: 'rejected', reason: '不允许' });

    await rejectApproval({ id: 'apr 1', reason: '不允许' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/approvals/apr%201/reject', {
      method: 'POST',
      data: { reason: '不允许' },
    });
  });
});
