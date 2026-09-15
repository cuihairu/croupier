/**
 * ExecutorUnboundAlert — T8/D2 执行边界空态。
 *
 * binding 指向的契约 unbound（上传物料未绑定运行时执行器）时服务端以
 * 409 executor_unbound 阻断（见 console/service.go ensureExecutorBound）；
 * 本组件是该错误码的统一前端呈现：显式空态 + 去绑定入口，禁止伪数据
 * 兜底或静默失败。入口暂跳 OpenAPI Sources 页（T9 编辑器内绑定抽屉
 * 落地后可升级为原地绑定）。
 */
import React from 'react';
import { FormattedMessage, getIntl, history } from '@umijs/max';
import { Alert, Button, Typography } from 'antd';
import { extractApiErrorCode } from '@/utils/apiError';

/** 服务端 409 稳定码（与 internal/api/console/service.go ensureExecutorBound 对齐） */
export const EXECUTOR_UNBOUND_CODE = 'executor_unbound';

export interface ExecutorUnboundAlertProps {
  /** 触发阻断的函数（可选，展示引导定位） */
  functionId?: string;
  style?: React.CSSProperties;
}

const ExecutorUnboundAlert: React.FC<ExecutorUnboundAlertProps> = ({ functionId, style }) => (
  <Alert
    type="warning"
    showIcon
    style={style}
    message={
      <FormattedMessage
        id="component.pageRenderer.executorUnbound.title"
        defaultMessage="未绑定执行器"
      />
    }
    description={
      <>
        <div>
          <FormattedMessage
            id="component.pageRenderer.executorUnbound.description"
            defaultMessage="该页面的函数来自上传物料，尚未绑定运行时执行器，执行已被阻断；请先绑定后再执行。"
          />
        </div>
        {functionId ? (
          <div>
            <Typography.Text code>{functionId}</Typography.Text>
          </div>
        ) : null}
      </>
    }
    action={
      <Button
        size="small"
        type="primary"
        onClick={() => history.push('/functions/openapi-sources')}
      >
        <FormattedMessage
          id="component.pageRenderer.executorUnbound.goBind"
          defaultMessage="去绑定"
        />
      </Button>
    }
  />
);

export default ExecutorUnboundAlert;

/** 执行失败 toast 文案：unbound 阻断给专属指引，其余回退调用方通用文案。
 * 供无内联错误展示位的提交路径（弹窗提交 / 行操作）复用。 */
export function executeErrorToastText(err: unknown, fallback: string): string {
  if (extractApiErrorCode(err) === EXECUTOR_UNBOUND_CODE) {
    return getIntl().formatMessage({
      id: 'component.pageRenderer.executorUnbound.toast',
      defaultMessage: '函数尚未绑定运行时执行器，请先完成绑定',
    });
  }
  return fallback;
}
