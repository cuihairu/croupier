import React, { useEffect, useState } from 'react';
import { Alert, App, Button, Input, Modal, Space, Tag, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import validator from '@rjsf/validator-ajv8';
import type { RJSFSchema } from '@rjsf/utils';
import { getFunctionDetail, invokeFunction, type FunctionInstance } from '@/services/api';
import { deriveSchemaDefaults } from '@/utils/json';
import type { JSONValue } from '@/types/dashboard';
import { resolveDescriptorSchema } from './shared';

const { Text } = Typography;

/** 在线调试弹窗：按函数 Schema 派生参数模板；
 * 参数预览只做本地 JSON Schema 校验，执行定向到当前 Service ID 的实例。 */
export default function DebugModal({
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
  const [debugPayload, setDebugPayload] = useState('{\n  \n}');
  const [debugSchema, setDebugSchema] = useState<RJSFSchema | null>(null);
  const [debugResult, setDebugResult] = useState<Record<string, JSONValue> | null>(null);
  const [debugLoading, setDebugLoading] = useState(false);

  // 打开调试面板：拉取函数 schema 并按类型派生默认参数骨架
  useEffect(() => {
    if (!open || !instance) return;
    let cancelled = false;
    setDebugResult(null);
    setDebugSchema(null);
    setDebugPayload('{\n  \n}');
    if (!instance.functionId) return;
    (async () => {
      try {
        const descriptor = await getFunctionDetail(instance.functionId);
        if (cancelled) return;
        const schema = resolveDescriptorSchema(descriptor);
        if (!schema) return;
        setDebugSchema(schema as RJSFSchema);
        const defaults = deriveSchemaDefaults(schema);
        setDebugPayload(JSON.stringify(defaults, null, 2));
      } catch {
        // schema 拉取失败不阻断调试，保留空模板由用户手写
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, instance]);

  const executeDebug = async (dryRun = false) => {
    if (!instance) return;

    const targetServiceId = instance.serviceId?.trim() || '';
    if (!dryRun && !targetServiceId) {
      message.error(
        intl.formatMessage({
          id: 'pages.functionsInstances.debug.error.missingServiceId',
          defaultMessage: '当前实例缺少 Service ID，无法定向执行；可使用参数预览。',
        }),
      );
      setDebugResult({
        success: false,
        mode: 'execute',
        error: intl.formatMessage({
          id: 'pages.functionsInstances.debug.error.missingServiceIdBlocked',
          defaultMessage: '当前实例缺少 Service ID，已阻止负载均衡调用。',
        }),
      });
      return;
    }

    setDebugLoading(true);
    let payload: JSONValue;
    try {
      payload = JSON.parse(debugPayload) as JSONValue;
    } catch {
      message.error(
        intl.formatMessage({
          id: 'pages.functionsInstances.debug.error.invalidJson',
          defaultMessage: '无效的 JSON 格式',
        }),
      );
      setDebugLoading(false);
      return;
    }

    if (debugSchema) {
      const validation = validator.validateFormData(payload, debugSchema);
      if (validation.errors?.length) {
        setDebugResult({
          success: false,
          mode: 'preview',
          validation: 'failed',
          errors: validation.errors.map((error) =>
            `${error.property || intl.formatMessage({ id: 'pages.functionsInstances.debug.validation.paramFallback', defaultMessage: '参数' })} ${error.message || intl.formatMessage({ id: 'pages.functionsInstances.debug.validation.schemaMismatch', defaultMessage: '不符合 Schema' })}`.trim(),
          ),
          payload,
        });
        setDebugLoading(false);
        return;
      }
    }

    // 参数预览只做 JSON Schema 校验与目标预览，不发起真实调用。
    if (dryRun) {
      setDebugResult({
        success: true,
        mode: 'preview',
        validation: debugSchema ? 'passed' : 'unavailable',
        functionId: instance.functionId,
        targetServiceId,
        agentId: instance.agentId,
        payload,
      });
      setDebugLoading(false);
      return;
    }

    try {
      const result = await invokeFunction(instance.functionId, payload, {
        // 定向到当前选中的实例，避免负载均衡落到其他 provider
        route: 'targeted',
        targetServiceId,
      });
      setDebugResult({
        success: true,
        mode: 'execute',
        functionId: instance.functionId,
        targetServiceId: instance.serviceId,
        traceId: result?.traceId || '',
        taskId: result?.taskId || '',
        result: (result?.result ?? result) as JSONValue,
      });
    } catch (e) {
      setDebugResult({
        success: false,
        error: {
          message:
            e instanceof Error
              ? e.message
              : intl.formatMessage({
                  id: 'pages.functionsInstances.debug.error.executeFailed',
                  defaultMessage: '调试执行失败',
                }),
        },
      });
    } finally {
      setDebugLoading(false);
    }
  };

  const debugFooter = [
    <Button key="cancel" onClick={onClose}>
      <FormattedMessage id="pages.functionsInstances.debug.button.cancel" defaultMessage="取消" />
    </Button>,
    <Button key="dryRun" onClick={() => executeDebug(true)} loading={debugLoading}>
      <FormattedMessage
        id="pages.functionsInstances.debug.button.preview"
        defaultMessage="参数预览"
      />
    </Button>,
    <Button
      key="execute"
      type="primary"
      danger
      onClick={() => executeDebug(false)}
      disabled={!instance?.serviceId?.trim()}
      loading={debugLoading}
    >
      <FormattedMessage id="pages.functionsInstances.debug.button.execute" defaultMessage="执行" />
    </Button>,
  ];

  return (
    <Modal
      title={intl.formatMessage(
        { id: 'pages.functionsInstances.debug.title', defaultMessage: '调试 - {functionId}' },
        { functionId: instance?.functionId ?? '' },
      )}
      open={open}
      onCancel={onClose}
      width="min(700px, calc(100vw - 16px))"
      footer={debugFooter}
    >
      <Space orientation="vertical" style={{ width: '100%' }} size="large">
        <Alert
          message={intl.formatMessage({
            id: 'pages.functionsInstances.debug.alert.message',
            defaultMessage: '调试请求将真实执行',
          })}
          description={
            instance?.serviceId?.trim()
              ? intl.formatMessage({
                  id: 'pages.functionsInstances.debug.alert.descriptionWithServiceId',
                  defaultMessage:
                    '参数预览只在浏览器本地校验 JSON Schema，不会调用服务；执行会定向发送到当前 Service ID。',
                })
              : intl.formatMessage({
                  id: 'pages.functionsInstances.debug.alert.descriptionWithoutServiceId',
                  defaultMessage:
                    '当前实例没有 Service ID，已禁用真实执行；参数预览仍可用于检查 JSON 和 Schema。',
                })
          }
          type="warning"
          showIcon
        />

        <div>
          <Text strong>
            <FormattedMessage
              id="pages.functionsInstances.debug.label.targetInstance"
              defaultMessage="目标实例:"
            />
          </Text>
          <div style={{ marginTop: 8 }}>
            <Tag color="blue">{instance?.agentId}</Tag>
            <Tag color="purple">{instance?.gameId || 'default'}</Tag>
            <Tag>{instance?.env || 'dev'}</Tag>
          </div>
        </div>

        <div>
          <Text strong>
            <FormattedMessage
              id="pages.functionsInstances.debug.label.requestParams"
              defaultMessage="请求参数 (JSON):"
            />
          </Text>
          <Input.TextArea
            style={{ marginTop: 8, fontFamily: 'monospace' }}
            rows={10}
            value={debugPayload}
            onChange={(e) => setDebugPayload(e.target.value)}
            placeholder='{\n  "param1": "value1"\n}'
          />
        </div>

        {debugResult && (
          <div>
            <Text strong>
              <FormattedMessage
                id="pages.functionsInstances.debug.label.result"
                defaultMessage="调试结果:"
              />
            </Text>
            <pre
              style={{
                marginTop: 8,
                padding: 12,
                background:
                  debugResult &&
                  typeof debugResult === 'object' &&
                  !Array.isArray(debugResult) &&
                  'success' in debugResult &&
                  debugResult.success
                    ? '#f6ffed'
                    : '#fff2f0',
                border: `1px solid ${debugResult && typeof debugResult === 'object' && !Array.isArray(debugResult) && 'success' in debugResult && debugResult.success ? '#b7eb8f' : '#ffccc7'}`,
                borderRadius: 8,
                fontSize: 12,
                maxHeight: 200,
                overflow: 'auto',
              }}
            >
              {JSON.stringify(debugResult, null, 2)}
            </pre>
          </div>
        )}
      </Space>
    </Modal>
  );
}
