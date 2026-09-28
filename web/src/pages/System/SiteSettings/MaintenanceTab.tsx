/**
 * 运维/系统维护子 Tab（OPEN-ISSUES #52）。
 *
 * 展示运行版本 / Git 提交 / 构建时间 / 运行开始时间 / 在线时长（快照来自
 * GET /ops/system/runtime，版本字段由构建期 ldflags 注入），并提供「检查
 * 更新」入口。检查更新按最小可行收口：仅拉取版本注记（更新源为可选 L3
 * 配置 system.updateCheckUrl），**不自动执行升级**；文档链接为 #49 划归
 * 本板块的登录后消费入口（site.docsUrl）。
 */
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Alert, App, Button, Card, Descriptions, Space, Tag, Typography } from 'antd';
import { SyncOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  checkSystemUpdate,
  getSystemRuntime,
  type SystemRuntimeInfo,
  type SystemUpdateCheckResult,
} from '@/services/api/opsStatus';
import { fetchSiteConfig } from '@/services/api/sites';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

/** 在线时长人性化：「d 天 h 小时 m 分钟 s 秒」，前导零单位省略，全零/未知回「—」。 */
export function humanizeUptime(seconds?: number): string {
  if (seconds === undefined || seconds === null || seconds < 0) return '—';
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  const parts: string[] = [];
  if (d) parts.push(`${d} 天`);
  if (h) parts.push(`${h} 小时`);
  if (m) parts.push(`${m} 分钟`);
  if (s || parts.length === 0) parts.push(`${s} 秒`);
  return parts.length > 0 ? parts.join(' ') : '—';
}

function orDash(v?: string): string {
  return v && v.trim() !== '' ? v : '—';
}

export default function MaintenanceTab() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，经 ref 转发保持回调依赖稳定
  const intlRef = useRef(intl);
  intlRef.current = intl;

  const [runtime, setRuntime] = useState<SystemRuntimeInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(false);
  const [checkResult, setCheckResult] = useState<SystemUpdateCheckResult | null>(null);
  const [docsUrl, setDocsUrl] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [info, cfg] = await Promise.all([getSystemRuntime(), fetchSiteConfig()]);
      setRuntime(info);
      setDocsUrl(cfg.docsUrl || '');
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.maintenance.error.loadFailed',
            defaultMessage: '加载运行信息失败',
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

  const runCheck = async () => {
    setChecking(true);
    try {
      const result = await checkSystemUpdate();
      setCheckResult(result);
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.maintenance.error.checkFailed',
            defaultMessage: '检查更新失败',
          }),
        ),
      );
    } finally {
      setChecking(false);
    }
  };

  const alertType = checkResult
    ? checkResult.hasUpdate
      ? 'warning'
      : checkResult.checked
        ? 'success'
        : 'info'
    : undefined;

  return (
    <Space orientation="vertical" style={{ width: '100%' }} size={16}>
      <Card
        title={
          <FormattedMessage
            id="pages.systemSiteSettings.maintenance.runtime"
            defaultMessage="运行信息"
          />
        }
        loading={loading}
      >
        <Descriptions column={1} size="small" style={{ maxWidth: 640 }}>
          <Descriptions.Item
            label={
              <FormattedMessage
                id="pages.systemSiteSettings.maintenance.version"
                defaultMessage="运行版本"
              />
            }
          >
            <Tag color="blue">{orDash(runtime?.version)}</Tag>
          </Descriptions.Item>
          <Descriptions.Item
            label={
              <FormattedMessage
                id="pages.systemSiteSettings.maintenance.gitCommit"
                defaultMessage="Git 提交"
              />
            }
          >
            <Text code>{orDash(runtime?.gitCommit)}</Text>
          </Descriptions.Item>
          <Descriptions.Item
            label={
              <FormattedMessage
                id="pages.systemSiteSettings.maintenance.buildTime"
                defaultMessage="构建时间"
              />
            }
          >
            {orDash(runtime?.buildTime)}
          </Descriptions.Item>
          <Descriptions.Item
            label={
              <FormattedMessage
                id="pages.systemSiteSettings.maintenance.startedAt"
                defaultMessage="运行开始时间"
              />
            }
          >
            {orDash(runtime?.startedAt)}
          </Descriptions.Item>
          <Descriptions.Item
            label={
              <FormattedMessage
                id="pages.systemSiteSettings.maintenance.uptime"
                defaultMessage="在线时长"
              />
            }
          >
            <span title={runtime?.uptimeSeconds !== undefined ? `${runtime.uptimeSeconds}` : undefined}>
              {humanizeUptime(runtime?.uptimeSeconds)}
            </span>
          </Descriptions.Item>
          <Descriptions.Item
            label={
              <FormattedMessage
                id="pages.systemSiteSettings.maintenance.docs"
                defaultMessage="文档（关于）"
              />
            }
          >
            {docsUrl ? (
              <a href={docsUrl} target="_blank" rel="noreferrer">
                {docsUrl}
              </a>
            ) : (
              '—'
            )}
          </Descriptions.Item>
        </Descriptions>
      </Card>
      <Card
        title={
          <FormattedMessage
            id="pages.systemSiteSettings.maintenance.update"
            defaultMessage="检查更新"
          />
        }
      >
        <Space orientation="vertical" style={{ width: '100%' }}>
          <Text type="secondary">
            <FormattedMessage
              id="pages.systemSiteSettings.maintenance.updateHint"
              defaultMessage="仅检查并提示版本注记，不会自动执行升级。更新源未配置时仅显示当前版本信息。"
            />
          </Text>
          <Button
            type="primary"
            icon={<SyncOutlined spin={checking} />}
            loading={checking}
            onClick={runCheck}
          >
            <FormattedMessage
              id="pages.systemSiteSettings.maintenance.checkNow"
              defaultMessage="检查更新"
            />
          </Button>
          {checkResult && alertType ? (
            <Alert
              type={alertType}
              showIcon
              title={
                checkResult.hasUpdate
                  ? intl.formatMessage({
                      id: 'pages.systemSiteSettings.maintenance.updateAvailable',
                      defaultMessage: '发现新版本',
                    })
                  : intl.formatMessage({
                      id: 'pages.systemSiteSettings.maintenance.upToDate',
                      defaultMessage: '版本检查结果',
                    })
              }
              description={checkResult.note}
            />
          ) : null}
        </Space>
      </Card>
    </Space>
  );
}
