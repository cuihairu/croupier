import React, { useEffect, useMemo, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Row,
  Space,
  Statistic,
  Table,
  Tag,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import {
  estimateCost,
  getCostBudget,
  getCostSummary,
  saveCostBudget,
  type CostEstimate,
  type CostSummary,
} from '@/services/api/ops';

type EstimateForm = {
  provider: string;
  instanceType: string;
  count: number;
  hours?: number;
  diskSizeGB?: number;
};
type BudgetForm = { monthlyUsd: number; notifyChannels?: string };

export default function OpsCostPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const [summary, setSummary] = useState<CostSummary | null>(null);
  const [estimate, setEstimate] = useState<CostEstimate | null>(null);
  const [budgetMonthlyUsd, setBudgetMonthlyUsd] = useState<number | undefined>();
  const [budgetChannels, setBudgetChannels] = useState<string>('');
  const [loadingSummary, setLoadingSummary] = useState(false);
  const [estimating, setEstimating] = useState(false);
  const [savingBudget, setSavingBudget] = useState(false);
  const [form] = Form.useForm<EstimateForm>();
  const [budgetForm] = Form.useForm<BudgetForm>();

  const load = async () => {
    setLoadingSummary(true);
    try {
      const [s, b] = await Promise.all([getCostSummary(), getCostBudget()]);
      setSummary(s);
      if (b) {
        setBudgetMonthlyUsd(b.monthlyUsd);
        setBudgetChannels((b.notifyChannels || []).join(','));
        budgetForm.setFieldsValue({
          monthlyUsd: b.monthlyUsd,
          notifyChannels: (b.notifyChannels || []).join(','),
        });
      }
    } catch (e) {
      message.error(e instanceof Error ? e.message : '加载成本数据失败');
    } finally {
      setLoadingSummary(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const doEstimate = async (values: EstimateForm) => {
    setEstimating(true);
    try {
      const result = await estimateCost(values);
      setEstimate(result);
    } catch (e) {
      message.error(e instanceof Error ? e.message : '估算失败');
    } finally {
      setEstimating(false);
    }
  };

  const saveBudget = async (values: BudgetForm) => {
    setSavingBudget(true);
    try {
      const channels = (values.notifyChannels || '')
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean);
      await saveCostBudget({ monthlyUsd: values.monthlyUsd, notifyChannels: channels });
      message.success('预算已保存');
    } catch (e) {
      message.error(e instanceof Error ? e.message : '保存预算失败');
    } finally {
      setSavingBudget(false);
    }
  };

  const columns = useMemo(
    () => [
      { title: 'Agent', dataIndex: 'agentId', key: 'agentId' },
      { title: 'Provider', dataIndex: 'provider', key: 'provider' },
      { title: '规格', dataIndex: 'instanceType', key: 'instanceType' },
      { title: '地域', dataIndex: 'region', key: 'region' },
      {
        title: '运行时长(h)',
        dataIndex: 'uptimeHours',
        key: 'uptimeHours',
        render: (v: number) => v.toFixed(1),
      },
      {
        title: '成本(USD)',
        dataIndex: 'costUsd',
        key: 'costUsd',
        render: (v: number) => `$${v.toFixed(4)}`,
      },
      {
        title: '闲置',
        dataIndex: 'idle',
        key: 'idle',
        render: (v: boolean) => (v ? <Tag color="orange">闲置</Tag> : <Tag>活跃</Tag>),
      },
    ],
    [],
  );

  const providerCards = useMemo(() => {
    if (!summary?.perProvider) return [];
    return Object.entries(summary.perProvider).map(([provider, usd]) => ({ provider, usd }));
  }, [summary]);

  return (
    <PageContainer>
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <Card title="创建前估算" bordered={false}>
          <Form
            form={form}
            layout="inline"
            onFinish={doEstimate}
            initialValues={{
              provider: 'hetzner',
              instanceType: 'cx21',
              count: 1,
              hours: 1,
              diskSizeGB: 40,
            }}
          >
            <Form.Item name="provider" rules={[{ required: true }]}>
              <Input placeholder="Provider" />
            </Form.Item>
            <Form.Item name="instanceType" rules={[{ required: true }]}>
              <Input placeholder="规格" />
            </Form.Item>
            <Form.Item name="count" rules={[{ required: true }]}>
              <InputNumber min={1} />
            </Form.Item>
            <Form.Item name="hours">
              <InputNumber min={0.1} step={0.5} placeholder="小时" />
            </Form.Item>
            <Form.Item name="diskSizeGB">
              <InputNumber min={0} placeholder="磁盘 GB" />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={estimating}>
              估算
            </Button>
          </Form>
          {estimate && (
            <Descriptions bordered size="small" style={{ marginTop: 16 }}>
              <Descriptions.Item label="规格">
                {estimate.provider}/{estimate.instanceType}
              </Descriptions.Item>
              <Descriptions.Item label="小时费率">
                ${estimate.ratePerHour.toFixed(4)}
              </Descriptions.Item>
              <Descriptions.Item label="总成本">${estimate.totalUsd.toFixed(4)}</Descriptions.Item>
              <Descriptions.Item label="价格来源">
                {estimate.knownRate ? (
                  <Tag color="green">内置目录</Tag>
                ) : (
                  <Tag color="red">未知（未计价）</Tag>
                )}
              </Descriptions.Item>
            </Descriptions>
          )}
        </Card>

        <Card title="成本总览" bordered={false} loading={loadingSummary}>
          <Row gutter={16}>
            <Col span={6}>
              <Statistic title="节点数" value={summary?.totalNodes ?? 0} />
            </Col>
            <Col span={6}>
              <Statistic title="总成本(USD)" value={summary?.totalUsd ?? 0} precision={4} />
            </Col>
            <Col span={6}>
              <Statistic title="闲置节点" value={summary?.idleCount ?? 0} />
            </Col>
            <Col span={6}>
              <Statistic title="预算(USD/月)" value={budgetMonthlyUsd ?? 0} precision={2} />
            </Col>
          </Row>
          {providerCards.length > 0 && (
            <Row gutter={16} style={{ marginTop: 16 }}>
              {providerCards.map((p) => (
                <Col key={p.provider} span={6}>
                  <Statistic title={p.provider} value={p.usd} precision={4} prefix="$" />
                </Col>
              ))}
            </Row>
          )}
          <Table
            rowKey="agentId"
            columns={columns}
            dataSource={summary?.nodes || []}
            pagination={{ pageSize: 10 }}
            style={{ marginTop: 16 }}
          />
        </Card>

        <Card title="月度预算" bordered={false}>
          <Form
            form={budgetForm}
            layout="inline"
            onFinish={saveBudget}
            initialValues={{ monthlyUsd: 1000, notifyChannels: '' }}
          >
            <Form.Item name="monthlyUsd" label="预算(USD)" rules={[{ required: true }]}>
              <InputNumber min={0} step={100} />
            </Form.Item>
            <Form.Item name="notifyChannels" label="通知渠道">
              <Input placeholder="ding_main,webhook" />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={savingBudget}>
              保存
            </Button>
          </Form>
        </Card>
      </Space>
    </PageContainer>
  );
}
