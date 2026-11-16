/** 邮件与补偿模板的固定分步表单（D1 StepsForm 复用接入）。
 *
 * GUX-011/012/013 标杆流程：目标与数量步骤即时汇总实际目标（人数/区服/
 * 全服提示，不杜撰服务端才知道的量）；提交前核对步骤给出结构化摘要与
 * 审批提示；执行回执展示接口真实返回的可追溯标识（任务 ID / 审批单号），
 * 未返回的字段不展示；executor 未绑定时以 ExecutorUnboundAlert 解释不可用，
 * 不伪装成普通失败。
 */

import React, { useMemo, useState } from 'react';
import { useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Progress,
  Radio,
  Select,
  Typography,
} from 'antd';
import { StepsForm } from '@ant-design/pro-components';
import OperationStageBar from '../OperationStageBar';
import type { OperationStageInput } from '../operationStages';
import ExecutorUnboundAlert, { EXECUTOR_UNBOUND_CODE } from '../ExecutorUnboundAlert';
import { extractApiErrorCode } from '@/utils/apiError';
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
  | { status: 'success'; taskId?: string }
  | { status: 'approval'; approvalId?: string; taskId?: string }
  | { status: 'error'; message: string; unbound: boolean };

const serverOptions = [
  { value: 's1', label: 'S1' },
  { value: 's2', label: 'S2' },
  { value: 's3', label: 'S3' },
];

/** 固定流程表单：任务信息 → 目标与数量 → 提交前核对 → 执行回执。 */
const CompensationWizard: React.FC<{
  execute: PageExecuteFn;
  /** create 绑定指向的函数（executor 未绑定时在回执区展示定位入口） */
  createFunctionId?: string;
  onCreate?: () => void;
}> = ({ execute, createFunctionId, onCreate }) => {
  const intl = useIntl();
  const [execution, setExecution] = useState<WizardExecutionState>({ status: 'idle' });
  const [range, setRange] = useState<CompensationWizardValues['receiverRange']>('all');
  const [type, setType] = useState<CompensationWizardValues['type']>('mail');
  // 目标步骤即时汇总（GUX-011）：客户端能确定的部分即时显示，人数以服务端
  // 为准的部分显式说明，不杜撰
  const [livePlayerCount, setLivePlayerCount] = useState(0);
  const [liveServer, setLiveServer] = useState<string>();
  const [liveAmount, setLiveAmount] = useState<number>();
  // 提交前核对（GUX-012）：前两步 onFinish 落值供 review 摘要展示；提交
  // 仍以 StepsForm 汇总的完整表单值为准
  const [capturedBasic, setCapturedBasic] = useState<Partial<CompensationWizardValues>>({});
  const [capturedTargets, setCapturedTargets] = useState<Partial<CompensationWizardValues>>({});

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
      optionMail: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.option.mail',
        defaultMessage: '邮件',
      }),
      optionCompensation: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.option.compensation',
        defaultMessage: '补偿',
      }),
      rangeAll: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.range.all',
        defaultMessage: '全服',
      }),
      rangeServer: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.range.server',
        defaultMessage: '指定区服',
      }),
      rangePlayers: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.range.players',
        defaultMessage: '指定玩家',
      }),
      selectedCount: (count: number) =>
        intl.formatMessage(
          {
            id: 'component.pageRenderer.compensationWizard.targets.selectedCount',
            defaultMessage: '已选 {count} 名玩家',
          },
          { count },
        ),
      allHint: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.targets.allHint',
        defaultMessage: '全服发放：实际目标人数以服务端为准',
      }),
      serverHint: (server: string) =>
        intl.formatMessage(
          {
            id: 'component.pageRenderer.compensationWizard.targets.serverHint',
            defaultMessage: '区服 {server} 全量玩家：实际目标人数以服务端为准',
          },
          { server },
        ),
      summaryTitle: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.summary.title',
        defaultMessage: '提交前核对',
      }),
      summaryTargets: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.summary.targets',
        defaultMessage: '目标',
      }),
      summaryPerAmount: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.summary.perAmount',
        defaultMessage: '每人数量',
      }),
      summaryTotal: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.summary.total',
        defaultMessage: '总量',
      }),
      approvalNote: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.summary.approvalNote',
        defaultMessage: '补偿任务提交后进入审批，审批通过后才会执行',
      }),
      mailNote: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.summary.mailNote',
        defaultMessage: '邮件任务提交后直接进入发送',
      }),
      receiptTaskId: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.receipt.taskId',
        defaultMessage: '任务 ID',
      }),
      receiptApprovalId: intl.formatMessage({
        id: 'component.pageRenderer.compensationWizard.receipt.approvalId',
        defaultMessage: '审批单号',
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
      const data =
        result.data && typeof result.data === 'object'
          ? (result.data as { id?: string; approvalId?: string })
          : undefined;
      const taskId = (result as { id?: string }).id ?? data?.id;
      const approvalId = (result as { approvalId?: string }).approvalId ?? data?.approvalId;
      if (approvalId) {
        setExecution({ status: 'approval', approvalId, taskId });
      } else {
        setExecution({ status: 'success', taskId });
      }
      return true;
    } catch (error) {
      setExecution({
        status: 'error',
        message: error instanceof Error ? error.message : String(error),
        // GUX-011：executor 未绑定是「不可用」而非普通失败，回执区给
        // ExecutorUnboundAlert 解释与去绑定入口
        unbound: extractApiErrorCode(error) === EXECUTOR_UNBOUND_CODE,
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

  // GUX-012 review 摘要：目标数只对指定玩家可确定；全服/区服不杜撰人数，
  // 总量同理只在可确定时展示
  const capturedPlayerIds = capturedTargets.targetPlayerIds ?? [];
  const capturedPerAmount = capturedTargets.perAmount;
  const capturedTotal =
    capturedBasic.type === 'compensation' &&
    capturedTargets.receiverRange === 'players' &&
    typeof capturedPerAmount === 'number'
      ? capturedPerAmount * capturedPlayerIds.length
      : undefined;

  return (
    <Card title={labels.title} style={{ marginBottom: 16 }}>
      <StepsForm<CompensationWizardValues> onFinish={submit}>
        <StepsForm.StepForm
          name="basic"
          title={labels.basic}
          initialValues={{ type: 'mail' }}
          onFinish={async (values) => {
            setCapturedBasic(values);
            if (values.type) setType(values.type);
            // pro-components 按 onFinish 返回值放行下一步：同步 undefined 会被挡，
            // 须显式 resolve true
            return true;
          }}
        >
          <Form.Item name="type" label={labels.type} rules={[{ required: true }]}>
            <Radio.Group
              options={[
                { value: 'mail', label: labels.optionMail },
                { value: 'compensation', label: labels.optionCompensation },
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
          onFinish={async (values) => {
            setCapturedTargets(values);
            if (values.receiverRange) setRange(values.receiverRange);
            return true;
          }}
        >
          <Form.Item name="receiverRange" label={labels.receiverRange} rules={[{ required: true }]}>
            <Radio.Group
              options={[
                { value: 'all', label: labels.rangeAll },
                { value: 'server', label: labels.rangeServer },
                { value: 'players', label: labels.rangePlayers },
              ]}
              onChange={(event) => setRange(event.target.value)}
            />
          </Form.Item>
          {range === 'server' && (
            <Form.Item name="targetServerId" label={labels.server} rules={[{ required: true }]}>
              <Select
                options={serverOptions}
                onChange={(value) =>
                  setLiveServer(typeof value === 'string' ? value : String(value))
                }
              />
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
                onChange={(value) => setLivePlayerCount(Array.isArray(value) ? value.length : 0)}
              />
            </Form.Item>
          )}
          {type === 'compensation' && (
            <Form.Item name="perAmount" label={labels.amount} rules={[{ required: true }]}>
              <InputNumber
                min={1}
                style={{ width: '100%' }}
                onChange={(value) => setLiveAmount(typeof value === 'number' ? value : undefined)}
              />
            </Form.Item>
          )}
          {/* GUX-011 即时目标汇总：客户端可确定的部分即时可见，服务端才知道
              的量显式说明，不杜撰 */}
          <Typography.Text type="secondary" style={{ display: 'block' }}>
            {range === 'all'
              ? labels.allHint
              : range === 'server'
                ? liveServer
                  ? labels.serverHint(liveServer)
                  : labels.allHint
                : labels.selectedCount(livePlayerCount)}
          </Typography.Text>
        </StepsForm.StepForm>
        <StepsForm.StepForm name="review" title={labels.review}>
          {/* GUX-012 提交前核对：实际类型/标题/范围/目标/数量结构化汇总 */}
          <Descriptions
            size="small"
            column={1}
            bordered
            title={labels.summaryTitle}
            style={{ marginBottom: 16 }}
          >
            <Descriptions.Item label={labels.type}>
              {type === 'compensation' ? labels.optionCompensation : labels.optionMail}
            </Descriptions.Item>
            {capturedBasic.title ? (
              <Descriptions.Item label={labels.taskTitle}>{capturedBasic.title}</Descriptions.Item>
            ) : null}
            <Descriptions.Item label={labels.receiverRange}>
              {range === 'all'
                ? labels.rangeAll
                : range === 'server'
                  ? labels.rangeServer
                  : labels.rangePlayers}
            </Descriptions.Item>
            {range === 'server' && capturedTargets.targetServerId ? (
              <Descriptions.Item label={labels.server}>
                {capturedTargets.targetServerId}
              </Descriptions.Item>
            ) : null}
            {range === 'players' && (
              <Descriptions.Item label={labels.summaryTargets}>
                {labels.selectedCount(capturedPlayerIds.length)}
              </Descriptions.Item>
            )}
            {type === 'compensation' && typeof capturedPerAmount === 'number' ? (
              <Descriptions.Item label={labels.summaryPerAmount}>
                {capturedPerAmount}
              </Descriptions.Item>
            ) : null}
            {type === 'compensation' &&
            typeof liveAmount === 'number' &&
            range === 'players' &&
            typeof capturedTotal === 'number' ? (
              <Descriptions.Item label={labels.summaryTotal}>{capturedTotal}</Descriptions.Item>
            ) : null}
          </Descriptions>
          <Typography.Text type={type === 'compensation' ? 'warning' : 'secondary'}>
            {type === 'compensation' ? labels.approvalNote : labels.mailNote}
          </Typography.Text>
          <Form.Item
            name="confirm"
            valuePropName="checked"
            rules={[
              {
                validator: (_rule, value: boolean | undefined) =>
                  value ? Promise.resolve() : Promise.reject(new Error(labels.confirm)),
              },
            ]}
            style={{ marginTop: 12 }}
          >
            <Checkbox>{labels.confirm}</Checkbox>
          </Form.Item>
        </StepsForm.StepForm>
      </StepsForm>

      {execution.status !== 'idle' && (
        <div style={{ marginTop: 16 }}>
          {execution.status === 'error' && execution.unbound ? (
            // GUX-011：无执行器=功能不可用，解释 + 去绑定入口，不做普通失败态
            <ExecutorUnboundAlert functionId={createFunctionId} />
          ) : (
            <>
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
                title={
                  execution.status === 'error'
                    ? labels.error
                    : execution.status === 'approval'
                      ? labels.approval
                      : labels.success
                }
                description={execution.status === 'error' ? execution.message : undefined}
                action={
                  execution.status === 'error' && !execution.unbound ? (
                    <Button onClick={() => setExecution({ status: 'idle' })}>{labels.retry}</Button>
                  ) : undefined
                }
              />
              {/* GUX-013 执行回执：只展示接口真实返回的可追溯标识 */}
              {(execution.status === 'approval' && (execution.taskId || execution.approvalId)) ||
              (execution.status === 'success' && execution.taskId) ? (
                <Descriptions size="small" column={1} bordered style={{ marginTop: 12 }}>
                  {execution.taskId ? (
                    <Descriptions.Item label={labels.receiptTaskId}>
                      <Typography.Text code>{execution.taskId}</Typography.Text>
                    </Descriptions.Item>
                  ) : null}
                  {execution.status === 'approval' && execution.approvalId ? (
                    <Descriptions.Item label={labels.receiptApprovalId}>
                      <Typography.Text code>{execution.approvalId}</Typography.Text>
                    </Descriptions.Item>
                  ) : null}
                </Descriptions>
              ) : null}
            </>
          )}
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
