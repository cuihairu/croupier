/** 邮件与补偿模板的固定分步表单（D1 StepsForm 复用接入）。 */

import React, { useMemo, useState } from 'react';
import { useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Form,
  Input,
  InputNumber,
  Progress,
  Radio,
  Select,
} from 'antd';
import { StepsForm } from '@ant-design/pro-components';
import OperationStageBar from '../OperationStageBar';
import type { OperationStageInput } from '../operationStages';
import type { BindingExecutionContext, JSONValue, PageExecuteFn } from '@/types/dashboard';
import { gameOpsDemoRuntimeStore } from './gameOpsDemoRuntime';

interface CompensationWizardValues {
  type: 'mail' | 'compensation';
  title: string;
  detail?: string;
  receiverRange: 'all' | 'server' | 'players';
  targetServerId?: string;
  targetPlayerIds?: string[];
  perAmount?: number;
  confirm?: boolean;
}

type WizardExecutionState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { status: 'success' }
  | { status: 'approval'; approvalId?: string }
  | { status: 'error'; message: string };

const serverOptions = [
  { value: 's1', label: 'S1' },
  { value: 's2', label: 'S2' },
  { value: 's3', label: 'S3' },
];

/** 固定流程表单：任务信息 → 目标与数量 → 提交状态反馈。 */
const CompensationWizard: React.FC<{
  execute: PageExecuteFn;
  onCreate?: () => void;
}> = ({ execute, onCreate }) => {
  const intl = useIntl();
  const [execution, setExecution] = useState<WizardExecutionState>({ status: 'idle' });
  const [range, setRange] = useState<CompensationWizardValues['receiverRange']>('all');
  const [type, setType] = useState<CompensationWizardValues['type']>('mail');

  const labels = useMemo(
    () => ({
      title: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.title',
        defaultMessage: '固定分步配置',
      }),
      basic: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.step.basic',
        defaultMessage: '任务信息',
      }),
      targets: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.step.targets',
        defaultMessage: '目标与数量',
      }),
      review: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.step.review',
        defaultMessage: '检查并提交',
      }),
      type: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.field.type',
        defaultMessage: '任务类型',
      }),
      taskTitle: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.field.title',
        defaultMessage: '标题/事由',
      }),
      taskDetail: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.field.detail',
        defaultMessage: '正文/原因',
      }),
      receiverRange: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.field.range',
        defaultMessage: '发放范围',
      }),
      server: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.field.server',
        defaultMessage: '区服',
      }),
      player: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.field.player',
        defaultMessage: '玩家 ID',
      }),
      amount: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.field.amount',
        defaultMessage: '每人数量（金）',
      }),
      success: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.success',
        defaultMessage: '任务已提交',
      }),
      approval: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.approval',
        defaultMessage: '已提交审批，等待审批通过后执行',
      }),
      error: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.error',
        defaultMessage: '提交失败',
      }),
      confirm: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.field.confirm',
        defaultMessage: '确认提交任务',
      }),
      retry: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.retry',
        defaultMessage: '重新提交',
      }),
    }),
    [intl],
  );

  const stageInput = useMemo<OperationStageInput>(() => {
    if (execution.status === 'submitting') {
      return {
        loading: true,
        hasError: false,
        resultKind: null,
        approvalStatus: null,
        continuationKind: null,
      };
    }
    if (execution.status === 'error') {
      return {
        loading: false,
        hasError: true,
        resultKind: null,
        approvalStatus: null,
        continuationKind: null,
      };
    }
    if (execution.status === 'approval') {
      return {
        loading: false,
        hasError: false,
        resultKind: 'approval',
        approvalStatus: 'pending',
        continuationKind: null,
      };
    }
    if (execution.status === 'success') {
      return {
        loading: false,
        hasError: false,
        resultKind: 'sync',
        approvalStatus: null,
        continuationKind: null,
      };
    }
    return {
      loading: false,
      hasError: false,
      resultKind: null,
      approvalStatus: null,
      continuationKind: null,
    };
  }, [execution]);

  const submit = async (rawValues: CompensationWizardValues) => {
    const values = rawValues;
    setExecution({ status: 'submitting' });
    try {
      const context: BindingExecutionContext = { form: values as unknown as JSONValue };
      const result = await execute('create', context);
      onCreate?.();
      const approvalId =
        result.approvalId ?? (result.data as { approvalId?: string } | undefined)?.approvalId;
      if (approvalId) {
        setExecution({ status: 'approval', approvalId });
      } else {
        setExecution({ status: 'success' });
      }
      return true;
    } catch (error) {
      setExecution({
        status: 'error',
        message: error instanceof Error ? error.message : String(error),
      });
      return false;
    }
  };

  const progressPercent =
    execution.status === 'approval' ? 70 : execution.status === 'submitting' ? 30 : 100;
  const progressStatus =
    execution.status === 'error'
      ? 'exception'
      : execution.status === 'approval' || execution.status === 'submitting'
        ? 'active'
        : 'success';

  return (
    <Card title={labels.title} style={{ marginBottom: 16 }}>
      <StepsForm<CompensationWizardValues> onFinish={submit}>
        <StepsForm.StepForm name="basic" title={labels.basic} initialValues={{ type: 'mail' }}>
          <Form.Item name="type" label={labels.type} rules={[{ required: true }]}>
            <Radio.Group
              options={[
                { value: 'mail', label: '邮件' },
                { value: 'compensation', label: '补偿' },
              ]}
              onChange={(event) => setType(event.target.value)}
            />
          </Form.Item>
          <Form.Item name="title" label={labels.taskTitle} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="detail" label={labels.taskDetail}>
            <Input.TextArea rows={3} />
          </Form.Item>
        </StepsForm.StepForm>
        <StepsForm.StepForm
          name="targets"
          title={labels.targets}
          initialValues={{ receiverRange: 'all' }}
        >
          <Form.Item name="receiverRange" label={labels.receiverRange} rules={[{ required: true }]}>
            <Radio.Group
              options={[
                { value: 'all', label: '全服' },
                { value: 'server', label: '指定区服' },
                { value: 'players', label: '指定玩家' },
              ]}
              onChange={(event) => setRange(event.target.value)}
            />
          </Form.Item>
          {range === 'server' && (
            <Form.Item name="targetServerId" label={labels.server} rules={[{ required: true }]}>
              <Select options={serverOptions} />
            </Form.Item>
          )}
          {range === 'players' && (
            <Form.Item name="targetPlayerIds" label={labels.player} rules={[{ required: true }]}>
              <Select
                mode="multiple"
                showSearch
                optionFilterProp="label"
                options={gameOpsDemoRuntimeStore.players.map((player) => ({
                  value: player.id,
                  label: `${player.id} · ${player.nickname} · ${player.serverId}`,
                }))}
              />
            </Form.Item>
          )}
          {type === 'compensation' && (
            <Form.Item name="perAmount" label={labels.amount} rules={[{ required: true }]}>
              <InputNumber min={1} style={{ width: '100%' }} />
            </Form.Item>
          )}
        </StepsForm.StepForm>
        <StepsForm.StepForm name="review" title={labels.review}>
          <Form.Item
            name="confirm"
            valuePropName="checked"
            rules={[
              {
                validator: (_rule, value: boolean | undefined) =>
                  value ? Promise.resolve() : Promise.reject(new Error(labels.confirm)),
              },
            ]}
          >
            <Checkbox>{labels.confirm}</Checkbox>
          </Form.Item>
        </StepsForm.StepForm>
      </StepsForm>

      {execution.status !== 'idle' && (
        <div style={{ marginTop: 16 }}>
          <Alert
            role="status"
            type={
              execution.status === 'error'
                ? 'error'
                : execution.status === 'approval'
                  ? 'info'
                  : 'success'
            }
            showIcon
            title={labels.review}
            description={
              execution.status === 'error'
                ? execution.message
                : execution.status === 'approval'
                  ? labels.approval
                  : labels.success
            }
            action={
              execution.status === 'error' ? (
                <Button onClick={() => setExecution({ status: 'idle' })}>{labels.retry}</Button>
              ) : undefined
            }
          />
          <OperationStageBar input={stageInput} />
          <Progress
            status={progressStatus}
            percent={progressPercent}
            showInfo={execution.status !== 'submitting'}
          />
        </div>
      )}
    </Card>
  );
};

export default CompensationWizard;
export type { CompensationWizardValues, WizardExecutionState };
