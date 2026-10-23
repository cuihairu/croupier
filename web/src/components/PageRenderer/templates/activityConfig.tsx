/**
 * activityConfig - 活动配置业务模板（默认菜单 operation 类）
 *
 * 覆盖业务闭环：活动列表（类型/时区/起止/状态/当前版本）→ 详情（版本历史）
 * → 新建活动（create，显式时区）→ 发布新版本 / 上线 / 下线（行操作）→
 * 回滚版本（危险操作）。
 *
 * 内置 demo 执行器（内存数据），接入真实环境时将 onExecute 替换为
 * `executePageBinding` 即可。
 *
 * @module components/PageRenderer/templates/activityConfig
 */

import React, { useMemo } from 'react';
import { useIntl } from '@umijs/max';
import { PageContainer } from '@ant-design/pro-components';
import PageRenderer from '@/components/PageRenderer';
import type { PageSpec, PageExecuteFn, PageExecutionResult } from '@/types/dashboard';
import {
  asRecord,
  createGameOpsDemoStore,
  createRequestIdFactory,
  type GameOpsDemoStore,
} from './gameOpsDemoData';

/** 活动配置页面规格 */
export const activityConfigPageSpec: PageSpec = {
  pageKey: 'template--activity-config',
  type: 'resource',
  resourceKey: 'activities',
  title: { 'zh-CN': '活动配置', 'en-US': 'Activity Config' },
  description: {
    'zh-CN': '活动列表、配置版本、发布与回滚（模板示例）',
    'en-US': 'Activity list, config versions, publish and rollback (template demo)',
  },
  category: {
    key: 'operation',
  },
  resource: {
    listView: {
      identityKey: 'id',
      columns: [
        {
          key: 'id',
          title: { 'zh-CN': '活动 ID', 'en-US': 'Activity ID' },
          dataType: 'string',
          width: 120,
          fixed: 'left',
          render: 'copy',
        },
        {
          key: 'name',
          title: { 'zh-CN': '活动名称', 'en-US': 'Name' },
          dataType: 'string',
          filterable: true,
        },
        {
          key: 'type',
          title: { 'zh-CN': '类型', 'en-US': 'Type' },
          dataType: 'enum',
          width: 120,
          filterable: true,
          render: 'tag',
          enum: [
            {
              value: 'login_gift',
              label: { 'zh-CN': '登录礼', 'en-US': 'Login Gift' },
              color: 'blue',
            },
            {
              value: 'rank_reward',
              label: { 'zh-CN': '排行奖励', 'en-US': 'Rank Reward' },
              color: 'purple',
            },
            {
              value: 'shop_discount',
              label: { 'zh-CN': '商店折扣', 'en-US': 'Shop Discount' },
              color: 'cyan',
            },
            {
              value: 'holiday_event',
              label: { 'zh-CN': '节日活动', 'en-US': 'Holiday Event' },
              color: 'magenta',
            },
          ],
        },
        {
          key: 'timezone',
          title: { 'zh-CN': '活动时区', 'en-US': 'Timezone' },
          dataType: 'enum',
          width: 170,
          filterable: true,
          enum: [
            {
              value: 'Asia/Shanghai',
              label: { 'zh-CN': 'Asia/Shanghai', 'en-US': 'Asia/Shanghai' },
            },
            { value: 'UTC', label: { 'zh-CN': 'UTC', 'en-US': 'UTC' } },
            {
              value: 'America/Los_Angeles',
              label: { 'zh-CN': 'America/Los_Angeles', 'en-US': 'America/Los_Angeles' },
            },
          ],
        },
        {
          key: 'startAt',
          title: { 'zh-CN': '开始时间', 'en-US': 'Start At' },
          dataType: 'datetime',
          width: 180,
          sortable: true,
        },
        {
          key: 'endAt',
          title: { 'zh-CN': '结束时间', 'en-US': 'End At' },
          dataType: 'datetime',
          width: 180,
          sortable: true,
        },
        {
          key: 'status',
          title: { 'zh-CN': '状态', 'en-US': 'Status' },
          dataType: 'enum',
          width: 110,
          filterable: true,
          render: 'tag',
          enum: [
            { value: 'draft', label: { 'zh-CN': '草稿', 'en-US': 'Draft' }, color: 'default' },
            {
              value: 'configured',
              label: { 'zh-CN': '已配置', 'en-US': 'Configured' },
              color: 'geekblue',
            },
            {
              value: 'published',
              label: { 'zh-CN': '已发布', 'en-US': 'Published' },
              color: 'blue',
            },
            { value: 'active', label: { 'zh-CN': '进行中', 'en-US': 'Active' }, color: 'green' },
            { value: 'ended', label: { 'zh-CN': '已结束', 'en-US': 'Ended' }, color: 'default' },
            {
              value: 'rolled_back',
              label: { 'zh-CN': '已回滚', 'en-US': 'Rolled Back' },
              color: 'volcano',
            },
          ],
        },
        {
          key: 'currentVersion',
          title: { 'zh-CN': '当前版本', 'en-US': 'Version' },
          dataType: 'number',
          width: 100,
          sortable: true,
        },
      ],
      pagination: { enabled: true, defaultSize: 10, pageSizes: [10, 20] },
      rowActions: [
        {
          key: 'publish',
          title: { 'zh-CN': '发布新版本', 'en-US': 'Publish Version' },
          type: 'primary',
          form: {
            jsonSchema: {
              type: 'object',
              properties: {
                note: { type: 'string', title: '版本说明' },
              },
              required: ['note'],
            },
            layout: 'vertical',
          },
          bindingId: 'action.publish',
        },
        {
          key: 'activate',
          title: { 'zh-CN': '上线', 'en-US': 'Activate' },
          confirm: true,
          confirmTitle: { 'zh-CN': '上线活动', 'en-US': 'Activate Activity' },
          confirmDescription: {
            'zh-CN': '上线后活动配置对玩家生效，确认上线？',
            'en-US': 'The published config will take effect for players. Activate?',
          },
          bindingId: 'action.activate',
        },
        {
          key: 'end',
          title: { 'zh-CN': '下线', 'en-US': 'End' },
          confirm: true,
          confirmTitle: { 'zh-CN': '下线活动', 'en-US': 'End Activity' },
          confirmDescription: {
            'zh-CN': '下线后活动立即对玩家不可见，确认下线？',
            'en-US': 'The activity will be hidden from players immediately. End it?',
          },
          bindingId: 'action.end',
        },
        {
          key: 'rollback',
          title: { 'zh-CN': '回滚版本', 'en-US': 'Rollback' },
          type: 'danger',
          form: {
            jsonSchema: {
              type: 'object',
              properties: {
                targetVersion: { type: 'number', title: '回滚目标版本号' },
              },
              required: ['targetVersion'],
            },
            layout: 'vertical',
          },
          bindingId: 'action.rollback',
        },
      ],
    },
    detailView: {
      layout: 'horizontal',
      fields: [
        { key: 'id', title: { 'zh-CN': '活动 ID', 'en-US': 'Activity ID' }, dataType: 'string' },
        { key: 'name', title: { 'zh-CN': '活动名称', 'en-US': 'Name' }, dataType: 'string' },
        { key: 'type', title: { 'zh-CN': '类型', 'en-US': 'Type' }, dataType: 'string' },
        {
          key: 'timezone',
          title: { 'zh-CN': '活动时区', 'en-US': 'Timezone' },
          dataType: 'string',
        },
        {
          key: 'startAt',
          title: { 'zh-CN': '开始时间', 'en-US': 'Start At' },
          dataType: 'datetime',
        },
        { key: 'endAt', title: { 'zh-CN': '结束时间', 'en-US': 'End At' }, dataType: 'datetime' },
        { key: 'status', title: { 'zh-CN': '状态', 'en-US': 'Status' }, dataType: 'string' },
        {
          key: 'versionHistory',
          title: { 'zh-CN': '版本历史', 'en-US': 'Version History' },
          dataType: 'string',
        },
        {
          key: 'currentVersion',
          title: { 'zh-CN': '当前版本', 'en-US': 'Version' },
          dataType: 'number',
        },
      ],
    },
    createForm: {
      jsonSchema: {
        type: 'object',
        properties: {
          name: { type: 'string', title: '活动名称' },
          type: {
            type: 'string',
            title: '活动类型',
            enum: ['login_gift', 'rank_reward', 'shop_discount', 'holiday_event'],
            enumNames: ['登录礼', '排行奖励', '商店折扣', '节日活动'],
          },
          timezone: {
            type: 'string',
            title: '活动时区',
            enum: ['Asia/Shanghai', 'UTC', 'America/Los_Angeles'],
            enumNames: ['Asia/Shanghai', 'UTC', 'America/Los_Angeles'],
          },
          startAt: { type: 'string', title: '开始时间（YYYY-MM-DD HH:mm，活动时区）' },
          endAt: { type: 'string', title: '结束时间（YYYY-MM-DD HH:mm，活动时区）' },
        },
        required: ['name', 'type', 'timezone'],
      },
      layout: 'vertical',
    },
  },
  bindings: [
    {
      id: 'list',
      functionId: 'activity.list',
      usage: 'query',
      selectors: {
        input: { assignments: [] },
        output: [
          { stateKey: 'items', source: '/items', shape: 'collection' },
          { stateKey: 'total', source: '/total', shape: 'scalar' },
        ],
      },
      execution: { mode: 'sync' },
    },
    {
      id: 'detail',
      functionId: 'activity.get',
      usage: 'detail',
      selectors: {
        input: { assignments: [{ target: '/id', source: { kind: 'row', path: '/id' } }] },
        output: [{ stateKey: 'detail', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync' },
    },
    {
      id: 'create',
      functionId: 'activity.create',
      usage: 'action',
      execution: { mode: 'sync' },
    },
    {
      id: 'action.publish',
      functionId: 'activity.publish_version',
      usage: 'action',
      selectors: {
        input: {
          assignments: [
            { target: '/id', source: { kind: 'row', path: '/id' } },
            { target: '/note', source: { kind: 'form', path: '/note' } },
          ],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync', requireConfirm: false },
    },
    {
      id: 'action.activate',
      functionId: 'activity.activate',
      usage: 'action',
      selectors: {
        input: {
          assignments: [{ target: '/id', source: { kind: 'row', path: '/id' } }],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync', requireConfirm: true },
    },
    {
      id: 'action.end',
      functionId: 'activity.end',
      usage: 'action',
      selectors: {
        input: {
          assignments: [{ target: '/id', source: { kind: 'row', path: '/id' } }],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync', requireConfirm: true },
    },
    {
      id: 'action.rollback',
      functionId: 'activity.rollback',
      usage: 'action',
      selectors: {
        input: {
          assignments: [
            { target: '/id', source: { kind: 'row', path: '/id' } },
            { target: '/targetVersion', source: { kind: 'form', path: '/targetVersion' } },
          ],
        },
        output: [{ stateKey: 'result', source: '', shape: 'object' }],
      },
      execution: { mode: 'sync', requireConfirm: true },
    },
  ],
};

/**
 * 创建活动配置 demo 执行器：内存实现 list/detail（含版本历史摘要）/
 * create 与发布新版本、上线、下线、回滚。状态机前置校验：
 * 上线要求已发布、下线要求进行中、回滚要求目标版本存在。
 */
export function createActivityConfigDemoExecute(
  store: GameOpsDemoStore = createGameOpsDemoStore(),
): PageExecuteFn {
  let activities = store.activities;
  let seq = activities.length + 1;
  const nextRequestId = createRequestIdFactory();

  const nextId = () => `act-demo-${++seq}`;

  return async (bindingId, context): Promise<PageExecutionResult> => {
    const form = asRecord(context.form);
    const row = asRecord(context.row);

    switch (bindingId) {
      case 'list': {
        const current = Number(form.current) || 1;
        const pageSize = Number(form.pageSize) || 10;
        let filtered = activities;
        if (form.name) {
          filtered = filtered.filter((activity) => activity.name.includes(String(form.name)));
        }
        if (form.status) {
          filtered = filtered.filter((activity) => activity.status === form.status);
        }
        if (form.type) {
          filtered = filtered.filter((activity) => activity.type === form.type);
        }
        const start = (current - 1) * pageSize;
        return {
          kind: 'sync',
          requestId: nextRequestId(),
          data: { items: filtered.slice(start, start + pageSize), total: filtered.length },
        };
      }
      case 'detail': {
        const activity = activities.find((item) => item.id === String(row.id));
        if (!activity) {
          return { kind: 'sync', requestId: nextRequestId(), data: null };
        }
        return {
          kind: 'sync',
          requestId: nextRequestId(),
          data: {
            ...activity,
            versionHistory: activity.versions
              .slice()
              .reverse()
              .map((version) => `v${version.version} ${version.note} · ${version.operator}`)
              .join('；'),
          },
        };
      }
      case 'create': {
        const name = String(form.name ?? '').trim();
        if (!name) {
          throw new Error('name_required');
        }
        const type = String(form.type ?? 'login_gift');
        const timezone = String(form.timezone ?? 'Asia/Shanghai');
        const activity = {
          id: nextId(),
          name,
          type: (['login_gift', 'rank_reward', 'shop_discount', 'holiday_event'].includes(type)
            ? type
            : 'login_gift') as 'login_gift',
          timezone: timezone as 'Asia/Shanghai' | 'UTC' | 'America/Los_Angeles',
          startAt: String(form.startAt ?? ''),
          endAt: String(form.endAt ?? ''),
          status: 'configured' as const,
          currentVersion: 0,
          versions: [],
        };
        activities = [activity, ...activities];
        store.pushOpRecord({
          operator: 'admin',
          action: 'activity.create',
          target: activity.id,
          result: 'success',
        });
        return { kind: 'sync', requestId: nextRequestId(), data: { id: activity.id } };
      }
      case 'action.publish': {
        const activity = activities.find((item) => item.id === String(row.id));
        if (!activity) {
          throw new Error('activity_not_found');
        }
        const version = activity.currentVersion + 1;
        activities = activities.map((item) =>
          item.id === activity.id
            ? {
                ...item,
                status: 'published',
                currentVersion: version,
                versions: [
                  ...item.versions,
                  {
                    version,
                    note: String(form.note ?? ''),
                    publishedAt: new Date().toISOString(),
                    operator: 'admin',
                  },
                ],
              }
            : item,
        );
        store.pushOpRecord({
          operator: 'admin',
          action: 'activity.publish',
          target: activity.id,
          result: 'success',
        });
        return { kind: 'sync', requestId: nextRequestId(), data: { version } };
      }
      case 'action.activate': {
        const activity = activities.find((item) => item.id === String(row.id));
        if (!activity) {
          throw new Error('activity_not_found');
        }
        if (activity.status !== 'published' && activity.status !== 'rolled_back') {
          throw new Error('activate_requires_published');
        }
        activities = activities.map((item) =>
          item.id === activity.id ? { ...item, status: 'active' } : item,
        );
        store.pushOpRecord({
          operator: 'admin',
          action: 'activity.activate',
          target: activity.id,
          result: 'success',
        });
        return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
      }
      case 'action.end': {
        const activity = activities.find((item) => item.id === String(row.id));
        if (!activity) {
          throw new Error('activity_not_found');
        }
        if (activity.status !== 'active' && activity.status !== 'published') {
          throw new Error('end_requires_active');
        }
        activities = activities.map((item) =>
          item.id === activity.id ? { ...item, status: 'ended' } : item,
        );
        store.pushOpRecord({
          operator: 'admin',
          action: 'activity.end',
          target: activity.id,
          result: 'success',
        });
        return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
      }
      case 'action.rollback': {
        const activity = activities.find((item) => item.id === String(row.id));
        if (!activity) {
          throw new Error('activity_not_found');
        }
        const targetVersion = Number(form.targetVersion) || 0;
        const exists = activity.versions.some((version) => version.version === targetVersion);
        if (!exists) {
          throw new Error('target_version_not_found');
        }
        activities = activities.map((item) =>
          item.id === activity.id
            ? { ...item, status: 'rolled_back', currentVersion: targetVersion }
            : item,
        );
        store.pushOpRecord({
          operator: 'admin',
          action: 'activity.rollback',
          target: activity.id,
          result: 'success',
        });
        return {
          kind: 'sync',
          requestId: nextRequestId(),
          data: { success: true, version: targetVersion },
        };
      }
      default:
        return { kind: 'sync', requestId: nextRequestId(), data: { success: true } };
    }
  };
}

/**
 * 活动配置模板页面：活动列表 + 版本历史详情 + 发布/上线/下线/回滚。
 */
const ActivityConfigTemplate: React.FC = () => {
  const intl = useIntl();
  const execute = useMemo(() => createActivityConfigDemoExecute(), []);
  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'component.pageRenderer.activityConfig.pageTitle',
        defaultMessage: '活动配置',
      })}
      subTitle={intl.formatMessage({
        id: 'component.pageRenderer.activityConfig.pageSubtitle',
        defaultMessage: '业务模板 · 活动列表 / 配置版本 / 发布 / 回滚',
      })}
    >
      <PageRenderer pageSpec={activityConfigPageSpec} onExecute={execute} />
    </PageContainer>
  );
};

export default ActivityConfigTemplate;
