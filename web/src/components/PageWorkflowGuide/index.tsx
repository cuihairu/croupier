import { Alert, Card, Steps } from 'antd';

export default function PageWorkflowGuide() {
  return (
    <Card size="small" title="创建完整页面：只走这一条默认路径">
      <Steps
        current={-1}
        responsive
        items={[
          {
            title: '注册函数',
            description: 'SDK / Agent 注册 FunctionContract，或导入 OpenAPI。',
          },
          {
            title: '查看默认页面',
            description: '平台自动生成 Proposal；无需手工创建页面。',
          },
          {
            title: '预览并发布',
            description: 'ready/basic 预览后直接发布到运行控制台。',
          },
          {
            title: '在控制台运行',
            description: '从已发布菜单进入，执行由发布快照约束。',
          },
        ]}
      />
      <Alert
        style={{ marginTop: 16 }}
        type="warning"
        showIcon
        message="只有 Proposal 显示“需要处理”时，才进入资源目录补全语义；只有需要改变默认展示时，才接受为草稿进行编辑。"
      />
    </Card>
  );
}
