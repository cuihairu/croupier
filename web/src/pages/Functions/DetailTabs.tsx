import React, { useEffect, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Drawer,
  Input,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import { StatisticCard } from '@ant-design/pro-components';
import { BarChartOutlined } from '@ant-design/icons';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import {
  deleteFunctionVersionFloor,
  diffContractVersions,
  getContractVersion,
  getFunctionAnalytics,
  getFunctionVersionFloor,
  listContractVersions,
  listFunctionWarnings,
  putFunctionVersionFloor,
  type ContractVersionDetail,
  type ContractVersionDiffResult,
  type ContractVersionDiffEntry,
  type ContractVersionItem,
} from '@/services/api/functions';
import {
  getExecutionLog,
  listExecutionLogs,
  type ExecutionLogDetail,
  type ExecutionLogItem,
} from '@/services/api/executionLogs';

type AnalyticsData = {
  totalCalls: number;
  successRate: number;
  avgLatency: number;
  callsToday: number;
};

const formatDateTime = (value?: string) => {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString();
};

export function HistoryTab({ functionId }: { functionId: string }) {
  const intl = useIntl();
  const [historyData, setHistoryData] = useState<ExecutionLogItem[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<ExecutionLogDetail | null>(null);

  useEffect(() => {
    const loadHistory = async () => {
      setHistoryLoading(true);
      try {
        // 真实调用记录（执行留痕）：操作人为发起调用的登录账号；
        // 游戏侧 SDK 直连调用没有控制台身份，actor 为空展示 '-'
        const resp = await listExecutionLogs({ functionId, page, pageSize });
        setHistoryData(resp.items ?? []);
        setTotal(resp.total ?? 0);
      } catch {
        setHistoryData([]);
        setTotal(0);
      } finally {
        setHistoryLoading(false);
      }
    };
    loadHistory();
  }, [functionId, page, pageSize]);

  const openDetail = async (id: number) => {
    setDetailOpen(true);
    setDetailLoading(true);
    try {
      setDetail(await getExecutionLog(id));
    } catch {
      setDetail(null);
    } finally {
      setDetailLoading(false);
    }
  };

  const renderPayload = (value: unknown) => {
    if (value === undefined || value === null) {
      return '-';
    }
    let text: string;
    try {
      text = JSON.stringify(value, null, 2);
    } catch {
      text = String(value);
    }
    return (
      <pre
        style={{
          margin: 0,
          maxHeight: 320,
          overflow: 'auto',
          fontSize: 12,
          background: 'rgba(128, 128, 128, 0.08)',
          padding: 8,
          borderRadius: 4,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
        }}
      >
        {text}
      </pre>
    );
  };

  return (
    <>
      <Table
        loading={historyLoading}
        dataSource={historyData}
        rowKey="id"
        columns={[
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.history.column.time',
              defaultMessage: '时间',
            }),
            dataIndex: 'createdAt',
            width: 170,
            render: (text: string) => formatDateTime(text),
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.history.column.operator',
              defaultMessage: '操作人',
            }),
            dataIndex: 'actor',
            width: 120,
            render: (text: string) => text || '-',
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.history.column.status',
              defaultMessage: '状态',
            }),
            dataIndex: 'status',
            width: 90,
            render: (text: string) => (
              <Tag color={text === 'ok' ? 'success' : 'error'}>
                {intl.formatMessage({
                  id:
                    text === 'ok'
                      ? 'pages.functionsDetail.history.status.ok'
                      : 'pages.functionsDetail.history.status.fail',
                  defaultMessage: text === 'ok' ? '成功' : '失败',
                })}
              </Tag>
            ),
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.history.column.duration',
              defaultMessage: '耗时',
            }),
            dataIndex: 'durationMs',
            width: 100,
            render: (text: number) => (text ? `${text} ms` : '-'),
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.history.column.source',
              defaultMessage: '来源',
            }),
            dataIndex: 'source',
            width: 100,
            render: (text: string) =>
              intl.formatMessage({
                id:
                  text === 'page'
                    ? 'pages.functionsDetail.history.source.page'
                    : 'pages.functionsDetail.history.source.invoke',
                defaultMessage: text === 'page' ? '页面执行' : '直接调用',
              }),
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.history.column.detailAction',
              defaultMessage: '操作',
            }),
            key: 'detailAction',
            width: 80,
            render: (_, record) => (
              <Button type="link" size="small" onClick={() => openDetail(record.id)}>
                <FormattedMessage
                  id="pages.functionsDetail.history.action.viewDetail"
                  defaultMessage="详情"
                />
              </Button>
            ),
          },
        ]}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          pageSizeOptions: [10, 20, 50],
          showTotal: (t) =>
            intl.formatMessage(
              {
                id: 'pages.functionsDetail.history.paginationTotal',
                defaultMessage: '共 {total} 条',
              },
              { total: t },
            ),
          onChange: (nextPage, nextSize) => {
            setPage(nextPage);
            setPageSize(nextSize);
          },
        }}
      />
      <Drawer
        open={detailOpen}
        width={560}
        loading={detailLoading}
        onClose={() => setDetailOpen(false)}
        title={
          detail
            ? intl.formatMessage(
                {
                  id: 'pages.functionsDetail.history.detail.title',
                  defaultMessage: '调用详情 · {functionId}',
                },
                { functionId: detail.functionId },
              )
            : intl.formatMessage({
                id: 'pages.functionsDetail.history.detail.titlePlain',
                defaultMessage: '调用详情',
              })
        }
      >
        {detail && (
          <>
            <Descriptions
              column={2}
              size="small"
              style={{ marginBottom: 16 }}
              items={[
                {
                  key: 'actor',
                  label: intl.formatMessage({
                    id: 'pages.functionsDetail.history.column.operator',
                    defaultMessage: '操作人',
                  }),
                  children: detail.actor || '-',
                },
                {
                  key: 'status',
                  label: intl.formatMessage({
                    id: 'pages.functionsDetail.history.column.status',
                    defaultMessage: '状态',
                  }),
                  children:
                    detail.status === 'ok'
                      ? intl.formatMessage({
                          id: 'pages.functionsDetail.history.status.ok',
                          defaultMessage: '成功',
                        })
                      : intl.formatMessage({
                          id: 'pages.functionsDetail.history.status.fail',
                          defaultMessage: '失败',
                        }),
                },
                {
                  key: 'time',
                  label: intl.formatMessage({
                    id: 'pages.functionsDetail.history.column.time',
                    defaultMessage: '时间',
                  }),
                  children: formatDateTime(detail.createdAt),
                },
                {
                  key: 'duration',
                  label: intl.formatMessage({
                    id: 'pages.functionsDetail.history.column.duration',
                    defaultMessage: '耗时',
                  }),
                  children: detail.durationMs ? `${detail.durationMs} ms` : '-',
                },
                {
                  key: 'source',
                  label: intl.formatMessage({
                    id: 'pages.functionsDetail.history.column.source',
                    defaultMessage: '来源',
                  }),
                  children:
                    detail.source === 'page'
                      ? intl.formatMessage({
                          id: 'pages.functionsDetail.history.source.page',
                          defaultMessage: '页面执行',
                        })
                      : intl.formatMessage({
                          id: 'pages.functionsDetail.history.source.invoke',
                          defaultMessage: '直接调用',
                        }),
                },
                {
                  key: 'pageKey',
                  label: intl.formatMessage({
                    id: 'pages.functionsDetail.history.detail.pageKey',
                    defaultMessage: '页面 Key',
                  }),
                  children: detail.pageKey || '-',
                },
              ]}
            />
            <Typography.Title level={5}>
              <FormattedMessage
                id="pages.functionsDetail.history.detail.request"
                defaultMessage="请求参数"
              />
            </Typography.Title>
            {renderPayload(detail.requestPayload)}
            <Typography.Title level={5} style={{ marginTop: 16 }}>
              <FormattedMessage
                id="pages.functionsDetail.history.detail.response"
                defaultMessage="执行结果"
              />
            </Typography.Title>
            {renderPayload(detail.responseBody)}
          </>
        )}
      </Drawer>
    </>
  );
}

export function AnalyticsTab({ functionId }: { functionId: string }) {
  const intl = useIntl();
  const [analyticsData, setAnalyticsData] = useState<AnalyticsData | null>(null);
  const [analyticsLoading, setAnalyticsLoading] = useState(false);

  useEffect(() => {
    const loadAnalytics = async () => {
      setAnalyticsLoading(true);
      try {
        const data = await getFunctionAnalytics(functionId);
        setAnalyticsData(data);
      } catch {
        setAnalyticsData(null);
      } finally {
        setAnalyticsLoading(false);
      }
    };
    loadAnalytics();
  }, [functionId]);

  return (
    <Row gutter={16}>
      <Col span={6}>
        <StatisticCard
          loading={analyticsLoading}
          statistic={{
            title: intl.formatMessage({
              id: 'pages.functionsDetail.analytics.totalCalls',
              defaultMessage: '总调用次数',
            }),
            value: analyticsData?.totalCalls || 0,
            prefix: <BarChartOutlined />,
          }}
        />
      </Col>
      <Col span={6}>
        <StatisticCard
          loading={analyticsLoading}
          statistic={{
            title: intl.formatMessage({
              id: 'pages.functionsDetail.analytics.successRate',
              defaultMessage: '成功率',
            }),
            value: analyticsData?.successRate || 0,
            suffix: '%',
            precision: 2,
            styles: {
              content: {
                color: (analyticsData?.successRate || 0) >= 95 ? '#3f8600' : '#cf1322',
              },
            },
          }}
        />
      </Col>
      <Col span={6}>
        <StatisticCard
          loading={analyticsLoading}
          statistic={{
            title: intl.formatMessage({
              id: 'pages.functionsDetail.analytics.avgLatency',
              defaultMessage: '平均延迟',
            }),
            value: analyticsData?.avgLatency || 0,
            suffix: 'ms',
            precision: 0,
          }}
        />
      </Col>
      <Col span={6}>
        <StatisticCard
          loading={analyticsLoading}
          statistic={{
            title: intl.formatMessage({
              id: 'pages.functionsDetail.analytics.callsToday',
              defaultMessage: '今日调用',
            }),
            value: analyticsData?.callsToday || 0,
          }}
        />
      </Col>
    </Row>
  );
}

export function WarningsTab({ functionId }: { functionId: string }) {
  const intl = useIntl();
  const [warningsData, setWarningsData] = useState<
    Array<{
      key: string;
      agentId?: string;
      functionId?: string;
      version?: string;
      code: string;
      message: string;
      count: number;
      firstSeen?: string;
      lastSeen?: string;
    }>
  >([]);
  const [warningsLoading, setWarningsLoading] = useState(false);

  useEffect(() => {
    const loadWarnings = async () => {
      setWarningsLoading(true);
      try {
        const res = await listFunctionWarnings({ functionId, limit: 200 });
        setWarningsData(Array.isArray(res?.items) ? res.items : []);
      } catch {
        setWarningsData([]);
      } finally {
        setWarningsLoading(false);
      }
    };
    loadWarnings();
  }, [functionId]);

  return (
    <>
      <Alert
        message={intl.formatMessage({
          id: 'pages.functionsDetail.warnings.alertMessage',
          defaultMessage: '注册告警',
        })}
        description={intl.formatMessage({
          id: 'pages.functionsDetail.warnings.alertDescription',
          defaultMessage:
            '这里显示函数注册校验告警（例如 function_id 格式错误、版本号不合法、重复注册去重）。',
        })}
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        action={
          <Button
            size="small"
            onClick={() =>
              history.push(`/functions/warnings?function_id=${encodeURIComponent(functionId)}`)
            }
          >
            <FormattedMessage
              id="pages.functionsDetail.warnings.viewAll"
              defaultMessage="查看全部"
            />
          </Button>
        }
      />
      <Table
        scroll={{ x: 850 }}
        loading={warningsLoading}
        dataSource={warningsData}
        rowKey="key"
        columns={[
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.warnings.column.code',
              defaultMessage: '代码',
            }),
            dataIndex: 'code',
            width: 180,
            render: (code: string) => <Tag color="orange">{code || '-'}</Tag>,
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.warnings.column.version',
              defaultMessage: '版本',
            }),
            dataIndex: 'version',
            width: 120,
            render: (v: string) => v || '-',
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.warnings.column.count',
              defaultMessage: '次数',
            }),
            dataIndex: 'count',
            width: 90,
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.warnings.column.lastSeen',
              defaultMessage: '最近时间',
            }),
            dataIndex: 'lastSeen',
            width: 180,
            render: (text: string) => formatDateTime(text),
          },
          { title: 'Agent', dataIndex: 'agentId', width: 220, ellipsis: true },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.warnings.column.details',
              defaultMessage: '详情',
            }),
            dataIndex: 'message',
            ellipsis: true,
          },
        ]}
        pagination={{ pageSize: 10 }}
      />
    </>
  );
}

// B2：函数契约变更历史（版本快照流 + 两版对比）
const CHANGE_TYPE_TONE: Record<ContractVersionItem['changeType'], string> = {
  created: 'success',
  updated: 'processing',
  removed: 'error',
};

function prettyJSON(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export function VersionsTab({ functionId }: { functionId: string }) {
  const intl = useIntl();
  const { message } = App.useApp();
  const [floor, setFloor] = useState('');
  const [floorInput, setFloorInput] = useState('');
  const [floorSaving, setFloorSaving] = useState(false);
  const [rows, setRows] = useState<ContractVersionItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);

  const [snapshotOpen, setSnapshotOpen] = useState(false);
  const [snapshotLoading, setSnapshotLoading] = useState(false);
  const [snapshot, setSnapshot] = useState<ContractVersionDetail | null>(null);

  const [fromSeq, setFromSeq] = useState<number | undefined>();
  const [toSeq, setToSeq] = useState<number | undefined>();
  const [diffResult, setDiffResult] = useState<ContractVersionDiffResult | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);

  // 版本门槛（函数级最低 SDK 版本）：与版本历史同页维护——它们共同
  // 回答「这个函数的契约/版本现在以谁为准」。
  useEffect(() => {
    let cancelled = false;
    getFunctionVersionFloor(functionId)
      .then((resp) => {
        if (cancelled) return;
        setFloor(resp.minVersion);
        setFloorInput(resp.minVersion);
      })
      .catch(() => {
        if (cancelled) return;
        setFloor('');
        setFloorInput('');
      });
    return () => {
      cancelled = true;
    };
  }, [functionId]);

  const saveFloor = async () => {
    const minVersion = floorInput.trim();
    if (!minVersion) return;
    setFloorSaving(true);
    try {
      const resp = await putFunctionVersionFloor(functionId, minVersion);
      setFloor(resp.minVersion);
      setFloorInput(resp.minVersion);
      message.success(
        intl.formatMessage({
          id: 'pages.functionsDetail.versions.floor.saved',
          defaultMessage: '版本门槛已更新',
        }),
      );
    } catch (err) {
      message.error(err instanceof Error ? err.message : String(err));
    } finally {
      setFloorSaving(false);
    }
  };

  const clearFloor = async () => {
    setFloorSaving(true);
    try {
      await deleteFunctionVersionFloor(functionId);
      setFloor('');
      setFloorInput('');
      message.success(
        intl.formatMessage({
          id: 'pages.functionsDetail.versions.floor.cleared',
          defaultMessage: '版本门槛已清除',
        }),
      );
    } catch (err) {
      message.error(err instanceof Error ? err.message : String(err));
    } finally {
      setFloorSaving(false);
    }
  };

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      try {
        const resp = await listContractVersions(functionId, { page, pageSize });
        if (!cancelled) {
          setRows(resp.items);
          setTotal(resp.total);
        }
      } catch {
        if (!cancelled) {
          setRows([]);
          setTotal(0);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [functionId, page, pageSize]);

  const openSnapshot = async (seq: number) => {
    setSnapshotOpen(true);
    setSnapshotLoading(true);
    try {
      setSnapshot(await getContractVersion(functionId, seq));
    } catch {
      setSnapshot(null);
    } finally {
      setSnapshotLoading(false);
    }
  };

  const runDiff = async () => {
    if (fromSeq === undefined || toSeq === undefined || fromSeq === toSeq) return;
    setDiffLoading(true);
    try {
      setDiffResult(await diffContractVersions(functionId, fromSeq, toSeq));
    } catch {
      setDiffResult(null);
    } finally {
      setDiffLoading(false);
    }
  };

  const seqOptions = rows.map((row) => ({ value: row.seq, label: `#${row.seq}` }));
  const changeTypeLabel = (type: ContractVersionItem['changeType']) => {
    if (type !== 'created' && type !== 'updated' && type !== 'removed') return type;
    return intl.formatMessage({
      id: `pages.functionsDetail.versions.changeType.${type}`,
      defaultMessage: type === 'created' ? '新建' : type === 'removed' ? '删除' : '更新',
    });
  };

  return (
    <Space orientation="vertical" size={16} style={{ width: '100%' }}>
      <Card
        size="small"
        title={intl.formatMessage({
          id: 'pages.functionsDetail.versions.floor.title',
          defaultMessage: '版本门槛（最低函数版本）',
        })}
      >
        <Space wrap>
          <Typography.Text type="secondary">
            <FormattedMessage
              id="pages.functionsDetail.versions.floor.description"
              defaultMessage="函数以低于该值的版本注册时不物化（只产生注册警告）——挡住滚动升级窗口里旧 game server 重注册造成的契约回退。留空表示不设置。"
            />
          </Typography.Text>
          <Input
            style={{ width: 160 }}
            placeholder="0.3.0"
            value={floorInput}
            disabled={floorSaving}
            onChange={(e) => setFloorInput(e.target.value)}
          />
          <Button
            type="primary"
            loading={floorSaving}
            disabled={!floorInput.trim() || floorInput.trim() === floor}
            onClick={saveFloor}
          >
            <FormattedMessage
              id="pages.functionsDetail.versions.floor.save"
              defaultMessage="保存"
            />
          </Button>
          <Button danger disabled={floorSaving || !floor} onClick={clearFloor}>
            <FormattedMessage
              id="pages.functionsDetail.versions.floor.clear"
              defaultMessage="清除"
            />
          </Button>
          {floor ? (
            <Tag color="blue">
              <FormattedMessage
                id="pages.functionsDetail.versions.floor.current"
                defaultMessage="当前门槛"
              />
              {`: ${floor}`}
            </Tag>
          ) : (
            <Tag>
              <FormattedMessage
                id="pages.functionsDetail.versions.floor.unset"
                defaultMessage="未设置"
              />
            </Tag>
          )}
        </Space>
      </Card>
      <Space wrap>
        <FormattedMessage
          id="pages.functionsDetail.versions.diffToolbar"
          defaultMessage="两版对比："
        />
        <Select
          style={{ width: 110 }}
          placeholder={intl.formatMessage({
            id: 'pages.functionsDetail.versions.fromSeq',
            defaultMessage: '起始版本',
          })}
          options={seqOptions}
          value={fromSeq}
          onChange={setFromSeq}
        />
        <Select
          style={{ width: 110 }}
          placeholder={intl.formatMessage({
            id: 'pages.functionsDetail.versions.toSeq',
            defaultMessage: '目标版本',
          })}
          options={seqOptions}
          value={toSeq}
          onChange={setToSeq}
        />
        <Button
          type="primary"
          disabled={fromSeq === undefined || toSeq === undefined || fromSeq === toSeq}
          loading={diffLoading}
          onClick={() => void runDiff()}
        >
          <FormattedMessage id="pages.functionsDetail.versions.diffAction" defaultMessage="对比" />
        </Button>
      </Space>
      {diffResult ? (
        <div>
          {diffResult.breaking ? (
            <Alert
              type="warning"
              showIcon
              message={intl.formatMessage(
                {
                  id: 'pages.functionsDetail.versions.diffBreaking',
                  defaultMessage:
                    '#{from} → #{to}：存在破坏性 schema 变更，绑定页面可能需要同步更新',
                },
                { from: diffResult.fromSeq, to: diffResult.toSeq },
              )}
            />
          ) : (
            <Alert
              type="info"
              showIcon
              message={intl.formatMessage(
                {
                  id: 'pages.functionsDetail.versions.diffSafe',
                  defaultMessage: '#{from} → #{to}：变更兼容',
                },
                { from: diffResult.fromSeq, to: diffResult.toSeq },
              )}
            />
          )}
          <Table<ContractVersionDiffEntry>
            style={{ marginTop: 8 }}
            size="small"
            rowKey={(record) => `${record.field}-${record.change ?? record.from ?? ''}`}
            dataSource={diffResult.changes}
            pagination={false}
            columns={[
              { title: 'field', dataIndex: 'field', width: 140 },
              { title: 'from', dataIndex: 'from', ellipsis: true },
              { title: 'to', dataIndex: 'to', ellipsis: true },
              {
                title: intl.formatMessage({
                  id: 'pages.functionsDetail.versions.column.findings',
                  defaultMessage: 'schema 变更',
                }),
                key: 'findings',
                width: 220,
                render: (_, record) =>
                  record.findings?.length
                    ? record.findings.map((finding) => (
                        <div key={`${finding.source}${finding.path}${finding.reason}`}>
                          <Tag color={finding.severity === 'breaking' ? 'red' : 'blue'}>
                            {finding.severity}
                          </Tag>
                          {finding.path} {finding.reason}
                        </div>
                      ))
                    : '-',
              },
            ]}
          />
        </div>
      ) : null}
      <Table<ContractVersionItem>
        loading={loading}
        rowKey="seq"
        dataSource={rows}
        columns={[
          { title: '#', dataIndex: 'seq', width: 60 },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.versions.column.time',
              defaultMessage: '时间',
            }),
            dataIndex: 'createdAt',
            width: 170,
            render: (text: string) => formatDateTime(text),
          },
          {
            title: 'version',
            dataIndex: 'version',
            width: 100,
            render: (text?: string) => text || '-',
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.versions.column.changeType',
              defaultMessage: '变更',
            }),
            dataIndex: 'changeType',
            width: 90,
            render: (type: ContractVersionItem['changeType']) => (
              <Tag color={CHANGE_TYPE_TONE[type]}>{changeTypeLabel(type)}</Tag>
            ),
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.versions.column.breaking',
              defaultMessage: '兼容性',
            }),
            dataIndex: 'breaking',
            width: 100,
            render: (breaking: boolean) =>
              breaking ? (
                <Tag color="orange">
                  <FormattedMessage
                    id="pages.functionsDetail.versions.breakingTag"
                    defaultMessage="破坏性"
                  />
                </Tag>
              ) : (
                <FormattedMessage
                  id="pages.functionsDetail.versions.compatibleTag"
                  defaultMessage="兼容"
                />
              ),
          },
          {
            title: 'source',
            dataIndex: 'source',
            width: 90,
            render: (text?: string) => text || '-',
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.versions.column.actor',
              defaultMessage: '触发方',
            }),
            dataIndex: 'actor',
            width: 110,
            render: (text?: string) => text || '-',
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.versions.column.changes',
              defaultMessage: '变更字段',
            }),
            key: 'changes',
            ellipsis: true,
            render: (_, record) =>
              record.diff?.length ? record.diff.map((entry) => entry.field).join('、') : '-',
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.versions.column.action',
              defaultMessage: '操作',
            }),
            key: 'action',
            width: 80,
            render: (_, record) => (
              <Button type="link" size="small" onClick={() => void openSnapshot(record.seq)}>
                <FormattedMessage
                  id="pages.functionsDetail.versions.action.snapshot"
                  defaultMessage="快照"
                />
              </Button>
            ),
          },
        ]}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          pageSizeOptions: [10, 20, 50],
          showTotal: (t) =>
            intl.formatMessage(
              {
                id: 'pages.functionsDetail.versions.paginationTotal',
                defaultMessage: '共 {total} 条',
              },
              { total: t },
            ),
          onChange: (nextPage, nextSize) => {
            setPage(nextPage);
            setPageSize(nextSize);
          },
        }}
      />
      <Drawer
        open={snapshotOpen}
        width={640}
        loading={snapshotLoading}
        onClose={() => setSnapshotOpen(false)}
        title={intl.formatMessage(
          {
            id: 'pages.functionsDetail.versions.snapshotTitle',
            defaultMessage: '版本 #{seq} 快照',
          },
          { seq: snapshot?.seq ?? '-' },
        )}
      >
        {snapshot ? (
          <pre
            style={{
              margin: 0,
              fontSize: 12,
              background: 'rgba(128, 128, 128, 0.08)',
              padding: 8,
              borderRadius: 4,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
            }}
          >
            {prettyJSON(snapshot.snapshot)}
          </pre>
        ) : null}
      </Drawer>
    </Space>
  );
}
