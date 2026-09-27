import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Select, type SelectProps } from 'antd';
import { useScopeReload } from '@/hooks/useScopeReload';

/**
 * 「可搜索下拉 + 服务端聚合选项」的统一封装（#21/#22/#23/#26/#33/#34 族）。
 *
 * 背景：这类过滤器的选项必须由**服务端聚合接口**提供（distinct 全集），
 * 不能从当前过滤后的列表推导——#14 实证：选一项后选项塌缩成一项。且选项
 * 数据同样是 scoped 的：顶栏切游戏/环境后必须重拉（#38 审计族）。这两件事
 * 每个页面手写一遍必然漂移，收敛到这里：
 * - 挂载拉一次；全局 scope 变化自动重拉（useScopeReload 内置）；
 * - 取数失败静默保留上次选项（选项接口是辅助面，不打断表单）；
 * - 带 count 的选项渲染为 `label (count)`（与服务端聚合接口的资源数语义一致）；
 * - `epoch` 变化强制重拉（如「编辑语义保存后分类计数变了」这类写后刷新）。
 */

export type ServerSelectOptionInput = string | { value: string; label?: string; count?: number };

export interface ServerOptionsSelectProps extends Omit<SelectProps, 'options' | 'loading'> {
  /** 服务端聚合数据源：返回 distinct 选项全集（string 或 {value,label?,count?}）。
   *  引用不稳定无妨——内部经 ref 转发，不会造成重复请求。 */
  fetchOptions: () => Promise<readonly ServerSelectOptionInput[] | null | undefined>;
  /** 变更即强制重拉（写操作后的计数刷新等）；默认不重拉。 */
  epoch?: unknown;
  /** 选项加载完成回调：页面需要选项数据做别的事（如概览统计）时使用。 */
  onOptionsLoaded?: (options: { value: string; label: string; count?: number }[]) => void;
}

export type ServerSelectOption = { value: string; label: string; count?: number };

export function normalizeServerSelectOptions(
  items: readonly ServerSelectOptionInput[],
): ServerSelectOption[] {
  return items.map((item) => {
    if (typeof item === 'string') return { value: item, label: item };
    return { value: item.value, label: item.label ?? item.value, count: item.count };
  });
}

export default function ServerOptionsSelect({
  fetchOptions,
  epoch,
  onOptionsLoaded,
  ...rest
}: ServerOptionsSelectProps) {
  const [options, setOptions] = useState<ServerSelectOption[]>([]);
  const [loading, setLoading] = useState(false);
  // fetchOptions 每次渲染都是新闭包是常态（内联箭头函数），经 ref 转发，
  // 保持 load 稳定引用，避免 effect 依赖抖动造成请求风暴。
  const fetcherRef = useRef(fetchOptions);
  fetcherRef.current = fetchOptions;
  const loadedRef = useRef(onOptionsLoaded);
  loadedRef.current = onOptionsLoaded;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const items = await fetcherRef.current();
      const normalized = normalizeServerSelectOptions(items ?? []);
      setOptions(normalized);
      loadedRef.current?.(normalized);
    } catch {
      // 选项聚合失败保留上次选项；不弹错（过滤器可用性优先）
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load, epoch]);

  // 顶栏切游戏/环境 → 选项全集随 scope 重拉
  useScopeReload(load);

  return (
    <Select
      showSearch
      allowClear
      optionFilterProp="label"
      loading={loading}
      options={options.map((option) => ({
        value: option.value,
        label: option.count == null ? option.label : `${option.label} (${option.count})`,
      }))}
      {...rest}
    />
  );
}
