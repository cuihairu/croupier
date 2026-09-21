import React, { useEffect, useState } from 'react';
import { Alert, Input, Modal, Typography } from 'antd';
import { useIntl } from '@umijs/max';

const { Text } = Typography;

type BatchFloorModalProps = {
  open: boolean;
  /** 已勾选函数数（弹窗文案与确认语义依赖它） */
  count: number;
  submitting: boolean;
  onSubmit: (minVersion: string) => void;
  onClose: () => void;
};

// 批量设置版本门槛弹窗：统一值语义——输入一个 minVersion 应用到全部
// 勾选函数。空值禁止提交（批量清除走独立按钮 + Popconfirm，不在弹窗里）。
// 独立成文件便于组件级测试（整页 index.tsx 渲染链过重）。
export default function BatchFloorModal({
  open,
  count,
  submitting,
  onSubmit,
  onClose,
}: BatchFloorModalProps) {
  const intl = useIntl();
  const [versionInput, setVersionInput] = useState('');

  // 每次打开重置输入（避免上次的值残留导致误提交）
  useEffect(() => {
    if (open) setVersionInput('');
  }, [open]);

  const handleOk = () => {
    const minVersion = versionInput.trim();
    if (!minVersion || submitting) return;
    onSubmit(minVersion);
  };

  return (
    <Modal
      title={intl.formatMessage({
        id: 'pages.functionsDirectory.batch.modalTitle',
        defaultMessage: '批量设置版本门槛',
      })}
      open={open}
      onOk={handleOk}
      onCancel={onClose}
      confirmLoading={submitting}
      okButtonProps={{ disabled: !versionInput.trim() }}
      destroyOnHidden
    >
      <Alert
        type="info"
        showIcon
        message={intl.formatMessage(
          {
            id: 'pages.functionsDirectory.batch.modalDescription',
            defaultMessage: '将把已选 {count} 个函数的最低可注册函数版本统一设为输入值。',
          },
          { count },
        )}
        style={{ marginBottom: 16 }}
      />
      <Text strong>
        {intl.formatMessage({
          id: 'pages.functionsDirectory.batch.versionLabel',
          defaultMessage: '最低函数版本',
        })}
      </Text>
      <Input
        style={{ marginTop: 8 }}
        placeholder={intl.formatMessage({
          id: 'pages.functionsDirectory.batch.versionPlaceholder',
          defaultMessage: '如 0.3.0',
        })}
        value={versionInput}
        onChange={(e) => setVersionInput(e.target.value)}
        onPressEnter={handleOk}
      />
    </Modal>
  );
}
