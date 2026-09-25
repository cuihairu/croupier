import React, { useEffect, useMemo, useState, useCallback } from 'react';
import { Card, Table, Space, Input, Button, DatePicker, Tag, Row, Col, Typography } from 'antd';
import type { Dayjs } from 'dayjs';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { auditRowKey, listAudit, type AuditEvent } from '@/services/api';
import { exportToCSV } from '@/utils/export';
import { formatDateTime } from '@/utils/format';

/** 过滤区小标签：控件分组可视化，全部走 i18n（禁止硬编码中文）。 */
function FilterLabel({ id, defaultMessage }: { id: string; defaultMessage: string }) {
  return (
    <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>
      <FormattedMessage id={id} defaultMessage={defaultMessage} />
    </Typography.Text>
  );
}

export default function LoginLogsPage() {
  const intl = useIntl();
  const [rows, setRows] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [actor, setActor] = useState<string>(
    () => new URLSearchParams(location.search).get('actor') || '',
  );
  const [ip, setIP] = useState<string>('');
  const [kinds, setKinds] = useState<string[]>(['login', 'login_fail', 'login_rate_limited']);
  const [timeRange, setTimeRange] = useState<[Dayjs | null, Dayjs | null] | null>(null);
  const [page, setPage] = useState<number>(1);
  const [size, setSize] = useState<number>(20);
  const [osSel, setOsSel] = useState<string[]>([]);
  const [brSel, setBrSel] = useState<string[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = { page, size };
      if (actor) params.actor = actor;
      if (ip) params.ip = ip;
      const want = kinds && kinds.length > 0 ? kinds : ['login'];
      params.kinds = want.join(',');
      if (timeRange && timeRange[0]) params.start = timeRange[0].toISOString();
      if (timeRange && timeRange[1]) params.end = timeRange[1].toISOString();
      const r = await listAudit(params);
      setRows(r.events || []);
    } finally {
      setLoading(false);
    }
  }, [page, size, actor, ip, kinds, timeRange]);

  useEffect(() => {
    load();
  }, [load]);

  const filtered = useMemo(() => {
    const osMatch = (ua: string) => {
      if (!osSel || osSel.length === 0) return true;
      const os = detectOS(ua);
      return osSel.includes(os);
    };
    const brMatch = (ua: string) => {
      if (!brSel || brSel.length === 0) return true;
      const br = detectBrowser(ua);
      return brSel.includes(br);
    };
    return (rows || []).filter((e: AuditEvent) => {
      const ua = String(e.meta?.ua || '');
      return osMatch(ua) && brMatch(ua);
    });
  }, [rows, osSel, brSel]);

  const paged = useMemo(() => {
    const start = (page - 1) * size;
    return filtered.slice(start, start + size);
  }, [filtered, page, size]);

  /**
   * 当前页的表格数据：预先算好行 key。
   *
   * 不在 `rowKey` 回调里用 index 兜底——antd 6 已废弃该参数
   * （`index` parameter of `rowKey` function is deprecated），会在 dev 模式
   * 刷告警。改为在切片时就把 key 落到行上，Table 用字符串 rowKey 读取。
   * 详见 docs/BUGS.md BUG-011。
   */
  const pagedWithKey = useMemo(
    () => paged.map((e, i) => ({ ...e, __rowKey: auditRowKey(e, i) })),
    [paged],
  );

  const exportCSV = () => {
    const arr = (filtered || []).map((e: AuditEvent) => {
      const ua = String(e.meta?.ua || '');
      return [
        new Date(e.time).toISOString(),
        e.kind,
        e.actor,
        String(e.meta?.ip || ''),
        String(e.meta?.ipRegion || ''),
        ua,
        detectOS(ua),
        detectBrowser(ua),
      ];
    });
    arr.unshift(['time', 'kind', 'actor', 'ip', 'region', 'ua', 'os', 'browser']);
    exportToCSV('login_logs.csv', arr);
  };

  function detectOS(ua: string): string {
    const s = String(ua || '');
    if (/android/i.test(s)) return 'Android';
    if (/iphone|ipad|ipod/i.test(s)) return 'iOS';
    if (/windows nt/i.test(s)) return 'Windows';
    if (/mac os x/i.test(s)) return 'macOS';
    if (/linux/i.test(s)) return 'Linux';
    return s ? 'Other' : '';
  }
  function detectBrowser(ua: string): string {
    const s = String(ua || '');
    if (/edg\//i.test(s)) return 'Edge';
    if (/chrome\//i.test(s)) return 'Chrome';
    if (/safari\//i.test(s) && !/chrome\//i.test(s)) return 'Safari';
    if (/firefox\//i.test(s)) return 'Firefox';
    return s ? 'Other' : '';
  }

  return (
    <PageContainer>
      <Card
        title={intl.formatMessage({
          id: 'pages.adminLogs.loginLog.title',
          defaultMessage: '登录日志',
        })}
      >
        {/* 过滤区分三行：输入/时间/操作 → 类型 Tag → 设备/浏览器 Tag（窄屏 Col 自动换行） */}
        <div data-testid="login-log-filters">
          <Row gutter={[12, 12]} align="bottom" style={{ marginBottom: 16 }}>
            <Col xs={24} sm={12} md={5}>
              <FilterLabel id="pages.adminLogs.loginLog.filter.actor" defaultMessage="操作者" />
              <Input
                placeholder={intl.formatMessage({
                  id: 'pages.adminLogs.loginLog.search.actor',
                  defaultMessage: '操作者',
                })}
                value={actor}
                onChange={(e) => setActor(e.target.value)}
                style={{ width: '100%' }}
              />
            </Col>
            <Col xs={24} sm={12} md={5}>
              <FilterLabel id="pages.adminLogs.loginLog.filter.ip" defaultMessage="IP" />
              <Input
                placeholder="IP"
                value={ip}
                onChange={(e) => setIP(e.target.value)}
                style={{ width: '100%' }}
              />
            </Col>
            <Col xs={24} sm={24} md={8}>
              <FilterLabel id="pages.adminLogs.loginLog.filter.time" defaultMessage="时间" />
              <DatePicker.RangePicker
                showTime
                value={timeRange as [Dayjs, Dayjs]}
                onChange={(dates) => setTimeRange(dates as [Dayjs | null, Dayjs | null] | null)}
                style={{ width: '100%' }}
              />
            </Col>
            <Col xs={12} sm={12} md={3}>
              <Button type="primary" onClick={load} block>
                <FormattedMessage
                  id="pages.adminLogs.loginLog.action.query"
                  defaultMessage="查询"
                />
              </Button>
            </Col>
            <Col xs={12} sm={12} md={3}>
              <Button onClick={exportCSV} block>
                <FormattedMessage
                  id="pages.adminLogs.loginLog.action.exportCsv"
                  defaultMessage="导出 CSV"
                />
              </Button>
            </Col>
          </Row>
          <Row gutter={[8, 8]} style={{ marginBottom: 16 }}>
            <Col xs={24}>
              <Space size={4} wrap>
                <span>
                  <FormattedMessage
                    id="pages.adminLogs.loginLog.filter.kind"
                    defaultMessage="类型:"
                  />
                </span>
                {['login', 'login_fail', 'login_rate_limited'].map((k) => (
                  <Tag
                    key={k}
                    color={kinds.includes(k) ? 'blue' : 'default'}
                    onClick={() => {
                      setKinds((prev) =>
                        prev.includes(k) ? prev.filter((x) => x !== k) : [...prev, k],
                      );
                    }}
                    style={{ cursor: 'pointer' }}
                  >
                    {k}
                  </Tag>
                ))}
              </Space>
            </Col>
          </Row>
          <Row gutter={[8, 8]} style={{ marginBottom: 16 }} align="middle">
            <Col xs={24}>
              <Space size={8} wrap>
                <span>
                  <FormattedMessage
                    id="pages.adminLogs.loginLog.filter.device"
                    defaultMessage="设备:"
                  />
                </span>
                <Space size={4}>
                  {['Windows', 'macOS', 'Linux', 'Android', 'iOS', 'Other'].map((os) => (
                    <Tag
                      key={os}
                      color={osSel.includes(os) ? 'blue' : 'default'}
                      onClick={() =>
                        setOsSel((prev) =>
                          prev.includes(os) ? prev.filter((x) => x !== os) : [...prev, os],
                        )
                      }
                      style={{ cursor: 'pointer' }}
                    >
                      {os}
                    </Tag>
                  ))}
                </Space>
                <span>
                  <FormattedMessage
                    id="pages.adminLogs.loginLog.filter.browser"
                    defaultMessage="浏览器:"
                  />
                </span>
                <Space size={4}>
                  {['Edge', 'Chrome', 'Safari', 'Firefox', 'Other'].map((br) => (
                    <Tag
                      key={br}
                      color={brSel.includes(br) ? 'blue' : 'default'}
                      onClick={() =>
                        setBrSel((prev) =>
                          prev.includes(br) ? prev.filter((x) => x !== br) : [...prev, br],
                        )
                      }
                      style={{ cursor: 'pointer' }}
                    >
                      {br}
                    </Tag>
                  ))}
                </Space>
                <Button
                  onClick={() => {
                    setOsSel([]);
                    setBrSel([]);
                  }}
                >
                  <FormattedMessage
                    id="pages.adminLogs.loginLog.action.clearDeviceFilter"
                    defaultMessage="清空设备/浏览器筛选"
                  />
                </Button>
              </Space>
            </Col>
          </Row>
        </div>
        <Table
          rowKey="__rowKey"
          loading={loading}
          columns={[
            {
              title: intl.formatMessage({
                id: 'pages.adminLogs.loginLog.column.time',
                defaultMessage: '时间',
              }),
              dataIndex: 'time',
              render: (t?: string) => formatDateTime(t ?? ''),
            },
            {
              title: intl.formatMessage({
                id: 'pages.adminLogs.loginLog.column.kind',
                defaultMessage: '类型',
              }),
              dataIndex: 'kind',
              render: (v) => (
                <Tag color={v === 'login' ? 'green' : v === 'login_fail' ? 'red' : 'gold'}>{v}</Tag>
              ),
            },
            {
              title: intl.formatMessage({
                id: 'pages.adminLogs.loginLog.column.actor',
                defaultMessage: '操作者',
              }),
              dataIndex: 'actor',
            },
            { title: 'IP', dataIndex: ['meta', 'ip'] },
            {
              title: intl.formatMessage({
                id: 'pages.adminLogs.loginLog.column.region',
                defaultMessage: '属地',
              }),
              render: (_, r) => {
                const v = String(r?.meta?.ipRegion || '');
                if (!v) return '-';
                if (v === '本地')
                  return (
                    <Tag color="blue">
                      <FormattedMessage
                        id="pages.adminLogs.loginLog.region.local"
                        defaultMessage="本地"
                      />
                    </Tag>
                  );
                if (v === '局域网')
                  return (
                    <Tag color="geekblue">
                      <FormattedMessage
                        id="pages.adminLogs.loginLog.region.lan"
                        defaultMessage="局域网"
                      />
                    </Tag>
                  );
                return v;
              },
            },
            {
              title: intl.formatMessage({
                id: 'pages.adminLogs.loginLog.column.device',
                defaultMessage: '设备',
              }),
              render: (_, r) => detectOS(String(r.meta?.ua || '')),
            },
            {
              title: intl.formatMessage({
                id: 'pages.adminLogs.loginLog.column.browser',
                defaultMessage: '浏览器',
              }),
              render: (_, r) => detectBrowser(String(r.meta?.ua || '')),
            },
          ]}
          dataSource={pagedWithKey}
          expandable={{
            expandedRowRender: (r) => {
              const ua = String(r?.meta?.ua || '');
              return (
                <div style={{ fontFamily: 'monospace', whiteSpace: 'pre-wrap' }}>
                  <FormattedMessage
                    id="pages.adminLogs.loginLog.expanded.ua"
                    defaultMessage={'浏览器: {browser}\\nUA: {ua}'}
                    values={{ browser: detectBrowser(ua) || '-', ua: ua || '-' }}
                  />
                </div>
              );
            },
          }}
          pagination={{
            current: page,
            pageSize: size,
            total: filtered.length,
            showSizeChanger: true,
            onChange: (p, ps) => {
              setPage(p);
              setSize(ps || 20);
            },
          }}
        />
      </Card>
    </PageContainer>
  );
}
