import React, { useState } from 'react';
import { Button, Card, Select, Space, Table } from 'antd';
import type { Dayjs } from 'dayjs';
import { FormattedMessage, useIntl } from '@umijs/max';
import { fetchAnalyticsPaymentsSummary } from '@/services/api/analytics';
import type { JSONValue } from '@/types/dashboard';
import { exportToCSV } from '@/utils/export';
import type { CompareItem, DeltaDim, DeltaMode, DeltaRows, DimData } from './types';

// 环比分析：对比同等长度的上一个时间窗口，展示收入与成功率涨幅 Top/Bottom
const DeltaSection: React.FC<{
  range: [Dayjs | null, Dayjs | null] | null;
  channel: string;
  platform: string;
  country: string;
  region?: string;
  city?: string;
}> = ({ range, channel, platform, country, region, city }) => {
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [dim, setDim] = useState<DeltaDim>('channel');
  const [mode, setMode] = useState<DeltaMode>('prev');
  const [rows, setRows] = useState<DeltaRows>({
    upRev: [],
    downRev: [],
    upRate: [],
    downRate: [],
    all: [],
  });
  const calc = async () => {
    if (!range || !range[0] || !range[1]) return;
    setLoading(true);
    try {
      const s1: Record<string, string | number> = {
        start: range[0].toISOString(),
        end: range[1].toISOString(),
      };
      if (channel) s1.channel = channel;
      if (platform) s1.platform = platform;
      if (country) s1.country = country;
      if (region) s1.region = region;
      if (city) s1.city = city;
      let pStart: Date, pEnd: Date;
      const startDate = new Date(range[0].toISOString());
      const endDate = new Date(range[1].toISOString());
      if (mode === 'prev') {
        const prevMs = endDate.getTime() - startDate.getTime();
        pEnd = new Date(startDate.getTime());
        pStart = new Date(pEnd.getTime() - prevMs);
      } else if (mode === 'prev_week') {
        pStart = new Date(startDate.getTime() - 7 * 24 * 3600 * 1000);
        pEnd = new Date(endDate.getTime() - 7 * 24 * 3600 * 1000);
      } else if (mode === 'prev_month') {
        pStart = new Date(startDate.getTime() - 30 * 24 * 3600 * 1000);
        pEnd = new Date(endDate.getTime() - 30 * 24 * 3600 * 1000);
      } else {
        // prev_year
        pStart = new Date(startDate.getTime() - 365 * 24 * 3600 * 1000);
        pEnd = new Date(endDate.getTime() - 365 * 24 * 3600 * 1000);
      }
      const s0: Record<string, string | number> = {
        start: pStart.toISOString(),
        end: pEnd.toISOString(),
      };
      if (channel) s0.channel = channel;
      if (platform) s0.platform = platform;
      if (country) s0.country = country;
      if (region) s0.region = region;
      if (city) s0.city = city;
      const cur = await fetchAnalyticsPaymentsSummary(s1);
      const pre = await fetchAnalyticsPaymentsSummary(s0);
      const arrCur =
        (dim === 'channel'
          ? cur.byChannel
          : dim === 'platform'
            ? cur.byPlatform
            : dim === 'country'
              ? cur.byCountry
              : dim === 'region'
                ? cur.byRegion
                : dim === 'city'
                  ? cur.byCity
                  : cur.byProduct) || [];
      const arrPreIdx: Record<string, Record<string, JSONValue>> = {};
      const arrPre =
        (dim === 'channel'
          ? pre.byChannel
          : dim === 'platform'
            ? pre.byPlatform
            : dim === 'country'
              ? pre.byCountry
              : dim === 'region'
                ? pre.byRegion
                : dim === 'city'
                  ? pre.byCity
                  : pre.byProduct) || [];
      arrPre.forEach((x: Record<string, JSONValue>) => {
        const k = String(dim === 'product' ? x['product_id'] : x[dim]);
        arrPreIdx[k] = x;
      });
      const items: CompareItem[] = arrCur.map((x: DimData) => {
        const record = x as unknown as Record<string, JSONValue>;
        const key = String(dim === 'product' ? record['product_id'] : record[dim]);
        const prev = arrPreIdx[key] || {};
        const revDelta = Number(x.revenueCents || 0) - Number(prev.revenueCents || 0);
        const rateDelta = Number(x.successRate || 0) - Number(prev.successRate || 0);
        return { key, cur: record, prev, revDelta, rateDelta };
      });
      const upRev = items
        .slice(0)
        .sort((a, b) => b.revDelta - a.revDelta)
        .slice(0, 5);
      const downRev = items
        .slice(0)
        .sort((a, b) => a.revDelta - b.revDelta)
        .slice(0, 5);
      const upRate = items
        .slice(0)
        .sort((a, b) => b.rateDelta - a.rateDelta)
        .slice(0, 5);
      const downRate = items
        .slice(0)
        .sort((a, b) => a.rateDelta - b.rateDelta)
        .slice(0, 5);
      setRows({ upRev, downRev, upRate, downRate, all: items });
    } finally {
      setLoading(false);
    }
  };
  return (
    <Card
      size="small"
      title={intl.formatMessage({
        id: 'pages.analyticsPayments.delta.title',
        defaultMessage: '环比分析（上一等长窗口）',
      })}
      style={{ marginTop: 16 }}
      extra={
        <Space>
          <Select<DeltaDim>
            value={dim}
            onChange={(v) => setDim(v)}
            options={[
              {
                label: intl.formatMessage({
                  id: 'pages.analyticsPayments.delta.dim.option.channel',
                  defaultMessage: '渠道',
                }),
                value: 'channel',
              },
              {
                label: intl.formatMessage({
                  id: 'pages.analyticsPayments.delta.dim.option.platform',
                  defaultMessage: '平台',
                }),
                value: 'platform',
              },
              {
                label: intl.formatMessage({
                  id: 'pages.analyticsPayments.delta.dim.option.country',
                  defaultMessage: '国家',
                }),
                value: 'country',
              },
              {
                label: intl.formatMessage({
                  id: 'pages.analyticsPayments.delta.dim.option.product',
                  defaultMessage: '商品',
                }),
                value: 'product',
              },
            ]}
          />
          <Select<DeltaMode>
            value={mode}
            onChange={(v) => setMode(v)}
            options={[
              {
                label: intl.formatMessage({
                  id: 'pages.analyticsPayments.delta.mode.option.prev',
                  defaultMessage: '上一等长窗口',
                }),
                value: 'prev',
              },
              {
                label: intl.formatMessage({
                  id: 'pages.analyticsPayments.delta.mode.option.prevWeek',
                  defaultMessage: '上一周',
                }),
                value: 'prev_week',
              },
              {
                label: intl.formatMessage({
                  id: 'pages.analyticsPayments.delta.mode.option.prevMonth',
                  defaultMessage: '上一月(30日)',
                }),
                value: 'prev_month',
              },
              {
                label: intl.formatMessage({
                  id: 'pages.analyticsPayments.delta.mode.option.prevYear',
                  defaultMessage: '上一年(365日)',
                }),
                value: 'prev_year',
              },
            ]}
          />
          <Button onClick={calc} loading={loading}>
            <FormattedMessage
              id="pages.analyticsPayments.delta.button.compute"
              defaultMessage="计算"
            />
          </Button>
          <Button
            onClick={() => {
              try {
                const rowsOut: (string | number)[][] = [
                  [
                    'dim',
                    'cur_revenue_cents',
                    'prev_revenue_cents',
                    'delta_revenue_cents',
                    'cur_success_rate(%)',
                    'prev_success_rate(%)',
                    'delta_success_rate(%)',
                  ],
                ];
                (rows.all || []).forEach((it: CompareItem) => {
                  rowsOut.push([
                    it.key,
                    Number(it.cur?.revenueCents || 0),
                    Number(it.prev?.revenueCents || 0),
                    Number(it.revDelta || 0),
                    Number(it.cur?.successRate || 0),
                    Number(it.prev?.successRate || 0),
                    Number(it.rateDelta || 0),
                  ]);
                });
                exportToCSV('payments_delta.csv', rowsOut);
              } catch {}
            }}
          >
            <FormattedMessage
              id="pages.analyticsPayments.delta.button.exportReport"
              defaultMessage="导出环比报告"
            />
          </Button>
        </Space>
      }
    >
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
        <div>
          <b>
            {intl.formatMessage({
              id: 'pages.analyticsPayments.delta.section.upRev',
              defaultMessage: '收入涨幅 Top5',
            })}
          </b>
          <Table<CompareItem>
            size="small"
            rowKey={(r: CompareItem) => String(r.key || '')}
            dataSource={rows.upRev}
            columns={[
              { title: dim, dataIndex: 'key' },
              {
                title: intl.formatMessage({
                  id: 'pages.analyticsPayments.delta.column.revDelta',
                  defaultMessage: '涨幅(分)',
                }),
                dataIndex: 'revDelta',
              },
            ]}
            pagination={false}
          />
        </div>
        <div>
          <b>
            {intl.formatMessage({
              id: 'pages.analyticsPayments.delta.section.downRev',
              defaultMessage: '收入降幅 Top5',
            })}
          </b>
          <Table<CompareItem>
            size="small"
            rowKey={(r: CompareItem) => String(r.key || '')}
            dataSource={rows.downRev}
            columns={[
              { title: dim, dataIndex: 'key' },
              {
                title: intl.formatMessage({
                  id: 'pages.analyticsPayments.delta.column.revDrop',
                  defaultMessage: '跌幅(分)',
                }),
                dataIndex: 'revDelta',
              },
            ]}
            pagination={false}
          />
        </div>
        <div>
          <b>
            {intl.formatMessage({
              id: 'pages.analyticsPayments.delta.section.upRate',
              defaultMessage: '成功率涨幅 Top5',
            })}
          </b>
          <Table<CompareItem>
            size="small"
            rowKey={(r: CompareItem) => String(r.key || '')}
            dataSource={rows.upRate}
            columns={[
              { title: dim, dataIndex: 'key' },
              {
                title: intl.formatMessage({
                  id: 'pages.analyticsPayments.delta.column.rateDelta',
                  defaultMessage: '涨幅(%)',
                }),
                dataIndex: 'rateDelta',
              },
            ]}
            pagination={false}
          />
        </div>
        <div>
          <b>
            {intl.formatMessage({
              id: 'pages.analyticsPayments.delta.section.downRate',
              defaultMessage: '成功率降幅 Top5',
            })}
          </b>
          <Table<CompareItem>
            size="small"
            rowKey={(r: CompareItem) => String(r.key || '')}
            dataSource={rows.downRate}
            columns={[
              { title: dim, dataIndex: 'key' },
              {
                title: intl.formatMessage({
                  id: 'pages.analyticsPayments.delta.column.rateDrop',
                  defaultMessage: '跌幅(%)',
                }),
                dataIndex: 'rateDelta',
              },
            ]}
            pagination={false}
          />
        </div>
      </div>
    </Card>
  );
};

export default DeltaSection;
