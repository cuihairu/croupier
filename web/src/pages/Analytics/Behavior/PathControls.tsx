import React, { useState } from 'react';
import { Button, Checkbox, Input, InputNumber, Select, Space, Table, Tag } from 'antd';
import type { Dayjs } from 'dayjs';
import { FormattedMessage, useIntl } from '@umijs/max';
import { exportToXLSX } from '@/utils/export';
import { fetchAnalyticsPaths } from '@/services/api/analytics';
import type { PathRow } from './types';

/** 路径分析（TopN）控件：筛选条件 + 路径表；
 * 「填充漏斗」把选中路径回填给漏斗卡片的 steps。 */
const PathControls: React.FC<{
  range: [Dayjs | null, Dayjs | null] | null;
  currentSteps?: string[];
  onUsePath?: (steps: string[]) => void;
}> = ({ range, currentSteps, onUsePath }) => {
  const intl = useIntl();
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
            {
              label: intl.formatMessage({
                id: 'pages.analyticsBehavior.option.bySession',
                defaultMessage: '按会话',
              }),
              value: 'session',
            },
            {
              label: intl.formatMessage({
                id: 'pages.analyticsBehavior.option.byUser',
                defaultMessage: '按用户',
              }),
              value: 'user',
            },
          ]}
        />
        <InputNumber
          value={steps}
          onChange={(v) => setSteps(Number(v || 5))}
          min={1}
          max={10}
          addonBefore={intl.formatMessage({
            id: 'pages.analyticsBehavior.path.filter.addonBefore.steps',
            defaultMessage: '步数',
          })}
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
          placeholder={intl.formatMessage({
            id: 'pages.analyticsBehavior.path.filter.placeholder.include',
            defaultMessage: '包含事件',
          })}
          style={{ minWidth: 200 }}
        />
        <Select
          mode="tags"
          value={exclude}
          onChange={(v) => setExclude(v)}
          placeholder={intl.formatMessage({
            id: 'pages.analyticsBehavior.path.filter.placeholder.exclude',
            defaultMessage: '排除事件',
          })}
          style={{ minWidth: 200 }}
        />
        <Checkbox checked={sameSess} onChange={(e) => setSameSess(e.target.checked)}>
          <FormattedMessage
            id="pages.analyticsBehavior.funnel.sameSession"
            defaultMessage="同会话"
          />
        </Checkbox>
        <InputNumber
          value={gapSec}
          onChange={(v) => setGapSec(Number(v || 0))}
          min={0}
          addonBefore={intl.formatMessage({
            id: 'pages.analyticsBehavior.path.filter.addonBefore.gapSec',
            defaultMessage: '步间秒数',
          })}
        />
        <Input
          placeholder={intl.formatMessage({
            id: 'pages.analyticsBehavior.path.filter.placeholder.pathRe',
            defaultMessage: '路径包含正则',
          })}
          value={pathRe}
          onChange={(e) => setPathRe(e.target.value)}
          style={{ width: 200 }}
        />
        <Input
          placeholder={intl.formatMessage({
            id: 'pages.analyticsBehavior.path.filter.placeholder.pathNotRe',
            defaultMessage: '路径排除正则',
          })}
          value={pathNotRe}
          onChange={(e) => setPathNotRe(e.target.value)}
          style={{ width: 200 }}
        />
        <Button type="primary" onClick={load} loading={loading}>
          <FormattedMessage
            id="pages.analyticsBehavior.path.button.compute"
            defaultMessage="计算路径"
          />
        </Button>
        <Button
          onClick={async () => {
            const rowsOut = [['path', 'groups']].concat(
              (rows || []).map((r: PathRow) => [String(r.path || ''), String(r.groups || '')]),
            );
            await exportToXLSX('paths.csv', [{ sheet: 'paths', rows: rowsOut }]);
          }}
        >
          <FormattedMessage
            id="pages.analyticsBehavior.button.exportCsv"
            defaultMessage="导出 CSV"
          />
        </Button>
      </Space>
      {(() => {
        try {
          const p = (currentSteps || []).join('>');
          if (!p || !pathRe.trim()) return null;
          const ok = new RegExp(pathRe).test(p);
          return (
            <div>
              <FormattedMessage
                id="pages.analyticsBehavior.path.matchFunnel.label"
                defaultMessage="与当前漏斗步骤匹配："
              />
              <Tag color={ok ? 'green' : 'red'}>
                {ok
                  ? intl.formatMessage({
                      id: 'pages.analyticsBehavior.path.matchFunnel.yes',
                      defaultMessage: '是',
                    })
                  : intl.formatMessage({
                      id: 'pages.analyticsBehavior.path.matchFunnel.no',
                      defaultMessage: '否',
                    })}
              </Tag>
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
          {
            title: intl.formatMessage({
              id: 'pages.analyticsBehavior.path.column.path',
              defaultMessage: '路径',
            }),
            dataIndex: 'path',
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
              id: 'pages.analyticsBehavior.path.column.actions',
              defaultMessage: '操作',
            }),
            render: (_: unknown, r: PathRow) => (
              <Space>
                <Button
                  size="small"
                  onClick={() => onUsePath && onUsePath(String(r.path || '').split('>'))}
                >
                  <FormattedMessage
                    id="pages.analyticsBehavior.path.button.fillFunnel"
                    defaultMessage="填充漏斗"
                  />
                </Button>
                <Button
                  size="small"
                  onClick={() => {
                    try {
                      navigator.clipboard.writeText(String(r.path || ''));
                    } catch {}
                  }}
                >
                  <FormattedMessage
                    id="pages.analyticsBehavior.path.button.copySteps"
                    defaultMessage="复制步骤"
                  />
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

export default PathControls;
