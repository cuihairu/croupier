import React, { useEffect, useRef, useState } from 'react';
import { Alert, App, Badge, Button, Descriptions, Drawer, Tabs, Tag, Typography } from 'antd';
import { BugOutlined, HistoryOutlined } from '@ant-design/icons';
import { getFunctionDetail, type FunctionInstance } from '@/services/api';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { InstanceDetail } from './shared';

const { Text } = Typography;

/** 实例详情抽屉：概览（注册元信息）+ 日志/调试入口。
 * 函数详情数据由抽屉内部拉取；日志与调试动作经回调上抛给页面打开对应弹窗。 */
export default function InstanceDetailDrawer({
  open,
  instance,
  onClose,
  onOpenLogs,
  onOpenDebug,
}: {
  open: boolean;
  instance: FunctionInstance | null;
  onClose: () => void;
  onOpenLogs: (instance: FunctionInstance) => void;
  onOpenDebug: (instance: FunctionInstance) => void;
}) {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useEffect 依赖会让详情
  // 拉取无限重建；经 ref 转发后依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [instanceDetail, setInstanceDetail] = useState<InstanceDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  useEffect(() => {
    if (!open || !instance) return;
    let cancelled = false;
    setDetailLoading(true);
    (async () => {
      try {
        // Fetch function detail to get more info
        const functionDetail = await getFunctionDetail(instance.functionId);
        if (!cancelled) {
          setInstanceDetail({
            instance,
            functionInfo: functionDetail,
            logs: [],
          });
        }
      } catch (e) {
        const errMsg =
          e instanceof Error
            ? e.message
            : intlRef.current.formatMessage({
                id: 'pages.functionsInstances.detail.error.operationFailed',
                defaultMessage: '操作失败',
              });
        message.error(
          errMsg ||
            intlRef.current.formatMessage({
              id: 'pages.functionsInstances.detail.error.loadFailed',
              defaultMessage: '加载详情失败',
            }),
        );
      } finally {
        if (!cancelled) setDetailLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, instance, message]);

  return (
    <Drawer
      title={intl.formatMessage({
        id: 'pages.functionsInstances.detail.title',
        defaultMessage: '实例详情',
      })}
      placement="right"
      width="min(720px, calc(100vw - 16px))"
      open={open}
      onClose={onClose}
      loading={detailLoading}
    >
      {instanceDetail && (
        <Tabs
          defaultActiveKey="overview"
          items={[
            {
              key: 'overview',
              label: intl.formatMessage({
                id: 'pages.functionsInstances.detail.tab.overview',
                defaultMessage: '概览',
              }),
              children: (
                <>
                  <Descriptions
                    title={intl.formatMessage({
                      id: 'pages.functionsInstances.detail.instanceInfo.title',
                      defaultMessage: '实例信息',
                    })}
                    bordered
                    column={2}
                    size="small"
                  >
                    <Descriptions.Item label="Agent ID" span={2}>
                      <Text code copyable>
                        {instanceDetail.instance.agentId}
                      </Text>
                    </Descriptions.Item>
                    <Descriptions.Item label="Service ID" span={2}>
                      <Text code copyable>
                        {instanceDetail.instance.serviceId}
                      </Text>
                    </Descriptions.Item>
                    <Descriptions.Item
                      label={intl.formatMessage({
                        id: 'pages.functionsInstances.detail.label.functionId',
                        defaultMessage: '函数ID',
                      })}
                      span={2}
                    >
                      <Text code copyable>
                        {instanceDetail.instance.functionId}
                      </Text>
                    </Descriptions.Item>
                    <Descriptions.Item
                      label={intl.formatMessage({
                        id: 'pages.functionsInstances.detail.label.addr',
                        defaultMessage: '地址',
                      })}
                      span={2}
                    >
                      <Text code copyable>
                        {instanceDetail.instance.addr}
                      </Text>
                    </Descriptions.Item>
                    <Descriptions.Item
                      label={intl.formatMessage({
                        id: 'pages.functionsInstances.detail.label.version',
                        defaultMessage: '版本',
                      })}
                    >
                      <Tag color="blue">{instanceDetail.instance.version || '-'}</Tag>
                    </Descriptions.Item>
                    <Descriptions.Item
                      label={intl.formatMessage({
                        id: 'pages.functionsInstances.detail.label.ownerInstance',
                        defaultMessage: '归属实例',
                      })}
                    >
                      {instanceDetail.instance.ownerInstance ? (
                        <Tag color="geekblue">{instanceDetail.instance.ownerInstance}</Tag>
                      ) : (
                        <span style={{ color: '#999' }}>
                          {intl.formatMessage({
                            id: 'pages.functionsInstances.column.ownerSelf',
                            defaultMessage: '本实例',
                          })}
                        </span>
                      )}
                    </Descriptions.Item>
                    <Descriptions.Item
                      label={intl.formatMessage({
                        id: 'pages.functionsInstances.detail.label.status',
                        defaultMessage: '状态',
                      })}
                    >
                      {/* 列表层 normalize 已把后端 active 映射为 running；
                          详情与列表保持同一判定，healthy 字段后端并不返回。 */}
                      <Badge
                        status={
                          instanceDetail.instance.status === 'running'
                            ? 'success'
                            : instanceDetail.instance.status === 'error'
                              ? 'error'
                              : 'default'
                        }
                        text={
                          instanceDetail.instance.status === 'running'
                            ? intl.formatMessage({
                                id: 'pages.functionsInstances.detail.status.running',
                                defaultMessage: '运行中',
                              })
                            : instanceDetail.instance.status === 'error'
                              ? intl.formatMessage({
                                  id: 'pages.functionsInstances.detail.status.error',
                                  defaultMessage: '错误',
                                })
                              : intl.formatMessage({
                                  id: 'pages.functionsInstances.detail.status.stopped',
                                  defaultMessage: '停止',
                                })
                        }
                      />
                    </Descriptions.Item>
                    <Descriptions.Item label="Game">
                      {instanceDetail.instance.gameId || '-'}
                    </Descriptions.Item>
                    <Descriptions.Item label="Env">
                      {instanceDetail.instance.env || '-'}
                    </Descriptions.Item>
                    <Descriptions.Item
                      label={intl.formatMessage({
                        id: 'pages.functionsInstances.detail.label.lastHeartbeat',
                        defaultMessage: '最后心跳',
                      })}
                      span={2}
                    >
                      {instanceDetail.instance.lastHeartbeat ||
                        instanceDetail.instance.lastSeen ||
                        '-'}
                    </Descriptions.Item>
                  </Descriptions>

                  <Alert
                    type="info"
                    showIcon
                    style={{ marginTop: 24 }}
                    message={intl.formatMessage({
                      id: 'pages.functionsInstances.detail.metricsAlert.message',
                      defaultMessage: '运行指标与最近调用尚未接入',
                    })}
                    description={intl.formatMessage({
                      id: 'pages.functionsInstances.detail.metricsAlert.description',
                      defaultMessage:
                        '后端当前只提供实例注册与函数详情，调用统计、最近调用链路仍缺少真实接口。',
                    })}
                  />
                </>
              ),
            },
            {
              key: 'logs',
              label: (
                <span>
                  <HistoryOutlined />{' '}
                  <FormattedMessage
                    id="pages.functionsInstances.detail.tab.logs"
                    defaultMessage="日志"
                  />
                </span>
              ),
              children: (
                <div>
                  <Alert
                    type="info"
                    showIcon
                    message={intl.formatMessage({
                      id: 'pages.functionsInstances.detail.logsAlert.message',
                      defaultMessage: '实例日志尚未接入',
                    })}
                    description={intl.formatMessage({
                      id: 'pages.functionsInstances.detail.logsAlert.description',
                      defaultMessage:
                        '当前没有可用的实例日志查询接口。此处保留为后续接入日志聚合系统。',
                    })}
                    style={{ marginBottom: 12 }}
                  />
                  <Button
                    size="small"
                    onClick={() => onOpenLogs(instanceDetail.instance)}
                    style={{ marginBottom: 12 }}
                  >
                    <FormattedMessage
                      id="pages.functionsInstances.detail.viewFullLogs"
                      defaultMessage="查看完整日志"
                    />
                  </Button>
                </div>
              ),
            },
            {
              key: 'debug',
              label: (
                <span>
                  <BugOutlined />{' '}
                  <FormattedMessage
                    id="pages.functionsInstances.detail.tab.debug"
                    defaultMessage="调试"
                  />
                </span>
              ),
              children: (
                <div>
                  <Alert
                    message={intl.formatMessage({
                      id: 'pages.functionsInstances.detail.debugAlert.message',
                      defaultMessage: '调试模式',
                    })}
                    description={intl.formatMessage({
                      id: 'pages.functionsInstances.detail.debugAlert.description',
                      defaultMessage:
                        '调试请求会定向到该实例执行；参数模板按函数 Schema 自动生成，可先做参数预览。缺少 Service ID 时只能预览，不能执行。',
                    })}
                    type="info"
                    showIcon
                    style={{ marginBottom: 16 }}
                  />
                  <Button
                    type="primary"
                    onClick={() => {
                      if (instanceDetail?.instance) onOpenDebug(instanceDetail.instance);
                    }}
                  >
                    <FormattedMessage
                      id="pages.functionsInstances.detail.openDebugPanel"
                      defaultMessage="打开调试面板"
                    />
                  </Button>
                </div>
              ),
            },
          ]}
        />
      )}
    </Drawer>
  );
}
