/**
 * CronJobsDrawer 覆盖收口（web 覆盖率巡检：加载 effect / titleWithAgent
 * 分支 / enabled 两翼此前零触达）。Drawer 内容挂在 portal，用 document 级查询。
 */
import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import CronJobsDrawer from '../CronJobsDrawer';
import { fetchNodeCronJobs, type NodeCronJob } from '@/services/api/ops';
import type { NodeRow } from '../shared';

jest.mock('@/services/api/ops', () => ({
  fetchNodeCronJobs: jest.fn(),
}));
const mockFetch = fetchNodeCronJobs as jest.MockedFunction<typeof fetchNodeCronJobs>;

const NODE: NodeRow = {
  agentId: 'agent-1',
} as unknown as NodeRow;

const JOB: NodeCronJob = {
  schedule: '*/5 * * * *',
  command: '/usr/bin/backup.sh',
  user: 'root',
  sourceFile: '/etc/crontab',
  enabled: true,
};

beforeEach(() => {
  jest.clearAllMocks();
});

describe('CronJobsDrawer', () => {
  it('node 非空：按 agentId 拉取并渲染条目，标题带 agentId，启用/停用两翼', async () => {
    mockFetch.mockResolvedValue([JOB, { ...JOB, enabled: false, command: '/bin/off.sh' }]);
    render(<CronJobsDrawer node={NODE} onClose={() => {}} />);

    expect(mockFetch).toHaveBeenCalledWith('agent-1');
    // titleWithAgent 分支
    expect(await screen.findByText('主机定时任务 · agent-1')).toBeInTheDocument();
    // 行渲染 + enabled render 两翼
    expect(await screen.findByText('/usr/bin/backup.sh')).toBeInTheDocument();
    expect(screen.getByText('启用')).toBeInTheDocument();
    expect(screen.getByText('停用')).toBeInTheDocument();
  });

  it('拉取失败：面板展示空态文案，不崩', async () => {
    mockFetch.mockRejectedValue(new Error('agent offline'));
    render(<CronJobsDrawer node={NODE} onClose={() => {}} />);

    expect(await screen.findByText('未读取到定时任务（或节点离线）')).toBeInTheDocument();
  });

  it('node 为空：effect 早退不发起拉取；抽屉未打开前内容不挂载', () => {
    const { baseElement } = render(<CronJobsDrawer node={null} onClose={() => {}} />);

    expect(mockFetch).not.toHaveBeenCalled();
    // antd Drawer open=false 且从未打开过：portal 内无表格内容
    expect(screen.queryByText('/usr/bin/backup.sh')).not.toBeInTheDocument();
    expect(baseElement.textContent).not.toContain('*/5 * * * *');
  });
});
