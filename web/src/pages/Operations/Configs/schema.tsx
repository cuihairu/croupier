import React from 'react';
import { Button, Space, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { FormattedMessage, getIntl } from '@umijs/max';
import type { ConfigItem, ConfigVersion } from '@/services/api/configs';
import { formatDateTime } from '@/utils/format';

// 展示 label/placeholder/title 经 getIntl 解析（SelectLang 切换语言会整页刷新重新求值）；
// filters/actions 的 key、CONFIG_FORMAT_OPTIONS 的 value 为行为契约，保持不动
const intl = getIntl();

export const CONFIG_FORMAT_OPTIONS = [
  { label: 'json', value: 'json' },
  { label: 'csv', value: 'csv' },
  { label: 'yaml', value: 'yaml' },
  { label: 'lua', value: 'lua' },
  { label: 'python', value: 'python' },
  { label: 'ini', value: 'ini' },
  { label: 'xml', value: 'xml' },
];

export type ConfigToolbarActionKey = 'query' | 'reset';

export const CONFIGS_TOOLBAR_SCHEMA = {
  filters: [
    { key: 'game', placeholder: 'Game', width: 140 },
    { key: 'env', placeholder: 'Env', width: 120 },
    {
      key: 'format',
      placeholder: intl.formatMessage({
        id: 'pages.operationsConfigs.toolbar.filter.formatPlaceholder',
        defaultMessage: '格式',
      }),
      width: 120,
    },
    {
      key: 'search',
      placeholder: intl.formatMessage({
        id: 'pages.operationsConfigs.toolbar.filter.searchPlaceholder',
        defaultMessage: '按 id 搜索',
      }),
      width: 300,
    },
  ] as Array<{ key: 'game' | 'env' | 'format' | 'search'; placeholder: string; width: number }>,
  actions: [
    {
      key: 'query',
      label: intl.formatMessage({
        id: 'pages.operationsConfigs.toolbar.action.query',
        defaultMessage: '查询',
      }),
      primary: true,
    },
    {
      key: 'reset',
      label: intl.formatMessage({
        id: 'pages.operationsConfigs.toolbar.action.reset',
        defaultMessage: '重置',
      }),
    },
  ] as Array<{ key: ConfigToolbarActionKey; label: string; primary?: boolean }>,
};

export function buildConfigColumns(
  onEdit: (id: string, format: string) => void,
): ColumnsType<ConfigItem> {
  return [
    { title: 'ID', dataIndex: 'id', width: 260, ellipsis: true },
    { title: 'Format', dataIndex: 'format', width: 100, render: (v) => <Tag>{v}</Tag> },
    { title: 'Game', dataIndex: 'gameId', width: 120 },
    { title: 'Env', dataIndex: 'env', width: 100 },
    { title: 'Latest', dataIndex: 'latestVersion', width: 80 },
    {
      title: intl.formatMessage({
        id: 'pages.operationsConfigs.column.config.actions',
        defaultMessage: '操作',
      }),
      key: 'act',
      width: 140,
      render: (_: unknown, r: ConfigItem) => (
        <Button size="small" onClick={() => onEdit(r.id, r.format)}>
          <FormattedMessage id="pages.operationsConfigs.column.config.edit" defaultMessage="编辑" />
        </Button>
      ),
    },
  ];
}

export function buildVersionColumns(
  onView: (version: number) => void,
  onDiff: (version: number) => void,
  onRollback: (version: number) => void,
): ColumnsType<ConfigVersion> {
  return [
    {
      title: intl.formatMessage({
        id: 'pages.operationsConfigs.column.version.version',
        defaultMessage: '版本',
      }),
      dataIndex: 'version',
      width: 80,
    },
    {
      title: intl.formatMessage({
        id: 'pages.operationsConfigs.column.version.createdAt',
        defaultMessage: '时间',
      }),
      dataIndex: 'createdAt',
      render: (v: string) => (v ? formatDateTime(v) : ''),
    },
    {
      title: intl.formatMessage({
        id: 'pages.operationsConfigs.column.version.createdBy',
        defaultMessage: '编辑者',
      }),
      dataIndex: 'createdBy',
      width: 120,
    },
    {
      title: intl.formatMessage({
        id: 'pages.operationsConfigs.column.version.message',
        defaultMessage: '说明',
      }),
      dataIndex: 'message',
      ellipsis: true,
    },
    {
      title: intl.formatMessage({
        id: 'pages.operationsConfigs.column.version.actions',
        defaultMessage: '操作',
      }),
      key: 'act',
      width: 220,
      render: (_: unknown, r: ConfigVersion) => (
        <Space>
          <Button size="small" onClick={() => onView(r.version)}>
            <FormattedMessage
              id="pages.operationsConfigs.column.version.view"
              defaultMessage="查看"
            />
          </Button>
          <Button size="small" onClick={() => onDiff(r.version)}>
            Diff
          </Button>
          <Button size="small" danger onClick={() => onRollback(r.version)}>
            <FormattedMessage
              id="pages.operationsConfigs.column.version.rollback"
              defaultMessage="回滚"
            />
          </Button>
        </Space>
      ),
    },
  ];
}
