import React, { useEffect, useMemo, useState } from 'react';
import { App, Card, Empty, Input, Modal, Radio, Space, Tag, Typography } from 'antd';
import type {
  ConflictResolution,
  MergeConflictInfo,
  MergeResponse,
} from '@/services/api/versioning';
import type { JSONValue } from '@/types/dashboard';

const { Paragraph, Text } = Typography;
const { TextArea } = Input;

type ManualResolutionMode = '' | 'draft' | 'latest' | 'custom';

interface ManualResolutionState {
  mode: ManualResolutionMode;
  customValue: string;
}

interface MergeConflictModalProps {
  open: boolean;
  loading?: boolean;
  preview: MergeResponse | null;
  onCancel: () => void;
  onSubmit: (payload: { conflicts: ConflictResolution[]; reason?: string }) => Promise<void>;
}

function formatJSONValue(value: JSONValue | undefined): string {
  if (typeof value === 'undefined') {
    return 'null';
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function defaultResolutionState(
  items: MergeConflictInfo[] | undefined,
): Record<string, ManualResolutionState> {
  if (!items?.length) {
    return {};
  }
  return Object.fromEntries(
    items.map((item) => [
      item.field,
      {
        mode: '',
        customValue: formatJSONValue(item.draftValue),
      },
    ]),
  );
}

export default function MergeConflictModal(props: MergeConflictModalProps) {
  const { open, loading = false, preview, onCancel, onSubmit } = props;
  const { message } = App.useApp();
  const [reason, setReason] = useState('');
  const [resolutions, setResolutions] = useState<Record<string, ManualResolutionState>>({});

  useEffect(() => {
    if (!open) {
      return;
    }
    setReason('');
    setResolutions(defaultResolutionState(preview?.conflictItems));
  }, [open, preview]);

  const conflictItems = useMemo(() => preview?.conflictItems || [], [preview]);
  const autoMergeItems = useMemo(() => preview?.autoMergeItems || [], [preview]);

  const canSubmit = useMemo(() => {
    return conflictItems.every((item) => {
      const current = resolutions[item.field];
      return Boolean(current?.mode);
    });
  }, [conflictItems, resolutions]);

  const updateResolution = (
    field: string,
    updater: (current: ManualResolutionState) => ManualResolutionState,
  ) => {
    setResolutions((current) => ({
      ...current,
      [field]: updater(current[field] || { mode: '', customValue: 'null' }),
    }));
  };

  const handleSubmit = async () => {
    const conflicts: ConflictResolution[] = [];
    for (const item of conflictItems) {
      const current = resolutions[item.field];
      if (!current?.mode) {
        return;
      }
      const resolution: ConflictResolution = {
        path: item.field,
        acceptNew: current.mode === 'latest',
      };
      if (current.mode === 'custom') {
        try {
          resolution.value = JSON.parse(current.customValue) as JSONValue;
        } catch {
          message.error(`字段 ${item.field} 的自定义 JSON 非法`);
          return;
        }
      }
      conflicts.push(resolution);
    }
    await onSubmit({
      conflicts,
      reason: reason.trim() || undefined,
    });
  };

  return (
    <Modal
      title="手动解决冲突"
      width={960}
      open={open}
      onCancel={onCancel}
      onOk={handleSubmit}
      okText="应用合并结果"
      okButtonProps={{ disabled: conflictItems.length > 0 && !canSubmit }}
      confirmLoading={loading}
      destroyOnClose
    >
      {preview ? (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Paragraph>
            <Text strong>{preview.message}</Text>
          </Paragraph>

          {autoMergeItems.length > 0 ? (
            <Card size="small" title={`将自动合并 ${autoMergeItems.length} 个展示字段`}>
              <Space direction="vertical" style={{ width: '100%' }}>
                {autoMergeItems.map((item) => (
                  <Space key={item.field} align="start">
                    <Tag color="blue">auto</Tag>
                    <Text code>{item.field}</Text>
                    <Text type="secondary">{item.reason}</Text>
                  </Space>
                ))}
              </Space>
            </Card>
          ) : null}

          {conflictItems.length > 0 ? (
            conflictItems.map((item) => {
              const current = resolutions[item.field] || { mode: '', customValue: 'null' };
              return (
                <Card
                  key={item.field}
                  size="small"
                  title={
                    <Space>
                      <Tag color="red">conflict</Tag>
                      <Text code>{item.field}</Text>
                    </Space>
                  }
                >
                  <Space direction="vertical" style={{ width: '100%' }} size="middle">
                    <Text type="secondary">{item.reason}</Text>
                    <Radio.Group
                      value={current.mode}
                      onChange={(event) => {
                        const nextMode = event.target.value as ManualResolutionMode;
                        updateResolution(item.field, (state) => ({ ...state, mode: nextMode }));
                      }}
                    >
                      <Space direction="vertical">
                        <Radio value="draft">保留当前草稿值</Radio>
                        <Radio value="latest">接受最新 Proposal 值</Radio>
                        <Radio value="custom">自定义 JSON</Radio>
                      </Space>
                    </Radio.Group>

                    <div
                      style={{
                        display: 'grid',
                        gap: 12,
                        gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
                      }}
                    >
                      <Card size="small" title="当前草稿值">
                        <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                          {formatJSONValue(item.draftValue)}
                        </pre>
                      </Card>
                      <Card size="small" title="最新 Proposal 值">
                        <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                          {formatJSONValue(item.latestValue)}
                        </pre>
                      </Card>
                    </div>

                    {current.mode === 'custom' ? (
                      <TextArea
                        rows={6}
                        value={current.customValue}
                        onChange={(event) => {
                          const nextValue = event.target.value;
                          updateResolution(item.field, (state) => ({
                            ...state,
                            customValue: nextValue,
                          }));
                        }}
                        placeholder="请输入合法 JSON"
                      />
                    ) : null}
                  </Space>
                </Card>
              );
            })
          ) : (
            <Empty description="当前没有冲突。可以直接接受最新 Proposal 快照。" />
          )}

          <TextArea
            rows={3}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder="可选：记录本次人工合并原因"
          />
        </Space>
      ) : (
        <Empty description="暂无冲突预览" />
      )}
    </Modal>
  );
}
