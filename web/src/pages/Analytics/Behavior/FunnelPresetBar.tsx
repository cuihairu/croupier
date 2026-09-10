import React, { useCallback, useEffect, useState } from 'react';
import { Button, Input, Modal, Select, Space, Table } from 'antd';
import type { Dayjs } from 'dayjs';
import { FormattedMessage, useIntl } from '@umijs/max';
import { formatDateTime } from '@/utils/format';
import type { FunnelPreset, ImportRow } from './types';

/** 漏斗预设条：localStorage 持久化的漏斗条件预设（保存/应用/重命名/
 * 导入导出/清空）。已实现但当前未挂载到漏斗卡片（保持与拆分前一致）。 */
const PresetBar: React.FC<{
  steps: string[];
  seq: boolean;
  sameSess: boolean;
  gapSec: number;
  range: [Dayjs | null, Dayjs | null] | null;
  onApply: (p: Record<string, string | number | boolean>) => void;
}> = ({ steps, seq, sameSess, gapSec, range, onApply }) => {
  const intl = useIntl();
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
      const input = prompt(
        intl.formatMessage({
          id: 'pages.analyticsBehavior.funnel.preset.prompt.name',
          defaultMessage: '预设名称',
        }),
        sel || '',
      );
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
        const ok = confirm(
          intl.formatMessage(
            {
              id: 'pages.analyticsBehavior.funnel.preset.confirm.overwrite',
              defaultMessage: `预设 "${name}" 已存在，是否覆盖？`,
            },
            { name },
          ),
        );
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
      const ok = confirm(
        intl.formatMessage(
          {
            id: 'pages.analyticsBehavior.funnel.preset.confirm.delete',
            defaultMessage: `删除预设 ${sel}？`,
          },
          { name: sel },
        ),
      );
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
      const input = prompt(
        intl.formatMessage({
          id: 'pages.analyticsBehavior.funnel.preset.prompt.rename',
          defaultMessage: '重命名预设',
        }),
        sel || '',
      );
      if (!input) return;
      const newName = input.trim();
      if (!newName) return;
      if (arr.some((x) => x.name === newName && x.name !== sel)) {
        const ok = confirm(
          intl.formatMessage(
            {
              id: 'pages.analyticsBehavior.funnel.preset.confirm.renameOverwrite',
              defaultMessage: `已存在名为 "${newName}" 的预设，确定覆盖为该名称？`,
            },
            { name: newName },
          ),
        );
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
      const ok = confirm(
        intl.formatMessage({
          id: 'pages.analyticsBehavior.funnel.preset.confirm.clearAll',
          defaultMessage: '清空全部预设？',
        }),
      );
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
        alert(
          intl.formatMessage({
            id: 'pages.analyticsBehavior.funnel.preset.alert.invalidJson',
            defaultMessage: 'JSON 需为数组',
          }),
        );
        return;
      }
      const cur = readAll();
      const names = new Set(cur.map((x) => x.name));
      const list = arr
        .filter((x) => x && x.name)
        .map((x: FunnelPreset, i: number) => ({
          key: x.name || String(i),
          name: String(x.name),
          status: names.has(x.name)
            ? intl.formatMessage({
                id: 'pages.analyticsBehavior.funnel.preset.status.overwrite',
                defaultMessage: '覆盖',
              })
            : intl.formatMessage({
                id: 'pages.analyticsBehavior.funnel.preset.status.added',
                defaultMessage: '新增',
              }),
          obj: x,
        }));
      setImpList(list);
      setImpSel(list.map((x) => x.key));
    } catch {
      alert(
        intl.formatMessage({
          id: 'pages.analyticsBehavior.funnel.preset.alert.parseFailed',
          defaultMessage: '解析失败',
        }),
      );
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
      alert(
        intl.formatMessage({
          id: 'pages.analyticsBehavior.funnel.preset.alert.importFailed',
          defaultMessage: '导入失败',
        }),
      );
    }
  };

  return (
    <div style={{ marginTop: 8 }}>
      <Space>
        <Select
          placeholder={intl.formatMessage({
            id: 'pages.analyticsBehavior.funnel.preset.filter.placeholder.select',
            defaultMessage: '选择预设',
          })}
          value={sel}
          onChange={(v) => setSel(v)}
          options={(list || []).map((x) => ({
            label: `${x.name}${x.lastUsed ? ' · ' + formatDateTime(x.lastUsed) : ''}`,
            value: x.name,
          }))}
          style={{ minWidth: 260 }}
        />
        <Button onClick={applyPreset} disabled={!sel}>
          <FormattedMessage
            id="pages.analyticsBehavior.funnel.preset.button.apply"
            defaultMessage="应用预设"
          />
        </Button>
        <Button onClick={savePreset}>
          <FormattedMessage
            id="pages.analyticsBehavior.funnel.preset.button.save"
            defaultMessage="保存为预设"
          />
        </Button>
        <Button onClick={renamePreset} disabled={!sel}>
          <FormattedMessage
            id="pages.analyticsBehavior.funnel.preset.button.rename"
            defaultMessage="重命名"
          />
        </Button>
        <Button danger onClick={delPreset} disabled={!sel}>
          <FormattedMessage
            id="pages.analyticsBehavior.funnel.preset.button.delete"
            defaultMessage="删除预设"
          />
        </Button>
        <Button onClick={exportPresets}>
          <FormattedMessage
            id="pages.analyticsBehavior.funnel.preset.button.exportAll"
            defaultMessage="导出全部"
          />
        </Button>
        <Button onClick={exportOne} disabled={!sel}>
          <FormattedMessage
            id="pages.analyticsBehavior.funnel.preset.button.exportCurrent"
            defaultMessage="导出当前"
          />
        </Button>
        <Button onClick={openImport}>
          <FormattedMessage
            id="pages.analyticsBehavior.funnel.preset.button.import"
            defaultMessage="导入预设"
          />
        </Button>
        <Button danger onClick={clearAll}>
          <FormattedMessage
            id="pages.analyticsBehavior.funnel.preset.button.clearAll"
            defaultMessage="清空全部"
          />
        </Button>
      </Space>
      <Modal
        open={impOpen}
        title={intl.formatMessage({
          id: 'pages.analyticsBehavior.funnel.preset.modalTitle.import',
          defaultMessage: '导入预设（预览）',
        })}
        onOk={doImport}
        onCancel={() => setImpOpen(false)}
        width={720}
        okText={intl.formatMessage({
          id: 'pages.analyticsBehavior.funnel.preset.okText',
          defaultMessage: '合并导入',
        })}
        cancelText={intl.formatMessage({
          id: 'pages.analyticsBehavior.funnel.preset.cancelText',
          defaultMessage: '取消',
        })}
      >
        <div style={{ marginBottom: 8 }}>
          <Input.TextArea
            rows={6}
            placeholder={intl.formatMessage({
              id: 'pages.analyticsBehavior.funnel.preset.filter.placeholder.importJson',
              defaultMessage:
                "粘贴 JSON 数组（'{'{name,steps,seq,sameSess,gapSec,start,end}'}', ...）",
            })}
            value={impText}
            onChange={(e) => setImpText(e.target.value)}
          />
          <div style={{ marginTop: 8 }}>
            <Button onClick={parseImport}>
              <FormattedMessage
                id="pages.analyticsBehavior.funnel.preset.button.parse"
                defaultMessage="解析"
              />
            </Button>
          </div>
        </div>
        {impList.length > 0 && (
          <Table<ImportRow>
            size="small"
            rowKey={(r: ImportRow) => r.key}
            dataSource={impList}
            rowSelection={{ selectedRowKeys: impSel, onChange: (keys) => setImpSel(keys) }}
            columns={[
              {
                title: intl.formatMessage({
                  id: 'pages.analyticsBehavior.funnel.preset.column.name',
                  defaultMessage: '名称',
                }),
                dataIndex: 'name',
              },
              {
                title: intl.formatMessage({
                  id: 'pages.analyticsBehavior.funnel.preset.action',
                  defaultMessage: '动作',
                }),
                dataIndex: 'status',
              },
            ]}
            pagination={false}
          />
        )}
      </Modal>
    </div>
  );
};

export default PresetBar;
