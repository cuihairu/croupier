import { Alert, Button, Card, Space, Tabs, Tag, Tooltip, Input } from 'antd';
import { ClockCircleOutlined, CopyOutlined } from '@ant-design/icons';
import { CodeEditor } from '@/components/MonacoDynamic';
import { formatDuration } from './types';

const { TextArea } = Input;

interface InvocationResponseProps {
  responseRaw: string;
  error: string;
  duration: number;
  onCopy: (value: string) => void;
}

export default function InvocationResponse({
  responseRaw,
  error,
  duration,
  onCopy,
}: InvocationResponseProps) {
  return (
    <Card
      size="small"
      title="响应"
      extra={
        <Space>
          {error ? <Tag color="red">失败</Tag> : <Tag color="green">成功</Tag>}
          {duration ? <Tag icon={<ClockCircleOutlined />}>{formatDuration(duration)}</Tag> : null}
          {responseRaw || error ? (
            <Tooltip title="复制响应">
              <Button
                size="small"
                icon={<CopyOutlined />}
                onClick={() => onCopy(responseRaw || error)}
              />
            </Tooltip>
          ) : null}
        </Space>
      }
    >
      {error ? (
        <Alert type="error" showIcon message="调用失败" description={error} />
      ) : (
        <Tabs
          items={[
            {
              key: 'pretty',
              label: '格式化',
              children: (
                <CodeEditor
                  value={responseRaw || 'null'}
                  language="json"
                  theme="vs-dark"
                  readOnly
                  height={320}
                  options={{ lineNumbers: 'on', folding: true, scrollBeyondLastLine: false }}
                />
              ),
            },
            {
              key: 'raw',
              label: '原始数据',
              children: (
                <TextArea
                  readOnly
                  value={responseRaw}
                  rows={12}
                  style={{ fontFamily: 'var(--ant-font-family-code, monospace)' }}
                />
              ),
            },
          ]}
        />
      )}
    </Card>
  );
}
