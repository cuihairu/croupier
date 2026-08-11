import type { FormPresentationSpec, JSONValue } from '@/types/dashboard';
import type { InvokeFunctionOptions } from '@/services/api';

export interface RequestHistoryItem {
  id: string;
  functionId: string;
  timestamp: string;
  duration: number;
  status: 'success' | 'error';
  request: JSONValue;
  options: InvokeFunctionOptions;
  response?: JSONValue;
  error?: string;
}

export type FormSchemaState =
  | { status: 'idle'; error?: undefined; spec?: undefined }
  | { status: 'ready'; spec: FormPresentationSpec; error?: undefined }
  | { status: 'unavailable'; error: string; spec?: undefined };

export function formatDuration(ms: number): string {
  return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(2)} s`;
}
