import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Drawer,
  Empty,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Row,
  Select,
  Space,
  Switch,
  Tooltip,
  Table,
  Tag,
  Typography,
} from 'antd';
import { ModalForm, PageContainer } from '@ant-design/pro-components';
import {
  DatabaseOutlined,
  PlusOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { FormattedMessage, useAccess, useIntl } from '@umijs/max';
import {
  type DBSourceKind,
  createDBSource,
  dbKindLabels,
  deleteDBSource,
  listDBSources,
  probeAll,
  updateDBSource,
  type DBSource,
  type ProbeResult,
} from '@/services/api/dbmon';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

/** 数据源表单值：对齐 createDBSource/updateDBSource 的 payload 形状；
 * dsn 掩码存储，编辑回填时留空表示不修改 */
type DBSourceFormValues = {
  name: string;
  driver: string;
  kind: string;
  dsn?: string;
  gameId?: string;
  env?: string;
  enabled?: boolean;
  lockWaitWarn?: number;
  connWarnRatio?: number;
};

/** 编辑回填：DSN 不回填（掩码存储），留空表示不修改 */
const toDBSourceFormValues = (src: DBSource): DBSourceFormValues => ({
  name: src.name,
  driver: src.driver,
  kind: src.kind,
  gameId: src.gameId,
  env: src.env,
  lockWaitWarn: src.lockWaitWarn || undefined,
  connWarnRatio: src.connWarnRatio || undefined,
  enabled: src.enabled,
});

export default function DBMonitorPage() {
  const { message } = App.useApp();
  const access = useAccess();
  const canManage = Boolean(access.canOpsManage);
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;

  const [sources, setSources] = useState<DBSource[]>([]);
  const [results, setResults] = useState<Record<number, ProbeResult>>({});
  const [probing, setProbing] = useState(false);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<DBSource | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await listDBSources();
      setSources(res.items || []);
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.opsDBMonitor.error.loadFailed',
            defaultMessage: '加载数据源失败',
          }),
        ),
      );
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    load();
  }, [load]);

  const runProbe = async () => {
    setProbing(true);
    try {
      const res = await probeAll();
      const map: Record<number, ProbeResult> = {};
      for (const r of res.results || []) map[r.sourceId] = r;
      setResults(map);
      message.success(
        intl.formatMessage({ id: 'pages.opsDBMonitor.probe.success', defaultMessage: '探测完成' }),
      );
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.opsDBMonitor.error.probeFailed',
            defaultMessage: '探测失败',
          }),
        ),
      );
    } finally {
      setProbing(false);
    }
  };

  const openCreate = () => {
    setEditing(null);
    setModalOpen(true);
  };

  const openEdit = (src: DBSource) => {
    setEditing(src);
    setModalOpen(true);
  };

  const onFinish = async (v: DBSourceFormValues) => {
    try {
      if (editing) {
        await updateDBSource(editing.id, v);
        message.success(
          intl.formatMessage({
            id: 'pages.opsDBMonitor.source.updated',
            defaultMessage: '数据源已更新',
          }),
        );
      } else {
        // 新增时 DSN 必填校验已保证非空，`|| ''` 仅满足 payload 的类型收窄
        await createDBSource({ ...v, dsn: v.dsn || '' });
        message.success(
          intl.formatMessage({
            id: 'pages.opsDBMonitor.source.created',
            defaultMessage: '数据源已登记',
          }),
        );
      }
      load();
      return true;
    } catch (error) {
      // 原语义：提交失败本地 toast（校验失败由 ModalForm 内置拦截，不走到这里）
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.opsDBMonitor.error.saveFailed',
            defaultMessage: '保存失败',
          }),
        ),
      );
      return false;
    }
  };

  const remove = async (src: DBSource) => {
    try {
      await deleteDBSource(src.id);
      message.success(
        intl.formatMessage({ id: 'pages.opsDBMonitor.source.deleted', defaultMessage: '已删除' }),
      );
      load();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.opsDBMonitor.error.deleteFailed',
            defaultMessage: '删除失败',
          }),
        ),
      );
    }
  };

  return (
    <PageContainer>
      <Card
        title={
          <Space>
            <DatabaseOutlined />
            <FormattedMessage id="pages.opsDBMonitor.card.title" defaultMessage="数据库监控" />
          </Space>
        }
        extra={
          <Space wrap>
            <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
              <FormattedMessage id="pages.opsDBMonitor.action.refresh" defaultMessage="刷新" />
            </Button>
            <Button
              type="primary"
              icon={<ThunderboltOutlined />}
              onClick={runProbe}
              loading={probing}
            >
              <FormattedMessage id="pages.opsDBMonitor.action.probe" defaultMessage="立即探测" />
            </Button>
            {canManage ? (
              <Button onClick={openCreate}>
                <FormattedMessage
                  id="pages.opsDBMonitor.action.register"
                  defaultMessage="登记数据库"
                />
              </Button>
            ) : null}
          </Space>
        }
      >
        {sources.length === 0 ? (
          <Empty
            description={intl.formatMessage({
              id: 'pages.opsDBMonitor.empty.description',
              defaultMessage:
                '尚未登记游戏数据库。登记只读账号后即可在此查看连接/锁等待/死锁指标。',
            })}
          />
        ) : (
          <Row gutter={[12, 12]}>
            {sources.map((src) => {
              const r = results[src.id];
              return (
                <Col xs={24} md={12} xl={8} key={src.id}>
                  <Card
                    size="small"
                    title={
                      <Space>
                        <DatabaseOutlined />
                        {src.name}
                        {!src.enabled ? (
                          <Tag>
                            <FormattedMessage
                              id="pages.opsDBMonitor.source.disabledTag"
                              defaultMessage="已停用"
                            />
                          </Tag>
                        ) : null}
                      </Space>
                    }
                    extra={
                      canManage ? (
                        <Space size={4}>
                          <Button size="small" onClick={() => openEdit(src)}>
                            <FormattedMessage
                              id="pages.opsDBMonitor.action.edit"
                              defaultMessage="编辑"
                            />
                          </Button>
                          <Popconfirm
                            title={intl.formatMessage(
                              {
                                id: 'pages.opsDBMonitor.confirm.delete',
                                defaultMessage: `删除「${src.name}」？`,
                              },
                              { name: src.name },
                            )}
                            onConfirm={() => remove(src)}
                          >
                            <Button size="small" danger type="text">
                              <FormattedMessage
                                id="pages.opsDBMonitor.action.delete"
                                defaultMessage="删除"
                              />
                            </Button>
                          </Popconfirm>
                        </Space>
                      ) : undefined
                    }
                  >
                    <Space orientation="vertical" size={4} style={{ width: '100%' }}>
                      <Space wrap size={4}>
                        <Tag color="blue">{src.driver}</Tag>
                        <Tag>{dbKindLabels[src.kind as DBSourceKind] || src.kind}</Tag>
                        {src.gameId ? (
                          <Tag color="geekblue">
                            {src.gameId}/{src.env}
                          </Tag>
                        ) : (
                          <Tag>
                            <FormattedMessage
                              id="pages.opsDBMonitor.source.globalTag"
                              defaultMessage="全局"
                            />
                          </Tag>
                        )}
                      </Space>
                      {r === undefined ? (
                        <Text type="secondary">
                          <FormattedMessage
                            id="pages.opsDBMonitor.probe.hint"
                            defaultMessage="点击「立即探测」获取实时状态"
                          />
                        </Text>
                      ) : r.ok ? (
                        <>
                          <Space size={16} style={{ marginTop: 8 }}>
                            <div>
                              <Text type="secondary">
                                <FormattedMessage
                                  id="pages.opsDBMonitor.metric.connections"
                                  defaultMessage="连接"
                                />
                              </Text>
                              <br />
                              <Text strong>
                                <Text strong>{r.connections?.current ?? '-'}</Text>
                                <Text type="secondary">
                                  /
                                  {r.connections && r.connections.max > 0 ? r.connections.max : '?'}
                                </Text>
                              </Text>
                            </div>
                            <div>
                              <Text type="secondary">
                                <FormattedMessage
                                  id="pages.opsDBMonitor.metric.lockWaits"
                                  defaultMessage="锁等待"
                                />
                              </Text>
                              <br />
                              {(r.lockWaits?.length || 0) > 0 ? (
                                <Tag color="red">
                                  {intl.formatMessage(
                                    {
                                      id: 'pages.opsDBMonitor.metric.lockWaitCount',
                                      defaultMessage: `${r.lockWaits!.length} 条`,
                                    },
                                    { count: r.lockWaits!.length },
                                  )}
                                </Tag>
                              ) : (
                                <Tag color="green">0</Tag>
                              )}
                            </div>
                            <div>
                              <Text type="secondary">
                                <FormattedMessage
                                  id="pages.opsDBMonitor.metric.deadlocks"
                                  defaultMessage="死锁累计"
                                />
                              </Text>
                              <br />
                              {r.deadlockCount !== undefined && r.deadlockCount !== null ? (
                                r.deadlockCount > 0 ? (
                                  <Tag color="volcano">{r.deadlockCount}</Tag>
                                ) : (
                                  <Tag color="green">0</Tag>
                                )
                              ) : (
                                <Tooltip
                                  title={intl.formatMessage({
                                    id: 'pages.opsDBMonitor.metric.deadlocksUnavailableTip',
                                    defaultMessage: '该云 RDS 不暴露此计数，请到云控制台查看',
                                  })}
                                >
                                  <Text type="secondary">
                                    <FormattedMessage
                                      id="pages.opsDBMonitor.metric.deadlocksUnavailable"
                                      defaultMessage="不可用"
                                    />
                                  </Text>
                                </Tooltip>
                              )}
                            </div>
                            <div>
                              <Text type="secondary">
                                <FormattedMessage
                                  id="pages.opsDBMonitor.metric.latency"
                                  defaultMessage="延迟"
                                />
                              </Text>
                              <br />
                              <Text>{r.latencyMs ?? '-'}ms</Text>
                            </div>
                          </Space>
                          {(r.lockWaits?.length || 0) > 0 ? (
                            <Table
                              size="small"
                              rowKey={(lw) => lw.waitId + lw.blockedBy}
                              dataSource={r.lockWaits}
                              pagination={false}
                              columns={[
                                {
                                  title: intl.formatMessage({
                                    id: 'pages.opsDBMonitor.column.waiter',
                                    defaultMessage: '等待者',
                                  }),
                                  dataIndex: 'waitId',
                                  width: 80,
                                },
                                {
                                  title: intl.formatMessage({
                                    id: 'pages.opsDBMonitor.column.blockedBy',
                                    defaultMessage: '阻塞者',
                                  }),
                                  dataIndex: 'blockedBy',
                                  width: 80,
                                },
                                {
                                  title: intl.formatMessage({
                                    id: 'pages.opsDBMonitor.column.waitSecs',
                                    defaultMessage: '等待(s)',
                                  }),
                                  dataIndex: 'waitSecs',
                                  width: 80,
                                  render: (v: number) =>
                                    v > 30 ? <Tag color="red">{v}</Tag> : <span>{v}</span>,
                                },
                                {
                                  title: intl.formatMessage({
                                    id: 'pages.opsDBMonitor.column.query',
                                    defaultMessage: '查询',
                                  }),
                                  dataIndex: 'query',
                                  ellipsis: true,
                                },
                              ]}
                            />
                          ) : null}
                        </>
                      ) : (
                        <Text type="danger">
                          {intl.formatMessage(
                            {
                              id: 'pages.opsDBMonitor.probe.failReason',
                              defaultMessage: `探测失败：${r.error}`,
                            },
                            { error: r.error },
                          )}
                        </Text>
                      )}
                      <Text type="secondary" style={{ fontSize: 12 }} code>
                        {src.dsnMask}
                      </Text>
                    </Space>
                  </Card>
                </Col>
              );
            })}
          </Row>
        )}
      </Card>

      <ModalForm<DBSourceFormValues>
        title={
          editing
            ? intl.formatMessage(
                {
                  id: 'pages.opsDBMonitor.modal.editTitle',
                  defaultMessage: `编辑数据源：${editing.name}`,
                },
                { name: editing.name },
              )
            : intl.formatMessage({
                id: 'pages.opsDBMonitor.modal.createTitle',
                defaultMessage: '登记游戏数据库',
              })
        }
        open={modalOpen}
        onOpenChange={setModalOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        // 原 footer 自定义按钮文案为「保存」，保持不变
        submitter={{
          searchConfig: {
            submitText: intl.formatMessage({
              id: 'pages.opsDBMonitor.modal.submit',
              defaultMessage: '保存',
            }),
          },
        }}
        layout="vertical"
        // destroyOnHidden 使弹窗每次关闭即卸载表单，重开时按最新 initialValues
        // 重新挂载：编辑回填 DSN 留空，新增回到 Form.Item 默认值（self/启用）
        initialValues={editing ? toDBSourceFormValues(editing) : undefined}
        onFinish={onFinish}
      >
        <Form.Item
          name="name"
          label={intl.formatMessage({
            id: 'pages.opsDBMonitor.form.name.label',
            defaultMessage: '名称',
          })}
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'pages.opsDBMonitor.form.name.required',
                defaultMessage: '请输入名称',
              }),
            },
          ]}
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.opsDBMonitor.form.name.placeholder',
              defaultMessage: '如 游戏主库-prod',
            })}
          />
        </Form.Item>
        <Space>
          <Form.Item
            name="driver"
            label={intl.formatMessage({
              id: 'pages.opsDBMonitor.form.driver.label',
              defaultMessage: '驱动',
            })}
            rules={[{ required: true }]}
          >
            <Select
              style={{ width: 120 }}
              options={[
                { label: 'MySQL', value: 'mysql' },
                { label: 'PostgreSQL', value: 'postgres' },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="kind"
            label={intl.formatMessage({
              id: 'pages.opsDBMonitor.form.kind.label',
              defaultMessage: '部署类型',
            })}
            initialValue="self"
          >
            <Select
              style={{ width: 130 }}
              options={Object.entries(dbKindLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
          </Form.Item>
          <Form.Item
            name="enabled"
            label={intl.formatMessage({
              id: 'pages.opsDBMonitor.form.enabled.label',
              defaultMessage: '启用',
            })}
            valuePropName="checked"
            initialValue={true}
          >
            <Switch />
          </Form.Item>
        </Space>
        <Form.Item
          name="dsn"
          label={intl.formatMessage({
            id: 'pages.opsDBMonitor.form.dsn.label',
            defaultMessage: '只读账号 DSN',
          })}
          extra={
            editing
              ? intl.formatMessage({
                  id: 'pages.opsDBMonitor.form.dsn.extraEdit',
                  defaultMessage: '编辑时留空表示不修改。务必使用只读监控账号，禁止 root/superuser',
                })
              : intl.formatMessage({
                  id: 'pages.opsDBMonitor.form.dsn.extraCreate',
                  defaultMessage: '务必使用只读监控账号，禁止 root/superuser',
                })
          }
          rules={
            editing
              ? []
              : [
                  {
                    required: true,
                    message: intl.formatMessage({
                      id: 'pages.opsDBMonitor.form.dsn.required',
                      defaultMessage: '请输入 DSN',
                    }),
                  },
                ]
          }
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.opsDBMonitor.form.dsn.placeholder',
              defaultMessage:
                'readonly:pass@tcp(10.0.0.1:3306)/game 或 postgres://ro:pass@10.0.0.2/game',
            })}
          />
        </Form.Item>
        <Space>
          <Form.Item
            name="gameId"
            label={intl.formatMessage({
              id: 'pages.opsDBMonitor.form.gameId.label',
              defaultMessage: 'gameId(可选)',
            })}
          >
            <Input
              style={{ width: 140 }}
              placeholder={intl.formatMessage({
                id: 'pages.opsDBMonitor.form.gameId.placeholder',
                defaultMessage: '归属游戏',
              })}
            />
          </Form.Item>
          <Form.Item
            name="env"
            label={intl.formatMessage({
              id: 'pages.opsDBMonitor.form.env.label',
              defaultMessage: 'env(可选)',
            })}
          >
            <Input style={{ width: 120 }} placeholder="prod" />
          </Form.Item>
        </Space>
        <Row gutter={12}>
          <Col span={12}>
            <Form.Item
              name="lockWaitWarn"
              label={intl.formatMessage({
                id: 'pages.opsDBMonitor.form.lockWaitWarn.label',
                defaultMessage: '锁等待告警阈值',
              })}
              extra={intl.formatMessage({
                id: 'pages.opsDBMonitor.form.lockWaitWarn.extra',
                defaultMessage: '条数，默认 5',
              })}
            >
              <InputNumber min={1} max={1000} style={{ width: '100%' }} placeholder="5" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="connWarnRatio"
              label={intl.formatMessage({
                id: 'pages.opsDBMonitor.form.connWarnRatio.label',
                defaultMessage: '连接水位告警',
              })}
              extra={intl.formatMessage({
                id: 'pages.opsDBMonitor.form.connWarnRatio.extra',
                defaultMessage: '百分比，默认 80',
              })}
            >
              <InputNumber min={10} max={100} style={{ width: '100%' }} placeholder="80" />
            </Form.Item>
          </Col>
        </Row>
      </ModalForm>
    </PageContainer>
  );
}
