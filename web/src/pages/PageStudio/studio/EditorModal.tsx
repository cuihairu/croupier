import { Alert, Button, Card, Col, Empty, Modal, Row, Space, Switch, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import PageEditor from '@/components/PageEditor';
import PageRenderer from '@/components/PageRenderer';
import type { PageSpec, PageSpecDraft } from '@/types/dashboard';

const { Text } = Typography;

/** 页面编辑弹窗：左侧 PageEditor + 可开关的实时预览（预览不执行函数）；
 * 保存/发布由工作台主页回调（错误明细弹窗也在主页统一处理）。 */
export default function EditorModal({
  open,
  pageKey,
  draft,
  livePreview,
  saving,
  onClose,
  onLivePreviewChange,
  onSave,
  onSpecChange,
}: {
  open: boolean;
  pageKey: string;
  draft: PageSpecDraft | null;
  livePreview: boolean;
  saving: boolean;
  onClose: () => void;
  onLivePreviewChange: (v: boolean) => void;
  onSave: (options?: { publishAfterSave?: boolean }) => void;
  onSpecChange: (value: PageSpec) => void;
}) {
  const intl = useIntl();
  return (
    <Modal
      title={
        <Space>
          <span>
            <FormattedMessage id="pages.pageStudio.studio.editor.title" defaultMessage="页面编辑" />
          </span>
          <Text type="secondary" code>
            {pageKey || '-'}
          </Text>
        </Space>
      }
      open={open}
      onCancel={onClose}
      width="100%"
      style={{ top: 16, maxWidth: 1600, paddingBottom: 0 }}
      styles={{ body: { height: 'calc(100vh - 120px)', overflow: 'hidden', paddingTop: 12 } }}
      footer={
        <Space>
          <Switch
            checkedChildren={intl.formatMessage({
              id: 'pages.pageStudio.studio.editor.previewOn',
              defaultMessage: '预览开',
            })}
            unCheckedChildren={intl.formatMessage({
              id: 'pages.pageStudio.studio.editor.previewOff',
              defaultMessage: '预览关',
            })}
            checked={livePreview}
            onChange={onLivePreviewChange}
          />
          <Button onClick={onClose}>
            <FormattedMessage id="pages.pageStudio.studio.editor.cancel" defaultMessage="取消" />
          </Button>
          <Button loading={saving} onClick={() => onSave()}>
            <FormattedMessage
              id="pages.pageStudio.studio.editor.saveDraft"
              defaultMessage="仅保存草稿"
            />
          </Button>
          <Button
            type="primary"
            loading={saving}
            onClick={() => onSave({ publishAfterSave: true })}
          >
            <FormattedMessage
              id="pages.pageStudio.studio.editor.saveAndPublish"
              defaultMessage="保存并发布"
            />
          </Button>
        </Space>
      }
    >
      {draft ? (
        <Row gutter={16} style={{ height: '100%' }}>
          <Col
            span={livePreview ? 13 : 24}
            style={{ height: '100%', overflow: 'auto', paddingRight: 4 }}
          >
            {draft.bindingFreshness && draft.bindingFreshness.length > 0 ? (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message={intl.formatMessage({
                  id: 'pages.pageStudio.studio.editor.bindingStaleTitle',
                  defaultMessage: '页面绑定与函数契约不一致（发布会校验失败）',
                })}
                description={
                  <ul style={{ margin: 0, paddingLeft: 18 }}>
                    {draft.bindingFreshness.slice(0, 8).map((item, i) => (
                      <li key={i}>
                        <code>{item.functionId || item.bindingId || '-'}</code>
                        {item.diagnostic?.message ? `：${item.diagnostic.message}` : ''}
                      </li>
                    ))}
                    {draft.bindingFreshness.length > 8 ? (
                      <li>
                        {intl.formatMessage(
                          {
                            id: 'pages.pageStudio.studio.editor.bindingStaleMore',
                            defaultMessage: '…以及另外 {count} 条',
                          },
                          { count: draft.bindingFreshness.length - 8 },
                        )}
                      </li>
                    ) : null}
                  </ul>
                }
              />
            ) : null}
            <PageEditor value={draft} onChange={onSpecChange} />
          </Col>
          {livePreview ? (
            <Col span={11} style={{ height: '100%', overflow: 'auto' }}>
              <Card
                size="small"
                title={intl.formatMessage({
                  id: 'pages.pageStudio.studio.editor.livePreview',
                  defaultMessage: '实时预览',
                })}
                extra={
                  <Text type="secondary">
                    <FormattedMessage
                      id="pages.pageStudio.studio.editor.previewHint"
                      defaultMessage="预览不执行函数；发布后请在运行控制台执行"
                    />
                  </Text>
                }
              >
                <PageRenderer
                  pageSpec={draft}
                  preview
                  onExecute={async () => {
                    throw new Error(
                      intl.formatMessage({
                        id: 'pages.pageStudio.studio.editor.previewExecuteError',
                        defaultMessage: 'Page Studio 预览不执行函数；发布后请在运行控制台执行。',
                      }),
                    );
                  }}
                />
              </Card>
            </Col>
          ) : null}
        </Row>
      ) : (
        <Empty
          description={intl.formatMessage({
            id: 'pages.pageStudio.studio.editor.empty',
            defaultMessage: '请选择页面',
          })}
        />
      )}
    </Modal>
  );
}
