/**
 * Console API 服务
 *
 * 运行控制台专用 API，从 PublishedPageSpec 读取数据。
 */

import { request } from '@umijs/max';
import type { ConsoleMenuSpec, PublishedPageSpec } from '@/types/dashboard';

const BASE = '/api/v1/console';

type ConsolePagesResponse = {
  items?: PublishedPageSpec[];
};

type ConsolePageResponse = {
  page?: PublishedPageSpec;
};

/** 获取运行控制台菜单 */
export async function getConsoleMenu(lang?: string): Promise<ConsoleMenuSpec> {
  return request<ConsoleMenuSpec>(`${BASE}/menu`, {
    method: 'GET',
    params: lang ? { lang } : undefined,
  });
}

/** 获取已发布页面列表 */
export async function listPublishedPages(): Promise<PublishedPageSpec[]> {
  const response = await request<ConsolePagesResponse>(`${BASE}/pages`, {
    method: 'GET',
  });
  return Array.isArray(response?.items) ? response.items : [];
}

/** 获取单个已发布页面 */
export async function getPublishedPage(pageKey: string): Promise<PublishedPageSpec> {
  const response = await request<ConsolePageResponse>(`${BASE}/pages/${encodeURIComponent(pageKey)}`, {
    method: 'GET',
  });
  if (!response?.page) {
    throw new Error(`published page not found: ${pageKey}`);
  }
  return response.page;
}
