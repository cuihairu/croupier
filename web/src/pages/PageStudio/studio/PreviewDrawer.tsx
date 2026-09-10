import { Drawer, Empty } from 'antd';
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
  return (
    <Drawer title="页面预览" width={900} open={open} onClose={onClose}>
      {draft ? (
        <PageRenderer
          pageSpec={draft}
          preview
          onExecute={async () => {
            throw new Error('Page Studio 预览不执行函数；发布后请在运行控制台执行。');
          }}
        />
      ) : (
        <Empty description="请选择页面" />
      )}
    </Drawer>
  );
}
