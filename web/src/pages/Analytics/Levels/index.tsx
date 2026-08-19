import React, { useCallback, useEffect, useState } from 'react';
import {
  Card,
  Space,
  DatePicker,
  Input,
  Button,
  Table,
  Tag,
  Select,
  Statistic,
  Row,
  Col,
} from 'antd';
import type { Dayjs } from 'dayjs';
import { PageContainer } from '@ant-design/pro-components';
import { exportToXLSX } from '@/utils/export';
import {
  fetchAnalyticsLevels,
  fetchAnalyticsLevelsEpisodes,
  fetchAnalyticsLevelsMaps,
} from '@/services/api/analytics';

export default function AnalyticsLevelsPage() {
  const [loading, setLoading] = useState(false);
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null);
  const [episode, setEpisode] = useState<string>('');
  const [data, setData] = useState<LevelsData | null>(null);
  const [seg, setSeg] = useState<'all' | 'new' | 'returning' | 'payer'>('all');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = { episode };
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsLevels(params);
      const perLevel = (r?.levels || []).map((item) => ({
        level: item.levelId,
        players: item.attempts,
        winRate: item.completionRate * 100,
        avgDurationSec: item.avgDuration,
        avgRetries: item.avgRetries,
      }));
      setData({
        perLevel,
        perLevelSegments: {},
        funnel: perLevel.map((item) => ({
          step: item.level,
          users: item.players,
          rate: item.winRate,
        })),
      });
    } finally {
      setLoading(false);
    }
  }, [episode, range]);
  useEffect(() => {
    void load();
  }, [load]);

  const exportCSV = () => {
    try {
      const rows = [['level', 'players', 'win_rate', 'avg_duration_sec', 'avg_retries']].concat(
        (data?.perLevel || []).map((x) => [
          String(x.level),
          String(x.players),
          String(x.winRate),
          String(x.avgDurationSec || ''),
          String(x.avgRetries || ''),
        ]),
      );
      const csv = rows.map((r) => r.map((x) => String(x ?? '')).join(',')).join('\n');
      const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'levels.csv';
      a.click();
      URL.revokeObjectURL(url);
    } catch {}
  };

  return (
    <PageContainer>
      <Space direction="vertical" style={{ width: '100%' }}>
        <Card
          title="关卡漏斗"
          extra={
            <Space>
              <Input
                placeholder="章节/地图（可选）"
                value={episode}
                onChange={(e) => setEpisode(e.target.value)}
                style={{ width: 200 }}
              />
              <DatePicker.RangePicker value={range} onChange={(dates) => setRange(dates)} />
              <Button type="primary" onClick={load}>
                查询
              </Button>
              <Button onClick={exportCSV}>导出 CSV</Button>
            </Space>
          }
        >
          <Table
            size="small"
            loading={loading}
            rowKey={(r) => {
              const row = r as unknown as Record<string, unknown>;
              return `${row.step || ''}|${row.users || ''}`;
            }}
            dataSource={data?.funnel || []}
            columns={[
              { title: '步骤', dataIndex: 'step' },
              { title: '玩家数', dataIndex: 'users' },
              {
                title: '转化率',
                dataIndex: 'rate',
                render: (v) => (v != null ? `${v}%` : '-'),
              },
            ]}
            pagination={false}
          />
          <div style={{ marginTop: 8 }}>
            <Button
              onClick={async () => {
                const rows = [['step', 'users', 'rate']].concat(
                  (data?.funnel || []).map((x) => [
                    String(x.step),
                    String(x.users),
                    String(x.rate || ''),
                  ]),
                );
                await exportToXLSX('levels_funnel.csv', [{ sheet: 'funnel', rows }]);
              }}
            >
              导出 CSV
            </Button>
          </div>
        </Card>
        <Card
          title="分关卡统计（胜率/难度/时长/复试）"
          extra={
            <Space>
              <Select
                value={seg}
                onChange={(v) => setSeg(v)}
                options={[
                  { label: '全部', value: 'all' },
                  { label: '新玩家', value: 'new' },
                  { label: '回流/老玩家', value: 'returning' },
                  { label: '付费玩家', value: 'payer' },
                ]}
              />
            </Space>
          }
        >
          <Table
            size="small"
            loading={loading}
            rowKey={(r) => r.level}
            dataSource={
              seg === 'all' ? data?.perLevel || [] : (data?.perLevelSegments || {})[seg] || []
            }
            columns={[
              { title: '关卡', dataIndex: 'level' },
              { title: '参与人数', dataIndex: 'players' },
              {
                title: '胜率',
                dataIndex: 'winRate',
                render: (v: number) => (v != null ? `${v.toFixed(2)}%` : '-'),
              },
              { title: '平均通关时长(s)', dataIndex: 'avgDurationSec' },
              { title: '平均复试次数', dataIndex: 'avgRetries' },
              {
                title: '难度',
                render: (_: unknown, r: LevelData) => {
                  const v = r?.difficulty != null ? String(r.difficulty) : '-';
                  const color = v === '高' ? 'red' : v === '中' ? 'gold' : 'default';
                  return <Tag color={color}>{v}</Tag>;
                },
              },
            ]}
          />
          <div style={{ marginTop: 8 }}>
            <Button
              onClick={async () => {
                const allRows = [
                  ['level', 'players', 'win_rate', 'avg_duration_sec', 'avg_retries', 'difficulty'],
                ].concat(
                  (data?.perLevel || []).map((x) => [
                    String(x.level),
                    String(x.players),
                    String(x.winRate),
                    String(x.avgDurationSec || ''),
                    String(x.avgRetries || ''),
                    String(x.difficulty || ''),
                  ]),
                );
                const segs = data?.perLevelSegments || {};
                const mk = (name: string, arr: LevelData[]) => ({
                  sheet: `per_level_${name}`,
                  rows: [['level', 'players', 'win_rate']].concat(
                    (arr || []).map((x) => [String(x.level), String(x.players), String(x.winRate)]),
                  ),
                });
                const sheets = [{ sheet: 'per_level', rows: allRows }];
                sheets.push(mk('new', segs['new'] || []));
                sheets.push(mk('returning', segs['returning'] || []));
                sheets.push(mk('payer', segs['payer'] || []));
                await exportToXLSX('levels_stats.csv', sheets);
              }}
            >
              导出 CSV
            </Button>
          </div>
        </Card>
        <LevelsSegmentsChart data={data} />
        <EpisodeFacets range={range} />
        <MapFacets range={range} />
      </Space>
    </PageContainer>
  );
}

interface LevelData {
  level: string;
  players: number;
  winRate: number;
  avgDurationSec?: number;
  avgRetries?: number;
  difficulty?: number;
}

interface LevelSegments {
  new?: LevelData[];
  returning?: LevelData[];
  payer?: LevelData[];
}

interface LevelsData {
  perLevel?: LevelData[];
  perLevelSegments?: LevelSegments;
  funnel?: { step: string; users: number; rate?: number }[];
}

const LevelsSegmentsChart: React.FC<{ data: LevelsData | null }> = ({ data }) => {
  try {
    const all = data?.perLevel || [];
    const segs = data?.perLevelSegments || {};
    if (!all.length) return null;
    // pick top 10 levels by players from all
    const top = all
      .slice(0)
      .sort((a, b) => Number(b.players || 0) - Number(a.players || 0))
      .slice(0, 10);
    const levels = top.map((x) => String(x.level));
    const find = (arr: LevelData[], lvl: string) =>
      (arr || []).find((x) => String(x.level) === lvl) || { winRate: 0 };
    const rows = levels.map((lvl) => ({
      lvl,
      all: Number(find(all, lvl).winRate || 0),
      new: Number(find(segs['new'] || [], lvl).winRate || 0),
      ret: Number(find(segs['returning'] || [], lvl).winRate || 0),
      pay: Number(find(segs['payer'] || [], lvl).winRate || 0),
    }));
    const w = 720,
      h = 240,
      left = 40,
      bottom = 30,
      right = 10,
      topm = 10;
    const maxY = Math.max(100, ...rows.flatMap((r) => [r.all, r.new, r.ret, r.pay]));
    const sx = (i: number) => left + ((w - left - right) * i) / Math.max(1, levels.length - 1);
    const sy = (v: number) => topm + (h - topm - bottom) * (1 - v / Math.max(1, maxY));
    const pathOf = (key: 'all' | 'new' | 'ret' | 'pay', color: string) => {
      const d = rows.map((r, i) => `${i ? 'L' : 'M'}${sx(i)},${sy(r[key])}`).join(' ');
      return <path d={d} fill="none" stroke={color} strokeWidth={2} />;
    };
    return (
      <div style={{ marginTop: 12 }}>
        <b>分群胜率对比（Top 10 关卡）</b>
        <svg
          width={w}
          height={h}
          style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
        >
          {/* axes */}
          <line x1={left} y1={topm} x2={left} y2={h - bottom} stroke="#ccc" />
          <line x1={left} y1={h - bottom} x2={w - right} y2={h - bottom} stroke="#ccc" />
          {/* labels */}
          {levels.map((lv, i) => (
            <text key={lv} x={sx(i)} y={h - bottom + 14} fontSize={10} textAnchor="middle">
              {lv}
            </text>
          ))}
          <text x={4} y={topm + 10} fontSize={10}>
            胜率(%)
          </text>
          {/* lines */}
          {pathOf('all', '#1677ff')}
          {pathOf('new', '#52c41a')}
          {pathOf('ret', '#faad14')}
          {pathOf('pay', '#f5222d')}
          {/* legend */}
          <g>
            <rect x={w - right - 260} y={topm + 6} width={250} height={20} fill="#fff" />
            <circle cx={w - right - 250} cy={topm + 16} r={3} fill="#1677ff" />
            <text x={w - right - 242} y={topm + 20} fontSize={10}>
              全部
            </text>
            <circle cx={w - right - 200} cy={topm + 16} r={3} fill="#52c41a" />
            <text x={w - right - 192} y={topm + 20} fontSize={10}>
              新
            </text>
            <circle cx={w - right - 170} cy={topm + 16} r={3} fill="#faad14" />
            <text x={w - right - 162} y={topm + 20} fontSize={10}>
              回流
            </text>
            <circle cx={w - right - 120} cy={topm + 16} r={3} fill="#f5222d" />
            <text x={w - right - 112} y={topm + 20} fontSize={10}>
              付费
            </text>
          </g>
        </svg>
      </div>
    );
  } catch {
    return null;
  }
};

interface EpisodeData {
  episode: string;
  players: number;
  completionRate: number;
  avgProgress: number;
}

interface MapData {
  map: string;
  heatMap: Array<Record<string, number>>;
  deathSpots: Array<Record<string, number>>;
}

const EpisodeFacets: React.FC<{ range: [Dayjs | null, Dayjs | null] | null }> = ({ range }) => {
  const [episodes, setEpisodes] = useState<EpisodeData[]>([]);
  const [loading, setLoading] = useState(false);
  const [limit, setLimit] = useState(6);
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = {};
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsLevelsEpisodes(params);
      setEpisodes(
        (r?.episodes || []).map((item) => ({
          episode: item.episodeId,
          players: item.players,
          completionRate: item.completionRate * 100,
          avgProgress: item.avgProgress * 100,
        })),
      );
    } finally {
      setLoading(false);
    }
  }, [range]);
  useEffect(() => {
    void load();
  }, [load]);
  const exportExcel = async () => {
    try {
      const sheets: { sheet: string; rows: string[][] }[] = [];
      (episodes || []).forEach((e) => {
        const rows = [
          ['episode', 'players', 'completion_rate', 'avg_progress'],
          [String(e.episode), String(e.players), String(e.completionRate), String(e.avgProgress)],
        ];
        sheets.push({ sheet: `ep_${String(e.episode || '')}`, rows });
      });
      await exportToXLSX('levels_episodes.csv', sheets);
    } catch {}
  };
  return (
    <Card
      title="按章节分面"
      extra={
        <Space>
          <Button onClick={load} loading={loading}>
            加载
          </Button>
          <Select
            value={limit}
            onChange={(v) => setLimit(v)}
            options={[
              { label: '6', value: 6 },
              { label: '9', value: 9 },
              { label: '12', value: 12 },
            ]}
          />
          <Button onClick={exportExcel}>导出 Excel（多 Sheet）</Button>
        </Space>
      }
    >
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: `repeat(${Math.max(1, Math.ceil(Math.sqrt(limit)))}, 1fr)`,
          gap: 12,
        }}
      >
        {(episodes || []).slice(0, limit).map((e, idx: number) => (
          <EpisodeFacet key={idx} episode={e} />
        ))}
      </div>
    </Card>
  );
};

const MapFacets: React.FC<{ range: [Dayjs | null, Dayjs | null] | null }> = ({ range }) => {
  const [maps, setMaps] = useState<MapData[]>([]);
  const [loading, setLoading] = useState(false);
  const [limit, setLimit] = useState(6);
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = {};
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsLevelsMaps(params);
      setMaps(
        (r?.maps || []).map((item) => ({
          map: item.mapId,
          heatMap: Array.isArray(item.heatMap) ? item.heatMap : [],
          deathSpots: Array.isArray(item.deathSpots) ? item.deathSpots : [],
        })),
      );
    } finally {
      setLoading(false);
    }
  }, [range]);
  const exportExcel = async () => {
    try {
      const sheets: { sheet: string; rows: string[][] }[] = [];
      (maps || []).forEach((e) => {
        const rows = [
          ['map', 'heat_points', 'death_points'],
          [String(e.map), String(e.heatMap.length), String(e.deathSpots.length)],
        ];
        sheets.push({ sheet: `map_${String(e.map || '')}`, rows });
      });
      await exportToXLSX('levels_maps.csv', sheets);
    } catch {}
  };
  return (
    <Card
      title="按地图分面"
      extra={
        <Space>
          <Button onClick={load} loading={loading}>
            加载
          </Button>
          <Select
            value={limit}
            onChange={(v) => setLimit(v)}
            options={[
              { label: '6', value: 6 },
              { label: '9', value: 9 },
              { label: '12', value: 12 },
            ]}
          />
          <Button onClick={exportExcel}>导出 Excel</Button>
        </Space>
      }
    >
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: `repeat(${Math.max(1, Math.ceil(Math.sqrt(limit)))}, 1fr)`,
          gap: 12,
        }}
      >
        {(maps || []).slice(0, limit).map((e, idx: number) => (
          <MapFacet key={idx} item={e} />
        ))}
      </div>
    </Card>
  );
};

const MapFacet: React.FC<{ item: MapData }> = ({ item }) => {
  return (
    <Card size="small" title={item.map || '-'}>
      <Row gutter={8}>
        <Col span={12}>
          <Statistic title="热力点" value={item.heatMap.length} />
        </Col>
        <Col span={12}>
          <Statistic title="死亡点" value={item.deathSpots.length} />
        </Col>
      </Row>
    </Card>
  );
};

const EpisodeFacet: React.FC<{ episode: EpisodeData }> = ({ episode }) => {
  return (
    <Card size="small" title={episode.episode || '-'}>
      <Row gutter={8}>
        <Col span={8}>
          <Statistic title="玩家" value={episode.players} />
        </Col>
        <Col span={8}>
          <Statistic title="完成率" value={episode.completionRate} suffix="%" precision={2} />
        </Col>
        <Col span={8}>
          <Statistic title="平均进度" value={episode.avgProgress} suffix="%" precision={2} />
        </Col>
      </Row>
    </Card>
  );
};
