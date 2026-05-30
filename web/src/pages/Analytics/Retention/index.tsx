import React, { useEffect, useState } from 'react';
import { Card, Space, DatePicker, Select, Button, Table } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { exportToXLSX } from '@/utils/export';
import { fetchAnalyticsRetention } from '@/services/api/analytics';

export default function AnalyticsRetentionPage() {
  const [loading, setLoading] = useState(false);
  const [range, setRange] = useState<any>(null);
  const [cohort, setCohort] = useState<'signup' | 'first_active'>('signup');
  const [data, setData] = useState<any>({ cohorts: [] });

  const load = async () => {
    setLoading(true);
    try {
      const params: any = { cohort };
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsRetention(params);
      setData(r || { cohorts: [] });
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    load();
  }, []);

  // 处理后端返回的数据结构
  const rowsData = (data?.cohorts || []).map((c: any, idx: number) => {
    // retention 数组格式: [Day1, Day3, Day7, Day14, Day30]
    const retention = c.retention || [];
    return {
      key: idx,
      cohort: c.cohort,
      users: c.users,
      d1: retention[0] || null,
      d3: retention[1] || null,
      d7: retention[2] || null,
      d14: retention[3] || null,
      d30: retention[4] || null,
    };
  });

  return (
    <PageContainer>
      <Card
        title="留存分析"
        extra={
          <Space>
            <Select
              value={cohort}
              onChange={setCohort as any}
              options={[
                { label: '按注册', value: 'signup' },
                { label: '按首次活跃', value: 'first_active' },
              ]}
            />
            <DatePicker.RangePicker value={range as any} onChange={setRange as any} />
            <Button type="primary" onClick={load}>
              查询
            </Button>
            <Button
              onClick={async () => {
                const rows = [['cohort', 'users', 'd1', 'd3', 'd7', 'd14', 'd30']].concat(
                  rowsData.map((r) => [r.cohort, r.users, r.d1, r.d3, r.d7, r.d14, r.d30]),
                );
                await exportToXLSX('retention.csv', [{ sheet: 'retention', rows }]);
              }}
            >
              导出 CSV
            </Button>
          </Space>
        }
      >
        <Table
          loading={loading}
          dataSource={rowsData}
          pagination={{ pageSize: 10 }}
          columns={[
            { title: 'Cohort', dataIndex: 'cohort', key: 'cohort' },
            {
              title: '用户数',
              dataIndex: 'users',
              key: 'users',
              render: (v: number) => v?.toLocaleString() || 0,
            },
            {
              title: 'D1',
              dataIndex: 'd1',
              key: 'd1',
              render: (v: number) => (v != null ? `${(v * 100).toFixed(2)}%` : '-'),
              sorter: (a: any, b: any) => (a.d1 || 0) - (b.d1 || 0),
            },
            {
              title: 'D3',
              dataIndex: 'd3',
              key: 'd3',
              render: (v: number) => (v != null ? `${(v * 100).toFixed(2)}%` : '-'),
              sorter: (a: any, b: any) => (a.d3 || 0) - (b.d3 || 0),
            },
            {
              title: 'D7',
              dataIndex: 'd7',
              key: 'd7',
              render: (v: number) => (v != null ? `${(v * 100).toFixed(2)}%` : '-'),
              sorter: (a: any, b: any) => (a.d7 || 0) - (b.d7 || 0),
            },
            {
              title: 'D14',
              dataIndex: 'd14',
              key: 'd14',
              render: (v: number) => (v != null ? `${(v * 100).toFixed(2)}%` : '-'),
              sorter: (a: any, b: any) => (a.d14 || 0) - (b.d14 || 0),
            },
            {
              title: 'D30',
              dataIndex: 'd30',
              key: 'd30',
              render: (v: number) => (v != null ? `${(v * 100).toFixed(2)}%` : '-'),
              sorter: (a: any, b: any) => (a.d30 || 0) - (b.d30 || 0),
            },
          ]}
        />
      </Card>
    </PageContainer>
  );
}
