import React, { useEffect, useState, useCallback } from 'react';
import { AutoComplete, Card, Space, DatePicker, Select, Button, Table, Tag } from 'antd';
import type { Dayjs } from 'dayjs';
import { PageContainer } from '@ant-design/pro-components';
import { exportToXLSX } from '@/utils/export';
import {
  fetchAnalyticsPaymentsSummary,
  fetchAnalyticsTransactions,
  fetchProductTrend,
} from '@/services/api/analytics';
import type { JSONValue } from '@/types/dashboard';

interface ChannelData {
  channel: string;
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
}

interface PlatformData {
  platform: string;
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
}

interface CountryData {
  country: string;
  countryCode?: string;
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
}

interface RegionData {
  region: string;
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
}

interface CityData {
  city: string;
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
}

interface ProductData {
  productId: string;
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
}

interface PaymentSummary {
  totals: Record<string, number>;
  byChannel: ChannelData[];
  byPlatform: PlatformData[];
  byCountry: CountryData[];
  byRegion: RegionData[];
  byCity: CityData[];
  byProduct: ProductData[];
  items: Array<{ date: string; revenue: number; transactions: number; users: number }>;
}

interface Transaction {
  orderId: string;
  userId: string;
  channel: string;
  platform?: string;
  country?: string;
  region?: string;
  city?: string;
  productId?: string;
  amount: number;
  status: string;
  time: string;
  amountCents?: number;
  reason?: string;
}

interface TransactionsResponse {
  transactions: Transaction[];
  total: number;
}

interface TrendPoint {
  time: string;
  amount: number;
  count: number;
}

interface TrendData {
  productId: string;
  points: TrendPoint[];
}

interface CompareItem {
  key: string;
  cur: Record<string, JSONValue>;
  prev: Record<string, JSONValue>;
  revDelta: number;
  rateDelta: number;
}

interface DeltaRows {
  upRev: CompareItem[];
  downRev: CompareItem[];
  upRate: CompareItem[];
  downRate: CompareItem[];
  all: CompareItem[];
}

type GeoDim = 'country' | 'region' | 'city';
type DeltaDim = 'channel' | 'platform' | 'country' | 'region' | 'city' | 'product';
type DeltaMode = 'prev' | 'prev_week' | 'prev_month' | 'prev_year';

interface FilterOption {
  label: string;
  value: string;
}

export default function AnalyticsPaymentsPage() {
  const [loading, setLoading] = useState(false);
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null);
  const [channel, setChannel] = useState<string>('');
  const [summary, setSummary] = useState<PaymentSummary>({
    totals: {},
    byChannel: [],
    byPlatform: [],
    byCountry: [],
    byRegion: [],
    byCity: [],
    byProduct: [],
    items: [],
  });
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
      setSummary(
        s || {
          totals: {},
          byChannel: [],
          byPlatform: [],
          byCountry: [],
          byRegion: [],
          byCity: [],
          byProduct: [],
          items: [],
        },
      );

      // Extract unique values for filters from the response
      if (s) {
        // Channels
        if (s.byChannel) {
          const channels = s.byChannel
            .filter((item: ChannelData) => item.channel)
            .map((item: ChannelData) => ({
              label: item.channel,
              value: item.channel,
            }));
          const uniqueChannels = channels.filter(
            (channel: FilterOption, index: number, self: FilterOption[]) =>
              index === self.findIndex((c: FilterOption) => c.value === channel.value),
          );
          setAvailableChannels(uniqueChannels);
        }

        // Platforms
        if (s.byPlatform) {
          const platforms = s.byPlatform
            .filter((item: PlatformData) => item.platform)
            .map((item: PlatformData) => ({
              label: item.platform,
              value: item.platform,
            }));
          const uniquePlatforms = platforms.filter(
            (platform: FilterOption, index: number, self: FilterOption[]) =>
              index === self.findIndex((p: FilterOption) => p.value === platform.value),
          );
          setAvailablePlatforms(uniquePlatforms);
        }

        // Countries
        if (s.byCountry) {
          const countries = s.byCountry
            .filter((item: CountryData) => item.country)
            .map((item: CountryData) => ({
              label: `${item.country} (${item.countryCode || ''})`,
              value: item.country,
            }));
          const uniqueCountries = countries.filter(
            (country: FilterOption, index: number, self: FilterOption[]) =>
              index === self.findIndex((c: FilterOption) => c.value === country.value),
          );
          setAvailableCountries(uniqueCountries);
        }

        // Regions
        if (s.byRegion) {
          const regions = s.byRegion
            .filter((item: RegionData) => item.region)
            .map((item: RegionData) => ({
              label: item.region,
              value: item.region,
            }));
          const uniqueRegions = regions.filter(
            (region: FilterOption, index: number, self: FilterOption[]) =>
              index === self.findIndex((r: FilterOption) => r.value === region.value),
          );
          setAvailableRegions(uniqueRegions);
        }

        // Cities
        if (s.byCity) {
          const cities = s.byCity
            .filter((item: CityData) => item.city)
            .map((item: CityData) => ({
              label: item.city,
              value: item.city,
            }));
          const uniqueCities = cities.filter(
            (city: FilterOption, index: number, self: FilterOption[]) =>
              index === self.findIndex((c: FilterOption) => c.value === city.value),
          );
          setAvailableCities(uniqueCities);
        }
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
              <b>
                按地区（{geoDim === 'country' ? '国家' : geoDim === 'region' ? '省/区域' : '城市'}）
              </b>
              <Table<CountryData | RegionData | CityData>
                size="small"
                rowKey={(r) => {
                  const record = r as unknown as Record<string, JSONValue>;
                  return String(
                    record[geoDim] ?? record.country ?? record.region ?? record.city ?? '',
                  );
                }}
                dataSource={
                  geoDim === 'country'
                    ? summary?.byCountry || []
                    : geoDim === 'region'
                      ? summary?.byRegion || []
                      : summary?.byCity || []
                }
                columns={[
                  {
                    title: geoDim === 'country' ? '国家' : geoDim === 'region' ? '省/区域' : '城市',
                    dataIndex: geoDim,
                  },
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
                data={
                  geoDim === 'country'
                    ? summary?.byCountry || []
                    : geoDim === 'region'
                      ? summary?.byRegion || []
                      : summary?.byCity || []
                }
                dimKey={geoDim}
                title={`Top ${
                  geoDim === 'country' ? '国家' : geoDim === 'region' ? '省/区域' : '城市'
                }（按收入）`}
              />
              <TopDimRate
                data={
                  geoDim === 'country'
                    ? summary?.byCountry || []
                    : geoDim === 'region'
                      ? summary?.byRegion || []
                      : summary?.byCity || []
                }
                dimKey={geoDim}
                title={`Top ${
                  geoDim === 'country' ? '国家' : geoDim === 'region' ? '省/区域' : '城市'
                }（按成功率）`}
              />
              <TopDimCombo
                data={
                  geoDim === 'country'
                    ? summary?.byCountry || []
                    : geoDim === 'region'
                      ? summary?.byRegion || []
                      : summary?.byCity || []
                }
                dimKey={geoDim}
                title={`Top ${
                  geoDim === 'country' ? '国家' : geoDim === 'region' ? '省/区域' : '城市'
                }（收入 & 成功率）`}
              />
              <ExportDimCSV
                data={
                  geoDim === 'country'
                    ? summary?.byCountry || []
                    : geoDim === 'region'
                      ? summary?.byRegion || []
                      : summary?.byCity || []
                }
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

interface DimBase {
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
  [key: string]: string | number | boolean | undefined;
}

type DimData =
  | ChannelData
  | PlatformData
  | CountryData
  | RegionData
  | CityData
  | ProductData
  | DimBase;

const TopProducts: React.FC<{ data: ProductData[] }> = ({ data }) => {
  try {
    const items = (data || [])
      .slice(0)
      .sort(
        (a: ProductData, b: ProductData) =>
          Number(b.revenueCents || 0) - Number(a.revenueCents || 0),
      )
      .slice(0, 10);
    if (!items.length) return null;
    const max = Math.max(...items.map((x: ProductData) => Number(x.revenueCents || 0)), 1);
    const w = 600,
      barH = 18,
      gap = 6,
      left = 140,
      right = 60,
      h = items.length * (barH + gap) + 20;
    const scale = (v: number) => ((w - left - right) * v) / max;
    return (
      <div style={{ marginTop: 12 }}>
        <b>Top 商品（按收入）</b>
        <svg
          width={w}
          height={h}
          style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
        >
          {items.map((it: ProductData, idx: number) => {
            const y = 10 + idx * (barH + gap);
            const val = Number(it.revenueCents || 0);
            return (
              <g key={idx}>
                <text x={4} y={y + barH - 4} fontSize={12} fill="#555">
                  {String(it.productId || '-')}
                </text>
                <rect x={left} y={y} width={Math.max(2, scale(val))} height={barH} fill="#1677ff" />
                <text
                  x={left + Math.max(2, scale(val)) + 6}
                  y={y + barH - 4}
                  fontSize={12}
                  fill="#333"
                >
                  {val}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch {
    return null;
  }
};

const TopDimBar: React.FC<{ data: DimData[]; dimKey: string; title: string }> = ({
  data,
  dimKey,
  title,
}) => {
  try {
    const items = (data || [])
      .slice(0)
      .sort((a, b) => Number(b.revenueCents || 0) - Number(a.revenueCents || 0))
      .slice(0, 10);
    if (!items.length) return null;
    const max = Math.max(...items.map((x) => Number(x.revenueCents || 0)), 1);
    const w = 600,
      barH = 18,
      gap = 6,
      left = 140,
      right = 60,
      h = items.length * (barH + gap) + 20;
    const scale = (v: number) => ((w - left - right) * v) / max;
    return (
      <div style={{ marginTop: 12 }}>
        <b>{title}</b>
        <svg
          width={w}
          height={h}
          style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
        >
          {items.map((it, idx: number) => {
            const y = 10 + idx * (barH + gap);
            const val = Number(it.revenueCents || 0);
            return (
              <g key={idx}>
                <text x={4} y={y + barH - 4} fontSize={12} fill="#555">
                  {String((it as Record<string, string | number>)[dimKey] || '-')}
                </text>
                <rect x={left} y={y} width={Math.max(2, scale(val))} height={barH} fill="#73d13d" />
                <text
                  x={left + Math.max(2, scale(val)) + 6}
                  y={y + barH - 4}
                  fontSize={12}
                  fill="#333"
                >
                  {val}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch {
    return null;
  }
};

const TopDimRate: React.FC<{ data: DimData[]; dimKey: string; title: string }> = ({
  data,
  dimKey,
  title,
}) => {
  try {
    const items = (data || [])
      .map((x) => ({ ...x, successRate: Number(x.successRate || 0) }))
      .filter((x) => isFinite(x.successRate))
      .sort((a, b) => b.successRate - a.successRate)
      .slice(0, 10);
    if (!items.length) return null;
    const max = Math.max(...items.map((x) => Number(x.successRate || 0)), 1);
    const w = 600,
      barH = 18,
      gap = 6,
      left = 140,
      right = 60,
      h = items.length * (barH + gap) + 20;
    const scale = (v: number) => ((w - left - right) * v) / max;
    return (
      <div style={{ marginTop: 12 }}>
        <b>{title}</b>
        <svg
          width={w}
          height={h}
          style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
        >
          {items.map((it, idx: number) => {
            const y = 10 + idx * (barH + gap);
            const val = Number(it.successRate || 0);
            return (
              <g key={idx}>
                <text x={4} y={y + barH - 4} fontSize={12} fill="#555">
                  {String((it as Record<string, string | number>)[dimKey] || '-')}
                </text>
                <rect x={left} y={y} width={Math.max(2, scale(val))} height={barH} fill="#faad14" />
                <text
                  x={left + Math.max(2, scale(val)) + 6}
                  y={y + barH - 4}
                  fontSize={12}
                  fill="#333"
                >
                  {val}%
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch {
    return null;
  }
};

// Combined revenue (bar) + success_rate (line) on same chart (two scales approximated visually)
const TopDimCombo: React.FC<{ data: DimData[]; dimKey: string; title: string }> = ({
  data,
  dimKey,
  title,
}) => {
  try {
    const items = (data || [])
      .slice(0)
      .sort((a, b) => Number(b.revenueCents || 0) - Number(a.revenueCents || 0))
      .slice(0, 10);
    if (!items.length) return null;
    const maxRev = Math.max(...items.map((x) => Number(x.revenueCents || 0)), 1);
    const maxRate = Math.max(...items.map((x) => Number(x.successRate || 0)), 1);
    const w = 720,
      barH = 18,
      gap = 10,
      left = 140,
      right = 80,
      topm = 16,
      h = items.length * (barH + gap) + topm + 10;
    const sRev = (v: number) => ((w - left - right) * v) / maxRev;
    const sRate = (v: number) => ((w - left - right) * v) / maxRate;
    return (
      <div style={{ marginTop: 12 }}>
        <b>{title}</b>
        <svg
          width={w}
          height={h}
          style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
        >
          {items.map((it, idx: number) => {
            const y = topm + idx * (barH + gap);
            const rev = Number(it.revenueCents || 0);
            const rate = Number(it.successRate || 0);
            return (
              <g key={idx}>
                <text x={4} y={y + barH - 4} fontSize={12} fill="#555">
                  {String((it as Record<string, string | number>)[dimKey] || '-')}
                </text>
                {/* revenue bar */}
                <rect x={left} y={y} width={Math.max(2, sRev(rev))} height={barH} fill="#69c0ff" />
                <text
                  x={left + Math.max(2, sRev(rev)) + 6}
                  y={y + barH - 4}
                  fontSize={12}
                  fill="#333"
                >
                  {rev}
                </text>
                {/* success rate markers (line) */}
                <circle
                  cx={left + Math.max(2, sRate(rate))}
                  cy={y + barH / 2}
                  r={3}
                  fill="#fa541c"
                />
                <text
                  x={left + Math.max(2, sRate(rate)) + 6}
                  y={y + barH / 2 + 4}
                  fontSize={10}
                  fill="#666"
                >
                  {rate}%
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch {
    return null;
  }
};

// Product conversion compare (success vs total)
const TopProductConv: React.FC<{ data: ProductData[] }> = ({ data }) => {
  try {
    const items = (data || [])
      .slice(0)
      .sort((a: ProductData, b: ProductData) => Number(b.total || 0) - Number(a.total || 0))
      .slice(0, 10);
    if (!items.length) return null;
    const max = Math.max(...items.map((x: ProductData) => Number(x.total || 0)), 1);
    const w = 720,
      barH = 16,
      gap = 8,
      left = 160,
      right = 80,
      topm = 16,
      h = items.length * (barH + gap) + topm + 14;
    const s = (v: number) => ((w - left - right) * v) / max;
    return (
      <div style={{ marginTop: 12 }}>
        <b>Top 商品（转化：成功 vs 总数）</b>
        <svg
          width={w}
          height={h}
          style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
        >
          {items.map((it: ProductData, idx: number) => {
            const y = topm + idx * (barH + gap);
            const succ = Number(it.success || 0);
            const tot = Number(it.total || 0);
            return (
              <g key={idx}>
                <text x={4} y={y + barH - 2} fontSize={12} fill="#555">
                  {String(it.productId || '-')}
                </text>
                {/* total (background) */}
                <rect x={left} y={y} width={Math.max(2, s(tot))} height={barH} fill="#f0f0f0" />
                {/* success */}
                <rect x={left} y={y} width={Math.max(2, s(succ))} height={barH} fill="#52c41a" />
                <text
                  x={left + Math.max(2, s(succ)) + 6}
                  y={y + barH - 2}
                  fontSize={12}
                  fill="#333"
                >
                  {succ}/{tot}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch {
    return null;
  }
};

// Export current dimension rows to CSV
const ExportDimCSV: React.FC<{
  data: DimData[];
  dimKey: string;
  name: string;
  includeConv?: boolean;
}> = ({ data, dimKey, name, includeConv }) => (
  <Button
    style={{ marginTop: 8 }}
    onClick={() => {
      try {
        const rows: string[][] = [['dim', 'revenue_cents', 'success', 'total', 'success_rate(%)']];
        (data || []).forEach((r) => {
          const record = r as Record<string, string | number>;
          rows.push([
            String(record[dimKey] ?? ''),
            String(record.revenueCents ?? 0),
            String(record.success ?? 0),
            String(record.total ?? 0),
            String(record.successRate ?? 0),
          ]);
        });
        if (includeConv) rows.push([]);
        const csv = rows.map((r) => r.map((x) => String(x ?? '')).join(',')).join('\n');
        const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `payments_${name}.csv`;
        a.click();
        URL.revokeObjectURL(url);
      } catch {}
    }}
  >
    导出 {name} CSV
  </Button>
);

// 环比分析：对比同等长度的上一个时间窗口，展示收入与成功率涨幅 Top/Bottom
const DeltaSection: React.FC<{
  range: [Dayjs | null, Dayjs | null] | null;
  channel: string;
  platform: string;
  country: string;
  region?: string;
  city?: string;
}> = ({ range, channel, platform, country, region, city }) => {
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
      title="环比分析（上一等长窗口）"
      style={{ marginTop: 16 }}
      extra={
        <Space>
          <Select<DeltaDim>
            value={dim}
            onChange={(v) => setDim(v)}
            options={[
              { label: '渠道', value: 'channel' },
              { label: '平台', value: 'platform' },
              { label: '国家', value: 'country' },
              { label: '商品', value: 'product' },
            ]}
          />
          <Select<DeltaMode>
            value={mode}
            onChange={(v) => setMode(v)}
            options={[
              { label: '上一等长窗口', value: 'prev' },
              { label: '上一周', value: 'prev_week' },
              { label: '上一月(30日)', value: 'prev_month' },
              { label: '上一年(365日)', value: 'prev_year' },
            ]}
          />
          <Button onClick={calc} loading={loading}>
            计算
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
                const csv = rowsOut
                  .map((r) => r.map((x: string | number) => String(x ?? '')).join(','))
                  .join('\n');
                const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = 'payments_delta.csv';
                a.click();
                URL.revokeObjectURL(url);
              } catch {}
            }}
          >
            导出环比报告
          </Button>
        </Space>
      }
    >
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
        <div>
          <b>收入涨幅 Top5</b>
          <Table<CompareItem>
            size="small"
            rowKey={(r: CompareItem) => String(r.key || '')}
            dataSource={rows.upRev}
            columns={[
              { title: dim, dataIndex: 'key' },
              { title: '涨幅(分)', dataIndex: 'revDelta' },
            ]}
            pagination={false}
          />
        </div>
        <div>
          <b>收入降幅 Top5</b>
          <Table<CompareItem>
            size="small"
            rowKey={(r: CompareItem) => String(r.key || '')}
            dataSource={rows.downRev}
            columns={[
              { title: dim, dataIndex: 'key' },
              { title: '跌幅(分)', dataIndex: 'revDelta' },
            ]}
            pagination={false}
          />
        </div>
        <div>
          <b>成功率涨幅 Top5</b>
          <Table<CompareItem>
            size="small"
            rowKey={(r: CompareItem) => String(r.key || '')}
            dataSource={rows.upRate}
            columns={[
              { title: dim, dataIndex: 'key' },
              { title: '涨幅(%)', dataIndex: 'rateDelta' },
            ]}
            pagination={false}
          />
        </div>
        <div>
          <b>成功率降幅 Top5</b>
          <Table<CompareItem>
            size="small"
            rowKey={(r: CompareItem) => String(r.key || '')}
            dataSource={rows.downRate}
            columns={[
              { title: dim, dataIndex: 'key' },
              { title: '跌幅(%)', dataIndex: 'rateDelta' },
            ]}
            pagination={false}
          />
        </div>
      </div>
    </Card>
  );
};

// TrendChart: two panels (revenue & success_rate) for multiple products
const TrendChart: React.FC<{ data: TrendData[] }> = ({ data }) => {
  try {
    const prods = (data || []) as TrendData[];
    if (!prods.length) return null;
    const colors = [
      '#1677ff',
      '#fa541c',
      '#52c41a',
      '#faad14',
      '#722ed1',
      '#13c2c2',
      '#eb2f96',
      '#2f54eb',
      '#a0d911',
      '#fa8c16',
    ];
    // collect times and compute scales
    const timesSet: Record<string, number> = {};
    let maxRev = 1,
      maxRate = 100;
    prods.forEach((p: TrendData) =>
      (p.points || []).forEach((pt: TrendPoint) => {
        timesSet[String(pt.time)] = 1;
        maxRev = Math.max(maxRev, Number(pt.amount || 0));
        const succ = Number(pt.amount || 0);
        const tot = Number(pt.count || 0);
        const rate = tot > 0 ? (succ * 100) / tot : 0;
        maxRate = Math.max(maxRate, rate);
      }),
    );
    const times = Object.keys(timesSet).sort();
    const w = 720,
      h = 180,
      left = 40,
      bottom = 24,
      right = 10,
      topm = 16;
    const sx = (t: string) => {
      const i = times.indexOf(t);
      if (i < 0) return left;
      return left + ((w - left - right) * i) / Math.max(1, times.length - 1);
    };
    const syRev = (v: number) => topm + (h - topm - bottom) * (1 - v / Math.max(1, maxRev));
    const syRate = (v: number) => topm + (h - topm - bottom) * (1 - v / Math.max(1, maxRate));
    const Path = ({
      vals,
      yfn,
      color,
    }: {
      vals: [string, number][];
      yfn: (v: number) => number;
      color: string;
    }) => {
      const d = vals
        .map(
          (pt: [string, number], idx: number) =>
            `${idx ? 'L' : 'M'}${sx(pt[0])},${yfn(Number(pt[1]))}`,
        )
        .join(' ');
      return <path d={d} fill="none" stroke={color} strokeWidth={2} />;
    };
    return (
      <div>
        <div style={{ marginTop: 8 }}>
          <b>收入趋势</b>
          <svg
            width={w}
            height={h}
            style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
          >
            <line x1={left} y1={topm} x2={left} y2={h - bottom} stroke="#ddd" />
            <line x1={left} y1={h - bottom} x2={w - right} y2={h - bottom} stroke="#ddd" />
            {prods.map((p: TrendData, i: number) => (
              <Path
                key={p.productId || i}
                vals={(p.points || []).map((pt: TrendPoint) => [pt.time, Number(pt.amount || 0)])}
                yfn={syRev}
                color={colors[i % colors.length]}
              />
            ))}
          </svg>
        </div>
        <div style={{ marginTop: 8 }}>
          <b>成功率趋势</b>
          <svg
            width={w}
            height={h}
            style={{ display: 'block', border: '1px solid #f0f0f0', background: '#fff' }}
          >
            <line x1={left} y1={topm} x2={left} y2={h - bottom} stroke="#ddd" />
            <line x1={left} y1={h - bottom} x2={w - right} y2={h - bottom} stroke="#ddd" />
            {prods.map((p: TrendData, i: number) => (
              <Path
                key={p.productId || i}
                vals={(p.points || []).map((pt: TrendPoint) => {
                  const succ = Number(pt.amount || 0);
                  const tot = Number(pt.count || 0);
                  const rate = tot > 0 ? (succ * 100) / tot : 0;
                  return [pt.time, rate] as [string, number];
                })}
                yfn={syRate}
                color={colors[i % colors.length]}
              />
            ))}
          </svg>
        </div>
        <div style={{ marginTop: 4 }}>
          <b>图例：</b>
          {prods.map((p: TrendData, i: number) => (
            <span key={p.productId || i} style={{ marginRight: 12 }}>
              <span
                style={{
                  display: 'inline-block',
                  width: 10,
                  height: 10,
                  background: colors[i % colors.length],
                  marginRight: 4,
                }}
              />
              {String(p.productId || '-')}
            </span>
          ))}
        </div>
      </div>
    );
  } catch {
    return null;
  }
};
