/**
 * 性能参数子 Tab（OPEN-ISSUES #53）。
 *
 * 性能参数卡：六键 L3 覆盖（阈值/上限 0 = 不启用），保存走 PUT /ops/performance；
 * 运行时快照卡：Go 进程统计 + 宿主机 CPU/内存/磁盘占用 + 阈值超限注记。
 *
 * 边界（诚实）：阈值本批仅注记不拦截请求；GOMAXPROCS 不随 maxThreadCount 热改；
 * cacheSize 仅存储回显（internal/cache 尚无容量上限语义可接）。
 */
import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  InputNumber,
  Row,
  Space,
  Tag,
  Typography,
} from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  fetchPerformance,
  savePerformance,
  type PerformanceSnapshot,
} from '@/services/api/performance';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

type PerfFormValues = {
  maxCpuPct: number;
  maxMemoryPct: number;
  maxDiskPct: number;
  maxConcurrent: number;
  maxThreadCount: number;
  cacheSizeMb: number;
};

/** 字节 → MB 展示（cacheSize 表单以 MB 编辑，保存换回字节）。 */
const MB = 1024 * 1024;

function SourceTagInline({ source }: { source?: string }) {
  const intl = useIntl();
  if (!source || source === 'default') return null;
  const label =
    source === 'database'
      ? intl.formatMessage({
          id: 'pages.systemSiteSettings.auth.sourceTag.ui',
          defaultMessage: 'UI',
        })
      : intl.formatMessage({
          id: 'pages.systemSiteSettings.auth.sourceTag.configFile',
          defaultMessage: '配置文件',
        });
  return (
    <Tag color={source === 'database' ? 'blue' : 'orange'} style={{ marginLeft: 6 }}>
      {label}
    </Tag>
  );
}

function formatBytes(bytes?: number): string {
  if (bytes === undefined || bytes === null || Number.isNaN(bytes)) return '—';
  if (bytes < MB) return `${bytes} B`;
  if (bytes < MB * MB) return `${(bytes / MB).toFixed(1)} MB`;
  return `${(bytes / (MB * MB)).toFixed(2)} GB`;
}

function pctText(v?: number): string {
  if (v === undefined || v === null || Number.isNaN(v)) return '—';
  return `${v.toFixed(1)}%`;
}

export default function PerformanceTab() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，经 ref 转发保持回调依赖稳定
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [form] = Form.useForm<PerfFormValues>();
  const [snap, setSnap] = useState<PerformanceSnapshot | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const applySnapshot = useCallback(
    (data: PerformanceSnapshot) => {
      setSnap(data);
      form.setFieldsValue({
        maxCpuPct: data.settings.maxCpuPct,
        maxMemoryPct: data.settings.maxMemoryPct,
        maxDiskPct: data.settings.maxDiskPct,
        maxConcurrent: data.settings.maxConcurrent,
        maxThreadCount: data.settings.maxThreadCount,
        cacheSizeMb: data.settings.cacheSize / MB,
      });
    },
    [form],
  );

  const load = useCallback(async () => {
    setLoading(true);
    try {
      applySnapshot(await fetchPerformance());
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.performance.error.load',
            defaultMessage: '加载性能数据失败',
          }),
        ),
      );
    } finally {
      setLoading(false);
    }
  }, [message, applySnapshot]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleSave = async () => {
    try {
      const v = await form.validateFields();
      setSaving(true);
      const data = await savePerformance({
        'perf.maxCpuPct': v.maxCpuPct ?? 0,
        'perf.maxMemoryPct': v.maxMemoryPct ?? 0,
        'perf.maxDiskPct': v.maxDiskPct ?? 0,
        'perf.maxConcurrent': v.maxConcurrent ?? 0,
        'perf.maxThreadCount': v.maxThreadCount ?? 0,
        'perf.cacheSize': Math.round((v.cacheSizeMb ?? 0) * MB),
      });
      applySnapshot(data);
      message.success(
        intlRef.current.formatMessage({
          id: 'pages.systemSiteSettings.performance.saved',
          defaultMessage: '性能参数已保存',
        }),
      );
    } catch (error) {
      if ((error as { errorFields?: unknown }).errorFields) return;
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.performance.error.save',
            defaultMessage: '保存失败',
          }),
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  const rt = snap?.runtime;
  const host = snap?.host;
  const overload = snap?.overload;

  return (
    <Space orientation="vertical" style={{ width: '100%' }} size={16}>
      <Alert
        type="info"
        showIcon
        title={intl.formatMessage({
          id: 'pages.systemSiteSettings.performance.boundary',
          defaultMessage:
            '阈值用于运行注记（超过时此处标红提示），本批不自动拦截请求；GOMAXPROCS 与缓存容量为只读展示',
        })}
      />
      <Card
        title={
          <FormattedMessage
            id="pages.systemSiteSettings.performance.settings.title"
            defaultMessage="性能参数"
          />
        }
        extra={
          <Button type="primary" size="small" loading={saving} onClick={() => void handleSave()}>
            <FormattedMessage
              id="pages.systemSiteSettings.performance.action.save"
              defaultMessage="保存"
            />
          </Button>
        }
      >
        <Form form={form} layout="vertical" size="small" style={{ maxWidth: 720 }}>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item
                name="maxCpuPct"
                label={
                  <Space size={4}>
                    <FormattedMessage
                      id="pages.systemSiteSettings.performance.field.maxCpuPct"
                      defaultMessage="CPU 阈值 %"
                    />
                    <SourceTagInline source={snap?.settings.sources?.maxCpuPct} />
                  </Space>
                }
                tooltip={intl.formatMessage({
                  id: 'pages.systemSiteSettings.performance.tooltip.zeroOff',
                  defaultMessage: '0 = 不启用该限制',
                })}
              >
                <InputNumber min={0} max={100} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="maxMemoryPct"
                label={
                  <Space size={4}>
                    <FormattedMessage
                      id="pages.systemSiteSettings.performance.field.maxMemoryPct"
                      defaultMessage="内存阈值 %"
                    />
                    <SourceTagInline source={snap?.settings.sources?.maxMemoryPct} />
                  </Space>
                }
              >
                <InputNumber min={0} max={100} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="maxDiskPct"
                label={
                  <Space size={4}>
                    <FormattedMessage
                      id="pages.systemSiteSettings.performance.field.maxDiskPct"
                      defaultMessage="磁盘阈值 %"
                    />
                    <SourceTagInline source={snap?.settings.sources?.maxDiskPct} />
                  </Space>
                }
              >
                <InputNumber min={0} max={100} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="maxConcurrent"
                label={
                  <Space size={4}>
                    <FormattedMessage
                      id="pages.systemSiteSettings.performance.field.maxConcurrent"
                      defaultMessage="并发请求上限"
                    />
                    <SourceTagInline source={snap?.settings.sources?.maxConcurrent} />
                  </Space>
                }
              >
                <InputNumber min={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="maxThreadCount"
                label={
                  <Space size={4}>
                    <FormattedMessage
                      id="pages.systemSiteSettings.performance.field.maxThreadCount"
                      defaultMessage="线程数上限"
                    />
                    <SourceTagInline source={snap?.settings.sources?.maxThreadCount} />
                  </Space>
                }
                tooltip={intl.formatMessage({
                  id: 'pages.systemSiteSettings.performance.tooltip.thread',
                  defaultMessage: '本批仅记录，不热改 GOMAXPROCS',
                })}
              >
                <InputNumber min={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="cacheSizeMb"
                label={
                  <Space size={4}>
                    <FormattedMessage
                      id="pages.systemSiteSettings.performance.field.cacheSizeMb"
                      defaultMessage="内存缓存 (MB)"
                    />
                    <SourceTagInline source={snap?.settings.sources?.cacheSize} />
                  </Space>
                }
                tooltip={intl.formatMessage({
                  id: 'pages.systemSiteSettings.performance.tooltip.cache',
                  defaultMessage: '本批仅存储回显，暂未接入运行时缓存',
                })}
              >
                <InputNumber min={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Card>
      <Card
        title={
          <FormattedMessage
            id="pages.systemSiteSettings.performance.runtime.title"
            defaultMessage="运行时统计"
          />
        }
        loading={loading}
        extra={
          <Button
            icon={<ReloadOutlined />}
            size="small"
            loading={loading}
            onClick={() => void load()}
          >
            <FormattedMessage
              id="pages.systemSiteSettings.performance.action.refresh"
              defaultMessage="刷新"
            />
          </Button>
        }
      >
        <Row gutter={24}>
          <Col span={12}>
            <Descriptions
              column={1}
              size="small"
              title={intl.formatMessage({
                id: 'pages.systemSiteSettings.performance.runtime.process',
                defaultMessage: '进程',
              })}
            >
              <Descriptions.Item label="GOMAXPROCS">{rt?.goMaxProcs ?? '—'}</Descriptions.Item>
              <Descriptions.Item label="Goroutines">{rt?.goroutines ?? '—'}</Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.systemSiteSettings.performance.runtime.heapAlloc',
                  defaultMessage: '堆内存（存活）',
                })}
              >
                {formatBytes(rt?.heapAllocBytes)}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.systemSiteSettings.performance.runtime.sysBytes',
                  defaultMessage: '进程总内存',
                })}
              >
                {formatBytes(rt?.sysBytes)}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.systemSiteSettings.performance.runtime.gc',
                  defaultMessage: 'GC 次数 / 累计暂停',
                })}
              >
                {rt ? `${rt.numGC} / ${rt.gcPauseMs.toFixed(1)} ms` : '—'}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.systemSiteSettings.performance.runtime.uptime',
                  defaultMessage: '在线时长',
                })}
              >
                {rt ? `${rt.uptimeSeconds} s` : '—'}
              </Descriptions.Item>
            </Descriptions>
          </Col>
          <Col span={12}>
            <Descriptions
              column={1}
              size="small"
              title={intl.formatMessage({
                id: 'pages.systemSiteSettings.performance.runtime.host',
                defaultMessage: '宿主机',
              })}
            >
              <Descriptions.Item
                label={
                  <Space size={6}>
                    CPU
                    {overload?.cpu && (
                      <Tag color="red">
                        <FormattedMessage
                          id="pages.systemSiteSettings.performance.overlimit"
                          defaultMessage="超阈值"
                        />
                      </Tag>
                    )}
                  </Space>
                }
              >
                <Text type={overload?.cpu ? 'danger' : undefined}>{pctText(host?.cpuPercent)}</Text>
              </Descriptions.Item>
              <Descriptions.Item
                label={
                  <Space size={6}>
                    {intl.formatMessage({
                      id: 'pages.systemSiteSettings.performance.field.memory',
                      defaultMessage: '内存',
                    })}
                    {overload?.memory && (
                      <Tag color="red">
                        <FormattedMessage
                          id="pages.systemSiteSettings.performance.overlimit"
                          defaultMessage="超阈值"
                        />
                      </Tag>
                    )}
                  </Space>
                }
              >
                <Text type={overload?.memory ? 'danger' : undefined}>
                  {pctText(host?.memoryUsedPct)}（{formatBytes(host?.memoryUsedBytes)} /{' '}
                  {formatBytes(host?.memoryTotalBytes)}）
                </Text>
              </Descriptions.Item>
              <Descriptions.Item
                label={
                  <Space size={6}>
                    {intl.formatMessage({
                      id: 'pages.systemSiteSettings.performance.field.disk',
                      defaultMessage: '磁盘',
                    })}
                    {overload?.disk && (
                      <Tag color="red">
                        <FormattedMessage
                          id="pages.systemSiteSettings.performance.overlimit"
                          defaultMessage="超阈值"
                        />
                      </Tag>
                    )}
                  </Space>
                }
              >
                <Text type={overload?.disk ? 'danger' : undefined}>
                  {pctText(host?.diskUsedPct)}（{formatBytes(host?.diskUsedBytes)} /{' '}
                  {formatBytes(host?.diskTotalBytes)}）
                </Text>
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.systemSiteSettings.performance.runtime.diskPath',
                  defaultMessage: '采样目录',
                })}
              >
                {host?.diskPath || '—'}
              </Descriptions.Item>
            </Descriptions>
          </Col>
        </Row>
      </Card>
    </Space>
  );
}
