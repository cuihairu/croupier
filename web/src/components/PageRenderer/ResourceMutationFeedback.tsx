/** D1 资源模板面写操作反馈：结果常驻可读，流程进度同源展示。
 *
 * ResourcePageRenderer 的创建/编辑/删除/行操作/批量操作共用本组件：
 * - 结果面用 Alert + Result，避免只靠会消失的 toast 追溯操作结果；
 * - 流程面用 Steps + Progress，展示「提交变更 → 同步列表」的当前阶段；
 * - 组件只消费渲染器传入的轻量状态，不发起请求、不改变刷新行为。
 */

import React from 'react';
import { Alert, Progress, Result, Steps } from 'antd';
import { CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';

export type ResourceMutationFeedbackState = 'running' | 'success' | 'error';

export interface ResourceMutationFeedbackProps {
  state: ResourceMutationFeedbackState;
  onClose: () => void;
}

const ResourceMutationFeedback: React.FC<ResourceMutationFeedbackProps> = ({ state, onClose }) => {
  const intl = useIntl();
  if (state === 'running') {
    return (
      <section data-testid="resource-mutation-feedback" aria-live="polite">
        <Steps
          size="small"
          items={[
            {
              title: intl.formatMessage({
                id: 'component.resourceRenderer.feedback.step.submit',
                defaultMessage: '提交变更',
              }),
              status: 'process',
            },
            {
              title: intl.formatMessage({
                id: 'component.resourceRenderer.feedback.step.sync',
                defaultMessage: '同步列表',
              }),
              status: 'wait',
            },
          ]}
        />
        <Progress
          data-testid="resource-mutation-progress"
          percent={40}
          status="active"
          showInfo={false}
          style={{ margin: '12px 0' }}
        />
        <Alert
          type="info"
          showIcon
          title={intl.formatMessage({
            id: 'component.resourceRenderer.feedback.running',
            defaultMessage: '资源操作正在执行，请勿重复提交',
          })}
        />
      </section>
    );
  }

  const succeeded = state === 'success';
  return (
    <section data-testid="resource-mutation-feedback" aria-live="polite">
      <Steps
        size="small"
        items={[
          {
            title: intl.formatMessage({
              id: 'component.resourceRenderer.feedback.step.submit',
              defaultMessage: '提交变更',
            }),
            status: succeeded ? 'finish' : 'error',
          },
          {
            title: intl.formatMessage({
              id: 'component.resourceRenderer.feedback.step.sync',
              defaultMessage: '同步列表',
            }),
            status: succeeded ? 'finish' : 'wait',
          },
        ]}
      />
      <Progress
        data-testid="resource-mutation-progress"
        percent={100}
        status={succeeded ? 'success' : 'exception'}
        style={{ margin: '12px 0' }}
      />
      <Result
        status={succeeded ? 'success' : 'error'}
        title={intl.formatMessage(
          succeeded
            ? {
                id: 'component.resourceRenderer.feedback.successTitle',
                defaultMessage: '资源操作已完成',
              }
            : {
                id: 'component.resourceRenderer.feedback.errorTitle',
                defaultMessage: '资源操作未完成',
              },
        )}
        subTitle={intl.formatMessage(
          succeeded
            ? {
                id: 'component.resourceRenderer.feedback.successDescription',
                defaultMessage: '列表刷新已由本次操作触发；如需核对最新数据，可再执行查询。',
              }
            : {
                id: 'component.resourceRenderer.feedback.errorDescription',
                defaultMessage: '本次变更未写入，详细原因见右上提示；修正后可重新提交。',
              },
        )}
        icon={succeeded ? <CheckCircleOutlined /> : <CloseCircleOutlined />}
        extra={
          <Alert
            type={succeeded ? 'success' : 'warning'}
            showIcon
            closable={{ onClose }}
            title={intl.formatMessage({
              id: 'component.resourceRenderer.feedback.keep',
              defaultMessage: '反馈会保留到下一次操作，可关闭',
            })}
          />
        }
      />
    </section>
  );
};

export default ResourceMutationFeedback;
