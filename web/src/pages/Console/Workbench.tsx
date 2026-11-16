/**
 * Console/Workbench - 运营工作台面板（GUX-003）
 *
 * 按现有权限聚合四类工作面板（GUX-002 设计：工作区首页展示该角色的
 * 待办/异常/最近处理/常用入口）：
 * - 待办：审批待办（audit:read）+ 未读消息（登录即可）；
 * - 异常：契约失效页面（来自已加载的发布页清单，无额外请求）+ 运维告警（ops:read）；
 * - 最近处理：我的执行记录（execution-logs `mine=true`，无需审计权限）；
 * - 常用入口：按权限过滤的固定入口（模板页与审批/菜单管理）。
 *
 * 面板语义：单面板失败只在该面板显示可定位错误并可重试，不拖垮整页；
 * 权限外面板整体不渲染（而非显示无权限提示）；数据缺失渲染真实空态。
 * 作用域切换时 scoped 请求（审批）经 useScope 依赖重拉。
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { FormattedMessage, history, useAccess } from '@umijs/max';
import { Alert, Button, Card, Col, Empty, Row, Skeleton, Space, Tag, Typography } from 'antd';
import {
  AuditOutlined,
  BellOutlined,
  ClockCircleOutlined,
  MenuOutlined,
  RightOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { listApprovals, type ApprovalRow } from '@/services/api/approvals';
import { unreadCount } from '@/services/api/messages';
import { listAlerts, type Alert as OpsAlert } from '@/services/api/alerts';
import { listExecutionLogs, type ExecutionLogItem } from '@/services/api/executionLogs';
import { useScope } from '@/hooks/useScopeReload';
import type { PublishedPageSpec } from '@/types/dashboard';
import { formatDateTime } from '@/utils/format';

type ConsoleWorkbenchAccess = {
  canConsoleRead?: boolean;
  canAuditRead?: boolean;
  canOpsRead?: boolean;
  canPageRead?: boolean;
};

type PanelState<T> = { loading: boolean; error: string; data: T | null };

/** 面板数据加载：enabled=false 不发请求；scopeKey 变化重拉（scoped 数据跟随
 * 顶栏游戏/环境切换）；tick 供重试按钮复跑同一加载器。 */
function usePanel<T>(
  enabled: boolean,
  load: () => Promise<T>,
  scopeKey: string,
): PanelState<T> & { reload: () => void } {
  const [state, setState] = useState<PanelState<T>>({ loading: enabled, error: '', data: null });
  const [tick, setTick] = useState(0);
  const loadRef = useRef(load);
  loadRef.current = load;
  useEffect(() => {
    if (!enabled) return;
    let mounted = true;
    setState({ loading: true, error: '', data: null });
    loadRef
      .current()
      .then((data) => {
        if (mounted) setState({ loading: false, error: '', data });
      })
      .catch((err: unknown) => {
        if (mounted)
          setState({
            loading: false,
            error: err instanceof Error ? err.message : String(err),
            data: null,
          });
      });
    return () => {
      mounted = false;
    };
  }, [enabled, scopeKey, tick]);
  const reload = useCallback(() => setTick((value) => value + 1), []);
  return { ...state, reload };
}

function PanelError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <Alert
      type="warning"
      showIcon
      title={message}
      action={
        <Button size="small" onClick={onRetry}>
          <FormattedMessage id="pages.console.workbench.retry" defaultMessage="重试" />
        </Button>
      }
    />
  );
}

function PanelEmpty({ id, defaultMessage }: { id: string; defaultMessage: string }) {
  return (
    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE}>
      <Typography.Text type="secondary">
        <FormattedMessage id={id} defaultMessage={defaultMessage} />
      </Typography.Text>
    </Empty>
  );
}

function entryButton(labelId: string, defaultMessage: string, path: string, icon?: ReactNode) {
  return (
    <Button key={path} icon={icon} onClick={() => history.push(path)}>
      <FormattedMessage id={labelId} defaultMessage={defaultMessage} />
    </Button>
  );
}

/** 待办卡：审批待办 + 未读消息。 */
function TodoCard({ access, scopeKey }: { access: ConsoleWorkbenchAccess; scopeKey: string }) {
  const approvals = usePanel(
    Boolean(access.canAuditRead),
    () => listApprovals({ page: 1, pageSize: 5, status: 'pending' }),
    scopeKey,
  );
  const messages = usePanel(true, () => unreadCount(), scopeKey);
  const approvalRows = approvals.data?.approvals ?? [];

  return (
    <Card
      title={
        <Space size={8}>
          <BellOutlined />
          <FormattedMessage id="pages.console.workbench.todo.title" defaultMessage="待办" />
        </Space>
      }
      size="small"
    >
      <Space orientation="vertical" size={12} style={{ width: '100%' }}>
        {access.canAuditRead ? (
          <div data-testid="workbench-approvals">
            <Typography.Text strong>
              <FormattedMessage
                id="pages.console.workbench.todo.approvals"
                defaultMessage="待审批"
              />
              {approvals.data ? (
                <Tag
                  color={approvalRows.length > 0 ? 'orange' : 'default'}
                  style={{ marginLeft: 8 }}
                >
                  {approvals.data.total ?? approvalRows.length}
                </Tag>
              ) : null}
            </Typography.Text>
            {approvals.loading ? (
              <Skeleton active paragraph={false} />
            ) : approvals.error ? (
              <PanelError message={approvals.error} onRetry={approvals.reload} />
            ) : approvalRows.length === 0 ? (
              <PanelEmpty
                id="pages.console.workbench.todo.approvalsEmpty"
                defaultMessage="暂无待审批"
              />
            ) : (
              <Space orientation="vertical" size={4} style={{ width: '100%' }}>
                {approvalRows.map((row: ApprovalRow) => (
                  <Button
                    key={row.id}
                    type="link"
                    size="small"
                    style={{ padding: 0, justifyContent: 'flex-start' }}
                    onClick={() => history.push('/approvals')}
                  >
                    <Typography.Text code style={{ fontSize: 12 }}>
                      {row.functionId}
                    </Typography.Text>
                    <Typography.Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>
                      {row.actor} · {formatDateTime(row.createdAt)}
                    </Typography.Text>
                  </Button>
                ))}
                <Button
                  type="link"
                  size="small"
                  style={{ padding: 0 }}
                  onClick={() => history.push('/approvals')}
                >
                  <FormattedMessage
                    id="pages.console.workbench.todo.goApprovals"
                    defaultMessage="去审批中心"
                  />
                  <RightOutlined style={{ fontSize: 10, marginLeft: 4 }} />
                </Button>
              </Space>
            )}
          </div>
        ) : null}
        <div data-testid="workbench-messages">
          <Button
            type="link"
            size="small"
            style={{ padding: 0 }}
            onClick={() => history.push('/admin/account/messages')}
          >
            <FormattedMessage
              id="pages.console.workbench.todo.unread"
              defaultMessage="未读消息 {count} 条"
              values={{ count: messages.data?.count ?? 0 }}
            />
          </Button>
          {messages.error ? (
            <PanelError message={messages.error} onRetry={messages.reload} />
          ) : null}
        </div>
        {!access.canAuditRead &&
        !messages.loading &&
        !messages.error &&
        (messages.data?.count ?? 0) === 0 ? (
          <PanelEmpty id="pages.console.workbench.todo.empty" defaultMessage="暂无待办" />
        ) : null}
      </Space>
    </Card>
  );
}

/** 异常卡：契约失效页面（来自发布页清单）+ 运维告警（ops:read）。 */
function AnomalyCard({
  access,
  scopeKey,
  pages,
}: {
  access: ConsoleWorkbenchAccess;
  scopeKey: string;
  pages: PublishedPageSpec[];
}) {
  const stalePages = pages.filter((page) => (page.bindingFreshness?.length || 0) > 0);
  const alerts = usePanel(
    Boolean(access.canOpsRead),
    () => listAlerts({ page: 1, pageSize: 5 }),
    scopeKey,
  );
  const alertRows = alerts.data?.items ?? [];
  const hasAnomaly = stalePages.length > 0 || alertRows.length > 0;

  return (
    <Card
      title={
        <Space size={8}>
          <WarningOutlined />
          <FormattedMessage id="pages.console.workbench.anomaly.title" defaultMessage="异常" />
        </Space>
      }
      size="small"
    >
      <Space orientation="vertical" size={12} style={{ width: '100%' }}>
        <div data-testid="workbench-stale">
          <Button
            type="link"
            size="small"
            style={{ padding: 0 }}
            onClick={() => history.push('/console/home')}
          >
            {stalePages.length > 0 ? (
              <>
                <FormattedMessage
                  id="pages.console.workbench.anomaly.stale"
                  defaultMessage="契约失效页面 {count} 个"
                  values={{ count: stalePages.length }}
                />
                <RightOutlined style={{ fontSize: 10, marginLeft: 4 }} />
              </>
            ) : (
              <FormattedMessage
                id="pages.console.workbench.anomaly.staleClear"
                defaultMessage="契约全部有效"
              />
            )}
          </Button>
          {stalePages.length > 0 ? (
            <Typography.Text type="secondary" style={{ display: 'block', fontSize: 12 }}>
              {stalePages
                .slice(0, 5)
                .map((page) => page.pageKey)
                .join('、')}
            </Typography.Text>
          ) : null}
        </div>
        {access.canOpsRead ? (
          <div data-testid="workbench-alerts">
            <Typography.Text strong>
              <FormattedMessage
                id="pages.console.workbench.anomaly.alerts"
                defaultMessage="运维告警"
              />
              {alerts.data ? (
                <Tag color={alertRows.length > 0 ? 'red' : 'default'} style={{ marginLeft: 8 }}>
                  {alerts.data.total ?? alertRows.length}
                </Tag>
              ) : null}
            </Typography.Text>
            {alerts.loading ? (
              <Skeleton active paragraph={false} />
            ) : alerts.error ? (
              <PanelError message={alerts.error} onRetry={alerts.reload} />
            ) : alertRows.length === 0 ? (
              <PanelEmpty
                id="pages.console.workbench.anomaly.alertsEmpty"
                defaultMessage="暂无告警"
              />
            ) : (
              <Space orientation="vertical" size={4} style={{ width: '100%' }}>
                {alertRows.map((row: OpsAlert) => (
                  <Button
                    key={row.id}
                    type="link"
                    size="small"
                    style={{ padding: 0, justifyContent: 'flex-start' }}
                    onClick={() => history.push('/risk/alerts')}
                  >
                    <Tag
                      color={row.level === 'critical' || row.level === 'high' ? 'red' : 'orange'}
                    >
                      {row.level}
                    </Tag>
                    <Typography.Text style={{ fontSize: 12 }} ellipsis>
                      {row.message}
                    </Typography.Text>
                  </Button>
                ))}
                <Button
                  type="link"
                  size="small"
                  style={{ padding: 0 }}
                  onClick={() => history.push('/risk/alerts')}
                >
                  <FormattedMessage
                    id="pages.console.workbench.anomaly.goAlerts"
                    defaultMessage="去告警中心"
                  />
                  <RightOutlined style={{ fontSize: 10, marginLeft: 4 }} />
                </Button>
              </Space>
            )}
          </div>
        ) : null}
        {!access.canOpsRead && !hasAnomaly ? (
          <PanelEmpty id="pages.console.workbench.anomaly.empty" defaultMessage="暂无异常" />
        ) : null}
      </Space>
    </Card>
  );
}

/** 最近处理卡：我的执行记录（mine=true，无需审计权限；有审计权限才给入口链接）。 */
function RecentCard({ access, scopeKey }: { access: ConsoleWorkbenchAccess; scopeKey: string }) {
  const logs = usePanel(
    true,
    () => listExecutionLogs({ page: 1, pageSize: 5, mine: true }),
    scopeKey,
  );
  const rows = logs.data?.items ?? [];

  return (
    <Card
      title={
        <Space size={8}>
          <ClockCircleOutlined />
          <FormattedMessage id="pages.console.workbench.recent.title" defaultMessage="最近处理" />
        </Space>
      }
      size="small"
      extra={
        access.canAuditRead ? (
          <Button
            type="link"
            size="small"
            style={{ padding: 0 }}
            onClick={() => history.push('/functions/execution-logs')}
          >
            <FormattedMessage
              id="pages.console.workbench.recent.goLogs"
              defaultMessage="执行记录"
            />
            <RightOutlined style={{ fontSize: 10, marginLeft: 4 }} />
          </Button>
        ) : undefined
      }
    >
      {logs.loading ? (
        <Skeleton active paragraph={false} />
      ) : logs.error ? (
        <PanelError message={logs.error} onRetry={logs.reload} />
      ) : rows.length === 0 ? (
        <PanelEmpty id="pages.console.workbench.recent.empty" defaultMessage="暂无执行记录" />
      ) : (
        <Space
          orientation="vertical"
          size={4}
          style={{ width: '100%' }}
          data-testid="workbench-recent-rows"
        >
          {rows.map((row: ExecutionLogItem) => (
            <div key={row.id}>
              <Space size={8} wrap>
                <Typography.Text code style={{ fontSize: 12 }}>
                  {row.functionId}
                </Typography.Text>
                <Tag
                  color={
                    row.status === 'success' ? 'green' : row.status === 'failed' ? 'red' : 'default'
                  }
                >
                  {row.status}
                </Tag>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  {formatDateTime(row.createdAt)}
                </Typography.Text>
              </Space>
            </div>
          ))}
        </Space>
      )}
    </Card>
  );
}

/** 运营工作台首页：待办 / 异常 / 最近处理 + 常用入口。 */
export default function ConsoleWorkbench({ pages }: { pages: PublishedPageSpec[] }) {
  const access = useAccess() as ConsoleWorkbenchAccess;
  const { scopeKey } = useScope();
  const entries = [
    ...(access.canAuditRead
      ? [
          entryButton(
            'pages.console.workbench.entry.approvals',
            '审批中心',
            '/approvals',
            // 图标为装饰件：aria-hidden 避免图标名（audit/menu）混入按钮可及名
            <AuditOutlined aria-hidden="true" key="icon" />,
          ),
        ]
      : []),
    entryButton(
      'pages.console.workbench.entry.playerManage',
      '玩家管理模板',
      '/console/templates/player-manage',
    ),
    entryButton(
      'pages.console.workbench.entry.mailCompensation',
      '邮件与补偿模板',
      '/console/templates/mail-compensation',
    ),
    entryButton(
      'pages.console.workbench.entry.orderReview',
      '订单审核模板',
      '/console/templates/order-review',
    ),
    entryButton(
      'pages.console.workbench.entry.banManagement',
      '封禁管理模板',
      '/console/templates/ban-management',
    ),
    entryButton(
      'pages.console.workbench.entry.activityConfig',
      '活动配置模板',
      '/console/templates/activity-config',
    ),
    ...(access.canPageRead
      ? [
          entryButton(
            'pages.console.workbench.entry.menu',
            '菜单管理',
            '/functions/menus',
            <MenuOutlined aria-hidden="true" key="icon" />,
          ),
        ]
      : []),
  ];

  return (
    <Space orientation="vertical" size={16} style={{ width: '100%' }}>
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={8}>
          <TodoCard access={access} scopeKey={scopeKey} />
        </Col>
        <Col xs={24} lg={8}>
          <AnomalyCard access={access} scopeKey={scopeKey} pages={pages} />
        </Col>
        <Col xs={24} lg={8}>
          <RecentCard access={access} scopeKey={scopeKey} />
        </Col>
      </Row>
      <Card
        title={
          <FormattedMessage id="pages.console.workbench.entries.title" defaultMessage="常用入口" />
        }
        size="small"
      >
        <Space wrap size={[8, 8]} data-testid="workbench-entries">
          {entries}
        </Space>
      </Card>
    </Space>
  );
}
