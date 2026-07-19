import { useParams, history } from '@umijs/max';
import { Spin, Button, Result } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import WorkspaceRenderer, { useWorkspaceConfig } from '@/components/WorkspaceRenderer';
import { trackWorkspaceEvent } from '@/services/workspace/telemetry';
import { buildWorkspaceQualityReport } from '@/services/workspace/quality';
import { useEffect } from 'react';

/**
 * 运行控制台 - 工作台页面
 * 用于渲染单个工作台配置
 */
export default function ConsoleWorkspace() {
  const params = useParams<{ objectKey: string }>();
  const objectKey = decodeURIComponent(params?.objectKey || '');

  const { config, loading, error, errorCode, reload } = useWorkspaceConfig(objectKey);
  const quality = config ? buildWorkspaceQualityReport(config) : null;

  useEffect(() => {
    if (objectKey) {
      trackWorkspaceEvent('workspace_page_open', {
        page: 'console_workspace',
        objectKey,
      });
    }
  }, [objectKey]);

  if (!objectKey) {
    return (
      <PageContainer title="运行控制台">
        <Result
          status="404"
          title="未找到工作台"
          subTitle="请从左侧菜单选择一个工作台"
          extra={
            <Button type="primary" onClick={() => history.push('/console')}>
              返回控制台
            </Button>
          }
        />
      </PageContainer>
    );
  }

  if (loading) {
    return (
      <PageContainer title="加载中...">
        <div style={{ textAlign: 'center', padding: '100px 0' }}>
          <Spin size="large" />
        </div>
      </PageContainer>
    );
  }

  if (errorCode === 'workspace_not_found') {
    return (
      <PageContainer title="工作台不存在">
        <Result
          status="404"
          title="工作台不存在"
          subTitle={`工作台 "${objectKey}" 未找到或未发布`}
          extra={
            <Button type="primary" onClick={() => history.push('/console')}>
              返回控制台
            </Button>
          }
        />
      </PageContainer>
    );
  }

  if (errorCode === 'forbidden') {
    return (
      <PageContainer title="无权限">
        <Result
          status="403"
          title="无访问权限"
          subTitle="您没有权限访问此工作台"
          extra={
            <Button type="primary" onClick={() => history.push('/console')}>
              返回控制台
            </Button>
          }
        />
      </PageContainer>
    );
  }

  if (error) {
    return (
      <PageContainer title="加载失败">
        <Result
          status="error"
          title="加载失败"
          subTitle={error}
          extra={[
            <Button key="retry" type="primary" onClick={reload}>
              重试
            </Button>,
            <Button key="back" onClick={() => history.push('/console')}>
              返回控制台
            </Button>,
          ]}
        />
      </PageContainer>
    );
  }

  return (
    <PageContainer
      title={config?.title || objectKey}
      subTitle={config?.description}
    >
      <WorkspaceRenderer
        objectKey={objectKey}
        runtimeMode="console"
      />
    </PageContainer>
  );
}
