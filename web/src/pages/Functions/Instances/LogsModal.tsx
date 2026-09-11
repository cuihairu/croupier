import React, { useEffect, useState } from 'react';
import { Alert, App, Button, Modal } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { FunctionInstance } from '@/services/api';

type LogEntry = {
  timestamp: string;
  level: string;
  message: string;
};

/** 实例日志弹窗：日志查询接口尚未接入，永远展示空态提示；
 * 保留数据渲染与导出逻辑，供后续接入日志聚合系统时复用。 */
export default function LogsModal({
  open,
  instance,
  onClose,
}: {
  open: boolean;
  instance: FunctionInstance | null;
  onClose: () => void;
}) {
  const { message } = App.useApp();
  const intl = useIntl();
  const [logsData, setLogsData] = useState<LogEntry[]>([]);

  useEffect(() => {
    if (!open) return;
    setLogsData([]);
  }, [open]);

  return (
    <Modal
      title={intl.formatMessage(
        { id: 'pages.functionsInstances.logs.title', defaultMessage: '日志 - {agentId}' },
        { agentId: instance?.agentId ?? '' },
      )}
      open={open}
      onCancel={onClose}
      width="min(800px, calc(100vw - 16px))"
      footer={[
        <Button key="close" onClick={onClose}>
          <FormattedMessage id="pages.functionsInstances.logs.close" defaultMessage="关闭" />
        </Button>,
        <Button
          key="export"
          onClick={() => {
            const text = logsData
              .map((l) => `[${l.timestamp}] [${l.level}] ${l.message}`)
              .join('\n');
            const blob = new Blob([text], { type: 'text/plain' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `instance-logs-${instance?.agentId}-${Date.now()}.log`;
            a.click();
            URL.revokeObjectURL(url);
            message.success(
              intl.formatMessage({
                id: 'pages.functionsInstances.logs.exported',
                defaultMessage: '日志已导出',
              }),
            );
          }}
        >
          <FormattedMessage id="pages.functionsInstances.logs.export" defaultMessage="导出日志" />
        </Button>,
      ]}
    >
      {logsData.length === 0 ? (
        <Alert
          type="info"
          showIcon
          message={intl.formatMessage({
            id: 'pages.functionsInstances.logs.empty',
            defaultMessage: '暂无日志数据',
          })}
          description={intl.formatMessage({
            id: 'pages.functionsInstances.logs.emptyDescription',
            defaultMessage: '实例日志查询接口尚未接入，当前不会展示伪造日志。',
          })}
        />
      ) : (
        <div
          style={{
            maxHeight: 400,
            overflow: 'auto',
            background: '#1e1e1e',
            padding: 12,
            borderRadius: 8,
          }}
        >
          {logsData.map((log, idx) => (
            <div key={idx} style={{ marginBottom: 8, fontFamily: 'monospace', fontSize: 12 }}>
              <span style={{ color: '#6b7280' }}>
                [{new Date(log.timestamp).toLocaleString('zh-CN')}]
              </span>
              <span
                style={{
                  color:
                    log.level === 'ERROR'
                      ? '#ef4444'
                      : log.level === 'WARN'
                        ? '#f59e0b'
                        : log.level === 'DEBUG'
                          ? '#8b5cf6'
                          : '#10b981',
                  marginLeft: 8,
                  marginRight: 8,
                }}
              >
                [{log.level}]
              </span>
              <span style={{ color: '#e5e7eb' }}>{log.message}</span>
            </div>
          ))}
        </div>
      )}
    </Modal>
  );
}
