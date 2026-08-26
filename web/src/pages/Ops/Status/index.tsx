import React, { useCallback, useEffect, useState } from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Form,
  Input,
  Popconfirm,
  Row,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { PlayCircleOutlined, ReloadOutlined, SafetyOutlined } from '@ant-design/icons';
import {
  getOpsHealth,
  getOpsMaintenance,
  getOpsMQ,
  getOpsServices,
  runOpsHealthCheck,
  updateOpsHealth,
  updateOpsMaintenance,
  type HealthCheck,
  type HealthRunResult,
  type OpsServiceItem,
} from '@/services/api/opsStatus';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

export default function OpsStatusPage() {
  const { message } = App.useApp();

  const [checks, setChecks] = useState<HealthCheck[]>([]);
  const [running, setRunning] = useState<Record<string, HealthRunResult>>({});
  const [loading, setLoading] = useState(false);
  const [services, setServices] = useState<OpsServiceItem[]>([]);
  const [mqStreams, setMqStreams] = useState<Array<{ name: string; length: number }>>([]);
  const [maintForm] = Form.useForm();
  const [maintSaving, setMaintSaving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [h, s, mq] = await Promise.all([getOpsHealth(), getOpsServices(), getOpsMQ()]);
      setChecks(h.checks);
      setServices(s);
      setMqStreams(Object.entries(mq?.lengths || {}).map(([name, length]) => ({ name, length })));
    } catch (error) {
      message.error(extractErrorMessage(error, '加载状态失败'));
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    load();
    (async () => {
      try {
        const m = await getOpsMaintenance();
        maintForm.setFieldsValue({
          enabled: Boolean(m.enabled),
          message: m.message || '',
          allowAdmins: m.allowAdmins !== false,
        });
      } catch {
        /* maintenance config is best-effort */
      }
    })();
  }, [load, maintForm]);

  const runCheck = async (id: string) => {
    try {
      const r = await runOpsHealthCheck(id);
      setRunning((prev) => ({ ...prev, [id]: r }));
      message[r.ok ? 'success' : 'error'](
        r.ok ? `${id} 正常（${r.latencyMs}ms）` : `${id} 异常：${r.error || '失败'}`,
      );
    } catch (error) {
      message.error(extractErrorMessage(error, '执行失败'));
    }
  };

  const toggleCheck = async (check: HealthCheck, enabled: boolean) => {
    try {
      const next = checks.map((c) => (c.id === check.id ? { ...c, enabled } : c));
      await updateOpsHealth({ enabled: true, checks: next });
      setChecks(next);
    } catch (error) {
      message.error(extractErrorMessage(error, '更新失败'));
    }
  };

  const saveMaintenance = async () => {
    const v = await maintForm.validateFields();
    setMaintSaving(true);
    try {
      await updateOpsMaintenance(v);
      message.success('维护模式已更新');
    } catch (error) {
      message.error(extractErrorMessage(error, '更新失败'));
    } finally {
      setMaintSaving(false);
    }
  };

  return (
    <PageContainer>
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={14}>
          <Card
            title="健康检查"
            extra={
              <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
                刷新
              </Button>
            }
          >
            <Table<HealthCheck>
              rowKey="id"
              size="small"
              dataSource={checks}
              pagination={false}
              locale={{ emptyText: '未配置健康检查项' }}
              columns={[
                { title: 'ID', dataIndex: 'id', width: 140 },
                { title: '名称', dataIndex: 'name' },
                { title: '类型', dataIndex: 'kind', width: 90 },
                { title: '目标', dataIndex: 'target', ellipsis: true },
                {
                  title: '启用',
                  dataIndex: 'enabled',
                  width: 70,
                  render: (v: boolean, c) => (
                    <Switch size="small" checked={v} onChange={(n) => toggleCheck(c, n)} />
                  ),
                },
                {
                  title: '操作',
                  width: 130,
                  render: (_: unknown, c) => {
                    const r = running[c.id];
                    return (
                      <Space>
                        <Button
                          size="small"
                          icon={<PlayCircleOutlined />}
                          disabled={!c.enabled}
                          onClick={() => runCheck(c.id)}
                        >
                          执行
                        </Button>
                        {r ? (
                          <Tag color={r.ok ? 'green' : 'red'}>
                            {r.ok ? `${r.latencyMs}ms` : '异常'}
                          </Tag>
                        ) : null}
                      </Space>
                    );
                  },
                },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} lg={10}>
          <Card
            title={
              <Space>
                <SafetyOutlined />
                维护模式
              </Space>
            }
          >
            <Form form={maintForm} layout="vertical" initialValues={{ allowAdmins: true }}>
              <Form.Item name="enabled" label="开启维护模式" valuePropName="checked">
                <Switch />
              </Form.Item>
              <Form.Item name="message" label="维护公告内容">
                <Input.TextArea rows={2} placeholder="系统维护中，预计 30 分钟" />
              </Form.Item>
              <Form.Item name="allowAdmins" label="管理员仍可访问" valuePropName="checked">
                <Switch />
              </Form.Item>
              <Popconfirm title="确认更新维护模式？" onConfirm={saveMaintenance}>
                <Button type="primary" loading={maintSaving}>
                  保存
                </Button>
              </Popconfirm>
            </Form>
          </Card>
        </Col>
        <Col xs={24} lg={14}>
          <Card title="服务状态">
            <Table<OpsServiceItem>
              rowKey="name"
              size="small"
              dataSource={services}
              pagination={false}
              locale={{ emptyText: '暂无服务数据' }}
              columns={[
                { title: '服务', dataIndex: 'name' },
                {
                  title: '状态',
                  dataIndex: 'status',
                  width: 90,
                  render: (v: string) => (
                    <Tag color={v === 'up' || v === 'healthy' ? 'green' : v ? 'red' : 'default'}>
                      {v || '-'}
                    </Tag>
                  ),
                },
                { title: '地址', dataIndex: 'addr', ellipsis: true },
                { title: '版本', dataIndex: 'version', width: 100 },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} lg={10}>
          <Card title="消息队列">
            {mqStreams.length === 0 ? (
              <Text type="secondary">暂无队列数据</Text>
            ) : (
              <Table
                rowKey="name"
                size="small"
                dataSource={mqStreams}
                pagination={false}
                columns={[
                  { title: '流', dataIndex: 'name' },
                  {
                    title: '积压',
                    dataIndex: 'length',
                    width: 100,
                    render: (v: number) =>
                      v > 10000 ? <Tag color="red">{v}</Tag> : <Tag>{v}</Tag>,
                  },
                ]}
              />
            )}
          </Card>
        </Col>
      </Row>
    </PageContainer>
  );
}
