/**
 * F9：远程选项源 hook。
 *
 * 按 RemoteOptionsSpec 调用对应 collection_query 函数拉取选项；
 * 会话级缓存（同 functionId+搜索词只拉一次）；失败静默降级为空选项
 * （widget 退化为普通输入）。labelPath/valuePath 支持 `*` 通配数组段。
 */

import { useEffect, useState } from 'react';
import { invokeFunction } from '@/services/api/functions';
import type { JSONValue, RemoteOptionsSpec } from '@/types/dashboard';

export interface RemoteOption {
  label: string;
  value: string;
}

/** 会话级缓存：key = functionId + '::' + search；失败结果不缓存 */
const cache = new Map<string, RemoteOption[]>();
const inflight = new Map<string, Promise<RemoteOption[]>>();

/** 清空会话级缓存（测试用；生产会话期内常驻）。 */
export function clearRemoteOptionsCache(): void {
  cache.clear();
  inflight.clear();
}

// JSON Pointer 取值，支持 `*` 通配数组段（路径形如 items 数组按名取子字段）。
export function selectByPointer(
  data: JSONValue | undefined,
  pointer?: string,
): JSONValue | undefined {
  if (!pointer || !pointer.startsWith('/')) return data as JSONValue;
  const segments = pointer
    .slice(1)
    .split('/')
    .map((token) => token.replace(/~1/g, '/').replace(/~0/g, '~'));
  return selectSegments(data as JSONValue, segments);
}

function selectSegments(data: JSONValue, segments: string[]): JSONValue | undefined {
  if (segments.length === 0) return data;
  const [head, ...rest] = segments;
  if (head === '*') {
    if (!Array.isArray(data)) return [];
    return data
      .map((item) => selectSegments(item, rest))
      .filter((item) => item !== undefined && item !== null);
  }
  if (Array.isArray(data)) {
    const index = Number(head);
    if (!Number.isInteger(index) || index < 0 || index >= data.length) return undefined;
    return selectSegments(data[index], rest);
  }
  if (
    data === null ||
    typeof data !== 'object' ||
    !Object.prototype.hasOwnProperty.call(data, head)
  ) {
    return undefined;
  }
  return selectSegments((data as Record<string, JSONValue>)[head], rest);
}

export function optionsFromResult(
  result: JSONValue | undefined,
  spec: RemoteOptionsSpec,
): RemoteOption[] {
  const labels = selectByPointer(result, spec.labelPath);
  const values = selectByPointer(result, spec.valuePath ?? spec.labelPath);
  const labelList = Array.isArray(labels) ? labels : [labels];
  const valueList = Array.isArray(values) ? values : [values];
  const options: RemoteOption[] = [];
  for (let i = 0; i < valueList.length; i += 1) {
    const value = valueList[i];
    if (value === undefined || value === null) continue;
    const rawLabel = labelList[i] ?? value;
    options.push({ label: String(rawLabel), value: String(value) });
  }
  return options;
}

async function fetchOptions(spec: RemoteOptionsSpec, search: string): Promise<RemoteOption[]> {
  const key = `${spec.functionId}::${search}`;
  const cached = cache.get(key);
  if (cached) return cached;
  const pending = inflight.get(key);
  if (pending) return pending;
  const payload: Record<string, JSONValue> = {};
  if (search && spec.searchParam) payload[spec.searchParam] = search;
  const promise = invokeFunction(spec.functionId, payload)
    .then((response) => {
      const result = (response?.result ?? response) as JSONValue | undefined;
      const options = optionsFromResult(result, spec);
      cache.set(key, options);
      inflight.delete(key);
      return options;
    })
    .catch(() => {
      // 失败降级：不缓存、不抛错（widget 退化为普通输入）
      inflight.delete(key);
      return [] as RemoteOption[];
    });
  inflight.set(key, promise);
  return promise;
}

export function useRemoteOptions(
  spec: RemoteOptionsSpec | undefined,
  search = '',
): { options: RemoteOption[]; loading: boolean } {
  const [state, setState] = useState<{ options: RemoteOption[]; loading: boolean }>({
    options: [],
    loading: false,
  });

  useEffect(() => {
    if (!spec?.functionId) {
      setState({ options: [], loading: false });
      return;
    }
    // 无 searchParam 时不响应搜索词（一次性数据源）
    const effectiveSearch = spec.searchParam ? search.trim() : '';
    const key = `${spec.functionId}::${effectiveSearch}`;
    const cached = cache.get(key);
    if (cached) {
      setState({ options: cached, loading: false });
      return;
    }
    let cancelled = false;
    setState((prev) => ({ options: effectiveSearch ? prev.options : [], loading: true }));
    fetchOptions(spec, effectiveSearch).then((options) => {
      if (!cancelled) setState({ options, loading: false });
    });
    return () => {
      cancelled = true;
    };
  }, [spec?.functionId, spec?.searchParam, search, spec]);

  return state;
}
