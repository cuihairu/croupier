import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Card, Table, Space, Tag, Button, Select, Input, App, Drawer, Tabs } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  deleteSilence,
  fetchOpsAlerts,
  fetchOpsConfig,
  listSilences,
  silenceOpsAlert,
  type OpsAlert,
  type OpsConfig,
  type OpsSilence,
} from '@/services/api/ops';
import AlertRulesTab from './AlertRulesTab';

export default function OpsAlertsPage() {
  const { message, modal } = App.useApp();
  const intl = useIntl();
  const [rows, setRows] = useState<OpsAlert[]>([]);
  const [loading, setLoading] = useState(false);
  const [sev, setSev] = useState<string>('');
  const [svc, setSvc] = useState<string>('');
  const [q, setQ] = useState<string>('');
  const [silences, setSilences] = useState<OpsSilence[]>([]);
  const [cfg, setCfg] = useState<OpsConfig>({});
  const [lk, setLk] = useState('');
  const [lv, setLv] = useState('');
  const [detail, setDetail] = useState<OpsAlert | null>(null);

  const toStringRecord = (input?: Record<string, unknown>): Record<string, string> =>
    Object.fromEntries(
      Object.entries(input || {}).map(([key, value]) => [key, String(value ?? '')]),
    );

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const r = await fetchOpsAlerts();
      setRows(r.alerts || []);
    } catch (e) {
      const errMsg =
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.opsAlerts.error.operationFailed',
              defaultMessage: '操作失败',
            });
      message.error(
        errMsg ||
          intl.formatMessage({
            id: 'pages.opsAlerts.error.loadFailed',
            defaultMessage: '加载失败',
          }),
      );
    } finally {
      setLoading(false);
    }
  }, [message, intl]);
  useEffect(() => {
    load();
  }, [load]);
  useEffect(() => {
    (async () => {
      try {
        const s = await listSilences();
        setSilences(s.silences || []);
      } catch {}
    })();
    (async () => {
      try {
        const c = await fetchOpsConfig();
        setCfg(c || {});
      } catch {}
    })();
  }, []);

  const serviceOptions = useMemo(
    () =>
      Array.from(new Set((rows || []).map((a) => a.service).filter(Boolean) as string[])).map(
        (v) => ({ label: v, value: v }),
      ),
    [rows],
  );
  const data = useMemo(
    () =>
      (rows || []).filter((a) => {
        if (sev && (a.severity || '') !== sev) return false;
        if (svc && (a.service || '') !== svc) return false;
        if (q) {
          const s = `${a.summary || ''} ${JSON.stringify(a.labels || {})}`.toLowerCase();
          if (!s.includes(q.toLowerCase())) return false;
        }
        if (lk) {
          const v = (a.labels || {})[lk];
          if (v == null) return false;
          if (lv && String(v) !== lv) return false;
        }
        return true;
      }),
    [rows, sev, svc, q, lk, lv],
  );

  const columns: ColumnsType<OpsAlert> = [
    {
      title: intl.formatMessage({
        id: 'pages.opsAlerts.column.severity',
        defaultMessage: '严重度',
      }),
      dataIndex: 'severity',
      width: 110,
      render: (v) => {
        const color = v === 'critical' ? 'red' : v === 'warning' ? 'gold' : 'blue';
        return v ? <Tag color={color}>{v}</Tag> : '';
      },
    },
    {
      title: intl.formatMessage({ id: 'pages.opsAlerts.column.service', defaultMessage: '服务' }),
      dataIndex: 'service',
      width: 180,
    },
    {
      title: intl.formatMessage({ id: 'pages.opsAlerts.column.instance', defaultMessage: '实例' }),
      dataIndex: 'instance',
      width: 200,
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'pages.opsAlerts.column.summary', defaultMessage: '摘要' }),
      dataIndex: 'summary',
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'pages.opsAlerts.column.duration', defaultMessage: '时长' }),
      dataIndex: 'duration',
      width: 140,
    },
    {
      title: intl.formatMessage({ id: 'pages.opsAlerts.column.status', defaultMessage: '状态' }),
      dataIndex: 'silenced',
      width: 100,
      render: (v) => (v ? <Tag>silenced</Tag> : <Tag color="volcano">firing</Tag>),
    },
    {
      title: intl.formatMessage({ id: 'pages.opsAlerts.column.actions', defaultMessage: '操作' }),
      width: 160,
      render: (_, r) => (
        <Space>
          {!r.silenced && (
            <Button
              size="small"
              onClick={() => {
                modal.confirm({
                  title: intl.formatMessage({
                    id: 'pages.opsAlerts.silence.modalTitle',
                    defaultMessage: '静默告警',
                  }),
                  content: intl.formatMessage({
                    id: 'pages.opsAlerts.silence.confirm1h',
                    defaultMessage: '静默 1 小时？',
                  }),
                  onOk: async () => {
                    try {
                      await silenceOpsAlert({
                        matchers: toStringRecord(r.labels),
                        duration: '1h',
                        comment: r.summary || '',
                      });
                      message.success(
                        intl.formatMessage({
                          id: 'pages.opsAlerts.silence.success',
                          defaultMessage: '已静默',
                        }),
                      );
                      load();
                    } catch (e) {
                      const errMsg =
                        e instanceof Error
                          ? e.message
                          : intl.formatMessage({
                              id: 'pages.opsAlerts.error.operationFailed',
                              defaultMessage: '操作失败',
                            });
                      message.error(
                        errMsg ||
                          intl.formatMessage({
                            id: 'pages.opsAlerts.error.silenceFailed',
                            defaultMessage: '静默失败',
                          }),
                      );
                    }
                  },
                });
              }}
            >
              <FormattedMessage id="pages.opsAlerts.silence.button1h" defaultMessage="静默1h" />
            </Button>
          )}
          {!r.silenced && (
            <Button
              size="small"
              onClick={() => {
                modal.confirm({
                  title: intl.formatMessage({
                    id: 'pages.opsAlerts.silence.modalTitle',
                    defaultMessage: '静默告警',
                  }),
                  content: intl.formatMessage({
                    id: 'pages.opsAlerts.silence.confirm24h',
                    defaultMessage: '静默 24 小时？',
                  }),
                  onOk: async () => {
                    try {
                      await silenceOpsAlert({
                        matchers: toStringRecord(r.labels),
                        duration: '24h',
                        comment: r.summary || '',
                      });
                      message.success(
                        intl.formatMessage({
                          id: 'pages.opsAlerts.silence.success',
                          defaultMessage: '已静默',
                        }),
                      );
                      load();
                    } catch (e) {
                      const errMsg =
                        e instanceof Error
                          ? e.message
                          : intl.formatMessage({
                              id: 'pages.opsAlerts.error.operationFailed',
                              defaultMessage: '操作失败',
                            });
                      message.error(
                        errMsg ||
                          intl.formatMessage({
                            id: 'pages.opsAlerts.error.silenceFailed',
                            defaultMessage: '静默失败',
                          }),
                      );
                    }
                  },
                });
              }}
            >
              <FormattedMessage id="pages.opsAlerts.silence.button1d" defaultMessage="静默1d" />
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <Tabs
        defaultActiveKey="alerts"
        items={[
          {
            key: 'alerts',
            label: intl.formatMessage({
              id: 'pages.opsAlerts.tab.alerts',
              defaultMessage: '告警列表',
            }),
            children: (
              <>
                <Card
                  title={intl.formatMessage({
                    id: 'pages.opsAlerts.title',
                    defaultMessage: '告警中心',
                  })}
                  extra={
                    <Space>
                      <Select
                        placeholder={intl.formatMessage({
                          id: 'pages.opsAlerts.filter.severity',
                          defaultMessage: '严重度',
                        })}
                        allowClear
                        style={{ width: 140 }}
                        value={sev || undefined}
                        onChange={(v) => setSev(v || '')}
                        options={[
                          { label: 'critical', value: 'critical' },
                          { label: 'warning', value: 'warning' },
                          { label: 'info', value: 'info' },
                        ]}
                      />
                      <Select
                        placeholder={intl.formatMessage({
                          id: 'pages.opsAlerts.filter.service',
                          defaultMessage: '服务',
                        })}
                        allowClear
                        style={{ width: 200 }}
                        value={svc || undefined}
                        onChange={(v) => setSvc(v || '')}
                        options={serviceOptions}
                      />
                      <Input
                        placeholder={intl.formatMessage({
                          id: 'pages.opsAlerts.filter.keyword',
                          defaultMessage: '关键词',
                        })}
                        value={q}
                        onChange={(e) => setQ(e.target.value)}
                        style={{ width: 220 }}
                      />
                      <Input
                        placeholder={intl.formatMessage({
                          id: 'pages.opsAlerts.filter.labelKey',
                          defaultMessage: '标签键',
                        })}
                        value={lk}
                        onChange={(e) => setLk(e.target.value)}
                        style={{ width: 160 }}
                      />
                      <Input
                        placeholder={intl.formatMessage({
                          id: 'pages.opsAlerts.filter.labelValueOptional',
                          defaultMessage: '标签值(可选)',
                        })}
                        value={lv}
                        onChange={(e) => setLv(e.target.value)}
                        style={{ width: 160 }}
                      />
                      {cfg?.grafanaExploreUrl && (
                        <Button onClick={() => window.open(cfg.grafanaExploreUrl, '_blank')}>
                          <FormattedMessage
                            id="pages.opsAlerts.action.openGrafana"
                            defaultMessage="打开 Grafana"
                          />
                        </Button>
                      )}
                      {cfg?.alertmanagerUrl && (
                        <Button
                          onClick={() => window.open(cfg.alertmanagerUrl + '/#/alerts', '_blank')}
                        >
                          <FormattedMessage
                            id="pages.opsAlerts.action.openAlertmanager"
                            defaultMessage="打开 AM"
                          />
                        </Button>
                      )}
                      <Button
                        onClick={() => {
                          load();
                          (async () => {
                            try {
                              const s = await listSilences();
                              setSilences(s.silences || []);
                            } catch {}
                          })();
                        }}
                      >
                        <FormattedMessage
                          id="pages.opsAlerts.action.refresh"
                          defaultMessage="刷新"
                        />
                      </Button>
                    </Space>
                  }
                >
                  <Table
                    scroll={{ x: 1000 }}
                    rowKey={(r) =>
                      `${r.service || ''}|${r.instance || ''}|${r.summary || ''}|${r.startsAt || ''}`
                    }
                    loading={loading}
                    dataSource={data}
                    columns={columns}
                    pagination={{ pageSize: 10 }}
                    onRow={(rec) => ({ onClick: () => setDetail(rec) })}
                  />
                </Card>
                <Card
                  title={intl.formatMessage({
                    id: 'pages.opsAlerts.silenceCard.title',
                    defaultMessage: '静默列表',
                  })}
                  style={{ marginTop: 16 }}
                  extra={
                    <Button
                      onClick={async () => {
                        try {
                          const s = await listSilences();
                          setSilences(s.silences || []);
                        } catch {}
                      }}
                    >
                      <FormattedMessage id="pages.opsAlerts.action.refresh" defaultMessage="刷新" />
                    </Button>
                  }
                >
                  <Table
                    rowKey={(r) => String(r.id)}
                    dataSource={silences}
                    columns={[
                      { title: 'ID', dataIndex: 'id', width: 220 },
                      {
                        title: intl.formatMessage({
                          id: 'pages.opsAlerts.silenceCard.column.creator',
                          defaultMessage: '创建者',
                        }),
                        dataIndex: 'createdBy',
                        width: 140,
                      },
                      {
                        title: intl.formatMessage({
                          id: 'pages.opsAlerts.silenceCard.column.time',
                          defaultMessage: '时间',
                        }),
                        render: (_: unknown, r: OpsSilence) =>
                          `${r.startAt || ''} -> ${r.endAt || ''}`,
                      },
                      {
                        title: intl.formatMessage({
                          id: 'pages.opsAlerts.column.actions',
                          defaultMessage: '操作',
                        }),
                        width: 160,
                        render: (_: unknown, r: OpsSilence) => (
                          <Space>
                            <Button
                              size="small"
                              onClick={() =>
                                window.open(
                                  (cfg?.alertmanagerUrl || '').replace(/\/$/, '') +
                                    `/#/silences/${encodeURIComponent(r.id)}`,
                                  '_blank',
                                )
                              }
                            >
                              <FormattedMessage
                                id="pages.opsAlerts.silenceCard.action.view"
                                defaultMessage="查看"
                              />
                            </Button>
                            <Button
                              size="small"
                              danger
                              onClick={() =>
                                modal.confirm({
                                  title: intl.formatMessage({
                                    id: 'pages.opsAlerts.silenceCard.unsilenceModalTitle',
                                    defaultMessage: '解除静默',
                                  }),
                                  content: intl.formatMessage(
                                    {
                                      id: 'pages.opsAlerts.silenceCard.confirm.unsilence',
                                      defaultMessage: '确定解除静默 {id}?',
                                    },
                                    { id: r.id },
                                  ),
                                  onOk: async () => {
                                    try {
                                      await deleteSilence(String(r.id));
                                      message.success(
                                        intl.formatMessage({
                                          id: 'pages.opsAlerts.silenceCard.success.unsilenced',
                                          defaultMessage: '已解除',
                                        }),
                                      );
                                      const s = await listSilences();
                                      setSilences(s.silences || []);
                                    } catch (e) {
                                      const errMsg =
                                        e instanceof Error
                                          ? e.message
                                          : intl.formatMessage({
                                              id: 'pages.opsAlerts.error.operationFailed',
                                              defaultMessage: '操作失败',
                                            });
                                      message.error(
                                        errMsg ||
                                          intl.formatMessage({
                                            id: 'pages.opsAlerts.error.operationFailed',
                                            defaultMessage: '操作失败',
                                          }),
                                      );
                                    }
                                  },
                                })
                              }
                            >
                              <FormattedMessage
                                id="pages.opsAlerts.silenceCard.action.unsilence"
                                defaultMessage="解除"
                              />
                            </Button>
                          </Space>
                        ),
                      },
                    ]}
                    pagination={{ pageSize: 10 }}
                  />
                </Card>
              </>
            ),
          },
          {
            key: 'rules',
            label: intl.formatMessage({
              id: 'pages.opsAlerts.tab.rules',
              defaultMessage: '告警规则',
            }),
            children: <AlertRulesTab />,
          },
        ]}
      />
      <Drawer
        title={intl.formatMessage({
          id: 'pages.opsAlerts.drawer.title',
          defaultMessage: '告警详情',
        })}
        width={720}
        open={!!detail}
        onClose={() => setDetail(null)}
      >
        {detail && (
          <Space orientation="vertical" style={{ width: '100%' }}>
            <div>
              <b>
                <FormattedMessage
                  id="pages.opsAlerts.drawer.severityLabel"
                  defaultMessage="严重度:"
                />
              </b>{' '}
              <Tag
                color={
                  detail.severity === 'critical'
                    ? 'red'
                    : detail.severity === 'warning'
                      ? 'gold'
                      : 'blue'
                }
              >
                {detail.severity}
              </Tag>
            </div>
            <div>
              <b>
                <FormattedMessage
                  id="pages.opsAlerts.drawer.serviceInstance"
                  defaultMessage="服务/实例:"
                />
              </b>{' '}
              {detail.service || '-'} / {detail.instance || '-'}
            </div>
            <div>
              <b>
                <FormattedMessage id="pages.opsAlerts.drawer.summaryLabel" defaultMessage="摘要:" />
              </b>{' '}
              {detail.summary || '-'}
            </div>
            <div>
              <b>
                <FormattedMessage id="pages.opsAlerts.drawer.startsAt" defaultMessage="开始时间:" />
              </b>{' '}
              {detail.startsAt || '-'}{' '}
              <b>
                <FormattedMessage
                  id="pages.opsAlerts.drawer.durationLabel"
                  defaultMessage="时长:"
                />
              </b>{' '}
              {detail.duration || '-'}
            </div>
            <div>
              <b>
                <FormattedMessage id="pages.opsAlerts.drawer.statusLabel" defaultMessage="状态:" />
              </b>{' '}
              {detail.silenced ? <Tag>silenced</Tag> : <Tag color="volcano">firing</Tag>}
            </div>
            <div>
              <div style={{ fontWeight: 600, marginBottom: 6 }}>
                <FormattedMessage id="pages.opsAlerts.drawer.labels" defaultMessage="标签" />
              </div>
              <div>
                {Object.entries(detail.labels || {}).map(([k, v]) => (
                  <Tag key={k}>
                    {k}:{String(v)}
                  </Tag>
                ))}
              </div>
            </div>
            <div>
              <div style={{ fontWeight: 600, marginBottom: 6 }}>
                <FormattedMessage id="pages.opsAlerts.drawer.annotations" defaultMessage="注释" />
              </div>
              <div>
                {Object.entries(detail.annotations || {}).map(([k, v]) => (
                  <div key={k}>
                    <b>{k}:</b> {String(v)}
                  </div>
                ))}
              </div>
            </div>
            <Space>
              {!detail.silenced && (
                <Button
                  onClick={() =>
                    modal.confirm({
                      title: intl.formatMessage({
                        id: 'pages.opsAlerts.silence.title1h',
                        defaultMessage: '静默 1 小时',
                      }),
                      onOk: async () => {
                        try {
                          await silenceOpsAlert({
                            matchers: toStringRecord(detail.labels),
                            duration: '1h',
                            comment: detail.summary || '',
                          });
                          message.success(
                            intl.formatMessage({
                              id: 'pages.opsAlerts.silence.success',
                              defaultMessage: '已静默',
                            }),
                          );
                          load();
                          setDetail(null);
                        } catch (e) {
                          const errMsg =
                            e instanceof Error
                              ? e.message
                              : intl.formatMessage({
                                  id: 'pages.opsAlerts.error.operationFailed',
                                  defaultMessage: '操作失败',
                                });
                          message.error(
                            errMsg ||
                              intl.formatMessage({
                                id: 'pages.opsAlerts.error.actionFailed',
                                defaultMessage: '失败',
                              }),
                          );
                        }
                      },
                    })
                  }
                >
                  <FormattedMessage id="pages.opsAlerts.silence.button1h" defaultMessage="静默1h" />
                </Button>
              )}
              {!detail.silenced && (
                <Button
                  onClick={() =>
                    modal.confirm({
                      title: intl.formatMessage({
                        id: 'pages.opsAlerts.silence.title6h',
                        defaultMessage: '静默 6 小时',
                      }),
                      onOk: async () => {
                        try {
                          await silenceOpsAlert({
                            matchers: toStringRecord(detail.labels),
                            duration: '6h',
                            comment: detail.summary || '',
                          });
                          message.success(
                            intl.formatMessage({
                              id: 'pages.opsAlerts.silence.success',
                              defaultMessage: '已静默',
                            }),
                          );
                          load();
                          setDetail(null);
                        } catch (e) {
                          const errMsg =
                            e instanceof Error
                              ? e.message
                              : intl.formatMessage({
                                  id: 'pages.opsAlerts.error.operationFailed',
                                  defaultMessage: '操作失败',
                                });
                          message.error(
                            errMsg ||
                              intl.formatMessage({
                                id: 'pages.opsAlerts.error.actionFailed',
                                defaultMessage: '失败',
                              }),
                          );
                        }
                      },
                    })
                  }
                >
                  <FormattedMessage id="pages.opsAlerts.silence.button6h" defaultMessage="静默6h" />
                </Button>
              )}
              {!detail.silenced && (
                <Button
                  onClick={() =>
                    modal.confirm({
                      title: intl.formatMessage({
                        id: 'pages.opsAlerts.silence.title24h',
                        defaultMessage: '静默 24 小时',
                      }),
                      onOk: async () => {
                        try {
                          await silenceOpsAlert({
                            matchers: toStringRecord(detail.labels),
                            duration: '24h',
                            comment: detail.summary || '',
                          });
                          message.success(
                            intl.formatMessage({
                              id: 'pages.opsAlerts.silence.success',
                              defaultMessage: '已静默',
                            }),
                          );
                          load();
                          setDetail(null);
                        } catch (e) {
                          const errMsg =
                            e instanceof Error
                              ? e.message
                              : intl.formatMessage({
                                  id: 'pages.opsAlerts.error.operationFailed',
                                  defaultMessage: '操作失败',
                                });
                          message.error(
                            errMsg ||
                              intl.formatMessage({
                                id: 'pages.opsAlerts.error.actionFailed',
                                defaultMessage: '失败',
                              }),
                          );
                        }
                      },
                    })
                  }
                >
                  <FormattedMessage id="pages.opsAlerts.silence.button1d" defaultMessage="静默1d" />
                </Button>
              )}
              {typeof (detail.annotations || {}).runbook_url === 'string' && (
                <Button
                  onClick={() =>
                    window.open((detail.annotations || {}).runbook_url as string, '_blank')
                  }
                >
                  <FormattedMessage
                    id="pages.opsAlerts.action.openRunbook"
                    defaultMessage="打开 Runbook"
                  />
                </Button>
              )}
              {cfg.grafanaExploreUrl && (
                <Button onClick={() => window.open(cfg.grafanaExploreUrl!, '_blank')}>
                  <FormattedMessage
                    id="pages.opsAlerts.action.openGrafana"
                    defaultMessage="打开 Grafana"
                  />
                </Button>
              )}
            </Space>
          </Space>
        )}
      </Drawer>
    </PageContainer>
  );
}
