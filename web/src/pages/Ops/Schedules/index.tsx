import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Drawer,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Select,
  Space,
  Statistic,
  Table,
  Tabs,
  Tag,
} from 'antd';
import { ModalForm, PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  createSchedule,
  deleteSchedule,
  listScheduleRuns,
  listSchedules,
  setScheduleStatus,
  triggerScheduleNow,
  type RunLogItem,
  type ScheduleItem,
} from '@/services/api/schedules';
import { listNodesCronJobsAll, type NodeCronJobsReport } from '@/services/api/nodes';
import type { NodeCronJob } from '@/services/api/ops';
import { useScopeReload } from '@/hooks/useScopeReload';
import { extractErrorMessage } from '@/utils/errors';
import { formatDateTime } from '@/utils/format';

/** 模块级文案助手接收 intl 的最小结构（@umijs/max 未导出 IntlShape 类型） */
type IntlFormatter = {
  formatMessage: (descriptor: { id: string; defaultMessage: string }) => string;
};

/** 调度状态 → Tag 颜色/文案；未知状态回退展示原始值 */
function getStatusTagMeta(status: string, intl: IntlFormatter): { color: string; label: string } {
  if (status === 'active')
    return {
      color: 'green',
      label: intl.formatMessage({
        id: 'pages.opsSchedules.status.active',
        defaultMessage: '运行中',
      }),
    };
  if (status === 'paused')
    return {
      color: 'default',
      label: intl.formatMessage({
        id: 'pages.opsSchedules.status.paused',
        defaultMessage: '已暂停',
      }),
    };
  if (status === 'dead_letter')
    return {
      color: 'red',
      label: intl.formatMessage({
        id: 'pages.opsSchedules.status.deadLetter',
        defaultMessage: '死信',
      }),
    };
  return { color: 'default', label: status };
}

/** 调度表单值：payload 在表单中为 JSON 文本，提交前解析为对象 */
type ScheduleFormValues = {
  name: string;
  cronExpr: string;
  functionId: string;
  payload?: string;
  maxFailedRuns?: number;
};

export default function SchedulesPage() {
  const { message, modal } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<ScheduleItem[]>([]);
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [open, setOpen] = useState(false);
  const [runsTarget, setRunsTarget] = useState<ScheduleItem | null>(null);
  const [runLogs, setRunLogs] = useState<RunLogItem[]>([]);
  const [runsLoading, setRunsLoading] = useState(false);
  // #24：宿主机任务采集（platform=平台创建调度之外的第二来源）
  const [hostReports, setHostReports] = useState<NodeCronJobsReport[]>([]);
  const [hostLoading, setHostLoading] = useState(false);
  const [hostFailed, setHostFailed] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await listSchedules({ status: statusFilter, pageSize: 100 });
      setRows(res?.items || []);
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.opsSchedules.error.loadFailed',
            defaultMessage: '加载定时任务失败',
          }),
        ),
      );
    } finally {
      setLoading(false);
    }
  }, [message, statusFilter]);
  // #24 族：定时任务按全局 scope（/api/v1/schedules 已显式带头）过滤，切游戏重拉
  useScopeReload(load);

  useEffect(() => {
    load();
  }, [load]);

  // #24：宿主机任务经服务端聚合接口（节点全量并行采集，服务端 30s 缓存）。
  // host 数据无 game scope 语义（节点全局），不参与 useScopeReload。
  const loadHost = useCallback(async () => {
    setHostLoading(true);
    try {
      const res = await listNodesCronJobsAll();
      setHostReports(res?.items || []);
      setHostFailed(false);
    } catch {
      // 采集链路（agent 离线等）是辅助面：失败保留上次数据，概览以「-」标注
      setHostFailed(true);
    } finally {
      setHostLoading(false);
    }
  }, []);

  useEffect(() => {
    loadHost();
  }, [loadHost]);

  const hostJobs = hostReports.flatMap((r) => r.jobs);
  const hostUnreachable = hostReports.filter((r) => !r.ok);
  // 概览统计：总数/启用为双来源合计；各来源分布单列（#24）
  const platformActive = rows.filter((r) => r.status === 'active').length;
  const hostEnabledCount = hostJobs.filter((j) => j.enabled).length;
  const totalAll = rows.length + (hostFailed ? 0 : hostJobs.length);
  const enabledAll = platformActive + (hostFailed ? 0 : hostEnabledCount);

  const openRuns = async (row: ScheduleItem) => {
    setRunsTarget(row);
    setRunsLoading(true);
    try {
      const res = await listScheduleRuns(row.id, { pageSize: 50 });
      setRunLogs(res?.items || []);
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.opsSchedules.error.loadRunsFailed',
            defaultMessage: '加载触发历史失败',
          }),
        ),
      );
    } finally {
      setRunsLoading(false);
    }
  };

  const onFinish = async (values: ScheduleFormValues) => {
    try {
      await createSchedule({
        name: values.name,
        cronExpr: values.cronExpr,
        functionId: values.functionId,
        payload: values.payload ? JSON.parse(values.payload) : {},
        maxFailedRuns: values.maxFailedRuns || 5,
      });
      message.success(
        intl.formatMessage({
          id: 'pages.opsSchedules.create.success',
          defaultMessage: '已创建，调度器将在下次到期自动触发',
        }),
      );
      load();
      return true;
    } catch (error) {
      // 原语义：payload JSON 解析失败与请求失败统一本地 toast，弹窗保持开启
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.opsSchedules.create.failed',
            defaultMessage: '创建失败',
          }),
        ),
      );
      return false;
    }
  };

  const columns: ColumnsType<ScheduleItem> = [
    {
      title: intl.formatMessage({ id: 'pages.opsSchedules.column.name', defaultMessage: '名称' }),
      dataIndex: 'name',
      width: 160,
    },
    {
      title: 'Cron',
      dataIndex: 'cronExpr',
      width: 130,
      render: (_: unknown, r: ScheduleItem) => <code>{r.cronExpr}</code>,
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsSchedules.column.function',
        defaultMessage: '函数',
      }),
      dataIndex: 'functionId',
      width: 170,
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsSchedules.column.scope',
        defaultMessage: '作用域',
      }),
      dataIndex: 'gameId',
      width: 130,
      render: (_: unknown, r: ScheduleItem) => (
        <Tag>
          {r.gameId}/{r.env}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsSchedules.column.status',
        defaultMessage: '状态',
      }),
      dataIndex: 'status',
      width: 100,
      render: (_: unknown, r: ScheduleItem) => {
        const tag = getStatusTagMeta(r.status, intl);
        return <Tag color={tag.color}>{tag.label}</Tag>;
      },
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsSchedules.column.consecutiveFailures',
        defaultMessage: '连续失败',
      }),
      dataIndex: 'consecutiveFailures',
      width: 90,
      render: (_: unknown, r: ScheduleItem) =>
        r.consecutiveFailures > 0 ? (
          <span style={{ color: r.consecutiveFailures >= r.maxFailedRuns ? '#cf1322' : '#d46b08' }}>
            {r.consecutiveFailures}/{r.maxFailedRuns}
          </span>
        ) : (
          '0'
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsSchedules.column.nextTriggerAt',
        defaultMessage: '下次触发',
      }),
      dataIndex: 'nextTriggerAt',
      width: 170,
      render: (_: unknown, r: ScheduleItem) => formatDateTime(r.nextTriggerAt ?? ''),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsSchedules.column.actions',
        defaultMessage: '操作',
      }),
      key: 'actions',
      width: 260,
      render: (_: unknown, row: ScheduleItem) => [
        <a key="trigger" onClick={() => doTrigger(row)}>
          <FormattedMessage id="pages.opsSchedules.action.triggerNow" defaultMessage="立即触发" />
        </a>,
        row.status === 'active' ? (
          <a key="pause" onClick={() => doStatus(row, 'paused')}>
            <FormattedMessage id="pages.opsSchedules.action.pause" defaultMessage="暂停" />
          </a>
        ) : (
          <a key="resume" onClick={() => doStatus(row, 'active')}>
            {row.status === 'dead_letter' ? (
              <FormattedMessage id="pages.opsSchedules.action.resume" defaultMessage="恢复" />
            ) : (
              <FormattedMessage id="pages.opsSchedules.action.enable" defaultMessage="启用" />
            )}
          </a>
        ),
        <a key="runs" onClick={() => openRuns(row)}>
          <FormattedMessage id="pages.opsSchedules.action.history" defaultMessage="历史" />
        </a>,
        <Popconfirm
          key="del"
          title={intl.formatMessage({
            id: 'pages.opsSchedules.delete.confirm',
            defaultMessage: '确认删除该调度？',
          })}
          onConfirm={async () => {
            try {
              await deleteSchedule(row.id);
              message.success(
                intl.formatMessage({
                  id: 'pages.opsSchedules.delete.success',
                  defaultMessage: '已删除',
                }),
              );
              load();
            } catch (error) {
              message.error(
                extractErrorMessage(
                  error,
                  intl.formatMessage({
                    id: 'pages.opsSchedules.delete.failed',
                    defaultMessage: '删除失败',
                  }),
                ),
              );
            }
          }}
        >
          <a style={{ color: '#cf1322' }}>
            <FormattedMessage id="pages.opsSchedules.action.delete" defaultMessage="删除" />
          </a>
        </Popconfirm>,
      ],
    },
  ];

  const doTrigger = async (row: ScheduleItem) => {
    try {
      const res = await triggerScheduleNow(row.id);
      message.success(
        intl.formatMessage(
          {
            id: 'pages.opsSchedules.trigger.success',
            defaultMessage: `已派发任务 ${res.taskRunId}`,
          },
          { taskRunId: res.taskRunId },
        ),
      );
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.opsSchedules.trigger.failed',
            defaultMessage: '触发失败',
          }),
        ),
      );
    }
  };

  const doStatus = async (row: ScheduleItem, status: 'active' | 'paused') => {
    const apply = async () => {
      try {
        await setScheduleStatus(row.id, status);
        message.success(
          status === 'active'
            ? intl.formatMessage({
                id: 'pages.opsSchedules.action.resumeSuccess',
                defaultMessage: '已恢复运行',
              })
            : intl.formatMessage({
                id: 'pages.opsSchedules.action.pauseSuccess',
                defaultMessage: '已暂停',
              }),
        );
        load();
      } catch (error) {
        message.error(
          extractErrorMessage(
            error,
            intl.formatMessage({
              id: 'pages.opsSchedules.error.operationFailed',
              defaultMessage: '操作失败',
            }),
          ),
        );
      }
    };
    if (row.status === 'dead_letter' && status === 'active') {
      modal.confirm({
        title: intl.formatMessage({
          id: 'pages.opsSchedules.recover.title',
          defaultMessage: '从死信恢复该调度？',
        }),
        content: intl.formatMessage({
          id: 'pages.opsSchedules.recover.content',
          defaultMessage: '连续失败计数将清零，并从下一个触发点重新开始。',
        }),
        onOk: apply,
      });
      return;
    }
    apply();
  };

  // #24：宿主机任务平铺行（节点信息随行展示；key 用节点+任务+命令组合兜底重复）
  const hostRows: (NodeCronJob & { key: string; nodeId: string; nodeName: string })[] =
    hostReports.flatMap((r) =>
      r.jobs.map((j, idx) => ({
        ...j,
        key: `${r.nodeId}#${j.sourceFile}#${idx}`,
        nodeId: r.nodeId,
        nodeName: r.nodeName,
      })),
    );

  const hostColumns: ColumnsType<NodeCronJob & { key: string; nodeId: string; nodeName: string }> =
    [
      {
        title: intl.formatMessage({
          id: 'pages.opsSchedules.host.column.node',
          defaultMessage: '节点',
        }),
        dataIndex: 'nodeName',
        width: 140,
        render: (_: unknown, r) => (
          <Space size={4}>
            <span>{r.nodeName || r.nodeId}</span>
            <Tag>{r.nodeId}</Tag>
          </Space>
        ),
      },
      {
        title: intl.formatMessage({
          id: 'pages.opsSchedules.host.column.task',
          defaultMessage: '任务',
        }),
        dataIndex: 'sourceFile',
        ellipsis: true,
      },
      {
        title: intl.formatMessage({
          id: 'pages.opsSchedules.host.column.schedule',
          defaultMessage: '计划',
        }),
        dataIndex: 'schedule',
        width: 200,
        render: (v: string) => <code>{v || '-'}</code>,
      },
      {
        title: intl.formatMessage({
          id: 'pages.opsSchedules.host.column.command',
          defaultMessage: '命令',
        }),
        dataIndex: 'command',
        ellipsis: true,
      },
      {
        title: intl.formatMessage({
          id: 'pages.opsSchedules.host.column.user',
          defaultMessage: '用户',
        }),
        dataIndex: 'user',
        width: 120,
        render: (v?: string) => v || '-',
      },
      {
        title: intl.formatMessage({
          id: 'pages.opsSchedules.host.column.enabled',
          defaultMessage: '启用',
        }),
        dataIndex: 'enabled',
        width: 80,
        render: (v: boolean) => (
          <Tag color={v ? 'green' : 'default'}>
            {v
              ? intl.formatMessage({
                  id: 'pages.opsSchedules.host.enabled.yes',
                  defaultMessage: '启用',
                })
              : intl.formatMessage({
                  id: 'pages.opsSchedules.host.enabled.no',
                  defaultMessage: '禁用',
                })}
          </Tag>
        ),
      },
    ];

  const hostView = (
    <Space orientation="vertical" style={{ width: '100%' }} size={12}>
      {hostUnreachable.length > 0 && (
        <Alert
          type="warning"
          showIcon
          title={intl.formatMessage({
            id: 'pages.opsSchedules.host.unreachable',
            defaultMessage: '以下节点不可达，未能采集宿主机任务',
          })}
          description={hostUnreachable
            .map((r) => `${r.nodeName || r.nodeId}: ${r.error || '-'}`)
            .join('；')}
        />
      )}
      <Table<NodeCronJob & { key: string; nodeId: string; nodeName: string }>
        rowKey="key"
        loading={hostLoading}
        columns={hostColumns}
        dataSource={hostRows}
        pagination={{ pageSize: 20, showSizeChanger: false }}
        size="middle"
      />
    </Space>
  );

  return (
    <PageContainer
      // #24：副标题从 subTitle（标题右侧同行）改为 content（独立成行）
      content={intl.formatMessage({
        id: 'pages.opsSchedules.title.sub',
        defaultMessage: '五字段 cron 定时触发函数；连续失败达到上限自动进入死信',
      })}
      extra={[
        <Select
          key="status"
          allowClear
          placeholder={intl.formatMessage({
            id: 'pages.opsSchedules.filter.statusPlaceholder',
            defaultMessage: '状态',
          })}
          style={{ width: 120 }}
          value={statusFilter}
          onChange={setStatusFilter}
          options={[
            {
              label: intl.formatMessage({
                id: 'pages.opsSchedules.status.active',
                defaultMessage: '运行中',
              }),
              value: 'active',
            },
            {
              label: intl.formatMessage({
                id: 'pages.opsSchedules.status.paused',
                defaultMessage: '已暂停',
              }),
              value: 'paused',
            },
            {
              label: intl.formatMessage({
                id: 'pages.opsSchedules.status.deadLetter',
                defaultMessage: '死信',
              }),
              value: 'dead_letter',
            },
          ]}
        />,
        <Button key="add" type="primary" onClick={() => setOpen(true)}>
          <FormattedMessage id="pages.opsSchedules.action.create" defaultMessage="新建调度" />
        </Button>,
      ]}
    >
      {/* #24：概览区——总数/启用为双来源合计，来源分布（平台/宿主机）单列；
          host 采集失败时相关数字以「-」诚实标注而非归零 */}
      <Card size="small" style={{ marginBottom: 16 }}>
        <Space size={48} wrap>
          <Statistic
            title={intl.formatMessage({
              id: 'pages.opsSchedules.overview.total',
              defaultMessage: '任务总数',
            })}
            value={totalAll}
            suffix={
              hostFailed
                ? intl.formatMessage({
                    id: 'pages.opsSchedules.overview.hostUnavailable',
                    defaultMessage: '（宿主机采集不可用）',
                  })
                : undefined
            }
          />
          <Statistic
            title={intl.formatMessage({
              id: 'pages.opsSchedules.overview.enabled',
              defaultMessage: '启用中',
            })}
            value={enabledAll}
          />
          <Statistic
            title={intl.formatMessage({
              id: 'pages.opsSchedules.overview.platform',
              defaultMessage: '平台调度',
            })}
            value={rows.length}
          />
          <Statistic
            title={intl.formatMessage({
              id: 'pages.opsSchedules.overview.host',
              defaultMessage: '宿主机任务',
            })}
            value={hostFailed ? '-' : hostJobs.length}
            suffix={
              hostFailed
                ? undefined
                : `(${hostReports.filter((r) => r.ok).length} ${intl.formatMessage({
                    id: 'pages.opsSchedules.overview.nodes',
                    defaultMessage: '个节点',
                  })})`
            }
          />
        </Space>
      </Card>

      <Tabs
        defaultActiveKey="platform"
        items={[
          {
            key: 'platform',
            label: `${intl.formatMessage({
              id: 'pages.opsSchedules.tab.platform',
              defaultMessage: '平台调度',
            })} (${rows.length})`,
            children: (
              <Table<ScheduleItem>
                rowKey="id"
                loading={loading}
                columns={columns}
                dataSource={rows}
                pagination={{ pageSize: 20, showSizeChanger: false }}
                size="middle"
              />
            ),
          },
          {
            key: 'host',
            label: `${intl.formatMessage({
              id: 'pages.opsSchedules.tab.host',
              defaultMessage: '宿主机任务',
            })} (${hostFailed ? '-' : hostJobs.length})`,
            children: hostView,
          },
        ]}
      />

      <ModalForm<ScheduleFormValues>
        title={intl.formatMessage({
          id: 'pages.opsSchedules.create.title',
          defaultMessage: '新建定时调度',
        })}
        open={open}
        onOpenChange={setOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{
          searchConfig: {
            submitText: intl.formatMessage({
              id: 'pages.opsSchedules.form.submit',
              defaultMessage: '确定',
            }),
          },
        }}
        layout="vertical"
        onFinish={onFinish}
      >
        <Form.Item
          name="name"
          label={intl.formatMessage({ id: 'pages.opsSchedules.form.name', defaultMessage: '名称' })}
          rules={[{ required: true }]}
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.opsSchedules.form.namePlaceholder',
              defaultMessage: '每日凌晨清理过期数据',
            })}
          />
        </Form.Item>
        <Form.Item
          name="cronExpr"
          label={intl.formatMessage({
            id: 'pages.opsSchedules.form.cronExpr',
            defaultMessage: 'Cron 表达式（分 时 日 月 周）',
          })}
          rules={[
            { required: true },
            {
              validator: (_, v: string) =>
                !v || v.trim().split(/\s+/).length === 5
                  ? Promise.resolve()
                  : Promise.reject(
                      new Error(
                        intl.formatMessage({
                          id: 'pages.opsSchedules.form.cronInvalid',
                          defaultMessage: '需要 5 个字段，如 "30 2 * * *"',
                        }),
                      ),
                    ),
            },
          ]}
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.opsSchedules.form.cronPlaceholder',
              defaultMessage: '30 2 * * *（每天 02:30）',
            })}
          />
        </Form.Item>
        <Form.Item
          name="functionId"
          label={intl.formatMessage({
            id: 'pages.opsSchedules.form.function',
            defaultMessage: '函数',
          })}
          rules={[{ required: true }]}
        >
          <Input placeholder="player.cleanup" />
        </Form.Item>
        <Form.Item
          name="payload"
          label={intl.formatMessage({
            id: 'pages.opsSchedules.form.payload',
            defaultMessage: '参数（JSON，可空）',
          })}
        >
          <Input.TextArea rows={3} placeholder='{"days": 30}' />
        </Form.Item>
        <Form.Item
          name="maxFailedRuns"
          label={intl.formatMessage({
            id: 'pages.opsSchedules.form.maxFailedRuns',
            defaultMessage: '连续失败上限（默认 5）',
          })}
          initialValue={5}
        >
          <InputNumber min={1} max={100} style={{ width: '100%' }} />
        </Form.Item>
      </ModalForm>

      <Drawer
        title={intl.formatMessage(
          {
            id: 'pages.opsSchedules.runs.title',
            defaultMessage: `触发历史：${runsTarget?.name || ''}`,
          },
          { name: runsTarget?.name || '' },
        )}
        open={!!runsTarget}
        onClose={() => setRunsTarget(null)}
        size={640}
      >
        <Table<RunLogItem>
          rowKey="id"
          loading={runsLoading}
          dataSource={runLogs}
          pagination={{ pageSize: 10 }}
          size="small"
          columns={[
            {
              title: intl.formatMessage({
                id: 'pages.opsSchedules.runs.column.slot',
                defaultMessage: '触发时间',
              }),
              dataIndex: 'slot',
              render: (v: string) => formatDateTime(v),
            },
            {
              title: intl.formatMessage({
                id: 'pages.opsSchedules.runs.column.result',
                defaultMessage: '结果',
              }),
              dataIndex: 'status',
              width: 100,
              render: (v: RunLogItem['status']) => (
                <Tag color={v === 'dispatched' ? 'green' : v === 'failed' ? 'red' : 'default'}>
                  {v}
                </Tag>
              ),
            },
            { title: 'TaskRun', dataIndex: 'taskRunId', render: (v?: string) => v || '-' },
            {
              title: intl.formatMessage({
                id: 'pages.opsSchedules.runs.column.message',
                defaultMessage: '说明',
              }),
              dataIndex: 'message',
              ellipsis: true,
            },
          ]}
        />
      </Drawer>
    </PageContainer>
  );
}
