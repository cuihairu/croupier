import { Alert, Button, Modal, Space, Typography } from 'antd';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import type { OpenAPISourcePipelineSummary } from '@/services/api/openapi';

const { Text } = Typography;

/** 上传即成页管线摘要 Modal（T7/D4）：上传成功后展示单请求内
 * 契约/组件模板/页面提案的生成计数与解析诊断，CTA 直达组合页
 * 编辑器与提案收件箱。摘要缺失（旧后端）时调用方不渲染本组件。 */
export default function PipelineSummaryModal({
  summary,
  onClose,
}: {
  summary: OpenAPISourcePipelineSummary | null;
  onClose: () => void;
}) {
  const intl = useIntl();
  const counters: Array<{ id: string; defaultMessage: string; count: number }> = [
    {
      id: 'pages.openapiSources.pipelineModal.operations',
      defaultMessage: '解析操作：{count}',
      count: summary?.operations ?? 0,
    },
    {
      id: 'pages.openapiSources.pipelineModal.contractsCreated',
      defaultMessage: '新建未绑定契约：{count}',
      count: summary?.contractsCreated ?? 0,
    },
    {
      id: 'pages.openapiSources.pipelineModal.templatesUpdated',
      defaultMessage: '更新组件模板：{count}',
      count: summary?.templatesUpdated ?? 0,
    },
    {
      id: 'pages.openapiSources.pipelineModal.proposalsCreated',
      defaultMessage: '生成页面提案：{count}',
      count: summary?.proposalsCreated ?? 0,
    },
  ];
  return (
    <Modal
      open={!!summary}
      title={intl.formatMessage({
        id: 'pages.openapiSources.pipelineModal.title',
        defaultMessage: '上传完成：契约、组件与页面提案已生成',
      })}
      footer={[
        <Button
          key="proposals"
          onClick={() => {
            onClose();
            history.push('/functions/pages');
          }}
        >
          <FormattedMessage
            id="pages.openapiSources.pipelineModal.viewProposals"
            defaultMessage="查看提案"
          />
        </Button>,
        <Button
          key="editor"
          type="primary"
          onClick={() => {
            onClose();
            history.push('/functions/pages/composite-editor');
          }}
        >
          <FormattedMessage
            id="pages.openapiSources.pipelineModal.openEditor"
            defaultMessage="打开编辑器"
          />
        </Button>,
      ]}
      onCancel={onClose}
    >
      <Space direction="vertical" size={4}>
        {counters.map((item) => (
          <Text key={item.id} style={{ fontSize: 13 }}>
            {intl.formatMessage(
              { id: item.id, defaultMessage: item.defaultMessage },
              { count: item.count },
            )}
          </Text>
        ))}
        {summary?.diagnostics?.length ? (
          <Space direction="vertical" size={4} style={{ marginTop: 8, width: '100%' }}>
            {summary.diagnostics.map((item) => (
              <Alert
                key={`${item.code}:${item.field || ''}:${item.message}`}
                type={
                  item.severity === 'error'
                    ? 'error'
                    : item.severity === 'warning'
                      ? 'warning'
                      : 'info'
                }
                showIcon
                message={`${item.code}${item.field ? ` @ ${item.field}` : ''}`}
                description={item.message}
              />
            ))}
          </Space>
        ) : null}
      </Space>
    </Modal>
  );
}
