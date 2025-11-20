import React, { useEffect, useState } from 'react';
import { Card, Space, DatePicker, Select, Button, Table } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { exportToXLSX } from '@/utils/export';
import { fetchAnalyticsRetention } from '@/services/croupier/analytics';

export default function AnalyticsRetentionPage() {
  const [loading, setLoading] = useState(false);
  const [range, setRange] = useState<any>(null);
  const [cohort, setCohort] = useState<'signup'|'first_active'>('signup');
  const [data, setData] = useState<any>({ cohorts: [] });

  const load = async () => {
    setLoading(true);
    try {
      const params: any = { cohort };
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsRetention(params);
      setData(r || { cohorts: [] });
    } finally { setLoading(false); }
  };
  useEffect(()=>{ load(); }, []);

  // 展示简单表格：cohort_date, total, d1, d7, d30（占位）
  const rowsData = (data?.cohorts || []).map((c:any, idx:number)=>({ key: idx, cohort_date: c.date, total: c.total, d1:c.d1, d7:c.d7, d30:c.d30 }));

  return (
    <PageContainer>
      <Card title="留存分析" extra={
        <Space>
          <Select value={cohort} onChange={setCohort as any} options={[{label:'按注册',value:'signup'},{label:'按首次活跃',value:'first_active'}]} />
          <DatePicker.RangePicker value={range as any} onChange={setRange as any} />
          <Button type="primary" onClick={load}>查询</Button>
          <Button onClick={async ()=>{
            const rows = [['cohort_date','total','d1','d7','d30']].concat(rowsData.map(r=>[r.cohort_date,r.total,r.d1,r.d7,r.d30]));
            await exportToXLSX('retention.xlsx', [{ sheet:'retention', rows }]);
          }}>导出 Excel</Button>
        </Space>
      }>
        <Table loading={loading} dataSource={rowsData} pagination={{ pageSize: 10 }}
          columns={[
            { title:'Cohort 日期', dataIndex:'cohort_date' },
            { title:'基数', dataIndex:'total' },
            { title:'D1', dataIndex:'d1', render:(v)=> v!=null? `${v}%`:'-' },
            { title:'D7', dataIndex:'d7', render:(v)=> v!=null? `${v}%`:'-' },
            { title:'D30', dataIndex:'d30', render:(v)=> v!=null? `${v}%`:'-' },
          ]}
        />
      </Card>
    </PageContainer>
  );
}
