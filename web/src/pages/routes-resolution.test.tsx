import { act, render, screen } from '@testing-library/react';
import LoginLogsPage from './Admin/LoginLogs';
import OperationLogsPage from './Admin/OperationLogs';
import TicketDetailPage from './Support/Tickets/Detail';

jest.mock('@/services/api', () => ({
  listAudit: jest.fn().mockResolvedValue({ events: [] }),
}));

jest.mock('@/services/api/support', () => ({
  getTicket: jest.fn().mockResolvedValue({}),
  listTicketComments: jest.fn().mockResolvedValue({ comments: [] }),
  updateTicket: jest.fn(),
  deleteTicket: jest.fn(),
  addTicketComment: jest.fn(),
  transitionTicket: jest.fn(),
}));

jest.mock('@/services/api/storage', () => ({ uploadAsset: jest.fn() }));

jest.mock('@umijs/max', () => ({
  history: { push: jest.fn() },
  useParams: () => ({ id: 'ticket-1' }),
  useModel: () => ({ initialState: { currentUser: { name: 'tester' } } }),
}));

describe('configured page route modules', () => {
  test.each([
    ['Admin/LoginLogs', LoginLogsPage, /登录日志/],
    ['Admin/OperationLogs', OperationLogsPage, /操作日志/],
    ['Support/Tickets/Detail', TicketDetailPage, /工单详情/],
  ])('%s resolves to the real page instead of a placeholder', async (_route, Component, title) => {
    await act(async () => {
      render(<Component />);
    });

    expect(screen.getByText(title)).toBeInTheDocument();
    expect(screen.queryByText(/建设中|敬请期待/)).not.toBeInTheDocument();
  });
});
