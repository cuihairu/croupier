import { useEffect, useMemo } from 'react';
import { Alert, Input, Radio, Select, Space, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { InvokeFunctionOptions } from '@/services/api';

const { Text } = Typography;

type RouteKind = NonNullable<InvokeFunctionOptions['route']>;

interface ExecutionOptionsProps {
  route: RouteKind;
  targetServiceId: string;
  hashKey: string;
  asyncMode: boolean;
  onRouteChange: (value: RouteKind) => void;
  onTargetServiceIdChange: (value: string) => void;
  onHashKeyChange: (value: string) => void;
  onAsyncModeChange: (value: boolean) => void;
}

export default function ExecutionOptions(props: ExecutionOptionsProps) {
  const { route, targetServiceId, hashKey, asyncMode } = props;
  const intl = useIntl();

  // 路由 × 执行模式合法性矩阵（与服务端 validateInvokeRoute 一致）：
  // broadcast 仅支持同步调用（异步广播被服务端 400 拒绝）；
  // targeted / hash 对同步与异步任务均有效。
  const routeOptions = useMemo(
    () => [
      {
        value: 'lb' as RouteKind,
        label: intl.formatMessage({
          id: 'pages.functionsInvoke.execution.routeOption.lb',
          defaultMessage: '负载均衡',
        }),
      },
      {
        value: 'targeted' as RouteKind,
        label: intl.formatMessage({
          id: 'pages.functionsInvoke.execution.routeOption.targeted',
          defaultMessage: '指定实例',
        }),
      },
      {
        value: 'hash' as RouteKind,
        label: intl.formatMessage({
          id: 'pages.functionsInvoke.execution.routeOption.hash',
          defaultMessage: '一致性哈希',
        }),
      },
      {
        value: 'broadcast' as RouteKind,
        label: intl.formatMessage({
          id: 'pages.functionsInvoke.execution.routeOption.broadcast',
          defaultMessage: '广播全部实例（仅同步）',
        }),
        disabled: asyncMode,
      },
    ],
    [asyncMode, intl],
  );

  // 切到异步任务时若停留在 broadcast，自动回落负载均衡（避免必然 400 的组合）。
  useEffect(() => {
    if (asyncMode && route === 'broadcast') {
      props.onRouteChange('lb');
    }
  }, [asyncMode, route, props]);

  return (
    <>
      <Space wrap size="middle">
        <Space>
          <Text type="secondary">
            <FormattedMessage
              id="pages.functionsInvoke.execution.route.label"
              defaultMessage="路由"
            />
          </Text>
          <Select
            value={route}
            style={{ width: 190 }}
            onChange={props.onRouteChange}
            options={routeOptions}
          />
        </Space>
        {route === 'targeted' ? (
          <Input
            value={targetServiceId}
            onChange={(event) => props.onTargetServiceIdChange(event.target.value)}
            placeholder={intl.formatMessage({
              id: 'pages.functionsInvoke.execution.targetedServiceId.placeholder',
              defaultMessage: '目标 service_id',
            })}
            style={{ width: 220 }}
          />
        ) : null}
        {route === 'hash' ? (
          <Input
            value={hashKey}
            onChange={(event) => props.onHashKeyChange(event.target.value)}
            placeholder="hash key"
            style={{ width: 180 }}
          />
        ) : null}
        <Radio checked={!asyncMode} onChange={() => props.onAsyncModeChange(false)}>
          <FormattedMessage id="pages.functionsInvoke.execution.mode.sync" defaultMessage="同步" />
        </Radio>
        <Radio checked={asyncMode} onChange={() => props.onAsyncModeChange(true)}>
          <FormattedMessage
            id="pages.functionsInvoke.execution.mode.async"
            defaultMessage="异步任务"
          />
        </Radio>
      </Space>
      {route === 'targeted' && !targetServiceId.trim() ? (
        <Alert
          style={{ marginTop: 12 }}
          type="warning"
          showIcon
          message={intl.formatMessage({
            id: 'pages.functionsInvoke.validation.targetedServiceId',
            defaultMessage: '指定实例路由需要填写 service_id',
          })}
        />
      ) : null}
      {route === 'hash' && !hashKey.trim() ? (
        <Alert
          style={{ marginTop: 12 }}
          type="warning"
          showIcon
          message={intl.formatMessage({
            id: 'pages.functionsInvoke.validation.hashKey',
            defaultMessage: '哈希路由需要填写 hash key',
          })}
        />
      ) : null}
      {asyncMode && route === 'broadcast' ? (
        <Alert
          style={{ marginTop: 12 }}
          type="warning"
          showIcon
          message={intl.formatMessage({
            id: 'pages.functionsInvoke.execution.broadcastAsyncWarning',
            defaultMessage: '广播仅支持同步调用——已自动切回负载均衡',
          })}
        />
      ) : null}
    </>
  );
}
