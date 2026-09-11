import { Drawer, Empty } from 'antd';
import { useIntl } from '@umijs/max';
import PageRenderer from '@/components/PageRenderer';
import type { PageSpecDraft } from '@/types/dashboard';

/** 页面预览抽屉：PageRenderer 预览态（不执行函数——发布后到运行控制台执行）。 */
export default function PreviewDrawer({
  open,
  draft,
  onClose,
}: {
  open: boolean;
  draft: PageSpecDraft | null;
  onClose: () => void;
}) {
  const intl = useIntl();
  return (
    <Drawer
      title={intl.formatMessage({
        id: 'pages.pageStudio.studio.preview.title',
        defaultMessage: '页面预览',
      })}
      width={900}
      open={open}
      onClose={onClose}
    >
      {draft ? (
        <PageRenderer
          pageSpec={draft}
          preview
          onExecute={async () => {
            // 与 EditorModal 预览同文案，共用 editor.* 键
            throw new Error(
              intl.formatMessage({
                id: 'pages.pageStudio.studio.editor.previewExecuteError',
                defaultMessage: 'Page Studio 预览不执行函数；发布后请在运行控制台执行。',
              }),
            );
          }}
        />
      ) : (
        <Empty
          description={intl.formatMessage({
            id: 'pages.pageStudio.studio.editor.empty',
            defaultMessage: '请选择页面',
          })}
        />
      )}
    </Drawer>
  );
}
