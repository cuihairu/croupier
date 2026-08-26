import React, { useCallback, useEffect, useState } from 'react';
import { App, Button, Card, Popconfirm, Space, Switch, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useModel } from '@umijs/max';
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

const DOMAIN_META: Record<FeatureDomain, { label: string; description: string; key: string }> = {
  dev: {
    label: '研发协作',
    description: '缺陷追踪、工具、版本发布、热更管理',
    key: 'features.dev',
  },
  support: {
    label: '客服系统',
    description: '工单、FAQ、反馈、玩家侧客服入口',
    key: 'features.support',
  },
  analytics: {
    label: '数据分析',
    description: '实时看板、留存、行为、支付分析',
    key: 'features.analytics',
  },
  ops: {
    label: '运维中心',
    description: '节点、任务、告警、限流、备份、证书、DB 监控',
    key: 'features.ops',
  },
  extensions: {
    label: '扩展中心',
    description: '扩展商店、安装与 Agent 同步',
    key: 'features.extensions',
  },
};

const DOMAINS: FeatureDomain[] = ['dev', 'support', 'analytics', 'ops', 'extensions'];

type FeatureRow = {
  domain: FeatureDomain;
  state?: FeatureDomainState;
};

export default function FeatureFlagsTab() {
  const { message } = App.useApp();
  const { setInitialState } = useModel('@@initialState');
  const [loading, setLoading] = useState(false);
  const [snapshot, setSnapshot] = useState<FeatureSnapshot | null>(null);
  const [switching, setSwitching] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setSnapshot(await fetchFeatureSettings());
    } catch (error) {
      message.error(extractErrorMessage(error, '加载功能开关失败'));
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
      message.success(next ? '已开启，界面菜单即时生效' : '已停用，对应菜单与接口同步隐藏');
      await load();
      await syncGlobalFeatures();
    } catch (error) {
      message.error(extractErrorMessage(error, '操作失败'));
    } finally {
      setSwitching(null);
    }
  };

  const clearOverride = async (domain: FeatureDomain) => {
    const meta = DOMAIN_META[domain];
    setSwitching(domain);
    try {
      await clearSiteSetting(meta.key);
      message.success('已恢复跟随部署配置');
      await load();
      await syncGlobalFeatures();
    } catch (error) {
      message.error(extractErrorMessage(error, '操作失败'));
    } finally {
      setSwitching(null);
    }
  };

  const columns: ColumnsType<FeatureRow> = [
    {
      title: '功能域',
      dataIndex: 'domain',
      width: 140,
      render: (_, row) => <Text strong>{DOMAIN_META[row.domain].label}</Text>,
    },
    {
      title: '说明',
      dataIndex: 'description',
      render: (_, row) => <Text type="secondary">{DOMAIN_META[row.domain].description}</Text>,
    },
    {
      title: '来源',
      dataIndex: 'overridden',
      width: 140,
      render: (_, row) =>
        row.state?.trimmedByConfig ? (
          <Tag color="red">部署已裁剪（重启生效）</Tag>
        ) : row.state?.overridden ? (
          <Tag color="orange">数据库覆盖</Tag>
        ) : (
          <Tag color="blue">跟随部署配置</Tag>
        ),
    },
    {
      title: '状态',
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
              title="删除数据库覆盖？"
              description="该域将恢复跟随部署配置文件的默认状态"
              onConfirm={() => clearOverride(row.domain)}
            >
              <Button size="small">恢复</Button>
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
        运行时软开关：保存后立即生效（菜单与接口同步隐藏），无需重启。 「部署已裁剪」表示
        server.yaml 中 featureFlags 显式关闭了该域——那是物理裁剪，
        只能修改配置文件并重启后在此开启。
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
