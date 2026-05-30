import React, { useEffect, useState } from 'react';
import { Alert, Button, Card, Col, Row, Statistic, Table, Tag } from 'antd';
import { BarChartOutlined } from '@ant-design/icons';
import { history } from '@umijs/max';
import {
  getFunctionAnalytics,
  getFunctionHistory,
  listFunctionWarnings,
} from '@/services/api/functions';

type HistoryRecord = {
  id: string;
  action: string;
  operator?: string;
  timestamp: string;
  details?: any;
};

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
  const [historyData, setHistoryData] = useState<HistoryRecord[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);

  useEffect(() => {
    const loadHistory = async () => {
      setHistoryLoading(true);
      try {
        const data = await getFunctionHistory(functionId);
        setHistoryData(data || []);
      } catch {
        setHistoryData([]);
      } finally {
        setHistoryLoading(false);
      }
    };
    loadHistory();
  }, [functionId]);

  return (
    <Table
      loading={historyLoading}
      dataSource={historyData}
      rowKey="id"
      columns={[
        { title: '操作', dataIndex: 'action', width: 150 },
        { title: '操作人', dataIndex: 'operator', width: 120 },
        {
          title: '时间',
          dataIndex: 'timestamp',
          width: 180,
          render: (text: string) => formatDateTime(text),
        },
        { title: '详情', dataIndex: 'details', ellipsis: true },
      ]}
      pagination={{ pageSize: 10 }}
    />
  );
}

export function AnalyticsTab({ functionId }: { functionId: string }) {
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
        <Card loading={analyticsLoading}>
          <Statistic
            title="总调用次数"
            value={analyticsData?.totalCalls || 0}
            prefix={<BarChartOutlined />}
          />
        </Card>
      </Col>
      <Col span={6}>
        <Card loading={analyticsLoading}>
          <Statistic
            title="成功率"
            value={analyticsData?.successRate || 0}
            suffix="%"
            precision={2}
            valueStyle={{ color: (analyticsData?.successRate || 0) >= 95 ? '#3f8600' : '#cf1322' }}
          />
        </Card>
      </Col>
      <Col span={6}>
        <Card loading={analyticsLoading}>
          <Statistic
            title="平均延迟"
            value={analyticsData?.avgLatency || 0}
            suffix="ms"
            precision={0}
          />
        </Card>
      </Col>
      <Col span={6}>
        <Card loading={analyticsLoading}>
          <Statistic title="今日调用" value={analyticsData?.callsToday || 0} />
        </Card>
      </Col>
    </Row>
  );
}

export function WarningsTab({ functionId }: { functionId: string }) {
  const [warningsData, setWarningsData] = useState<any[]>([]);
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
        message="注册告警"
        description="这里显示函数注册校验告警（例如 function_id 格式错误、版本号不合法、重复注册去重）。"
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        action={
          <Button
            size="small"
            onClick={() =>
              history.push(
                `/system/functions/warnings?function_id=${encodeURIComponent(functionId)}`,
              )
            }
          >
            查看全部
          </Button>
        }
      />
      <Table
        loading={warningsLoading}
        dataSource={warningsData}
        rowKey="key"
        columns={[
          {
            title: '代码',
            dataIndex: 'code',
            width: 180,
            render: (code: string) => <Tag color="orange">{code || '-'}</Tag>,
          },
          { title: '版本', dataIndex: 'version', width: 120, render: (v: string) => v || '-' },
          { title: '次数', dataIndex: 'count', width: 90 },
          {
            title: '最近时间',
            dataIndex: 'lastSeen',
            width: 180,
            render: (text: string) => formatDateTime(text),
          },
          { title: 'Agent', dataIndex: 'agentId', width: 220, ellipsis: true },
          { title: '详情', dataIndex: 'message', ellipsis: true },
        ]}
        pagination={{ pageSize: 10 }}
      />
    </>
  );
}
