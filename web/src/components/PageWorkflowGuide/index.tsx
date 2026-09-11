import { Alert, Card, Steps } from 'antd';
import { useIntl } from '@umijs/max';

export default function PageWorkflowGuide() {
  const intl = useIntl();
  return (
    <Card
      size="small"
      title={intl.formatMessage({
        id: 'component.pageWorkflowGuide.cardTitle',
        defaultMessage: '创建完整页面：只走这一条默认路径',
      })}
    >
      <Steps
        current={-1}
        responsive
        items={[
          {
            title: intl.formatMessage({
              id: 'component.pageWorkflowGuide.step.registerFunction.title',
              defaultMessage: '注册函数',
            }),
            description: intl.formatMessage({
              id: 'component.pageWorkflowGuide.step.registerFunction.description',
              defaultMessage: 'SDK / Agent 注册 FunctionContract，或导入 OpenAPI。',
            }),
          },
          {
            title: intl.formatMessage({
              id: 'component.pageWorkflowGuide.step.defaultPage.title',
              defaultMessage: '查看默认页面',
            }),
            description: intl.formatMessage({
              id: 'component.pageWorkflowGuide.step.defaultPage.description',
              defaultMessage: '平台自动生成 Proposal；无需手工创建页面。',
            }),
          },
          {
            title: intl.formatMessage({
              id: 'component.pageWorkflowGuide.step.previewPublish.title',
              defaultMessage: '预览并发布',
            }),
            description: intl.formatMessage({
              id: 'component.pageWorkflowGuide.step.previewPublish.description',
              defaultMessage: 'ready/basic 预览后直接发布到运行控制台。',
            }),
          },
          {
            title: intl.formatMessage({
              id: 'component.pageWorkflowGuide.step.consoleRun.title',
              defaultMessage: '在控制台运行',
            }),
            description: intl.formatMessage({
              id: 'component.pageWorkflowGuide.step.consoleRun.description',
              defaultMessage: '从已发布菜单进入，执行由发布快照约束。',
            }),
          },
        ]}
      />
      <Alert
        style={{ marginTop: 16 }}
        type="warning"
        showIcon
        message={intl.formatMessage({
          id: 'component.pageWorkflowGuide.notice',
          defaultMessage:
            '只有 Proposal 显示“需要处理”时，才进入资源目录补全语义；只有需要改变默认展示时，才接受为草稿进行编辑。',
        })}
      />
    </Card>
  );
}
