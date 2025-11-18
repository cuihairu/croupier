import React, { useEffect, useState } from 'react';
import { Card, Space, DatePicker, Select, Button, Row, Col, Statistic, Divider, Table } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { exportToXLSX } from '@/utils/export';
import { fetchAnalyticsOverview } from '@/services/croupier/analytics';

export default function AnalyticsOverviewPage() {
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [range, setRange] = useState<any>(null);
  const [channel, setChannel] = useState<string>('');
  const [platform, setPlatform] = useState<string>('');
  const [data, setData] = useState<any>({});

  const load = async () => {
    setLoading(true);
    try {
      const params: any = {};
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      if (channel) params.channel = channel;
      if (platform) params.platform = platform;
      const r = await fetchAnalyticsOverview(params);
      setData(r || {});
    } finally { setLoading(false); }
  };
  useEffect(()=>{ load(); }, []);

  const exportExcel = async () => {
    // Sheet 1: summary; Sheet 2: series (new_users/peak_online/revenue)
    const summary = [
      ['metric','value'],
      ['dau', data?.dau||0],['wau', data?.wau||0],['mau', data?.mau||0],
      ['new_users', data?.new_users||0],['registered_total', data?.registered_total||0],
      ['retention_d1', data?.d1||0],['retention_d7', data?.d7||0],['retention_d30', data?.d30||0],
      ['pay_rate', data?.pay_rate||0],['arpu', data?.arpu||0],['arppu', data?.arppu||0],
      ['revenue_cents', data?.revenue_cents||0],
    ];
    const ser = data?.series||{};
    const seriesRows = [['time','new_users','peak_online','revenue_cents']];
    const len = Math.max(ser?.new_users?.length||0, ser?.peak_online?.length||0, ser?.revenue_cents?.length||0);
    for (let i=0;i<len;i++) {
      const t = ser?.new_users?.[i]?.[0] || ser?.peak_online?.[i]?.[0] || ser?.revenue_cents?.[i]?.[0] || '';
      const nu = ser?.new_users?.[i]?.[1] ?? '';
      const po = ser?.peak_online?.[i]?.[1] ?? '';
      const rv = ser?.revenue_cents?.[i]?.[1] ?? '';
      seriesRows.push([t, nu, po, rv]);
    }
    await exportToXLSX('overview.xlsx', [
      { sheet: 'summary', rows: summary },
      { sheet: 'series', rows: seriesRows },
    ]);
  };

  const Spark: React.FC<{ data?: [string|number, number][] }>= ({ data }) => {
    const w=300,h=60,p=4;
    const pts = (data||[]).map(d=> [Number(d[0]), Number(d[1])]);
    if (pts.length===0) return <div style={{height:h}}/>;
    const xs = pts.map(p=>p[0]); const ys = pts.map(p=>p[1]);
    const x0 = Math.min(...xs), x1 = Math.max(...xs), y0 = Math.min(...ys), y1 = Math.max(...ys);
    const sx = (x:number)=> x1===x0? p : p + (w-2*p) * (x - x0)/(x1-x0);
    const sy = (y:number)=> y1===y0? h-p : h - (p + (h-2*p) * (y - y0)/(y1-y0));
    const d = pts.map((pt,i)=> `${i?'L':'M'}${sx(pt[0])},${sy(pt[1])}`).join(' ');
    return (
      <svg width={w} height={h} style={{ display:'block' }}>
        <path d={d} fill="none" stroke="#1677ff" strokeWidth={2} />
      </svg>
    );
  };

  return (
    <PageContainer>
      <Card title={intl.formatMessage({ id: 'pages.analytics.overview.title' } || '概览 KPI')}>
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Row gutter={[16,16]}>
            <Col span={4}><Card loading={loading}><Statistic title="DAU" value={data?.dau||0} /></Card></Col>
            <Col span={4}><Card loading={loading}><Statistic title="WAU" value={data?.wau||0} /></Card></Col>
            <Col span={4}><Card loading={loading}><Statistic title="MAU" value={data?.mau||0} /></Card></Col>
            <Col span={4}><Card loading={loading}><Statistic title="新增" value={data?.new_users||0} /></Card></Col>
            <Col span={4}><Card loading={loading}><Statistic title="注册用户总数" value={data?.registered_total||0} /></Card></Col>
            <Col span={4}><Card loading={loading}><Statistic title="收入(分)" value={data?.revenue_cents||0} /></Card></Col>
          </Row>
          <Row gutter={[16,16]}>
            <Col span={8}><Card loading={loading}><Statistic title="付费率" suffix="%" value={data?.pay_rate||0} /></Card></Col>
            <Col span={8}><Card loading={loading}><Statistic title="ARPU" value={data?.arpu||0} /></Card></Col>
            <Col span={8}><Card loading={loading}><Statistic title="ARPPU" value={data?.arppu||0} /></Card></Col>
          </Row>
          <Divider />
          <Row gutter={[16,16]}>
            <Col span={8}><Card loading={loading}><Statistic title="D1 留存" value={data?.d1||0} suffix="%" /></Card></Col>
            <Col span={8}><Card loading={loading}><Statistic title="D7 留存" value={data?.d7||0} suffix="%" /></Card></Col>
            <Col span={8}><Card loading={loading}><Statistic title="D30 留存" value={data?.d30||0} suffix="%" /></Card></Col>
          </Row>
          <Divider />
          <Row gutter={[16,16]}>
            <Col span={8}><Card size="small" title="每日新增（曲线）"><Spark data={data?.series?.new_users||[]} /></Card></Col>
            <Col span={8}><Card size="small" title="每日峰值在线（曲线）"><Spark data={data?.series?.peak_online||[]} /></Card></Col>
            <Col span={8}><Card size="small" title="每日充值(分)（曲线）"><Spark data={data?.series?.revenue_cents||[]} /></Card></Col>
          </Row>
        </Space>
      </Card>
    </PageContainer>
  );
}
