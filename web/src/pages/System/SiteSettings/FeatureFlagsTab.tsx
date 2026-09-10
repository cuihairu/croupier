import React, { useCallback, useEffect, useRef, useState } from 'react';
import { App, Button, Card, Popconfirm, Space, Switch, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { FormattedMessage, useIntl, useModel } from '@umijs/max';
import {
  clearSiteSetting,
  fetchFeatureSettings,
  setSiteSetting,
  type FeatureDomain,
  type FeatureDomainState,
  type FeatureSnapshot,
} from '@/services/api/sites';
import { fetchServerFeatures } from '@/services/api/features';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

// key 是 L3 设置键（行为契约）；label/description 展示文案经 textId/textDefault 走 intl
const DOMAIN_META: Record<
  FeatureDomain,
  {
    key: string;
    labelId: string;
    labelDefault: string;
    descriptionId: string;
    descriptionDefault: string;
  }
> = {
  dev: {
    key: 'features.dev',
    labelId: 'pages.systemSiteSettings.featureFlags.domain.dev.label',
    labelDefault: '研发协作',
    descriptionId: 'pages.systemSiteSettings.featureFlags.domain.dev.description',
    descriptionDefault: '缺陷追踪、工具、版本发布、热更管理',
  },
  support: {
    key: 'features.support',
    labelId: 'pages.systemSiteSettings.featureFlags.domain.support.label',
    labelDefault: '客服系统',
    descriptionId: 'pages.systemSiteSettings.featureFlags.domain.support.description',
    descriptionDefault: '工单、FAQ、反馈、玩家侧客服入口',
  },
  analytics: {
    key: 'features.analytics',
    labelId: 'pages.systemSiteSettings.featureFlags.domain.analytics.label',
    labelDefault: '数据分析',
    descriptionId: 'pages.systemSiteSettings.featureFlags.domain.analytics.description',
    descriptionDefault: '实时看板、留存、行为、支付分析',
  },
  ops: {
    key: 'features.ops',
    labelId: 'pages.systemSiteSettings.featureFlags.domain.ops.label',
    labelDefault: '运维中心',
    descriptionId: 'pages.systemSiteSettings.featureFlags.domain.ops.description',
    descriptionDefault: '节点、任务、告警、限流、备份、证书、DB 监控',
  },
  extensions: {
    key: 'features.extensions',
    labelId: 'pages.systemSiteSettings.featureFlags.domain.extensions.label',
    labelDefault: '扩展中心',
    descriptionId: 'pages.systemSiteSettings.featureFlags.domain.extensions.description',
    descriptionDefault: '扩展商店、安装与 Agent 同步',
  },
};

const DOMAINS: FeatureDomain[] = ['dev', 'support', 'analytics', 'ops', 'extensions'];

type FeatureRow = {
  domain: FeatureDomain;
  state?: FeatureDomainState;
};

export default function FeatureFlagsTab() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const { setInitialState } = useModel('@@initialState');
  const [loading, setLoading] = useState(false);
  const [snapshot, setSnapshot] = useState<FeatureSnapshot | null>(null);
  const [switching, setSwitching] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setSnapshot(await fetchFeatureSettings());
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.featureFlags.error.loadFailed',
            defaultMessage: '加载功能开关失败',
          }),
        ),
      );
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    load();
  }, [load]);

  // 刷新全局 features 缓存（菜单/路由显隐跟随合成值）。
  const syncGlobalFeatures = useCallback(async () => {
    const features = await fetchServerFeatures();
    await setInitialState((prev) => ({ ...prev, features }));
  }, [setInitialState]);

  const toggle = async (domain: FeatureDomain, next: boolean) => {
    const meta = DOMAIN_META[domain];
    setSwitching(domain);
    try {
      await setSiteSetting(meta.key, next);
      message.success(
        intl.formatMessage({
          id: next
            ? 'pages.systemSiteSettings.featureFlags.toggle.on'
            : 'pages.systemSiteSettings.featureFlags.toggle.off',
          defaultMessage: next ? '已开启，界面菜单即时生效' : '已停用，对应菜单与接口同步隐藏',
        }),
      );
      await load();
      await syncGlobalFeatures();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.systemSiteSettings.featureFlags.error.operationFailed',
            defaultMessage: '操作失败',
          }),
        ),
      );
    } finally {
      setSwitching(null);
    }
  };

  const clearOverride = async (domain: FeatureDomain) => {
    const meta = DOMAIN_META[domain];
    setSwitching(domain);
    try {
      await clearSiteSetting(meta.key);
      message.success(
        intl.formatMessage({
          id: 'pages.systemSiteSettings.featureFlags.override.cleared',
          defaultMessage: '已恢复跟随部署配置',
        }),
      );
      await load();
      await syncGlobalFeatures();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.systemSiteSettings.featureFlags.error.operationFailed',
            defaultMessage: '操作失败',
          }),
        ),
      );
    } finally {
      setSwitching(null);
    }
  };

  const columns: ColumnsType<FeatureRow> = [
    {
      title: intl.formatMessage({
        id: 'pages.systemSiteSettings.featureFlags.column.domain',
        defaultMessage: '功能域',
      }),
      dataIndex: 'domain',
      width: 140,
      render: (_, row) => (
        <Text strong>
          {intl.formatMessage({
            id: DOMAIN_META[row.domain].labelId,
            defaultMessage: DOMAIN_META[row.domain].labelDefault,
          })}
        </Text>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.systemSiteSettings.featureFlags.column.description',
        defaultMessage: '说明',
      }),
      dataIndex: 'description',
      render: (_, row) => (
        <Text type="secondary">
          {intl.formatMessage({
            id: DOMAIN_META[row.domain].descriptionId,
            defaultMessage: DOMAIN_META[row.domain].descriptionDefault,
          })}
        </Text>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.systemSiteSettings.featureFlags.column.source',
        defaultMessage: '来源',
      }),
      dataIndex: 'overridden',
      width: 140,
      render: (_, row) =>
        row.state?.trimmedByConfig ? (
          <Tag color="red">
            <FormattedMessage
              id="pages.systemSiteSettings.featureFlags.source.trimmed"
              defaultMessage="部署已裁剪（重启生效）"
            />
          </Tag>
        ) : row.state?.overridden ? (
          <Tag color="orange">
            <FormattedMessage
              id="pages.systemSiteSettings.featureFlags.source.dbOverride"
              defaultMessage="数据库覆盖"
            />
          </Tag>
        ) : (
          <Tag color="blue">
            <FormattedMessage
              id="pages.systemSiteSettings.featureFlags.source.deployConfig"
              defaultMessage="跟随部署配置"
            />
          </Tag>
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.systemSiteSettings.featureFlags.column.status',
        defaultMessage: '状态',
      }),
      dataIndex: 'enabled',
      width: 160,
      render: (_, row) => (
        <Space>
          <Switch
            checked={row.state?.enabled ?? true}
            loading={switching === row.domain}
            disabled={row.state?.trimmedByConfig && !row.state?.enabled}
            onChange={(next) => toggle(row.domain, next)}
          />
          {row.state?.overridden ? (
            <Popconfirm
              title={intl.formatMessage({
                id: 'pages.systemSiteSettings.featureFlags.override.clearConfirm',
                defaultMessage: '删除数据库覆盖？',
              })}
              description={intl.formatMessage({
                id: 'pages.systemSiteSettings.featureFlags.override.clearDescription',
                defaultMessage: '该域将恢复跟随部署配置文件的默认状态',
              })}
              onConfirm={() => clearOverride(row.domain)}
            >
              <Button size="small">
                <FormattedMessage
                  id="pages.systemSiteSettings.featureFlags.override.restore"
                  defaultMessage="恢复"
                />
              </Button>
            </Popconfirm>
          ) : null}
        </Space>
      ),
    },
  ];

  const data: FeatureRow[] = DOMAINS.map((domain) => ({
    domain,
    state: snapshot?.domains?.[domain],
  }));

  return (
    <Card loading={loading}>
      <Text type="secondary">
        <FormattedMessage
          id="pages.systemSiteSettings.featureFlags.hint"
          defaultMessage="运行时软开关：保存后立即生效（菜单与接口同步隐藏），无需重启。「部署已裁剪」表示 server.yaml 中 featureFlags 显式关闭了该域——那是物理裁剪，只能修改配置文件并重启后在此开启。"
        />
      </Text>
      <Table<FeatureRow>
        style={{ marginTop: 16 }}
        rowKey="domain"
        columns={columns}
        dataSource={data}
        pagination={false}
        size="middle"
      />
    </Card>
  );
}
