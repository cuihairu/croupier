/** @jest-environment jsdom */

import {
  clearWorkspaceAlertRecords,
  initWorkspaceAlerting,
  listWorkspaceAlertRecords,
  recordWorkspaceAlertFromTelemetry,
} from '@/services/workspace/alerts';

describe('workspace alert bridge', () => {
  beforeEach(() => {
    clearWorkspaceAlertRecords();
    initWorkspaceAlerting();
  });

  it('records critical alert for workspace_render_error', () => {
    recordWorkspaceAlertFromTelemetry('workspace_render_error', {
      objectKey: 'player',
      reason: 'unsupported_layout',
    });
    const records = listWorkspaceAlertRecords();
    expect(records).toHaveLength(1);
    expect(records[0].title).toBe('关键渲染失败告警');
    expect(records[0].level).toBe('critical');
    expect(records[0].event).toBe('workspace_render_error');
  });

  it('records warning alert for publish errors', () => {
    recordWorkspaceAlertFromTelemetry('workspace_publish_error', {
      objectKey: 'player',
      error: 'publish failed',
    });
    const records = listWorkspaceAlertRecords();
    expect(records).toHaveLength(1);
    expect(records[0].title).toBe('发布后异常告警');
    expect(records[0].level).toBe('warning');
    expect(records[0].event).toBe('workspace_publish_error');
  });

  it('ignores non-alert telemetry events', () => {
    recordWorkspaceAlertFromTelemetry('workspace_page_open', { page: 'workspaces_index' });
    const records = listWorkspaceAlertRecords();
    expect(records).toHaveLength(0);
  });
});
