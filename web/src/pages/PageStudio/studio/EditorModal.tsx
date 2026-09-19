import { useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Col,
  Empty,
  Modal,
  Row,
  Space,
  Switch,
  TreeSelect,
  Typography,
} from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import PageEditor from '@/components/PageEditor';
import PageRenderer from '@/components/PageRenderer';
import type { MenuItem } from '@/services/api/menu';
import type { PageSpec, PageSpecDraft } from '@/types/dashboard';
import { toMenuTreeData } from './menuTree';

const { Text } = Typography;

/** 页面编辑弹窗：左侧 PageEditor + 可开关的实时预览（预览不执行函数）；
 * 保存/发布由工作台主页回调（错误明细弹窗也在主页统一处理）。
 * footer 提供挂载菜单选择（T-M8 后分类 key 不再进菜单，控制台导航只由
 * menu_items 驱动）：改动过才随保存提交（menuId null=解除挂载）。 */
export default function EditorModal({
  open,
  pageKey,
  draft,
  livePreview,
  saving,
  menus,
  currentMenuId,
  onClose,
  onLivePreviewChange,
  onSave,
  onSpecChange,
  onSyncSelectors,
}: {
  open: boolean;
  pageKey: string;
  draft: PageSpecDraft | null;
  livePreview: boolean;
  saving: boolean;
  /** 当前 scope 菜单树（listMenus）；空时挂载选择降级为占位提示。 */
  menus: MenuItem[];
  /** 当前挂载的 menu_items.id（null/缺省=未挂载），打开时回显。 */
  currentMenuId?: number | null;
  onClose: () => void;
  onLivePreviewChange: (v: boolean) => void;
  onSave: (options?: { publishAfterSave?: boolean; menuId?: number | null }) => void;
  onSpecChange: (value: PageSpec) => void;
  onSyncSelectors: () => void;
}) {
  const intl = useIntl();
  const [menuId, setMenuId] = useState<number | null>(currentMenuId ?? null);
  // 是否改动过挂载：未改动不随保存提交（避免每次保存都调挂载 API）
  const [menuDirty, setMenuDirty] = useState(false);

  useEffect(() => {
    setMenuId(currentMenuId ?? null);
    setMenuDirty(false);
  }, [currentMenuId]);

  const menuTreeData = useMemo(() => toMenuTreeData(menus, intl.locale), [menus, intl.locale]);
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
          <Button loading={saving} onClick={() => onSave(menuDirty ? { menuId } : undefined)}>
            <FormattedMessage
              id="pages.pageStudio.studio.editor.saveDraft"
              defaultMessage="仅保存草稿"
            />
          </Button>
          <TreeSelect
            style={{ minWidth: 240 }}
            value={menuId ?? undefined}
            treeData={menuTreeData}
            treeDefaultExpandAll
            allowClear
            disabled={menus.length === 0}
            placeholder={intl.formatMessage({
              id:
                menus.length > 0
                  ? 'pages.pageStudio.studio.editor.mountPlaceholder'
                  : 'pages.pageStudio.studio.editor.mountNoMenus',
              defaultMessage:
                menus.length > 0
                  ? '挂载到菜单（可选，清空解除）'
                  : '暂无菜单，可先在「菜单管理」创建',
            })}
            onChange={(value: number | undefined) => {
              setMenuId(value ?? null);
              setMenuDirty(true);
            }}
          />
          <Button
            type="primary"
            loading={saving}
            onClick={() => onSave({ publishAfterSave: true, ...(menuDirty ? { menuId } : {}) })}
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
                  <>
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
                    <Button size="small" style={{ marginTop: 8 }} onClick={onSyncSelectors}>
                      <FormattedMessage
                        id="pages.pageStudio.studio.editor.syncSelectors"
                        defaultMessage="一键同步 Selector"
                      />
                    </Button>
                  </>
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
