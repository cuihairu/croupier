import { Alert, Input, Radio, Select, Space, Typography } from 'antd';
import type { InvokeFunctionOptions } from '@/services/api';

const { Text } = Typography;

interface ExecutionOptionsProps {
  route: NonNullable<InvokeFunctionOptions['route']>;
  targetServiceId: string;
  hashKey: string;
  asyncMode: boolean;
  onRouteChange: (value: NonNullable<InvokeFunctionOptions['route']>) => void;
  onTargetServiceIdChange: (value: string) => void;
  onHashKeyChange: (value: string) => void;
  onAsyncModeChange: (value: boolean) => void;
}

export default function ExecutionOptions(props: ExecutionOptionsProps) {
  const { route, targetServiceId, hashKey, asyncMode } = props;
  return (
    <>
      <Space wrap size="middle">
        <Space>
          <Text type="secondary">路由</Text>
          <Select
            value={route}
            style={{ width: 130 }}
            onChange={props.onRouteChange}
            options={[
              { value: 'lb', label: '负载均衡' },
              { value: 'broadcast', label: '广播全部实例' },
              { value: 'targeted', label: '指定实例' },
              { value: 'hash', label: '一致性哈希' },
            ]}
          />
        </Space>
        {route === 'targeted' ? (
          <Input
            value={targetServiceId}
            onChange={(event) => props.onTargetServiceIdChange(event.target.value)}
            placeholder="目标 service_id"
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
          同步
        </Radio>
        <Radio checked={asyncMode} onChange={() => props.onAsyncModeChange(true)}>
          异步任务
        </Radio>
      </Space>
      {route === 'targeted' && !targetServiceId.trim() ? (
        <Alert
          style={{ marginTop: 12 }}
          type="warning"
          showIcon
          message="指定实例路由需要填写 service_id"
        />
      ) : null}
      {route === 'hash' && !hashKey.trim() ? (
        <Alert
          style={{ marginTop: 12 }}
          type="warning"
          showIcon
          message="哈希路由需要填写 hash key"
        />
      ) : null}
    </>
  );
}
