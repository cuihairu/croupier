import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Col, Row, Select, Space, Table, Tabs, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { Line } from '@ant-design/charts';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';
import { CompareValueView } from '@/components/CompareValueView';
import {
  fetchIncidentReportLeaderboard,
  fetchIncidentReportSummary,
  fetchIncidentReportTrend,
  fetchIncidentResponsibility,
  type CompareDelta,
  type LeaderboardRow,
  type ReportLeaderboard,
  type ResponsibilityReport,
} from '@/services/api/incident';

type PeriodType = 'week' | 'month' | 'quarter' | 'year';

const PERIODS: PeriodType[] = ['week', 'month', 'quarter', 'year'];

// 与后端口径一致（incident-reports.md §5.1）：周=ISO 周一 00:00 起。
// 前端只做档位键的 ±1 期导航，区间解析以服务端为准。
function isoWeekOf(d: Dayjs): { year: number; week: number } {
  const date = new Date(Date.UTC(d.year(), d.month(), d.date()));
  const dayNum = date.getUTCDay() || 7;
  date.setUTCDate(date.getUTCDate() + 4 - dayNum);
  const year = date.getUTCFullYear();
  const jan4 = new Date(Date.UTC(year, 0, 4));
  const jan4Day = jan4.getUTCDay() || 7;
  jan4.setUTCDate(jan4.getUTCDate() + 4 - jan4Day);
  const week = Math.ceil((date.getTime() - jan4.getTime()) / (7 * 24 * 3600 * 1000)) + 1;
  return { year, week };
}

function shiftPeriod(period: PeriodType, key: string, delta: number): string {
  if (period === 'week') {
    const m = /^(\d{4})-W(\d{1,2})$/.exec(key);
    if (!m) return '';
    // ISO 首周锚点：1 月 4 日恒在 ISO 第 1 周
    const d = dayjs(`${m[1]}-01-04`).add((Number(m[2]) - 1 + delta) * 7, 'day');
    const iso = isoWeekOf(d);
    return `${iso.year}-W${String(iso.week).padStart(2, '0')}`;
  }
  if (period === 'month') {
    const d = dayjs(`${key}-01`).add(delta, 'month');
    return d.format('YYYY-MM');
  }
  if (period === 'quarter') {
    const m = /^(\d{4})-Q([1-4])$/.exec(key);
    if (!m) return '';
    const q = Number(m[2]) - 1 + delta;
    return `${Number(m[1]) + Math.floor(q / 4)}-Q${(((q % 4) + 4) % 4) + 1}`;
  }
  const y = Number(key);
  return Number.isFinite(y) ? String(y + delta) : '';
}

const DASHED = { lineDash: [4, 4] } as const;

export default function IncidentReportsPage() {
  const intl = useIntl();
  const [period, setPeriod] = useState<PeriodType>('week');
  const [periodKey, setPeriodKey] = useState<string>('');
  const [view, setView] = useState<string>('responsible');
  const [board, setBoard] = useState<string>('incidents');

  const text = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  const step = useCallback(
    (delta: number) => {
      setPeriodKey((k) => {
        if (!k) return k; // 当前周期没有键可步进，先读回接口给的 periodKey
        return shiftPeriod(period, k, delta);
      });
    },
    [period],
  );

  return (
    <PageContainer>
      <Tabs
        activeKey={period}
        onChange={(k) => {
          setPeriod(k as PeriodType);
          setPeriodKey('');
        }}
        items={PERIODS.map((p) => ({
          key: p,
          label: <FormattedMessage id={`pages.incidentReports.period.${p}`} defaultMessage={p} />,
        }))}
      />
      <Space style={{ marginBottom: 16 }} wrap>
        <Typography.Text>
          <FormattedMessage id="pages.incidentReports.periodKey" defaultMessage="周期" />
        </Typography.Text>
        <Typography.Text strong code>
          {periodKey || text('pages.incidentReports.currentPeriod', '当前')}
        </Typography.Text>
        <Button size="small" onClick={() => step(-1)} disabled={!periodKey}>
          {text('pages.incidentReports.prevPeriod', '上一期')}
        </Button>
        <Button size="small" onClick={() => setPeriodKey('')}>
          {text('pages.incidentReports.thisPeriod', '回到当前')}
        </Button>
      </Space>
      <SummarySection period={period} periodKey={periodKey} onLoaded={setPeriodKey} />
      <TrendSection period={period} />
      <LeaderboardSection
        period={period}
        periodKey={periodKey}
        view={view}
        board={board}
        onView={setView}
        onBoard={setBoard}
      />
      {(period === 'week' || period === 'month') && (
        <ResponsibilitySection period={period} periodKey={periodKey} />
      )}
    </PageContainer>
  );
}

function SummarySection({
  period,
  periodKey,
  onLoaded,
}: {
  period: PeriodType;
  periodKey: string;
  onLoaded: (key: string) => void;
}) {
  const intl = useIntl();
  const [summary, setSummary] = React.useState<Awaited<
    ReturnType<typeof fetchIncidentReportSummary>
  > | null>(null);
  const [error, setError] = useState<string>('');
  const text = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  useEffect(() => {
    let alive = true;
    fetchIncidentReportSummary(period, periodKey || undefined)
      .then((s) => {
        if (!alive) return;
        setSummary(s);
        onLoaded(s.periodKey);
      })
      .catch((e: unknown) => {
        if (alive) setError(String((e as Error)?.message || e));
      });
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [period, periodKey]);

  if (error) {
    return (
      <Alert
        type="error"
        showIcon
        title={text('pages.incidentReports.summary.loadFailed', '汇总加载失败')}
        description={error}
        style={{ marginBottom: 16 }}
      />
    );
  }
  const m = summary?.metrics;
  return (
    <Row gutter={16} style={{ marginBottom: 16 }}>
      <Col span={6}>
        <Card size="small" title={text('pages.incidentReports.stats.total', '事故总量（含缺陷）')}>
          <CompareValueView value={m?.total} />
        </Card>
      </Col>
      <Col span={6}>
        <Card size="small" title={text('pages.incidentReports.stats.duration', '影响时长')}>
          <CompareValueView value={m?.duration} />
        </Card>
      </Col>
      <Col span={6}>
        <Card size="small" title={text('pages.incidentReports.stats.mttr', 'MTTR')}>
          <CompareValueView value={m?.mttr} lowerIsBetter />
        </Card>
      </Col>
      <Col span={6}>
        <Card size="small" title={text('pages.incidentReports.stats.recurrence', '复发率')}>
          <CompareValueView value={m?.recurrence} precision={2} lowerIsBetter />
        </Card>
      </Col>
      {summary && summary.categoryBreakdown.length > 0 && (
        <Col span={24} style={{ marginTop: 8 }}>
          <Card size="small" title={text('pages.incidentReports.categoryBreakdown', '分类占比')}>
            <Space size={16} wrap>
              {summary.categoryBreakdown.map((c) => (
                <Tag key={c.categoryId}>
                  {c.name || c.categoryId}: {c.count}
                  {c.sharePct !== null && c.sharePct !== undefined
                    ? ` (${c.sharePct.toFixed(1)}%)`
                    : ''}
                </Tag>
              ))}
            </Space>
          </Card>
        </Col>
      )}
    </Row>
  );
}

function TrendSection({ period }: { period: PeriodType }) {
  const intl = useIntl();
  const [data, setData] = useState<Awaited<ReturnType<typeof fetchIncidentReportTrend>> | null>(
    null,
  );
  const text = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  useEffect(() => {
    let alive = true;
    // 趋势窗口独立于报表档位：近 30 天/12 周/12 个月（后端缺省口径）
    const bucket =
      period === 'month' || period === 'quarter' || period === 'year' ? 'month' : 'day';
    fetchIncidentReportTrend({ bucket })
      .then((t) => {
        if (alive) setData(t);
      })
      .catch(() => {
        if (alive) setData(null);
      });
    return () => {
      alive = false;
    };
  }, [period]);

  const chartData = useMemo(() => {
    if (!data) return [];
    const cur = (data.points || []).map((p) => ({
      t: p.t,
      value: p.total,
      type: text('pages.incidentReports.trend.current', '本期'),
    }));
    const yoy = (data.yoyPoints || []).map((p) => ({
      t: p.t,
      value: p.total,
      type: text('pages.incidentReports.trend.yoy', '去年同期'),
    }));
    return [...cur, ...yoy];
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data]);

  return (
    <Card
      size="small"
      title={text('pages.incidentReports.trend.title', '事故趋势')}
      style={{ marginBottom: 16 }}
      extra={
        data?.yoyMissing ? (
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            <FormattedMessage
              id="pages.incidentReports.trend.yoyMissing"
              defaultMessage="无去年同期数据"
            />
          </Typography.Text>
        ) : null
      }
    >
      {chartData.length === 0 ? (
        <Typography.Text type="secondary">
          <FormattedMessage id="pages.incidentReports.trend.empty" defaultMessage="暂无数据" />
        </Typography.Text>
      ) : (
        <Line
          height={260}
          data={chartData}
          xField="t"
          yField="value"
          seriesField="type"
          smooth
          lineStyle={(d: { type?: string }) =>
            d?.type === text('pages.incidentReports.trend.yoy', '去年同期') ? DASHED : {}
          }
          yAxis={{ label: { formatter: (v: string) => String(Math.round(Number(v))) } }}
        />
      )}
    </Card>
  );
}

function LeaderboardSection({
  period,
  periodKey,
  view,
  board,
  onView,
  onBoard,
}: {
  period: PeriodType;
  periodKey: string;
  view: string;
  board: string;
  onView: (v: string) => void;
  onBoard: (v: string) => void;
}) {
  const intl = useIntl();
  const [data, setData] = useState<ReportLeaderboard | null>(null);
  const [loading, setLoading] = useState(false);
  const text = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  useEffect(() => {
    let alive = true;
    setLoading(true);
    fetchIncidentReportLeaderboard({ period, key: periodKey || undefined, view, board })
      .then((d) => {
        if (alive) setData(d);
      })
      .catch(() => {
        if (alive) setData(null);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [period, periodKey, view, board]);

  const columns: ColumnsType<LeaderboardRow> = [
    {
      title:
        view === 'responsible'
          ? text('pages.incidentReports.lb.responsible', '责任人')
          : text('pages.incidentReports.lb.category', '类别'),
      dataIndex: 'label',
      render: (_, row) =>
        view === 'category' ? row.categoryName || row.label || row.key : row.label || row.key,
    },
    { title: text('pages.incidentReports.lb.value', '榜值'), dataIndex: 'value', width: 100 },
    {
      title: text('pages.incidentReports.lb.incidents', '事故'),
      dataIndex: 'incidents',
      width: 80,
    },
    { title: text('pages.incidentReports.lb.bugs', '缺陷'), dataIndex: 'bugs', width: 80 },
    {
      title: text('pages.incidentReports.lb.duration', '影响时长'),
      dataIndex: 'durationMs',
      width: 120,
      render: (v: number) => (v > 0 ? `${Math.round(v / 3600000)}h` : '-'),
    },
    {
      title: 'MTTR',
      dataIndex: 'mttrMs',
      width: 110,
      render: (v?: number | null) => (v ? `${Math.round(v / 3600000)}h` : '-'),
    },
    {
      title: text('pages.incidentReports.compare.prev', '环比'),
      width: 140,
      render: (_, row) => <DeltaCell delta={row.prev} />,
    },
    {
      title: text('pages.incidentReports.compare.yoy', '同比'),
      width: 140,
      render: (_, row) => <DeltaCell delta={row.yoy} />,
    },
  ];

  return (
    <Card
      size="small"
      title={text('pages.incidentReports.lb.title', '排行榜')}
      style={{ marginBottom: 16 }}
      extra={
        <Space>
          <Select
            size="small"
            value={view}
            onChange={onView}
            style={{ width: 120 }}
            options={[
              {
                value: 'responsible',
                label: text('pages.incidentReports.lb.byResponsible', '按责任人'),
              },
              { value: 'category', label: text('pages.incidentReports.lb.byCategory', '按类别') },
            ]}
          />
          <Select
            size="small"
            value={board}
            onChange={onBoard}
            style={{ width: 140 }}
            options={[
              {
                value: 'incidents',
                label: text('pages.incidentReports.lb.boardIncidents', '事故数'),
              },
              {
                value: 'duration',
                label: text('pages.incidentReports.lb.boardDuration', '影响时长'),
              },
              { value: 'mttr', label: text('pages.incidentReports.lb.boardMttr', 'MTTR') },
            ]}
          />
        </Space>
      }
    >
      <Table<LeaderboardRow>
        rowKey="key"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={data?.rows || []}
        pagination={false}
      />
    </Card>
  );
}

function DeltaCell({ delta }: { delta?: CompareDelta }) {
  if (!delta) return <span>-</span>;
  if (delta.missing) {
    return (
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        <FormattedMessage id="pages.incidentReports.compare.noData" defaultMessage="无数据" />
      </Typography.Text>
    );
  }
  const v = delta.delta;
  if (v === null || v === undefined) return <span>-</span>;
  const color = v > 0 ? 'danger' : v < 0 ? 'success' : 'secondary';
  return (
    <Typography.Text type={color} style={{ fontSize: 12 }}>
      {v > 0 ? '+' : ''}
      {v.toLocaleString()}
      {delta.pct !== null && delta.pct !== undefined ? ` (${Math.abs(delta.pct).toFixed(1)}%)` : ''}
    </Typography.Text>
  );
}

function ResponsibilitySection({ period, periodKey }: { period: PeriodType; periodKey: string }) {
  const intl = useIntl();
  const [report, setReport] = useState<ResponsibilityReport | null>(null);
  const text = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  useEffect(() => {
    let alive = true;
    fetchIncidentResponsibility(period, periodKey || undefined)
      .then((r) => {
        if (alive) setReport(r);
      })
      .catch(() => {
        if (alive) setReport(null);
      });
    return () => {
      alive = false;
    };
  }, [period, periodKey]);

  if (!report) return null;
  return (
    <Card size="small" title={text('pages.incidentReports.resp.title', '周期责任人报告')}>
      {report.unattributed.length > 0 && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
          title={text(
            'pages.incidentReports.resp.unattributed',
            `${report.unattributed.length} 条事故未归因，请补全`,
          )}
        />
      )}
      <Table
        rowKey="categoryId"
        size="small"
        pagination={false}
        columns={[
          { title: text('pages.incidentReports.resp.category', '类别'), dataIndex: 'name' },
          { title: text('pages.incidentReports.resp.leader', '负责人'), dataIndex: 'leader' },
          { title: text('pages.incidentReports.resp.count', '数量'), dataIndex: 'count' },
          {
            title: text('pages.incidentReports.compare.prev', '环比'),
            render: (_, r) => <DeltaCell delta={r.prev} />,
          },
          {
            title: text('pages.incidentReports.compare.yoy', '同比'),
            render: (_, r) => <DeltaCell delta={r.yoy} />,
          },
          {
            title: text('pages.incidentReports.resp.byResponsible', '按责任人'),
            dataIndex: 'byResponsible',
            render: (m: Record<string, number>) =>
              Object.entries(m)
                .map(([k, v]) => `${k}:${v}`)
                .join('  '),
          },
        ]}
        dataSource={report.categories}
        style={{ marginBottom: 16 }}
      />
      <Table
        rowKey="key"
        size="small"
        pagination={false}
        columns={[
          { title: text('pages.incidentReports.lb.responsible', '责任人'), dataIndex: 'label' },
          { title: text('pages.incidentReports.resp.count', '数量'), dataIndex: 'total' },
          {
            title: text('pages.incidentReports.compare.prev', '环比'),
            render: (_, r) => <DeltaCell delta={r.prev} />,
          },
          {
            title: text('pages.incidentReports.compare.yoy', '同比'),
            render: (_, r) => <DeltaCell delta={r.yoy} />,
          },
          {
            title: text('pages.incidentReports.resp.byCategory', '按类别'),
            dataIndex: 'byCategory',
            render: (m: Record<string, number>) =>
              Object.entries(m)
                .map(([k, v]) => `${k}:${v}`)
                .join('  '),
          },
          {
            title: 'MTTR',
            dataIndex: 'mttrMs',
            render: (v?: number | null) => (v ? `${Math.round(v / 3600000)}h` : '-'),
          },
          {
            title: text('pages.incidentReports.resp.recurrences', '复发'),
            dataIndex: 'recurrences',
          },
        ]}
        dataSource={report.responsibles}
      />
    </Card>
  );
}
