import React, { useCallback, useEffect, useState } from 'react';
import { Alert, Card, Col, Row, Space } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { Line, Column } from '@ant-design/charts';
import { useIntl } from '@umijs/max';
import {
  fetchWarehouseDAU,
  fetchWarehouseOnline,
  fetchWarehouseRevenue,
} from '@/services/api/analytics';

type WarehouseStatus = 'loading' | 'ready' | 'disabled' | 'error';

type DAUSeries = Array<{ date: string; value: number; type: string }>;
type OnlineSeries = Array<{ minute: string; value: number }>;
type RevenueSeries = Array<{ date: string; value: number }>;

export default function AnalyticsWarehousePage() {
  const intl = useIntl();
  const [status, setStatus] = useState<WarehouseStatus>('loading');
  const [dau, setDau] = useState<DAUSeries>([]);
  const [online, setOnline] = useState<OnlineSeries>([]);
  const [revenue, setRevenue] = useState<RevenueSeries>([]);

  const load = useCallback(async () => {
    setStatus('loading');
    try {
      const [d, o, r] = await Promise.all([
        fetchWarehouseDAU({ days: 14 }),
        fetchWarehouseOnline({ minutes: 60 }),
        fetchWarehouseRevenue({ days: 14 }),
      ]);
      setDau(
        (d?.points || []).flatMap((p) => [
          { date: p.date, value: Number(p.dau || 0), type: 'DAU' },
          { date: p.date, value: Number(p.newUsers || 0), type: '新增' },
        ]),
      );
      setOnline(
        (o?.points || []).map((p) => ({
          minute: p.minute ? p.minute.slice(11, 16) : '',
          value: Number(p.online || 0),
        })),
      );
      setRevenue(
        (r?.points || []).map((p) => ({ date: p.date, value: Number(p.revenueCents || 0) })),
      );
      setStatus('ready');
    } catch (err) {
      const resp = (err as { response?: { status?: number } })?.response;
      if (resp?.status === 503) {
        setStatus('disabled');
      } else {
        setStatus('error');
      }
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  if (status === 'disabled') {
    return (
      <PageContainer>
        <Alert
          type="info"
          showIcon
          message={intl.formatMessage({ id: 'pages.analytics.warehouse.disabled.title' })}
          description={intl.formatMessage({
            id: 'pages.analytics.warehouse.disabled.description',
          })}
        />
      </PageContainer>
    );
  }

  if (status === 'error') {
    return (
      <PageContainer>
        <Alert
          type="error"
          showIcon
          message={intl.formatMessage({ id: 'pages.analytics.warehouse.error.title' })}
          action={
            <a onClick={load} href="#">
              {intl.formatMessage({ id: 'pages.analytics.warehouse.retry' })}
            </a>
          }
        />
      </PageContainer>
    );
  }

  return (
    <PageContainer>
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <Card
          title={intl.formatMessage({ id: 'pages.analytics.warehouse.title' })}
          loading={status === 'loading'}
        >
          <Row gutter={[16, 16]}>
            <Col span={12}>
              <Card size="small" title="DAU / 新增用户（近 14 天）">
                <Line
                  data={dau}
                  xField="date"
                  yField="value"
                  seriesField="type"
                  height={260}
                  smooth
                  point={false}
                />
              </Card>
            </Col>
            <Col span={12}>
              <Card size="small" title="分钟在线（近 60 分钟）">
                <Line
                  data={online}
                  xField="minute"
                  yField="value"
                  height={260}
                  smooth
                  point={false}
                />
              </Card>
            </Col>
          </Row>
          <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
            <Col span={24}>
              <Card size="small" title="日收入（近 14 天，单位：分）">
                <Column data={revenue} xField="date" yField="value" height={260} />
              </Card>
            </Col>
          </Row>
        </Card>
      </Space>
    </PageContainer>
  );
}
