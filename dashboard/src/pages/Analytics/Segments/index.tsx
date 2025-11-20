import React, { useEffect, useState } from 'react';
import { Card, Space, DatePicker, Select, Button, Table, Tag } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { fetchAnalyticsSegments } from '@/services/croupier/analytics';

export default function AnalyticsSegmentsPage() {
  const [loading, setLoading] = useState(false);
  const [range, setRange] = useState<any>(null);
  const [breakdown, setBreakdown] = useState<string>('rfm');
  const [data, setData] = useState<any>({ segments: [] });

  const load = async () => {
    setLoading(true);
    try {
      const params: any = { breakdown };
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsSegments(params);
      setData(r||{ segments: [] });
    } finally { setLoading(false); }
  };
  useEffect(()=>{ /* not auto-load */ }, []);

  return (
    <PageContainer>
      <Card title="人群分层" extra={<Space>
        <Select value={breakdown} onChange={setBreakdown as any} style={{ width: 180 }} options={[{label:'RFM 分层',value:'rfm'},{label:'留存×付费',value:'retention_pay'},{label:'国家/平台/渠道',value:'geo_platform_channel'}]} />
        <DatePicker.RangePicker value={range as any} onChange={setRange as any} />
        <Button type="primary" onClick={load}>查询</Button>
      </Space>}>
        <Table size="small" loading={loading} rowKey={(r:any)=> String(r.name ?? r.segment ?? '')}
          dataSource={data?.segments||[]}
          columns={[{title:'Segment',dataIndex:'name',render:(v:any)=> <Tag>{String(v)}</Tag>},{title:'人数',dataIndex:'users'},{title:'占比',dataIndex:'ratio',render:(v:any)=> v!=null? `${v}%`:'-'},{title:'ARPU',dataIndex:'arpu'}]}
          pagination={{ pageSize: 10 }}
        />
      </Card>
    </PageContainer>
  );
}
