import { useEffect, useState, useCallback } from 'react';
import { AutoComplete, Card, Space, DatePicker, Select, Button, Table, Tag } from 'antd';
import type { Dayjs } from 'dayjs';
import { PageContainer } from '@ant-design/pro-components';
import { exportToXLSX } from '@/utils/export';
import {
  fetchAnalyticsPaymentsSummary,
  fetchAnalyticsTransactions,
  fetchProductTrend,
} from '@/services/api/analytics';
import {
  ExportDimCSV,
  TopDimBar,
  TopDimCombo,
  TopDimRate,
  TopProducts,
  TopProductConv,
  TrendChart,
} from './charts';
import DeltaSection from './DeltaSection';
import type {
  ChannelData,
  CityData,
  CountryData,
  FilterOption,
  GeoDim,
  PaymentSummary,
  PlatformData,
  ProductData,
  RegionData,
  Transaction,
  TransactionsResponse,
  TrendData,
  TrendPoint,
} from './types';
import type { JSONValue } from '@/types/dashboard';

const EMPTY_SUMMARY: PaymentSummary = {
  totals: {},
  byChannel: [],
  byPlatform: [],
  byCountry: [],
  byRegion: [],
  byCity: [],
  byProduct: [],
  items: [],
};

/** 从维度数据提取去重后的筛选选项（label 可选定制，默认即维度值）。 */
function buildUniqueOptions<T>(
  data: T[] | undefined,
  getValue: (item: T) => string | undefined,
  getLabel?: (item: T) => string,
): FilterOption[] {
  const options = (data || [])
    .filter((item) => getValue(item))
    .map((item) => {
      const value = getValue(item) as string;
      return { label: getLabel ? getLabel(item) : value, value };
    });
  return options.filter(
    (option, index, self) => index === self.findIndex((o) => o.value === option.value),
  );
}

export default function AnalyticsPaymentsPage() {
  const [loading, setLoading] = useState(false);
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null);
  const [channel, setChannel] = useState<string>('');
  const [summary, setSummary] = useState<PaymentSummary>(EMPTY_SUMMARY);
  const [tx, setTx] = useState<TransactionsResponse>({ transactions: [], total: 0 });
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [platform, setPlatform] = useState<string>('');
  const [country, setCountry] = useState<string>('');
  const [region, setRegion] = useState<string>('');
  const [city, setCity] = useState<string>('');
  const [geoDim, setGeoDim] = useState<GeoDim>('region');
  const [prodIds, setProdIds] = useState<string[]>([]);
  const [gran, setGran] = useState<'minute' | 'hour'>('hour');
  const [trend, setTrend] = useState<TrendData[]>([]);

  // Available options for filters
  const [availableChannels, setAvailableChannels] = useState<FilterOption[]>([]);
  const [availablePlatforms, setAvailablePlatforms] = useState<FilterOption[]>([]);
  const [availableCountries, setAvailableCountries] = useState<FilterOption[]>([]);
  const [availableRegions, setAvailableRegions] = useState<FilterOption[]>([]);
  const [availableCities, setAvailableCities] = useState<FilterOption[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = { page, size };
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      if (channel) params.channel = channel;
      if (platform) params.platform = platform;
      if (country) params.country = country;
      if (region) params.region = region;
      if (city) params.city = city;
      const s = await fetchAnalyticsPaymentsSummary(params);
      setSummary(s || EMPTY_SUMMARY);

      // Extract unique values for filters from the response
      if (s) {
        setAvailableChannels(buildUniqueOptions(s.byChannel, (item: ChannelData) => item.channel));
        setAvailablePlatforms(
          buildUniqueOptions(s.byPlatform, (item: PlatformData) => item.platform),
        );
        setAvailableCountries(
          buildUniqueOptions(
            s.byCountry,
            (item: CountryData) => item.country,
            (item: CountryData) => `${item.country} (${item.countryCode || ''})`,
          ),
        );
        setAvailableRegions(buildUniqueOptions(s.byRegion, (item: RegionData) => item.region));
        setAvailableCities(buildUniqueOptions(s.byCity, (item: CityData) => item.city));
      }

      const t = await fetchAnalyticsTransactions(params);
      setTx(t || { transactions: [], total: 0 });
    } finally {
      setLoading(false);
    }
  }, [page, size, range, channel, platform, country, region, city]);
  useEffect(() => {
    load();
  }, [load]);

  const filterOption = (input: string, option?: FilterOption): boolean => {
    const needle = (input || '').toLowerCase();
    return (
      String(option?.value ?? '')
        .toLowerCase()
        .includes(needle) ||
      String(option?.label ?? '')
        .toLowerCase()
        .includes(needle)
    );
  };

  const hasBreakdowns =
    summary.byChannel.length > 0 ||
    summary.byPlatform.length > 0 ||
    summary.byCountry.length > 0 ||
    summary.byRegion.length > 0 ||
    summary.byCity.length > 0 ||
    summary.byProduct.length > 0;

  const geoRows =
    geoDim === 'country'
      ? summary?.byCountry || []
      : geoDim === 'region'
        ? summary?.byRegion || []
        : summary?.byCity || [];
  const geoLabel = geoDim === 'country' ? '国家' : geoDim === 'region' ? '省/区域' : '城市';

  return (
    <PageContainer>
      <Card
        title="支付分析"
        extra={
          <Space>
            <DatePicker.RangePicker
              value={range as [Dayjs, Dayjs]}
              onChange={(dates) => setRange(dates as [Dayjs | null, Dayjs | null] | null)}
            />
            <AutoComplete
              allowClear
              placeholder="渠道"
              value={channel}
              onChange={setChannel}
              style={{ width: 140 }}
              options={availableChannels}
              filterOption={filterOption}
            />
            <AutoComplete
              allowClear
              placeholder="平台"
              value={platform}
              onChange={setPlatform}
              style={{ width: 140 }}
              options={availablePlatforms}
              filterOption={filterOption}
            />
            <AutoComplete
              allowClear
              placeholder="国家"
              value={country}
              onChange={setCountry}
              style={{ width: 120 }}
              options={availableCountries}
              filterOption={filterOption}
            />
            <AutoComplete
              allowClear
              placeholder="省/区域"
              value={region}
              onChange={setRegion}
              style={{ width: 140 }}
              options={availableRegions}
              filterOption={filterOption}
            />
            <AutoComplete
              allowClear
              placeholder="城市"
              value={city}
              onChange={setCity}
              style={{ width: 140 }}
              options={availableCities}
              filterOption={filterOption}
            />
            <Select<GeoDim>
              value={geoDim}
              onChange={(v) => setGeoDim(v)}
              style={{ width: 120 }}
              options={[
                { label: '按国家', value: 'country' },
                { label: '按省/区域', value: 'region' },
                { label: '按城市', value: 'city' },
              ]}
            />
            <Button
              type="primary"
              onClick={() => {
                setPage(1);
                load();
              }}
            >
              查询
            </Button>
            <Button
              onClick={async () => {
                const ch: string[][] = [
                  ['channel', 'revenue_cents', 'success', 'total', 'success_rate(%)'],
                ].concat(
                  (summary?.byChannel || []).map((r: ChannelData) => [
                    String(r.channel),
                    String(r.revenueCents),
                    String(r.success),
                    String(r.total),
                    String(r.successRate),
                  ]),
                );
                const pf: string[][] = [
                  ['platform', 'revenue_cents', 'success', 'total', 'success_rate(%)'],
                ].concat(
                  (summary?.byPlatform || []).map((r: PlatformData) => [
                    String(r.platform),
                    String(r.revenueCents),
                    String(r.success),
                    String(r.total),
                    String(r.successRate),
                  ]),
                );
                const co: string[][] = [
                  ['country', 'revenue_cents', 'success', 'total', 'success_rate(%)'],
                ].concat(
                  (summary?.byCountry || []).map((r: CountryData) => [
                    String(r.country),
                    String(r.revenueCents),
                    String(r.success),
                    String(r.total),
                    String(r.successRate),
                  ]),
                );
                const rg: string[][] = [
                  ['region', 'revenue_cents', 'success', 'total', 'success_rate(%)'],
                ].concat(
                  (summary?.byRegion || []).map((r: RegionData) => [
                    String(r.region),
                    String(r.revenueCents),
                    String(r.success),
                    String(r.total),
                    String(r.successRate),
                  ]),
                );
                const ct: string[][] = [
                  ['city', 'revenue_cents', 'success', 'total', 'success_rate(%)'],
                ].concat(
                  (summary?.byCity || []).map((r: CityData) => [
                    String(r.city),
                    String(r.revenueCents),
                    String(r.success),
                    String(r.total),
                    String(r.successRate),
                  ]),
                );
                const pr: string[][] = [
                  ['product_id', 'revenue_cents', 'success', 'total', 'success_rate(%)'],
                ].concat(
                  (summary?.byProduct || []).map((r: ProductData) => [
                    String(r.productId),
                    String(r.revenueCents),
                    String(r.success),
                    String(r.total),
                    String(r.successRate),
                  ]),
                );
                await exportToXLSX('payments_summary.csv', [
                  { sheet: 'by_channel', rows: ch },
                  { sheet: 'by_platform', rows: pf },
                  { sheet: 'by_country', rows: co },
                  { sheet: 'by_region', rows: rg },
                  { sheet: 'by_city', rows: ct },
                  { sheet: 'by_product', rows: pr },
                ]);
              }}
            >
              导出汇总 CSV
            </Button>
          </Space>
        }
      >
        <Space size={16} wrap>
          <Tag color="blue">收入: {summary?.totals?.revenue || 0}</Tag>
          <Tag color="gold">交易数: {summary?.totals?.transactions || 0}</Tag>
          <Tag color="green">付费用户: {summary?.totals?.users || 0}</Tag>
        </Space>
        <Card size="small" title="按日汇总" style={{ marginTop: 12 }}>
          <Table
            size="small"
            rowKey={(row) => row.date}
            dataSource={summary.items}
            pagination={false}
            columns={[
              { title: '日期', dataIndex: 'date' },
              { title: '收入', dataIndex: 'revenue' },
              { title: '交易数', dataIndex: 'transactions' },
              { title: '付费用户', dataIndex: 'users' },
            ]}
          />
        </Card>
        {hasBreakdowns ? (
          <>
            <div style={{ marginTop: 12 }}>
              <b>按渠道</b>
              <Table<ChannelData>
                size="small"
                rowKey={(r: ChannelData) => String(r.channel || '')}
                dataSource={summary?.byChannel || []}
                columns={[
                  { title: '渠道', dataIndex: 'channel' },
                  { title: '收入(分)', dataIndex: 'revenue_cents' },
                  { title: '成功数', dataIndex: 'success' },
                  { title: '总数', dataIndex: 'total' },
                  {
                    title: '成功率',
                    dataIndex: 'success_rate',
                    render: (v: number) => (v != null ? `${v}%` : '-'),
                  },
                ]}
                pagination={false}
              />
              <TopDimBar
                data={summary?.byChannel || []}
                dimKey="channel"
                title="Top 渠道（按收入）"
              />
              <TopDimCombo
                data={summary?.byChannel || []}
                dimKey="channel"
                title="Top 渠道（收入 & 成功率）"
              />
              <ExportDimCSV data={summary?.byChannel || []} dimKey="channel" name="channels" />
            </div>
            <div style={{ marginTop: 12 }}>
              <b>按平台</b>
              <Table<PlatformData>
                size="small"
                rowKey={(r: PlatformData) => String(r.platform || '')}
                dataSource={summary?.byPlatform || []}
                columns={[
                  { title: '平台', dataIndex: 'platform' },
                  { title: '收入(分)', dataIndex: 'revenue_cents' },
                  { title: '成功数', dataIndex: 'success' },
                  { title: '总数', dataIndex: 'total' },
                  {
                    title: '成功率',
                    dataIndex: 'success_rate',
                    render: (v: number) => (v != null ? `${v}%` : '-'),
                  },
                ]}
                pagination={false}
              />
              <TopDimBar
                data={summary?.byPlatform || []}
                dimKey="platform"
                title="Top 平台（按收入）"
              />
              <TopDimRate
                data={summary?.byPlatform || []}
                dimKey="platform"
                title="Top 平台（按成功率）"
              />
              <TopDimCombo
                data={summary?.byPlatform || []}
                dimKey="platform"
                title="Top 平台（收入 & 成功率）"
              />
              <ExportDimCSV data={summary?.byPlatform || []} dimKey="platform" name="platforms" />
            </div>
            <div style={{ marginTop: 12 }}>
              <b>按地区（{geoLabel}）</b>
              <Table<CountryData | RegionData | CityData>
                size="small"
                rowKey={(r) => {
                  const record = r as unknown as Record<string, JSONValue>;
                  return String(
                    record[geoDim] ?? record.country ?? record.region ?? record.city ?? '',
                  );
                }}
                dataSource={geoRows}
                columns={[
                  { title: geoLabel, dataIndex: geoDim },
                  { title: '收入(分)', dataIndex: 'revenue_cents' },
                  { title: '成功数', dataIndex: 'success' },
                  { title: '总数', dataIndex: 'total' },
                  {
                    title: '成功率',
                    dataIndex: 'success_rate',
                    render: (v: number) => (v != null ? `${v}%` : '-'),
                  },
                ]}
                pagination={false}
              />
              <TopDimBar data={geoRows} dimKey={geoDim} title={`Top ${geoLabel}（按收入）`} />
              <TopDimRate data={geoRows} dimKey={geoDim} title={`Top ${geoLabel}（按成功率）`} />
              <TopDimCombo
                data={geoRows}
                dimKey={geoDim}
                title={`Top ${geoLabel}（收入 & 成功率）`}
              />
              <ExportDimCSV
                data={geoRows}
                dimKey={geoDim}
                name={
                  geoDim === 'country' ? 'countries' : geoDim === 'region' ? 'regions' : 'cities'
                }
              />
            </div>
            <div style={{ marginTop: 12 }}>
              <b>按商品</b>
              <Table<ProductData>
                size="small"
                rowKey={(r: ProductData) => String(r.productId || '')}
                dataSource={summary?.byProduct || []}
                columns={[
                  { title: '商品', dataIndex: 'product_id' },
                  { title: '收入(分)', dataIndex: 'revenue_cents' },
                  { title: '成功数', dataIndex: 'success' },
                  { title: '总数', dataIndex: 'total' },
                  {
                    title: '成功率',
                    dataIndex: 'success_rate',
                    render: (v: number) => (v != null ? `${v}%` : '-'),
                  },
                ]}
                pagination={false}
              />
              <TopProducts data={summary?.byProduct || []} />
              <TopProductConv data={summary?.byProduct || []} />
              <ExportDimCSV
                data={summary?.byProduct || []}
                dimKey="product_id"
                name="products"
                includeConv
              />
            </div>
          </>
        ) : (
          <Tag color="blue" style={{ marginTop: 12 }}>
            当前后端仅提供按日支付汇总；渠道、地区和商品维度暂无可用数据。
          </Tag>
        )}
        <div style={{ marginTop: 16 }}>
          <Card size="small" title="SKU 转化趋势">
            <Space style={{ marginBottom: 8 }}>
              <Select<string[]>
                mode="tags"
                allowClear
                placeholder="product_id（支持多选）"
                value={prodIds}
                onChange={(v) => setProdIds(v)}
                style={{ minWidth: 360 }}
              />
              <Select<'minute' | 'hour'>
                value={gran}
                onChange={(v) => setGran(v)}
                options={[
                  { label: '小时', value: 'hour' },
                  { label: '分钟', value: 'minute' },
                ]}
              />
              <Button
                type="primary"
                onClick={async () => {
                  try {
                    if (!range || !range[0] || !range[1] || (prodIds || []).length === 0) return;
                    const params: Record<string, string | number> = {
                      start: range[0].toISOString(),
                      end: range[1].toISOString(),
                      productId: prodIds.join(','),
                      granularity: gran,
                    };
                    if (channel) params.channel = channel;
                    if (platform) params.platform = platform;
                    if (country) params.country = country;
                    const r = await fetchProductTrend(params);
                    setTrend(r?.products || []);
                  } catch {}
                }}
              >
                查询
              </Button>
              <Button
                onClick={async () => {
                  try {
                    const rows: (string | number)[][] = [
                      ['ts', 'product_id', 'success', 'total', 'revenue_cents', 'success_rate(%)'],
                    ];
                    (trend || []).forEach((p: TrendData) => {
                      (p.points || []).forEach((pt: TrendPoint) => {
                        const ts = pt.time;
                        const succ = pt.amount || 0;
                        const tot = pt.count || 0;
                        const rev = pt.amount || 0;
                        const rate = tot > 0 ? Math.round((succ * 10000) / tot) / 100 : 0;
                        rows.push([ts, p.productId, succ, tot, rev, rate]);
                      });
                    });
                    const csv = rows
                      .map((r) => r.map((x: string | number) => String(x ?? '')).join(','))
                      .join('\n');
                    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
                    const url = URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.href = url;
                    a.download = 'product_trend.csv';
                    a.click();
                    URL.revokeObjectURL(url);
                  } catch {}
                }}
              >
                导出 CSV
              </Button>
            </Space>
            <TrendChart data={trend} />
          </Card>
        </div>
        <Table<Transaction>
          style={{ marginTop: 12 }}
          size="small"
          loading={loading}
          rowKey={(r: Transaction) => String(r.orderId || `${r.userId || ''}|${r.time || ''}`)}
          dataSource={tx?.transactions || []}
          columns={[
            { title: '时间', dataIndex: 'time' },
            { title: '订单', dataIndex: 'order_id' },
            { title: '用户', dataIndex: 'user_id' },
            { title: '金额(分)', dataIndex: 'amount_cents' },
            { title: '状态', dataIndex: 'status' },
            { title: '渠道', dataIndex: 'channel' },
            { title: '原因', dataIndex: 'reason' },
          ]}
          pagination={{
            current: page,
            pageSize: size,
            total: tx?.total || 0,
            showSizeChanger: true,
            onChange: (p, ps) => {
              setPage(p);
              setSize(ps || 20);
            },
          }}
        />
        <div style={{ marginTop: 8 }}>
          <Button
            onClick={async () => {
              const rows = [
                ['time', 'order_id', 'user_id', 'amount_cents', 'status', 'channel', 'reason'],
              ].concat(
                (tx?.transactions || []).map((r: Transaction) => [
                  String(r.time),
                  String(r.orderId),
                  String(r.userId),
                  String(r.amountCents ?? ''),
                  String(r.status),
                  String(r.channel),
                  String(r.reason ?? ''),
                ]),
              );
              await exportToXLSX('payments.csv', [{ sheet: 'transactions', rows }]);
            }}
          >
            导出 CSV
          </Button>
        </div>
        <DeltaSection
          range={range}
          channel={channel}
          platform={platform}
          country={country}
          region={region}
          city={city}
        />
      </Card>
    </PageContainer>
  );
}
