import React, { useState } from 'react';
import { Button, Select, Space, Table } from 'antd';
import type { Dayjs } from 'dayjs';
import { FormattedMessage, useIntl } from '@umijs/max';
import { exportToXLSX } from '@/utils/export';
import { fetchAnalyticsAdoption, fetchAnalyticsAdoptionBreakdown } from '@/services/api/analytics';
import type { AdoptionBreakdownRow, AdoptionRow } from './types';

/** 功能采用率控件：总体采用率 + 按渠道/平台/国家分层采用率。 */
const AdoptionControls: React.FC<{ range: [Dayjs | null, Dayjs | null] | null }> = ({ range }) => {
  const intl = useIntl();
  const [features, setFeatures] = useState<string[]>([]);
  const [per, setPer] = useState<'user' | 'session'>('user');
  const [rows, setRows] = useState<AdoptionRow[]>([]);
  const [baseline, setBaseline] = useState<number>(0);
  const [loading, setLoading] = useState(false);
  const [by, setBy] = useState<'channel' | 'platform' | 'country'>('channel');
  const load = async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = { features: features.join(','), per };
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsAdoption(params);
      setRows(r?.features || []);
      setBaseline(Number(r?.baseline || 0));
    } finally {
      setLoading(false);
    }
  };
  const [rowsDim, setRowsDim] = useState<AdoptionBreakdownRow[]>([]);
  const loadDim = async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = { features: features.join(','), per, by };
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsAdoptionBreakdown(params);
      setRowsDim(r?.rows || []);
    } finally {
      setLoading(false);
    }
  };
  return (
    <Space orientation="vertical" style={{ width: '100%' }}>
      <Space>
        <Select
          mode="tags"
          value={features}
          onChange={(v) => setFeatures(v)}
          placeholder={intl.formatMessage({
            id: 'pages.analyticsBehavior.adoption.filter.placeholder.features',
            defaultMessage: '功能事件（如：first_pay, open_store）',
          })}
          style={{ minWidth: 360 }}
        />
        <Select
          value={per}
          onChange={(v) => setPer(v)}
          options={[
            {
              label: intl.formatMessage({
                id: 'pages.analyticsBehavior.option.byUser',
                defaultMessage: '按用户',
              }),
              value: 'user',
            },
            {
              label: intl.formatMessage({
                id: 'pages.analyticsBehavior.option.bySession',
                defaultMessage: '按会话',
              }),
              value: 'session',
            },
          ]}
        />
        <Button type="primary" onClick={load} loading={loading}>
          <FormattedMessage
            id="pages.analyticsBehavior.adoption.button.compute"
            defaultMessage="计算采用率"
          />
        </Button>
        <Button
          onClick={async () => {
            const rowsOut = [['feature', 'groups', 'rate(%)', 'baseline']].concat(
              (rows || []).map((r: AdoptionRow) => [
                r.feature,
                String(r.groups),
                String(r.rate),
                String(baseline),
              ]),
            );
            await exportToXLSX('adoption.csv', [{ sheet: 'adoption', rows: rowsOut }]);
          }}
        >
          <FormattedMessage
            id="pages.analyticsBehavior.button.exportCsv"
            defaultMessage="导出 CSV"
          />
        </Button>
      </Space>
      <div>
        {intl.formatMessage(
          {
            id: 'pages.analyticsBehavior.adoption.baseline',
            defaultMessage: '基数（{per}）：{baseline}',
          },
          {
            per:
              per === 'user'
                ? intl.formatMessage({
                    id: 'pages.analyticsBehavior.adoption.perLabel.user',
                    defaultMessage: '用户',
                  })
                : intl.formatMessage({
                    id: 'pages.analyticsBehavior.adoption.perLabel.session',
                    defaultMessage: '会话',
                  }),
            baseline,
          },
        )}
      </div>
      <Table<AdoptionRow>
        size="small"
        loading={loading}
        rowKey={(r: AdoptionRow) => r.feature}
        dataSource={rows}
        columns={[
          {
            title: intl.formatMessage({
              id: 'pages.analyticsBehavior.adoption.column.feature',
              defaultMessage: '功能事件',
            }),
            dataIndex: 'feature',
          },
          {
            title: intl.formatMessage({
              id: 'pages.analyticsBehavior.column.groups',
              defaultMessage: '分组数',
            }),
            dataIndex: 'groups',
          },
          {
            title: intl.formatMessage({
              id: 'pages.analyticsBehavior.adoption.column.rate',
              defaultMessage: '采用率',
            }),
            dataIndex: 'rate',
            render: (v: number) => (v != null ? `${v}%` : '-'),
          },
        ]}
        pagination={{ pageSize: 10 }}
      />
      <Space>
        <Select<'channel' | 'platform' | 'country'>
          value={by}
          onChange={(v) => setBy(v)}
          options={[
            {
              label: intl.formatMessage({
                id: 'pages.analyticsBehavior.adoption.breakdown.option.byChannel',
                defaultMessage: '按渠道',
              }),
              value: 'channel',
            },
            {
              label: intl.formatMessage({
                id: 'pages.analyticsBehavior.adoption.breakdown.option.byPlatform',
                defaultMessage: '按平台',
              }),
              value: 'platform',
            },
            {
              label: intl.formatMessage({
                id: 'pages.analyticsBehavior.adoption.breakdown.option.byCountry',
                defaultMessage: '按国家',
              }),
              value: 'country',
            },
          ]}
        />
        <Button onClick={loadDim} loading={loading}>
          <FormattedMessage
            id="pages.analyticsBehavior.adoption.button.breakdown"
            defaultMessage="分层采用率"
          />
        </Button>
        <Button
          onClick={async () => {
            const rowsOut = [['dim', 'baseline', 'groups', 'rate(%)']].concat(
              (rowsDim || []).map((r: AdoptionBreakdownRow) => [
                r.dim,
                String(r.baseline),
                String(r.groups),
                String(r.rate),
              ]),
            );
            await exportToXLSX('adoption_breakdown.csv', [
              { sheet: 'adoption_breakdown', rows: rowsOut },
            ]);
          }}
        >
          <FormattedMessage
            id="pages.analyticsBehavior.button.exportCsv"
            defaultMessage="导出 CSV"
          />
        </Button>
      </Space>
      <Table<AdoptionBreakdownRow>
        size="small"
        loading={loading}
        rowKey={(r: AdoptionBreakdownRow) => `${r.dim || ''}|${r.baseline || ''}|${r.groups || ''}`}
        dataSource={rowsDim}
        columns={[
          {
            title: intl.formatMessage({
              id: 'pages.analyticsBehavior.adoption.breakdown.column.dim',
              defaultMessage: '分层',
            }),
            dataIndex: 'dim',
          },
          {
            title: intl.formatMessage({
              id: 'pages.analyticsBehavior.adoption.breakdown.column.baseline',
              defaultMessage: '基数',
            }),
            dataIndex: 'baseline',
          },
          {
            title: intl.formatMessage({
              id: 'pages.analyticsBehavior.column.groups',
              defaultMessage: '分组数',
            }),
            dataIndex: 'groups',
          },
          {
            title: intl.formatMessage({
              id: 'pages.analyticsBehavior.adoption.column.rate',
              defaultMessage: '采用率',
            }),
            dataIndex: 'rate',
            render: (v: number) => (v != null ? `${v}%` : '-'),
          },
        ]}
        pagination={{ pageSize: 10 }}
      />
    </Space>
  );
};

export default AdoptionControls;
