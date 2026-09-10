import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Descriptions,
  Drawer,
  Input,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { listOpsTasks, type OpsTask, listOpsFunctions } from '@/services/api/ops';
import {
  cancelTask,
  fetchTaskResult,
  subscribeTaskEvents,
  type TaskEventSubscription,
} from '@/services/api/functions';
import { StandardFilterBar, StandardListSection, SummaryOverview } from '@/components';
import type { JSONValue } from '@/types/dashboard';
import { formatDateTime } from '@/utils/format';
import { FormattedMessage, useIntl } from '@umijs/max';
const { Paragraph, Text } = Typography;

/** 模块级文案助手接收 intl 的最小结构（@umijs/max 未导出 IntlShape 类型） */
type IntlFormatter = {
  formatMessage: (descriptor: { id: string; defaultMessage: string }) => string;
};

function getTaskStatusMeta(state: string | undefined, intl: IntlFormatter) {
  if (state === 'running')
    return {
      color: 'blue',
      text: intl.formatMessage({ id: 'pages.opsJobs.status.running', defaultMessage: '运行中' }),
    };
  if (state === 'succeeded')
    return {
      color: 'green',
      text: intl.formatMessage({
        id: 'pages.opsJobs.status.succeeded',
        defaultMessage: '已成功',
      }),
    };
  if (state === 'failed')
    return {
      color: 'red',
      text: intl.formatMessage({ id: 'pages.opsJobs.status.failed', defaultMessage: '已失败' }),
    };
  if (state === 'canceled')
    return {
      color: 'default',
      text: intl.formatMessage({
        id: 'pages.opsJobs.status.canceled',
        defaultMessage: '已取消',
      }),
    };
  return { color: 'default', text: state || '-' };
}

export default function OpsTasksPage() {
  const { message, modal } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [rows, setRows] = useState<OpsTask[]>([]);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<string>('');
  const [fid, setFid] = useState<string>('');
  const [actor, setActor] = useState<string>('');
  const [funcs, setFuncs] = useState<string[]>([]);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [detail, setDetail] = useState<OpsTask | null>(null);
  const [stream, setStream] = useState<string[]>([]);
  const [result, setResult] = useState<{
    state?: string;
    payload?: JSONValue;
    error?: string;
  } | null>(null);
  const subRef = useRef<TaskEventSubscription | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = {};
      if (status) params.status = status;
      if (fid) params.functionId = fid;
      const actorValue = actor.trim();
      if (actorValue) params.actor = actorValue;
      const r = await listOpsTasks({ ...params, page, size: pageSize });
      setRows(r.tasks || []);
      setTotal(r.total ?? (r.tasks || []).length);
    } catch (e) {
      const errMsg =
        e instanceof Error
          ? e.message
          : intlRef.current.formatMessage({
              id: 'pages.opsJobs.error.operationFailed',
              defaultMessage: '操作失败',
            });
      message.error(
        errMsg ||
          intlRef.current.formatMessage({
            id: 'pages.opsJobs.error.loadFailed',
            defaultMessage: '加载失败',
          }),
      );
    } finally {
      setLoading(false);
    }
  }, [status, fid, actor, page, pageSize, message]);
  useEffect(() => {
    load();
  }, [load]);
  useEffect(() => {
    (async () => {
      try {
        const s = await listOpsFunctions();
        setFuncs((s.functions || []).map((x) => x.id));
      } catch {}
    })();
  }, []);

  // 运行中/成功/失败/函数均为当前页口径（接口按页返回），概览文案已标注；
  // 全量任务数用分页 total（与表格「共 N 条」同源）。
  const summary = useMemo(() => {
    const runningCount = rows.filter((item) => item.state === 'running').length;
    const succeededCount = rows.filter((item) => item.state === 'succeeded').length;
    const failedCount = rows.filter((item) => item.state === 'failed').length;
    const functionCount = new Set(rows.map((item) => item.functionId).filter(Boolean)).size;
    return { runningCount, succeededCount, failedCount, functionCount };
  }, [rows]);

  const resultRows = useMemo(() => {
    return rows.filter((item) => {
      if (status && item.state !== status) return false;
      if (fid && item.functionId !== fid) return false;
      const actorValue = actor.trim().toLowerCase();
      if (actorValue && !(item.actor || '').toLowerCase().includes(actorValue)) return false;
      return true;
    });
  }, [actor, fid, rows, status]);

  const hasFilters = Boolean(status || fid || actor.trim());
  const statusText = status ? getTaskStatusMeta(status, intl).text : '';
  const filterSummary = [
    status
      ? intl.formatMessage(
          { id: 'pages.opsJobs.filter.statusItem', defaultMessage: `状态 ${statusText}` },
          { value: statusText },
        )
      : null,
    fid
      ? intl.formatMessage(
          { id: 'pages.opsJobs.filter.functionItem', defaultMessage: `函数 ${fid}` },
          { value: fid },
        )
      : null,
    actor.trim()
      ? intl.formatMessage(
          {
            id: 'pages.opsJobs.filter.actorItem',
            defaultMessage: `操作者 ${actor.trim()}`,
          },
          { value: actor.trim() },
        )
      : null,
  ]
    .filter(Boolean)
    .join(' / ');

  // Auto-subscribe to task events for a running task when opening detail.
  // Polls GET /api/v1/tasks/:id/events (the server exposes a polled JSON
  // endpoint, not SSE — see subscribeTaskEvents).
  useEffect(() => {
    if (!detail) return;
    setResult(null);
    setStream([]);
    if (detail.state !== 'running') return;
    try {
      if (subRef.current) {
        subRef.current.close();
        subRef.current = null;
      }
    } catch {}
    const id = detail.id;
    const push = (type: string, data: JSONValue) => {
      setStream((prev) =>
        [...prev, `${type}: ${typeof data === 'string' ? data : JSON.stringify(data)}`].slice(-200),
      );
    };
    const sub = subscribeTaskEvents(id, {
      onEvent: (ev) => {
        const body = ev.message || ev.payload;
        push(ev.type, body);
      },
      onError: (err) => push('error', String(err)),
      onDone: async () => {
        // stream finished: fetch final result and refresh list
        try {
          sub.close();
        } catch {}
        subRef.current = null;
        try {
          const r = await fetchTaskResult(id);
          setResult(r as { state?: string; payload?: JSONValue; error?: string });
        } catch {}
        load();
      },
    });
    subRef.current = sub;
    return () => {
      try {
        sub.close();
      } catch {}
      subRef.current = null;
    };
  }, [detail, load]);

  // 取消是不可逆的中止操作：表格与抽屉两处入口统一在此确认
  const handleCancelTask = (task: OpsTask) => {
    modal.confirm({
      title: intl.formatMessage({ id: 'pages.opsJobs.cancel.title', defaultMessage: '取消任务' }),
      content: intl.formatMessage(
        {
          id: 'pages.opsJobs.cancel.content',
          defaultMessage: `确定取消任务 ${task.id} 吗？运行中的执行将被中止。`,
        },
        { id: task.id },
      ),
      okButtonProps: { danger: true },
      okText: intl.formatMessage({
        id: 'pages.opsJobs.cancel.confirm',
        defaultMessage: '取消任务',
      }),
      cancelText: intl.formatMessage({ id: 'pages.opsJobs.cancel.back', defaultMessage: '返回' }),
      onOk: async () => {
        try {
          await cancelTask(task.id);
          message.success(
            intl.formatMessage({ id: 'pages.opsJobs.cancel.success', defaultMessage: '已取消' }),
          );
          if (detail?.id === task.id) {
            setDetail({ ...task, state: 'canceled' });
          }
          load();
        } catch (e) {
          const errMsg =
            e instanceof Error
              ? e.message
              : intl.formatMessage({
                  id: 'pages.opsJobs.error.operationFailed',
                  defaultMessage: '操作失败',
                });
          message.error(
            errMsg ||
              intl.formatMessage({ id: 'pages.opsJobs.cancel.failed', defaultMessage: '取消失败' }),
          );
        }
      },
    });
  };

  const columns: ColumnsType<OpsTask> = [
    { title: 'TaskID', dataIndex: 'id', width: 220 },
    {
      title: intl.formatMessage({ id: 'pages.opsJobs.column.function', defaultMessage: '函数' }),
      dataIndex: 'functionId',
      width: 220,
    },
    {
      title: intl.formatMessage({ id: 'pages.opsJobs.column.status', defaultMessage: '状态' }),
      dataIndex: 'state',
      width: 120,
      render: (v) => {
        const meta = getTaskStatusMeta(v, intl);
        return <Tag color={meta.color}>{meta.text}</Tag>;
      },
    },
    {
      title: intl.formatMessage({ id: 'pages.opsJobs.column.actor', defaultMessage: '操作者' }),
      dataIndex: 'actor',
      width: 140,
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsJobs.column.gameEnv',
        defaultMessage: '游戏/环境',
      }),
      key: 'ge',
      width: 160,
      render: (_, r) => `${r.gameId || ''}/${r.env || ''}`,
    },
    {
      title: intl.formatMessage({ id: 'pages.opsJobs.column.duration', defaultMessage: '耗时' }),
      dataIndex: 'durationMs',
      width: 120,
      render: (v) => (typeof v === 'number' && v > 0 ? `${(v / 1000).toFixed(2)}s` : '-'),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsJobs.column.startedAt',
        defaultMessage: '开始时间',
      }),
      dataIndex: 'startedAt',
      width: 180,
      render: (v) => formatDateTime(v ?? ''),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsJobs.column.endedAt',
        defaultMessage: '结束时间',
      }),
      dataIndex: 'endedAt',
      width: 180,
      render: (v) => formatDateTime(v ?? ''),
    },
    {
      title: intl.formatMessage({ id: 'pages.opsJobs.column.addr', defaultMessage: '服务地址' }),
      dataIndex: 'addr',
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'pages.opsJobs.column.actions', defaultMessage: '操作' }),
      width: 160,
      render: (_, r) => (
        <Space>
          <Button size="small" type="primary" ghost onClick={() => setDetail(r)}>
            <FormattedMessage id="pages.opsJobs.action.viewDetail" defaultMessage="查看详情" />
          </Button>
          {r.state === 'running' && (
            <Button size="small" danger onClick={() => handleCancelTask(r)}>
              <FormattedMessage id="pages.opsJobs.action.cancel" defaultMessage="取消" />
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer
      title={intl.formatMessage({ id: 'pages.opsJobs.title.main', defaultMessage: '任务监控' })}
      subTitle={intl.formatMessage({
        id: 'pages.opsJobs.title.sub',
        defaultMessage: '先缩小任务范围，再查看执行细节、事件流和最终结果',
      })}
    >
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <SummaryOverview
          title={intl.formatMessage({
            id: 'pages.opsJobs.summary.title',
            defaultMessage: '任务概览',
          })}
          description={intl.formatMessage({
            id: 'pages.opsJobs.summary.description',
            defaultMessage:
              '这个页面优先服务排查和追踪，不把所有信息一次性堆进表格。先按状态、函数或操作者收敛范围，再进详情查看。',
          })}
          items={[
            {
              color: '#1677ff',
              text: intl.formatMessage(
                { id: 'pages.opsJobs.summary.taskTotal', defaultMessage: `任务 ${total}` },
                { total },
              ),
            },
            {
              color: '#2f54eb',
              text: intl.formatMessage(
                {
                  id: 'pages.opsJobs.summary.running',
                  defaultMessage: `运行中 ${summary.runningCount}（当前页）`,
                },
                { count: summary.runningCount },
              ),
            },
            {
              color: '#52c41a',
              text: intl.formatMessage(
                {
                  id: 'pages.opsJobs.summary.succeeded',
                  defaultMessage: `成功 ${summary.succeededCount}（当前页）`,
                },
                { count: summary.succeededCount },
              ),
            },
            {
              color: '#ff4d4f',
              text: intl.formatMessage(
                {
                  id: 'pages.opsJobs.summary.failed',
                  defaultMessage: `失败 ${summary.failedCount}（当前页）`,
                },
                { count: summary.failedCount },
              ),
            },
            {
              color: '#722ed1',
              text: intl.formatMessage(
                {
                  id: 'pages.opsJobs.summary.function',
                  defaultMessage: `函数 ${summary.functionCount}（当前页）`,
                },
                { count: summary.functionCount },
              ),
            },
          ]}
          hint={intl.formatMessage({
            id: 'pages.opsJobs.summary.hint',
            defaultMessage:
              '推荐路径：先筛选任务，再打开详情查看事件流和结果，不必在主表里同时处理所有上下文。',
          })}
        />

        <StandardListSection
          title={intl.formatMessage({ id: 'pages.opsJobs.list.title', defaultMessage: '任务列表' })}
          extra={
            <Button onClick={load}>
              <FormattedMessage id="pages.opsJobs.list.refresh" defaultMessage="刷新" />
            </Button>
          }
        >
          <StandardFilterBar
            resultText={intl.formatMessage(
              {
                id: 'pages.opsJobs.list.resultCount',
                defaultMessage: `当前结果 ${resultRows.length} 个任务`,
              },
              { count: resultRows.length },
            )}
            controls={
              <>
                <Select
                  placeholder={intl.formatMessage({
                    id: 'pages.opsJobs.filter.statusPlaceholder',
                    defaultMessage: '状态',
                  })}
                  allowClear
                  style={{ width: 140 }}
                  value={status || undefined}
                  onChange={(v) => setStatus(v || '')}
                  options={[
                    {
                      label: intl.formatMessage({
                        id: 'pages.opsJobs.status.running',
                        defaultMessage: '运行中',
                      }),
                      value: 'running',
                    },
                    {
                      label: intl.formatMessage({
                        id: 'pages.opsJobs.status.succeeded',
                        defaultMessage: '已成功',
                      }),
                      value: 'succeeded',
                    },
                    {
                      label: intl.formatMessage({
                        id: 'pages.opsJobs.status.failed',
                        defaultMessage: '已失败',
                      }),
                      value: 'failed',
                    },
                    {
                      label: intl.formatMessage({
                        id: 'pages.opsJobs.status.canceled',
                        defaultMessage: '已取消',
                      }),
                      value: 'canceled',
                    },
                  ]}
                />
                <Select
                  showSearch
                  placeholder={intl.formatMessage({
                    id: 'pages.opsJobs.filter.functionPlaceholder',
                    defaultMessage: '函数',
                  })}
                  allowClear
                  style={{ width: 240 }}
                  value={fid || undefined}
                  onChange={(v) => setFid(v || '')}
                  options={funcs.map((id) => ({ label: id, value: id }))}
                />
                <Input
                  allowClear
                  placeholder={intl.formatMessage({
                    id: 'pages.opsJobs.filter.actorPlaceholder',
                    defaultMessage: '按操作者过滤',
                  })}
                  value={actor}
                  onChange={(e) => setActor(e.target.value)}
                  style={{ width: 180 }}
                />
                {hasFilters && (
                  <Button
                    onClick={() => {
                      setStatus('');
                      setFid('');
                      setActor('');
                    }}
                  >
                    <FormattedMessage id="pages.opsJobs.filter.clear" defaultMessage="清空筛选" />
                  </Button>
                )}
              </>
            }
          />
          {hasFilters ? (
            <Alert
              style={{ marginBottom: 12 }}
              type="info"
              showIcon
              message={intl.formatMessage({
                id: 'pages.opsJobs.filter.activeMessage',
                defaultMessage: '当前正在查看筛选后的任务范围',
              })}
              description={intl.formatMessage(
                {
                  id: 'pages.opsJobs.filter.activeDescription',
                  defaultMessage: `已生效条件：${filterSummary}`,
                },
                { filters: filterSummary },
              )}
            />
          ) : null}
          <Table
            rowKey={(r) => r.id}
            loading={loading}
            dataSource={resultRows}
            columns={columns}
            pagination={{
              current: page,
              pageSize,
              total,
              showSizeChanger: true,
              pageSizeOptions: [10, 20, 50],
              showTotal: (t) =>
                intl.formatMessage(
                  { id: 'pages.opsJobs.pagination.total', defaultMessage: `共 ${t} 条` },
                  { total: t },
                ),
              onChange: (nextPage, nextSize) => {
                setPage(nextPage);
                setPageSize(nextSize);
              },
            }}
            onRow={(rec) => ({ onDoubleClick: () => setDetail(rec) })}
            scroll={{ x: 1280 }}
            locale={{
              emptyText: hasFilters
                ? intl.formatMessage({
                    id: 'pages.opsJobs.empty.filtered',
                    defaultMessage: '当前筛选条件下没有匹配任务，请调整筛选后重试。',
                  })
                : intl.formatMessage({
                    id: 'pages.opsJobs.empty.none',
                    defaultMessage: '暂时没有任务数据，先触发任务后再回来查看。',
                  }),
            }}
          />
        </StandardListSection>
      </Space>

      <Drawer
        title={intl.formatMessage({ id: 'pages.opsJobs.detail.title', defaultMessage: '任务详情' })}
        width={720}
        open={!!detail}
        onClose={() => {
          setDetail(null);
          setStream([]);
          setResult(null);
          if (subRef.current) {
            subRef.current.close();
            subRef.current = null;
          }
        }}
        extra={
          <Space>
            {detail?.state === 'running' && (
              <Button
                danger
                onClick={async () => {
                  if (!detail) return;
                  await handleCancelTask(detail);
                }}
              >
                <FormattedMessage id="pages.opsJobs.action.cancel" defaultMessage="取消" />
              </Button>
            )}
            <Button
              onClick={async () => {
                if (!detail) return;
                try {
                  const r = await fetchTaskResult(detail.id);
                  setResult(r as { state?: string; payload?: JSONValue; error?: string });
                  message.success(
                    intl.formatMessage(
                      {
                        id: 'pages.opsJobs.detail.resultState',
                        defaultMessage: `状态：${String(r?.state)}`,
                      },
                      { state: String(r?.state) },
                    ),
                  );
                  load();
                } catch (e) {
                  const errMsg =
                    e instanceof Error
                      ? e.message
                      : intl.formatMessage({
                          id: 'pages.opsJobs.error.operationFailed',
                          defaultMessage: '操作失败',
                        });
                  message.error(
                    errMsg ||
                      intl.formatMessage({
                        id: 'pages.opsJobs.error.queryFailed',
                        defaultMessage: '查询失败',
                      }),
                  );
                }
              }}
            >
              <FormattedMessage id="pages.opsJobs.detail.refreshResult" defaultMessage="刷新结果" />
            </Button>
          </Space>
        }
      >
        {detail && (
          <Space orientation="vertical" style={{ width: '100%' }}>
            <SummaryOverview
              title={intl.formatMessage({
                id: 'pages.opsJobs.detail.statusTitle',
                defaultMessage: '任务状态',
              })}
              description={intl.formatMessage({
                id: 'pages.opsJobs.detail.statusDescription',
                defaultMessage:
                  '先判断任务当前处于运行、成功还是失败，再决定要不要刷新结果或查看事件流。',
              })}
              items={[
                {
                  color: getTaskStatusMeta(detail.state, intl).color,
                  text: getTaskStatusMeta(detail.state, intl).text,
                },
                { color: '#1677ff', text: detail.functionId || '-' },
                {
                  color: '#722ed1',
                  text:
                    detail.actor ||
                    intl.formatMessage({
                      id: 'pages.opsJobs.detail.actorUnknown',
                      defaultMessage: '未知操作者',
                    }),
                },
                {
                  color: '#13c2c2',
                  text:
                    typeof detail.durationMs === 'number' && detail.durationMs > 0
                      ? intl.formatMessage(
                          {
                            id: 'pages.opsJobs.detail.durationValue',
                            defaultMessage: `耗时 ${(detail.durationMs / 1000).toFixed(2)}s`,
                          },
                          { duration: (detail.durationMs / 1000).toFixed(2) },
                        )
                      : intl.formatMessage({
                          id: 'pages.opsJobs.detail.durationUnknown',
                          defaultMessage: '耗时未知',
                        }),
                },
              ]}
              hint={
                detail.state === 'running'
                  ? intl.formatMessage({
                      id: 'pages.opsJobs.detail.hintRunning',
                      defaultMessage: '任务仍在运行，优先观察事件流；只有确认需要中止时再取消。',
                    })
                  : detail.state === 'failed'
                    ? intl.formatMessage({
                        id: 'pages.opsJobs.detail.hintFailed',
                        defaultMessage: '任务已经失败，建议先看错误信息，再核对结果和事件流。',
                      })
                    : intl.formatMessage({
                        id: 'pages.opsJobs.detail.hintFinished',
                        defaultMessage: '任务已结束，可以直接查看结果和事件流。',
                      })
              }
              hintType={detail.state === 'failed' ? 'warning' : 'info'}
            />

            <Descriptions bordered size="small" column={1}>
              <Descriptions.Item label="TaskID">
                <Text code copyable>
                  {detail.id}
                </Text>
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.opsJobs.detail.label.function',
                  defaultMessage: '函数',
                })}
              >
                {detail.functionId}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.opsJobs.detail.label.status',
                  defaultMessage: '状态',
                })}
              >
                <Tag color={getTaskStatusMeta(detail.state, intl).color}>
                  {getTaskStatusMeta(detail.state, intl).text}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.opsJobs.detail.label.actor',
                  defaultMessage: '操作者',
                })}
              >
                {detail.actor || '-'}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.opsJobs.detail.label.gameEnv',
                  defaultMessage: '游戏/环境',
                })}
              >
                {(detail.gameId || '') + '/' + (detail.env || '')}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.opsJobs.detail.label.addr',
                  defaultMessage: '服务地址',
                })}
              >
                {detail.addr || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="Trace">{detail.traceId || '-'}</Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.opsJobs.detail.label.startedAt',
                  defaultMessage: '开始时间',
                })}
              >
                {detail.startedAt ? formatDateTime(detail.startedAt) : '-'}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.opsJobs.detail.label.endedAt',
                  defaultMessage: '结束时间',
                })}
              >
                {detail.endedAt ? formatDateTime(detail.endedAt) : '-'}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.opsJobs.detail.label.duration',
                  defaultMessage: '耗时',
                })}
              >
                {typeof detail.durationMs === 'number' && detail.durationMs > 0
                  ? `${(detail.durationMs / 1000).toFixed(2)}s`
                  : '-'}
              </Descriptions.Item>
            </Descriptions>
            {detail.error && (
              <Paragraph copyable={{ text: detail.error }} style={{ color: '#ff4d4f' }}>
                {intl.formatMessage(
                  {
                    id: 'pages.opsJobs.detail.errorPrefix',
                    defaultMessage: `错误：${detail.error}`,
                  },
                  { error: detail.error },
                )}
              </Paragraph>
            )}
            <Card
              size="small"
              title={
                result?.state
                  ? intl.formatMessage(
                      {
                        id: 'pages.opsJobs.detail.resultTitleWithState',
                        defaultMessage: `结果（${result.state}）`,
                      },
                      { state: result.state },
                    )
                  : intl.formatMessage({
                      id: 'pages.opsJobs.detail.resultTitle',
                      defaultMessage: '结果',
                    })
              }
            >
              {result ? (
                <Space orientation="vertical" size={10} style={{ width: '100%' }}>
                  {result.error && (
                    <Paragraph copyable={{ text: result.error }} style={{ color: '#ff4d4f' }}>
                      {intl.formatMessage(
                        {
                          id: 'pages.opsJobs.detail.errorPrefix',
                          defaultMessage: `错误：${result.error}`,
                        },
                        { error: result.error },
                      )}
                    </Paragraph>
                  )}
                  {result.payload ? (
                    <Paragraph copyable={{ text: JSON.stringify(result.payload) }}>
                      <pre style={{ whiteSpace: 'pre-wrap' }}>
                        {JSON.stringify(result.payload, null, 2)}
                      </pre>
                    </Paragraph>
                  ) : (
                    <Alert
                      type="info"
                      showIcon
                      message={intl.formatMessage({
                        id: 'pages.opsJobs.detail.noResult.message',
                        defaultMessage: '当前还没有结果数据',
                      })}
                      description={intl.formatMessage({
                        id: 'pages.opsJobs.detail.noResult.description',
                        defaultMessage:
                          '运行中的任务通常要先看事件流；已结束但没有结果时，可以手动刷新结果后再确认。',
                      })}
                    />
                  )}
                </Space>
              ) : (
                <Alert
                  type="info"
                  showIcon
                  message={intl.formatMessage({
                    id: 'pages.opsJobs.detail.resultNotLoaded.message',
                    defaultMessage: '结果尚未加载',
                  })}
                  description={intl.formatMessage({
                    id: 'pages.opsJobs.detail.resultNotLoaded.description',
                    defaultMessage: '建议先点击“刷新结果”或等待任务完成后自动拉取。',
                  })}
                />
              )}
            </Card>
            <Card
              size="small"
              title={intl.formatMessage({
                id: 'pages.opsJobs.stream.title',
                defaultMessage: '事件流',
              })}
              extra={
                <Space>
                  <Button
                    size="small"
                    onClick={() => {
                      if (!detail) return;
                      try {
                        if (subRef.current) {
                          subRef.current.close();
                          subRef.current = null;
                        }
                      } catch {}
                      const push = (type: string, data: JSONValue) =>
                        setStream((prev) =>
                          [
                            ...prev,
                            `${type}: ${typeof data === 'string' ? data : JSON.stringify(data)}`,
                          ].slice(-200),
                        );
                      const sub = subscribeTaskEvents(detail.id, {
                        onEvent: (ev) => push(ev.type, ev.message || ev.payload),
                        onError: (err) => push('error', String(err)),
                        onDone: async () => {
                          try {
                            sub.close();
                          } catch {}
                          subRef.current = null;
                          try {
                            const r = await fetchTaskResult(detail.id);
                            setResult(r as { state?: string; payload?: JSONValue; error?: string });
                          } catch {}
                          load();
                        },
                      });
                      subRef.current = sub;
                    }}
                  >
                    <FormattedMessage id="pages.opsJobs.stream.connect" defaultMessage="连接" />
                  </Button>
                  <Button
                    size="small"
                    onClick={() => {
                      try {
                        if (subRef.current) subRef.current.close();
                      } catch {}
                      subRef.current = null;
                    }}
                  >
                    <FormattedMessage id="pages.opsJobs.stream.disconnect" defaultMessage="断开" />
                  </Button>
                </Space>
              }
            >
              <div
                style={{
                  maxHeight: 200,
                  overflow: 'auto',
                  fontFamily: 'monospace',
                  fontSize: 12,
                  background: '#fafafa',
                  padding: 8,
                  border: '1px solid #f0f0f0',
                }}
              >
                {stream.length > 0 ? (
                  stream.map((ln, i) => <div key={i}>{ln}</div>)
                ) : (
                  <Typography.Text type="secondary">
                    <FormattedMessage
                      id="pages.opsJobs.stream.empty"
                      defaultMessage="还没有事件流输出。运行中的任务可以先连接事件流；已结束任务可能不会再产生新事件。"
                    />
                  </Typography.Text>
                )}
              </div>
            </Card>
          </Space>
        )}
      </Drawer>
    </PageContainer>
  );
}
