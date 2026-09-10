import { Button, Modal, Typography } from 'antd';

const { Paragraph } = Typography;

/** 合并变更弹窗：自动合并 / 手动处理冲突二选一（策略执行在工作台主页）。 */
export default function MergeModal({
  open,
  loading,
  onCancel,
  onAutoMerge,
  onManualMerge,
}: {
  open: boolean;
  loading: boolean;
  onCancel: () => void;
  onAutoMerge: () => void;
  onManualMerge: () => void;
}) {
  return (
    <Modal
      title="合并变更"
      open={open}
      onCancel={onCancel}
      footer={[
        <Button key="cancel" onClick={onCancel}>
          取消
        </Button>,
        <Button key="auto" loading={loading} onClick={onAutoMerge}>
          自动合并
        </Button>,
        <Button key="manual" type="primary" loading={loading} onClick={onManualMerge}>
          手动处理冲突
        </Button>,
      ]}
    >
      <Paragraph>
        自动合并只会写入展示字段；binding、selector、权限、风险、审批和执行模式必须人工确认后重新发布。
      </Paragraph>
    </Modal>
  );
}
