import type { JSONValue } from '@/types/dashboard';

export interface EventRow {
  id?: string;
  event?: string;
  time?: string;
  userId?: string;
  [key: string]: JSONValue | undefined;
}

export interface FunnelStep {
  step: string;
  users: number;
  rate: number;
}

export interface PathRow {
  path: string;
  groups: number;
}

export interface AdoptionRow {
  feature: string;
  groups: number;
  rate: number;
}

export interface AdoptionBreakdownRow {
  dim: string;
  baseline: number;
  groups: number;
  rate: number;
}

export interface FunnelPreset {
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

export interface ImportRow {
  key: string;
  name: string;
  status: string;
  obj: FunnelPreset;
}
