import React, { useEffect, useState } from 'react';
import {
  Card,
  Space,
  DatePicker,
  Input,
  Button,
  Table,
  Select,
  Switch,
  InputNumber,
  Checkbox,
} from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { PageContainer } from '@ant-design/pro-components';
import { exportToXLSX } from '@/utils/export';
import { fetchAnalyticsEvents, fetchAnalyticsFunnel } from '@/services/api/analytics';
import type { EventRow, FunnelStep } from './types';
import PathControls from './PathControls';
import AdoptionControls from './AdoptionControls';

export default function AnalyticsBehaviorPage() {
  const [loading, setLoading] = useState(false);
  const [eventName, setEventName] = useState<string>('');
  const [propKey, setPropKey] = useState<string>('');
  const [propVal, setPropVal] = useState<string>('');
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null);
  const [rows, setRows] = useState<EventRow[]>([]);

  const load = async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = {
        event: eventName,
        propKey: propKey,
        propVal: propVal,
      };
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsEvents(params);
      setRows(r?.events || []);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    void load();
    // The endpoint is part of the analytics contract; an empty event filter means all events.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const [steps, setSteps] = useState<string[]>([]);
  const [funnel, setFunnel] = useState<FunnelStep[]>([]);
  const [seq, setSeq] = useState<boolean>(false);
  const [sameSess, setSameSess] = useState<boolean>(false);
  const [gapSec, setGapSec] = useState<number>(0);
  const loadFunnel = async (overrideSteps?: string[]) => {
    setLoading(true);
    try {
      const st = overrideSteps && overrideSteps.length > 0 ? overrideSteps : steps;
      const params: Record<string, string | number> = {
        steps: st.join(','),
        sequential: seq ? 1 : 0,
      };
      if (sameSess) params.sameSession = 1;
      if (gapSec && gapSec > 0) params.gapSec = gapSec;
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsFunnel(params);
      setFunnel(r?.steps || []);
    } finally {
      setLoading(false);
    }
  };
  // Parse query to prefill funnel and auto compute
  useEffect(() => {
    try {
      const qs = new URLSearchParams(window.location.search);
      const st = (qs.get('steps') || '')
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean);
      if (st.length > 0) setSteps(st);
      const oseq = qs.get('sequential');
      if (oseq === '1') setSeq(true);
      const oss = qs.get('same_session');
      if (oss === '1') setSameSess(true);
      const og = parseInt(qs.get('gap_sec') || '0', 10);
      if (!isNaN(og) && og > 0) setGapSec(og);
      const s = qs.get('start');
      const e = qs.get('end');
      if (s && e) {
        try {
          const startDate = dayjs(s);
          const endDate = dayjs(e);
          if (startDate.isValid() && endDate.isValid()) {
            setRange([startDate, endDate]);
          }
        } catch {}
      }
      if (st.length > 0) {
        setTimeout(() => loadFunnel(st), 0);
      }
    } catch {}
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <PageContainer>
      <Space orientation="vertical" style={{ width: '100%' }}>
        <Card
          title="事件探索"
          extra={
            <Space>
              <Input
                placeholder="事件名"
                value={eventName}
                onChange={(e) => setEventName(e.target.value)}
                style={{ width: 160 }}
              />
              <Input
                placeholder="属性Key"
                value={propKey}
                onChange={(e) => setPropKey(e.target.value)}
                style={{ width: 140 }}
              />
              <Input
                placeholder="属性值"
                value={propVal}
                onChange={(e) => setPropVal(e.target.value)}
                style={{ width: 140 }}
              />
              <DatePicker.RangePicker
                value={range as [Dayjs, Dayjs]}
                onChange={(dates) => setRange(dates as [Dayjs | null, Dayjs | null] | null)}
              />
              <Button type="primary" onClick={load}>
                查询
              </Button>
            </Space>
          }
        >
          <Table<EventRow>
            size="small"
            loading={loading}
            rowKey={(r: EventRow) => r.id || `${r.event || ''}-${r.time || ''}`}
            dataSource={rows}
            columns={[
              { title: '时间', dataIndex: 'time' },
              { title: '事件', dataIndex: 'event' },
              { title: '用户', dataIndex: 'user_id' },
            ]}
          />
          <div style={{ marginTop: 8 }}>
            <Button
              onClick={async () => {
                const rowsOut = [['time', 'event', 'user_id']].concat(
                  (rows || []).map((r: EventRow) => [r.time || '', r.event || '', r.userId || '']),
                );
                await exportToXLSX('events.csv', [{ sheet: 'events', rows: rowsOut }]);
              }}
            >
              导出 CSV
            </Button>
          </div>
        </Card>

        <div id="funnel-anchor" />
        <div id="funnel-anchor" />
        <Card
          title="漏斗"
          extra={
            <Space>
              <Select
                mode="tags"
                placeholder="步骤（事件名）"
                value={steps}
                onChange={(v) => setSteps(v)}
                style={{ minWidth: 360 }}
              />
              <Switch
                checked={seq}
                onChange={setSeq}
                checkedChildren="顺序"
                unCheckedChildren="不强制顺序"
              />
              <Checkbox checked={sameSess} onChange={(e) => setSameSess(e.target.checked)}>
                同会话
              </Checkbox>
              <InputNumber
                placeholder="步间最大秒数"
                value={gapSec}
                onChange={(v) => setGapSec(Number(v || 0))}
                min={0}
                style={{ width: 140 }}
              />
              <Button type="primary" onClick={() => loadFunnel()}>
                计算
              </Button>
              <Button
                onClick={() => {
                  try {
                    const params = new URLSearchParams();
                    if (steps && steps.length > 0) params.set('steps', steps.join(','));
                    if (seq) params.set('sequential', '1');
                    if (sameSess) params.set('same_session', '1');
                    if (gapSec > 0) params.set('gap_sec', String(gapSec));
                    if (range && range[0]) params.set('start', range[0].toISOString());
                    if (range && range[1]) params.set('end', range[1].toISOString());
                    const url = `${location.pathname}?${params.toString()}`;
                    navigator.clipboard.writeText(`${location.origin}${url}`);
                  } catch {}
                }}
              >
                复制链接
              </Button>
            </Space>
          }
        >
          <Table<FunnelStep>
            size="small"
            pagination={false}
            dataSource={(funnel || []).map((s: FunnelStep, _i: number) => ({
              step: s.step,
              users: s.users,
              rate: s.rate,
            }))}
            columns={[
              { title: '步骤', dataIndex: 'step' },
              { title: '人数', dataIndex: 'users' },
              {
                title: '转化率',
                dataIndex: 'rate',
                render: (v: number) => (v != null ? `${v}%` : '-'),
              },
            ]}
          />
          <div style={{ marginTop: 8 }}>
            <Button
              onClick={async () => {
                const rowsOut = [['step', 'users', 'rate']].concat(
                  (funnel || []).map((s: FunnelStep) => [
                    String(s.step),
                    String(s.users),
                    String(s.rate),
                  ]),
                );
                await exportToXLSX('funnel.csv', [{ sheet: 'funnel', rows: rowsOut }]);
              }}
            >
              导出 CSV
            </Button>
          </div>
        </Card>

        <Card title="路径分析（TopN）">
          <PathControls
            range={range}
            currentSteps={steps}
            onUsePath={(p) => {
              setSteps(p);
              setTimeout(() => loadFunnel(p), 0);
              const el = document.getElementById('funnel-anchor');
              if (el) el.scrollIntoView({ behavior: 'smooth' });
            }}
          />
        </Card>

        <Card title="功能采用率">
          <AdoptionControls range={range} />
        </Card>
      </Space>
    </PageContainer>
  );
}
