import React, { useEffect, useState, useCallback } from 'react';
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
  Tag,
  Modal,
} from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { PageContainer } from '@ant-design/pro-components';
import { exportToXLSX } from '@/utils/export';
import {
  fetchAnalyticsEvents,
  fetchAnalyticsFunnel,
  fetchAnalyticsPaths,
  fetchAnalyticsAdoption,
  fetchAnalyticsAdoptionBreakdown,
} from '@/services/api/analytics';
import type { JSONValue } from '@/types/dashboard';

interface EventRow {
  id?: string;
  event?: string;
  time?: string;
  userId?: string;
  [key: string]: JSONValue | undefined;
}

interface FunnelStep {
  step: string;
  users: number;
  rate: number;
}

interface PathRow {
  path: string;
  groups: number;
}

interface AdoptionRow {
  feature: string;
  groups: number;
  rate: number;
}

interface AdoptionBreakdownRow {
  dim: string;
  baseline: number;
  groups: number;
  rate: number;
}

interface FunnelPreset {
  name: string;
  steps: string;
  sequential: number;
  sameSession: number;
  gapSec: number;
  start?: string;
  end?: string;
  lastUsed?: string;
  [key: string]: string | number | boolean | undefined;
}

interface ImportRow {
  key: string;
  name: string;
  status: string;
  obj: FunnelPreset;
}

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

const PathControls: React.FC<{
  range: [Dayjs | null, Dayjs | null] | null;
  currentSteps?: string[];
  onUsePath?: (steps: string[]) => void;
}> = ({ range, currentSteps, onUsePath }) => {
  const [per, setPer] = useState<'session' | 'user'>('session');
  const [steps, setSteps] = useState<number>(5);
  const [limit, setLimit] = useState<number>(50);
  const [include, setInclude] = useState<string[]>([]);
  const [exclude, setExclude] = useState<string[]>([]);
  const [sameSess, setSameSess] = useState<boolean>(false);
  const [gapSec, setGapSec] = useState<number>(0);
  const [pathRe, setPathRe] = useState<string>('');
  const [pathNotRe, setPathNotRe] = useState<string>('');
  const [rows, setRows] = useState<PathRow[]>([]);
  const [loading, setLoading] = useState(false);
  const load = async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = { per, steps, limit };
      if (include.length > 0) params.include = include.join(',');
      if (exclude.length > 0) params.exclude = exclude.join(',');
      if (sameSess) params.sameSession = 1;
      if (gapSec && gapSec > 0) params.gapSec = gapSec;
      if (pathRe.trim()) params.pathRe = pathRe.trim();
      if (pathNotRe.trim()) params.pathNotRe = pathNotRe.trim();
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsPaths(params);
      setRows(r?.paths || []);
    } finally {
      setLoading(false);
    }
  };
  return (
    <Space orientation="vertical" style={{ width: '100%' }}>
      <Space>
        <Select
          value={per}
          onChange={(v) => setPer(v)}
          options={[
            { label: '按会话', value: 'session' },
            { label: '按用户', value: 'user' },
          ]}
        />
        <InputNumber
          value={steps}
          onChange={(v) => setSteps(Number(v || 5))}
          min={1}
          max={10}
          addonBefore="步数"
        />
        <InputNumber
          value={limit}
          onChange={(v) => setLimit(Number(v || 50))}
          min={10}
          max={500}
          addonBefore="TopN"
        />
        <Select
          mode="tags"
          value={include}
          onChange={(v) => setInclude(v)}
          placeholder="包含事件"
          style={{ minWidth: 200 }}
        />
        <Select
          mode="tags"
          value={exclude}
          onChange={(v) => setExclude(v)}
          placeholder="排除事件"
          style={{ minWidth: 200 }}
        />
        <Checkbox checked={sameSess} onChange={(e) => setSameSess(e.target.checked)}>
          同会话
        </Checkbox>
        <InputNumber
          value={gapSec}
          onChange={(v) => setGapSec(Number(v || 0))}
          min={0}
          addonBefore="步间秒数"
        />
        <Input
          placeholder="路径包含正则"
          value={pathRe}
          onChange={(e) => setPathRe(e.target.value)}
          style={{ width: 200 }}
        />
        <Input
          placeholder="路径排除正则"
          value={pathNotRe}
          onChange={(e) => setPathNotRe(e.target.value)}
          style={{ width: 200 }}
        />
        <Button type="primary" onClick={load} loading={loading}>
          计算路径
        </Button>
        <Button
          onClick={async () => {
            const rowsOut = [['path', 'groups']].concat(
              (rows || []).map((r: PathRow) => [String(r.path || ''), String(r.groups || '')]),
            );
            await exportToXLSX('paths.csv', [{ sheet: 'paths', rows: rowsOut }]);
          }}
        >
          导出 CSV
        </Button>
      </Space>
      {(() => {
        try {
          const p = (currentSteps || []).join('>');
          if (!p || !pathRe.trim()) return null;
          const ok = new RegExp(pathRe).test(p);
          return (
            <div>
              与当前漏斗步骤匹配：<Tag color={ok ? 'green' : 'red'}>{ok ? '是' : '否'}</Tag>
            </div>
          );
        } catch {
          return null;
        }
      })()}
      <Table<PathRow>
        size="small"
        loading={loading}
        rowKey={(r: PathRow) => `${r.path || ''}|${r.groups || ''}`}
        dataSource={rows}
        columns={[
          { title: '路径', dataIndex: 'path' },
          { title: '分组数', dataIndex: 'groups' },
          {
            title: '操作',
            render: (_: unknown, r: PathRow) => (
              <Space>
                <Button
                  size="small"
                  onClick={() => onUsePath && onUsePath(String(r.path || '').split('>'))}
                >
                  填充漏斗
                </Button>
                <Button
                  size="small"
                  onClick={() => {
                    try {
                      navigator.clipboard.writeText(String(r.path || ''));
                    } catch {}
                  }}
                >
                  复制步骤
                </Button>
              </Space>
            ),
          },
        ]}
        pagination={{ pageSize: 10 }}
      />
    </Space>
  );
};

// Preset bar for funnel
// eslint-disable-next-line @typescript-eslint/no-unused-vars
const PresetBar: React.FC<{
  steps: string[];
  seq: boolean;
  sameSess: boolean;
  gapSec: number;
  range: [Dayjs | null, Dayjs | null] | null;
  onApply: (p: Record<string, string | number | boolean>) => void;
}> = ({ steps, seq, sameSess, gapSec, range, onApply }) => {
  const KEY = 'analytics:funnel_presets';
  const [list, setList] = useState<FunnelPreset[]>([]);
  const [sel, setSel] = useState<string>('');

  const readAll = (): FunnelPreset[] => {
    try {
      const txt = localStorage.getItem(KEY);
      const arr = txt ? JSON.parse(txt) : [];
      return Array.isArray(arr) ? arr : [];
    } catch {
      return [];
    }
  };
  const writeAll = (arr: FunnelPreset[]) => {
    try {
      localStorage.setItem(KEY, JSON.stringify(arr));
    } catch {}
  };
  const sortPresets = (arr: FunnelPreset[]) => {
    return [...arr].sort((a, b) => {
      const ta = a.lastUsed ? new Date(a.lastUsed).getTime() : 0;
      const tb = b.lastUsed ? new Date(b.lastUsed).getTime() : 0;
      if (tb !== ta) return tb - ta;
      return String(a.name || '').localeCompare(String(b.name || ''));
    });
  };
  const loadList = useCallback(() => {
    setList(sortPresets(readAll()));
  }, []);
  useEffect(() => {
    loadList();
  }, [loadList]);

  const savePreset = () => {
    try {
      const input = prompt('预设名称', sel || '');
      if (!input) return;
      const name = input.trim();
      if (!name) return;
      const nowObj: FunnelPreset = {
        name,
        steps: steps.join(','),
        sequential: seq ? 1 : 0,
        sameSession: sameSess ? 1 : 0,
        gapSec: gapSec,
        start: range?.[0]?.toISOString?.(),
        end: range?.[1]?.toISOString?.(),
        lastUsed: new Date().toISOString(),
      };
      const arr = readAll();
      const exists = arr.find((x) => x.name === name);
      if (exists) {
        const ok = confirm(`预设 "${name}" 已存在，是否覆盖？`);
        if (!ok) return;
      }
      const others = arr.filter((x) => x.name !== name);
      writeAll(sortPresets([...others, nowObj]));
      setSel(name);
      loadList();
    } catch {}
  };

  const applyPreset = () => {
    try {
      if (!sel) return;
      const arr = readAll();
      const found = arr.find((x) => x.name === sel);
      if (!found) return;
      // update lastUsed
      found.lastUsed = new Date().toISOString();
      writeAll(sortPresets([...arr.filter((x) => x.name !== sel), found]));
      loadList();
      // 过滤掉 undefined 值，确保类型安全
      const cleaned: Record<string, string | number | boolean> = {};
      Object.entries(found).forEach(([key, value]) => {
        if (value !== undefined) {
          cleaned[key] = value;
        }
      });
      onApply(cleaned);
    } catch {}
  };

  const delPreset = () => {
    try {
      if (!sel) return;
      const ok = confirm(`删除预设 ${sel}？`);
      if (!ok) return;
      const arr = readAll();
      writeAll(arr.filter((x) => x.name !== sel));
      setSel('');
      loadList();
    } catch {}
  };

  const renamePreset = () => {
    try {
      if (!sel) return;
      const arr = readAll();
      const found = arr.find((x) => x.name === sel);
      if (!found) return;
      const input = prompt('重命名预设', sel || '');
      if (!input) return;
      const newName = input.trim();
      if (!newName) return;
      if (arr.some((x) => x.name === newName && x.name !== sel)) {
        const ok = confirm(`已存在名为 "${newName}" 的预设，确定覆盖为该名称？`);
        if (!ok) return;
        // remove target name
        for (let i = arr.length - 1; i >= 0; i--) if (arr[i].name === newName) arr.splice(i, 1);
      }
      found.name = newName;
      found.lastUsed = new Date().toISOString();
      writeAll(sortPresets(arr));
      setSel(newName);
      loadList();
    } catch {}
  };

  const exportOne = () => {
    try {
      if (!sel) return;
      const arr = readAll();
      const found = arr.find((x) => x.name === sel);
      if (!found) return;
      const blob = new Blob([JSON.stringify([found], null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `funnel_preset_${sel}.json`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {}
  };

  const clearAll = () => {
    try {
      const ok = confirm('清空全部预设？');
      if (!ok) return;
      writeAll([]);
      setSel('');
      loadList();
    } catch {}
  };

  const exportPresets = () => {
    try {
      const arr = readAll();
      const blob = new Blob([JSON.stringify(arr, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'funnel_presets.json';
      a.click();
      URL.revokeObjectURL(url);
    } catch {}
  };
  // Import preview modal
  const [impOpen, setImpOpen] = useState(false);
  const [impText, setImpText] = useState('');
  const [impList, setImpList] = useState<ImportRow[]>([]);
  const [impSel, setImpSel] = useState<React.Key[]>([]);
  const openImport = () => {
    setImpText('');
    setImpList([]);
    setImpSel([]);
    setImpOpen(true);
  };
  const parseImport = () => {
    try {
      const arr = JSON.parse(impText);
      if (!Array.isArray(arr)) {
        alert('JSON 需为数组');
        return;
      }
      const cur = readAll();
      const names = new Set(cur.map((x) => x.name));
      const list = arr
        .filter((x) => x && x.name)
        .map((x: FunnelPreset, i: number) => ({
          key: x.name || String(i),
          name: String(x.name),
          status: names.has(x.name) ? '覆盖' : '新增',
          obj: x,
        }));
      setImpList(list);
      setImpSel(list.map((x) => x.key));
    } catch {
      alert('解析失败');
    }
  };
  const doImport = () => {
    try {
      const cur = readAll();
      const map: Record<string, FunnelPreset> = {};
      cur.forEach((x) => (map[x.name] = x));
      impList.forEach((row) => {
        if (impSel.includes(row.key)) {
          map[row.name] = row.obj as FunnelPreset;
        }
      });
      writeAll(sortPresets(Object.values(map)));
      setImpOpen(false);
      setSel('');
      loadList();
    } catch {
      alert('导入失败');
    }
  };

  return (
    <div style={{ marginTop: 8 }}>
      <Space>
        <Select
          placeholder="选择预设"
          value={sel}
          onChange={(v) => setSel(v)}
          options={(list || []).map((x) => ({
            label: `${x.name}${x.lastUsed ? ' · ' + new Date(x.lastUsed).toLocaleString() : ''}`,
            value: x.name,
          }))}
          style={{ minWidth: 260 }}
        />
        <Button onClick={applyPreset} disabled={!sel}>
          应用预设
        </Button>
        <Button onClick={savePreset}>保存为预设</Button>
        <Button onClick={renamePreset} disabled={!sel}>
          重命名
        </Button>
        <Button danger onClick={delPreset} disabled={!sel}>
          删除预设
        </Button>
        <Button onClick={exportPresets}>导出全部</Button>
        <Button onClick={exportOne} disabled={!sel}>
          导出当前
        </Button>
        <Button onClick={openImport}>导入预设</Button>
        <Button danger onClick={clearAll}>
          清空全部
        </Button>
      </Space>
      <Modal
        open={impOpen}
        title="导入预设（预览）"
        onOk={doImport}
        onCancel={() => setImpOpen(false)}
        width={720}
        okText="合并导入"
        cancelText="取消"
      >
        <div style={{ marginBottom: 8 }}>
          <Input.TextArea
            rows={6}
            placeholder="粘贴 JSON 数组（[{name,steps,seq,sameSess,gapSec,start,end}, ...]）"
            value={impText}
            onChange={(e) => setImpText(e.target.value)}
          />
          <div style={{ marginTop: 8 }}>
            <Button onClick={parseImport}>解析</Button>
          </div>
        </div>
        {impList.length > 0 && (
          <Table<ImportRow>
            size="small"
            rowKey={(r: ImportRow) => r.key}
            dataSource={impList}
            rowSelection={{ selectedRowKeys: impSel, onChange: (keys) => setImpSel(keys) }}
            columns={[
              { title: '名称', dataIndex: 'name' },
              { title: '动作', dataIndex: 'status' },
            ]}
            pagination={false}
          />
        )}
      </Modal>
    </div>
  );
};

const AdoptionControls: React.FC<{ range: [Dayjs | null, Dayjs | null] | null }> = ({ range }) => {
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
          placeholder="功能事件（如：first_pay, open_store）"
          style={{ minWidth: 360 }}
        />
        <Select
          value={per}
          onChange={(v) => setPer(v)}
          options={[
            { label: '按用户', value: 'user' },
            { label: '按会话', value: 'session' },
          ]}
        />
        <Button type="primary" onClick={load} loading={loading}>
          计算采用率
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
          导出 CSV
        </Button>
      </Space>
      <div>
        基数（{per === 'user' ? '用户' : '会话'}）：{baseline}
      </div>
      <Table<AdoptionRow>
        size="small"
        loading={loading}
        rowKey={(r: AdoptionRow) => r.feature}
        dataSource={rows}
        columns={[
          { title: '功能事件', dataIndex: 'feature' },
          { title: '分组数', dataIndex: 'groups' },
          {
            title: '采用率',
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
            { label: '按渠道', value: 'channel' },
            { label: '按平台', value: 'platform' },
            { label: '按国家', value: 'country' },
          ]}
        />
        <Button onClick={loadDim} loading={loading}>
          分层采用率
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
          导出 CSV
        </Button>
      </Space>
      <Table<AdoptionBreakdownRow>
        size="small"
        loading={loading}
        rowKey={(r: AdoptionBreakdownRow) => `${r.dim || ''}|${r.baseline || ''}|${r.groups || ''}`}
        dataSource={rowsDim}
        columns={[
          { title: '分层', dataIndex: 'dim' },
          { title: '基数', dataIndex: 'baseline' },
          { title: '分组数', dataIndex: 'groups' },
          {
            title: '采用率',
            dataIndex: 'rate',
            render: (v: number) => (v != null ? `${v}%` : '-'),
          },
        ]}
        pagination={{ pageSize: 10 }}
      />
    </Space>
  );
};
