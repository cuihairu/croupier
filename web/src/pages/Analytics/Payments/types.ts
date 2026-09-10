import type { JSONValue } from '@/types/dashboard';

/** 支付分析页共享类型。 */

export interface ChannelData {
  channel: string;
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
}

export interface PlatformData {
  platform: string;
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
}

export interface CountryData {
  country: string;
  countryCode?: string;
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
}

export interface RegionData {
  region: string;
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
}

export interface CityData {
  city: string;
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
}

export interface ProductData {
  productId: string;
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
}

export interface PaymentSummary {
  totals: Record<string, number>;
  byChannel: ChannelData[];
  byPlatform: PlatformData[];
  byCountry: CountryData[];
  byRegion: RegionData[];
  byCity: CityData[];
  byProduct: ProductData[];
  items: Array<{ date: string; revenue: number; transactions: number; users: number }>;
}

export interface Transaction {
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

export interface TransactionsResponse {
  transactions: Transaction[];
  total: number;
}

export interface TrendPoint {
  time: string;
  amount: number;
  count: number;
}

export interface TrendData {
  productId: string;
  points: TrendPoint[];
}

export interface CompareItem {
  key: string;
  cur: Record<string, JSONValue>;
  prev: Record<string, JSONValue>;
  revDelta: number;
  rateDelta: number;
}

export interface DeltaRows {
  upRev: CompareItem[];
  downRev: CompareItem[];
  upRate: CompareItem[];
  downRate: CompareItem[];
  all: CompareItem[];
}

export type GeoDim = 'country' | 'region' | 'city';
export type DeltaDim = 'channel' | 'platform' | 'country' | 'region' | 'city' | 'product';
export type DeltaMode = 'prev' | 'prev_week' | 'prev_month' | 'prev_year';

export interface FilterOption {
  label: string;
  value: string;
}

export interface DimBase {
  revenueCents: number;
  success: number;
  total: number;
  successRate: number;
  [key: string]: string | number | boolean | undefined;
}

export type DimData =
  ChannelData | PlatformData | CountryData | RegionData | CityData | ProductData | DimBase;
