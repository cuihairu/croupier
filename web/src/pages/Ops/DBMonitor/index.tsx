import React, { useCallback, useEffect, useState } from 'react';
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
  Modal,
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
import { PageContainer } from '@ant-design/pro-components';
import {
  DatabaseOutlined,
  PlusOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { useAccess } from '@umijs/max';
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

export default function DBMonitorPage() {
  const { message } = App.useApp();
  const access = useAccess();
  const canManage = Boolean(access.canOpsManage);

  const [sources, setSources] = useState<DBSource[]>([]);
  const [results, setResults] = useState<Record<number, ProbeResult>>({});
  const [probing, setProbing] = useState(false);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<DBSource | null>(null);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await listDBSources();
      setSources(res.items || []);
    } catch (error) {
      message.error(extractErrorMessage(error, '加载数据源失败'));
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
      message.success('探测完成');
    } catch (error) {
      message.error(extractErrorMessage(error, '探测失败'));
    } finally {
      setProbing(false);
    }
  };

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    setModalOpen(true);
  };

  const openEdit = (src: DBSource) => {
    setEditing(src);
    form.setFieldsValue({
      name: src.name,
      driver: src.driver,
      kind: src.kind,
      gameId: src.gameId,
      env: src.env,
      lockWaitWarn: src.lockWaitWarn || undefined,
      connWarnRatio: src.connWarnRatio || undefined,
      enabled: src.enabled,
      // DSN 不回填（掩码存储），留空表示不修改
    });
    setModalOpen(true);
  };

  const submit = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      if (editing) {
        await updateDBSource(editing.id, v);
        message.success('数据源已更新');
      } else {
        await createDBSource(v);
        message.success('数据源已登记');
      }
      setModalOpen(false);
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '保存失败'));
    } finally {
      setSaving(false);
    }
  };

  const remove = async (src: DBSource) => {
    try {
      await deleteDBSource(src.id);
      message.success('已删除');
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '删除失败'));
    }
  };

  return (
    <PageContainer>
      <Card
        title={
          <Space>
            <DatabaseOutlined />
            数据库监控
          </Space>
        }
        extra={
          <Space wrap>
            <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
              刷新
            </Button>
            <Button
              type="primary"
              icon={<ThunderboltOutlined />}
              onClick={runProbe}
              loading={probing}
            >
              立即探测
            </Button>
            {canManage ? <Button onClick={openCreate}>登记数据库</Button> : null}
          </Space>
        }
      >
        {sources.length === 0 ? (
          <Empty description="尚未登记游戏数据库。登记只读账号后即可在此查看连接/锁等待/死锁指标。" />
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
                        {!src.enabled ? <Tag>已停用</Tag> : null}
                      </Space>
                    }
                    extra={
                      canManage ? (
                        <Space size={4}>
                          <Button size="small" onClick={() => openEdit(src)}>
                            编辑
                          </Button>
                          <Popconfirm title={`删除「${src.name}」？`} onConfirm={() => remove(src)}>
                            <Button size="small" danger type="text">
                              删除
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
                          <Tag>全局</Tag>
                        )}
                      </Space>
                      {r === undefined ? (
                        <Text type="secondary">点击「立即探测」获取实时状态</Text>
                      ) : r.ok ? (
                        <>
                          <Space size={16} style={{ marginTop: 8 }}>
                            <div>
                              <Text type="secondary">连接</Text>
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
                              <Text type="secondary">锁等待</Text>
                              <br />
                              {(r.lockWaits?.length || 0) > 0 ? (
                                <Tag color="red">{r.lockWaits!.length} 条</Tag>
                              ) : (
                                <Tag color="green">0</Tag>
                              )}
                            </div>
                            <div>
                              <Text type="secondary">死锁累计</Text>
                              <br />
                              {r.deadlockCount !== undefined && r.deadlockCount !== null ? (
                                r.deadlockCount > 0 ? (
                                  <Tag color="volcano">{r.deadlockCount}</Tag>
                                ) : (
                                  <Tag color="green">0</Tag>
                                )
                              ) : (
                                <Tooltip title="该云 RDS 不暴露此计数，请到云控制台查看">
                                  <Text type="secondary">不可用</Text>
                                </Tooltip>
                              )}
                            </div>
                            <div>
                              <Text type="secondary">延迟</Text>
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
                                { title: '等待者', dataIndex: 'waitId', width: 80 },
                                { title: '阻塞者', dataIndex: 'blockedBy', width: 80 },
                                {
                                  title: '等待(s)',
                                  dataIndex: 'waitSecs',
                                  width: 80,
                                  render: (v: number) =>
                                    v > 30 ? <Tag color="red">{v}</Tag> : <span>{v}</span>,
                                },
                                { title: '查询', dataIndex: 'query', ellipsis: true },
                              ]}
                            />
                          ) : null}
                        </>
                      ) : (
                        <Text type="danger">探测失败：{r.error}</Text>
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

      <Modal
        title={editing ? `编辑数据源：${editing.name}` : '登记游戏数据库'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        footer={
          <Space>
            <Button onClick={() => setModalOpen(false)}>取消</Button>
            <Button type="primary" loading={saving} onClick={submit}>
              保存
            </Button>
          </Space>
        }
        destroyOnHidden
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="如 游戏主库-prod" />
          </Form.Item>
          <Space>
            <Form.Item name="driver" label="驱动" rules={[{ required: true }]}>
              <Select
                style={{ width: 120 }}
                options={[
                  { label: 'MySQL', value: 'mysql' },
                  { label: 'PostgreSQL', value: 'postgres' },
                ]}
              />
            </Form.Item>
            <Form.Item name="kind" label="部署类型" initialValue="self">
              <Select
                style={{ width: 130 }}
                options={Object.entries(dbKindLabels).map(([value, label]) => ({
                  label,
                  value,
                }))}
              />
            </Form.Item>
            <Form.Item name="enabled" label="启用" valuePropName="checked" initialValue={true}>
              <Switch />
            </Form.Item>
          </Space>
          <Form.Item
            name="dsn"
            label="只读账号 DSN"
            extra={
              editing
                ? '编辑时留空表示不修改。务必使用只读监控账号，禁止 root/superuser'
                : '务必使用只读监控账号，禁止 root/superuser'
            }
            rules={editing ? [] : [{ required: true, message: '请输入 DSN' }]}
          >
            <Input placeholder="readonly:pass@tcp(10.0.0.1:3306)/game 或 postgres://ro:pass@10.0.0.2/game" />
          </Form.Item>
          <Space>
            <Form.Item name="gameId" label="gameId(可选)">
              <Input style={{ width: 140 }} placeholder="归属游戏" />
            </Form.Item>
            <Form.Item name="env" label="env(可选)">
              <Input style={{ width: 120 }} placeholder="prod" />
            </Form.Item>
          </Space>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item name="lockWaitWarn" label="锁等待告警阈值" extra="条数，默认 5">
                <InputNumber min={1} max={1000} style={{ width: '100%' }} placeholder="5" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="connWarnRatio" label="连接水位告警" extra="百分比，默认 80">
                <InputNumber min={10} max={100} style={{ width: '100%' }} placeholder="80" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>
    </PageContainer>
  );
}
